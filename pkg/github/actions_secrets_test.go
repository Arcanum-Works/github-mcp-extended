package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	mcplog "github.com/github/github-mcp-server/pkg/log"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/nacl/box"
)

const (
	getReposActionsSecretsByOwnerByRepo            = "GET /repos/{owner}/{repo}/actions/secrets"
	getReposActionsSecretsByOwnerByRepoByName      = "GET /repos/{owner}/{repo}/actions/secrets/{secret_name}"
	putReposActionsSecretsByOwnerByRepoByName      = "PUT /repos/{owner}/{repo}/actions/secrets/{secret_name}"
	deleteReposActionsSecretsByOwnerByRepoByName   = "DELETE /repos/{owner}/{repo}/actions/secrets/{secret_name}"
	getReposActionsSecretsPublicKeyByOwnerByRepo   = "GET /repos/{owner}/{repo}/actions/secrets/public-key"
	putReposEnvSecretsByOwnerByRepoByEnvByName     = "PUT /repos/{owner}/{repo}/environments/{environment_name}/secrets/{secret_name}"
	getReposEnvSecretsPublicKeyByOwnerByRepoByEnv  = "GET /repos/{owner}/{repo}/environments/{environment_name}/secrets/public-key"
	getReposEnvSecretsByOwnerByRepoByEnvironmentNm = "GET /repos/{owner}/{repo}/environments/{environment_name}/secrets"
)

// testSecretKeypair generates a throwaway keypair so the tests can prove a
// stored secret really is the sealed value, without any real credential ever
// being involved.
func testSecretKeypair(t *testing.T) (publicKeyB64 string, publicKey, privateKey *[32]byte) {
	t.Helper()
	pub, priv, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(pub[:]), pub, priv
}

func Test_encryptSecretValue(t *testing.T) {
	t.Parallel()

	publicKeyB64, publicKey, privateKey := testSecretKeypair(t)

	encrypted, err := encryptSecretValue(publicKeyB64, "not-a-real-token")
	require.NoError(t, err)

	// The ciphertext must not carry the plaintext in any recoverable form.
	assert.NotContains(t, encrypted, "not-a-real-token")

	raw, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)

	opened, ok := box.OpenAnonymous(nil, raw, publicKey, privateKey)
	require.True(t, ok, "the sealed box should open with the matching private key")
	assert.Equal(t, "not-a-real-token", string(opened))

	// Sealed boxes use a fresh ephemeral key each time, so the same input
	// encrypts differently on every call.
	again, err := encryptSecretValue(publicKeyB64, "not-a-real-token")
	require.NoError(t, err)
	assert.NotEqual(t, encrypted, again)
}

func Test_encryptSecretValue_rejectsBadKeys(t *testing.T) {
	t.Parallel()

	_, err := encryptSecretValue("not base64!", "not-a-real-token")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "not-a-real-token", "an error must never quote the secret value")

	_, err = encryptSecretValue(base64.StdEncoding.EncodeToString([]byte("too short")), "not-a-real-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 32")
	assert.NotContains(t, err.Error(), "not-a-real-token")
}

func Test_ActionsSecretsRead(t *testing.T) {
	t.Parallel()

	serverTool := ActionsSecretsRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_secrets_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "actions_secrets_read tool should be read-only")

	t.Run("listing returns metadata and no values", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsSecretsByOwnerByRepo, `{"total_count":1,"secrets":[
				{"name":"DEPLOY_TOKEN","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-02-01T00:00:00Z"}
			]}`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		text := getTextResult(t, result).Text
		// The output shape has no place to put a value even if one existed.
		assert.NotContains(t, strings.ToLower(text), "\"value\"")

		var secrets []MinimalActionsSecret
		require.NoError(t, json.Unmarshal([]byte(text), &secrets))
		require.Len(t, secrets, 1)
		assert.Equal(t, "DEPLOY_TOKEN", secrets[0].Name)
		assert.Equal(t, "repository", secrets[0].Scope)
	})

	t.Run("environment scope needs an environment name", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list",
			"owner":  "owner",
			"repo":   "repo",
			"scope":  "environment",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "environment_name is required")
	})

	t.Run("get one secret's metadata", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsSecretsByOwnerByRepoByName,
				`{"name":"DEPLOY_TOKEN","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-02-01T00:00:00Z"}`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "get",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_TOKEN",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var secret MinimalActionsSecret
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &secret))
		assert.Equal(t, "DEPLOY_TOKEN", secret.Name)
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposActionsSecretsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Must have admin rights"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to list Actions secrets")
	})
}

