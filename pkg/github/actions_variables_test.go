package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	getReposActionsVariablesByOwnerByRepo         = "GET /repos/{owner}/{repo}/actions/variables"
	postReposActionsVariablesByOwnerByRepo        = "POST /repos/{owner}/{repo}/actions/variables"
	getReposActionsVariablesByOwnerByRepoByName   = "GET /repos/{owner}/{repo}/actions/variables/{name}"
	patchReposActionsVariablesByOwnerByRepoByName = "PATCH /repos/{owner}/{repo}/actions/variables/{name}"
	delReposActionsVariablesByOwnerByRepoByName   = "DELETE /repos/{owner}/{repo}/actions/variables/{name}"
	getReposEnvVariablesByOwnerByRepoByEnv        = "GET /repos/{owner}/{repo}/environments/{environment_name}/variables"
	postReposEnvVariablesByOwnerByRepoByEnv       = "POST /repos/{owner}/{repo}/environments/{environment_name}/variables"
	getOrgsActionsVariablesByOrg                  = "GET /orgs/{org}/actions/variables"
)

func Test_ActionsVariablesRead(t *testing.T) {
	t.Parallel()

	serverTool := ActionsVariablesRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_variables_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "actions_variables_read tool should be read-only")

	t.Run("list repository variables", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsVariablesByOwnerByRepo, `{"total_count":1,"variables":[
				{"name":"DEPLOY_REGION","value":"eu-west-1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-02-01T00:00:00Z"}
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

		var variables []MinimalActionsVariable
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &variables))
		require.Len(t, variables, 1)
		assert.Equal(t, "DEPLOY_REGION", variables[0].Name)
		// Variables are not secret, so the value is part of the answer.
		assert.Equal(t, "eu-west-1", variables[0].Value)
		assert.Equal(t, "repository", variables[0].Scope)
	})

	t.Run("list environment variables", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposEnvVariablesByOwnerByRepoByEnv,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Contains(t, r.URL.Path, "/environments/staging/variables")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"total_count":1,"variables":[{"name":"API_URL","value":"https://staging.example.com"}]}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":           "list",
			"owner":            "owner",
			"repo":             "repo",
			"scope":            "environment",
			"environment_name": "staging",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var variables []MinimalActionsVariable
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &variables))
		require.Len(t, variables, 1)
		assert.Equal(t, "environment", variables[0].Scope)
	})

	t.Run("organization scope defaults to the owner and needs no repo", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getOrgsActionsVariablesByOrg,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/orgs/owner/actions/variables", r.URL.Path)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"total_count":1,"variables":[{"name":"ORG_WIDE","value":"yes","visibility":"all"}]}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list",
			"owner":  "owner",
			"scope":  "organization",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var variables []MinimalActionsVariable
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &variables))
		require.Len(t, variables, 1)
		assert.Equal(t, "all", variables[0].Visibility)
	})

	t.Run("get one variable", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsVariablesByOwnerByRepoByName, `{"name":"DEPLOY_REGION","value":"eu-west-1"}`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "get",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_REGION",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var variable MinimalActionsVariable
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &variable))
		assert.Equal(t, "eu-west-1", variable.Value)
	})

	t.Run("an unknown scope is refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list",
			"owner":  "owner",
			"repo":   "repo",
			"scope":  "enterprise",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown scope: enterprise")
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposActionsVariablesByOwnerByRepo,
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
		assert.Contains(t, getErrorResult(t, result).Text, "failed to list Actions variables")
	})
}

func Test_ActionsVariableWrite(t *testing.T) {
	t.Parallel()

	serverTool := ActionsVariableWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_variable_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "actions_variable_write tool should not be read-only")

	t.Run("a new variable is created", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposActionsVariablesByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "DEPLOY_REGION", payload["name"])
					assert.Equal(t, "eu-west-1", payload["value"])

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
			"name":   "DEPLOY_REGION",
			"value":  "eu-west-1",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "variable_set")
	})

	t.Run("an existing variable falls back to an update", func(t *testing.T) {
		updated := false
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposActionsVariablesByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					// GitHub has no upsert: creating an existing name conflicts.
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"message": "Variable already exists"}`))
				}),
			),
			WithRequestMatchHandler(patchReposActionsVariablesByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					updated = true
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "us-east-1", payload["value"])

					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create_or_update",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_REGION",
			"value":  "us-east-1",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.True(t, updated, "the conflicting create should be followed by an update")
	})

	t.Run("an environment variable is written to the environment", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposEnvVariablesByOwnerByRepoByEnv,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Contains(t, r.URL.Path, "/environments/staging/variables")
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
			"name":             "API_URL",
			"value":            "https://staging.example.com",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("create_or_update without a value is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create_or_update",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_REGION",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "value")
	})

	t.Run("organization scope is not offered by the write tool", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "delete",
			"owner":  "owner",
			"repo":   "repo",
			"scope":  "organization",
			"name":   "ORG_WIDE",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "not supported by this tool")
	})

	t.Run("delete a variable", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(delReposActionsVariablesByOwnerByRepoByName,
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
			"name":   "DEPLOY_REGION",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "variable_deleted")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "rename",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "DEPLOY_REGION",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method: rename")
	})
}