func Test_ActionsSecretWrite(t *testing.T) {
	t.Parallel()

	serverTool := ActionsSecretWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_secret_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "actions_secret_write tool should not be read-only")

	t.Run("the stored value is sealed and never echoed back", func(t *testing.T) {
		publicKeyB64, publicKey, privateKey := testSecretKeypair(t)
		const secretValue = "fixture-value-not-a-real-credential"

		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsSecretsPublicKeyByOwnerByRepo,
				`{"key_id":"568250167242549743","key":"`+publicKeyB64+`"}`),
			WithRequestMatchHandler(putReposActionsSecretsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					assert.Equal(t, "568250167242549743", payload["key_id"])

					encrypted, ok := payload["encrypted_value"].(string)
					require.True(t, ok)
					// What crosses the wire must be the ciphertext, never the
					// plaintext.
					assert.NotContains(t, encrypted, secretValue)

					raw, err := base64.StdEncoding.DecodeString(encrypted)
					require.NoError(t, err)
					opened, ok := box.OpenAnonymous(nil, raw, publicKey, privateKey)
					require.True(t, ok, "GitHub must be able to open the sealed box")
					assert.Equal(t, secretValue, string(opened))

					w.WriteHeader(http.StatusCreated)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create_or_update",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_TOKEN",
			"value":  secretValue,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		// The result names the secret and nothing else.
		text := getTextResult(t, result).Text
		assert.NotContains(t, text, secretValue)
		assert.Contains(t, text, "secret_stored")
		assert.Contains(t, text, "DEPLOY_TOKEN")
	})

	t.Run("an environment secret uses the environment's own key", func(t *testing.T) {
		publicKeyB64, _, _ := testSecretKeypair(t)
		usedEnvironmentKey := false

		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposEnvSecretsPublicKeyByOwnerByRepoByEnv,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					usedEnvironmentKey = true
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"key_id":"1","key":"` + publicKeyB64 + `"}`))
				}),
			),
			WithRequestMatchHandler(putReposEnvSecretsByOwnerByRepoByEnvByName,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Contains(t, r.URL.Path, "/environments/staging/secrets/DEPLOY_TOKEN")
					w.WriteHeader(http.StatusCreated)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":           "create_or_update",
			"owner":            "owner",
			"repo":             "repo",
			"scope":            "environment",
			"environment_name": "staging",
			"name":             "DEPLOY_TOKEN",
			"value":            "fixture-value",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.True(t, usedEnvironmentKey)
	})

	t.Run("an empty value is refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create_or_update",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_TOKEN",
			"value":  "",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "missing required parameter: value")
	})

	t.Run("a failed write does not quote the value", func(t *testing.T) {
		publicKeyB64, _, _ := testSecretKeypair(t)
		const secretValue = "fixture-value-not-a-real-credential"

		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsSecretsPublicKeyByOwnerByRepo,
				`{"key_id":"1","key":"`+publicKeyB64+`"}`),
			WithRequestMatchHandler(putReposActionsSecretsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Must have admin rights"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create_or_update",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_TOKEN",
			"value":  secretValue,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		text := getErrorResult(t, result).Text
		assert.Contains(t, text, "failed to store Actions secret")
		assert.NotContains(t, text, secretValue)
	})

	t.Run("delete a secret", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteReposActionsSecretsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "delete",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_TOKEN",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "secret_deleted")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "reveal",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_TOKEN",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method: reveal")
	})
}

// Test_ActionsSecretToolsExposeNoValueField guards the property that matters
// most here: no secret tool's schema or snapshot offers a way to read a value
// back out.
func Test_ActionsSecretToolsExposeNoValueField(t *testing.T) {
	t.Parallel()

	readTool := ActionsSecretsRead(translations.NullTranslationHelper).Tool
	readJSON, err := json.Marshal(readTool)
	require.NoError(t, err)
	assert.NotContains(t, string(readJSON), `"value"`, "the read tool must not accept or describe a value")

	var metadata MinimalActionsSecret
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(t, err)
	assert.NotContains(t, string(metadataJSON), "value")
}

// Test_ActionsSecretWriteDeclaresValueSensitive closes the loop between the
// tool and the command logger: pkg/log redacts what the tool layer declares,
// so the declaration itself has to be guarded here. Without it the plaintext
// value would be written to the log file whenever --enable-command-logging is
// on.
func Test_ActionsSecretWriteDeclaresValueSensitive(t *testing.T) {
	t.Parallel()

	assert.True(t, mcplog.IsSensitiveToolParam("actions_secret_write", "value"),
		"actions_secret_write must declare its value parameter sensitive so the command logger redacts it")
}
