package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/crypto/nacl/box"
)

const (
	secretsMethodList    = "list"
	secretsMethodGet     = "get"
	secretWriteMethodSet = "create_or_update"
	secretWriteMethodDel = "delete"
)

// MinimalActionsSecret is the metadata GitHub keeps about a secret. There is no
// value field, and there is no API that would return one: GitHub stores secrets
// write-only, and this tool does not work around that.
type MinimalActionsSecret struct {
	Name       string `json:"name"`
	Scope      string `json:"scope"`
	Visibility string `json:"visibility,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

func convertToMinimalActionsSecret(secret *github.Secret, scope string) MinimalActionsSecret {
	return MinimalActionsSecret{
		Name:       secret.Name,
		Scope:      scope,
		Visibility: secret.Visibility,
		CreatedAt:  secret.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  secret.UpdatedAt.Format(time.RFC3339),
	}
}

// ActionsSecretsRead creates a tool to read Actions secret metadata.
func ActionsSecretsRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name: "actions_secrets_read",
			Description: t("TOOL_ACTIONS_SECRETS_READ_DESCRIPTION", "List the GitHub Actions secrets configured on a repository or one of its environments, and when each was last updated. "+
				"Only names and timestamps are returned: GitHub stores secret values write-only and no API can read one back. To see a value, look at what the workflow does with it."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ACTIONS_SECRETS_READ_USER_TITLE", "Read Actions secret metadata"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The method to execute: 'list' for every secret in scope, 'get' for one secret's metadata by name",
						Enum:        []any{secretsMethodList, secretsMethodGet},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"scope": actionsScopeSchema(false),
					"environment_name": {
						Type:        "string",
						Description: "Environment name. Required when scope is 'environment'.",
					},
					"name": {
						Type:        "string",
						Description: "Secret name. Required for 'get'.",
					},
				},
				Required: []string{"method", "owner", "repo"},
			}),
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			scope, environment, _, errResult := resolveActionsScope(args, false)
			if errResult != nil {
				return errResult, nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			listOpts := &github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case secretsMethodList:
				var (
					secrets *github.Secrets
					resp    *github.Response
				)
				if scope == actionsScopeEnvironment {
					secrets, resp, err = client.Actions.ListEnvSecrets(ctx, owner, repo, environment, listOpts)
				} else {
					secrets, resp, err = client.Actions.ListRepoSecrets(ctx, owner, repo, listOpts)
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list Actions secrets", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalActionsSecret, 0, len(secrets.Secrets))
				for _, secret := range secrets.Secrets {
					if secret != nil {
						minimal = append(minimal, convertToMinimalActionsSecret(secret, scope))
					}
				}

				return marshalDeploymentResult(minimal, nil)

			case secretsMethodGet:
				name, err := RequiredParam[string](args, "name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				var (
					secret *github.Secret
					resp   *github.Response
				)
				if scope == actionsScopeEnvironment {
					secret, resp, err = client.Actions.GetEnvSecret(ctx, owner, repo, environment, name)
				} else {
					secret, resp, err = client.Actions.GetRepoSecret(ctx, owner, repo, name)
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get Actions secret '%s'", name), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(convertToMinimalActionsSecret(secret, scope), nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list, get", method)), nil, nil
			}
		},
	)
}

// ActionsSecretWrite creates a tool to store and remove Actions secrets.
func ActionsSecretWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name: "actions_secret_write",
			Description: t("TOOL_ACTIONS_SECRET_WRITE_DESCRIPTION", "Store or remove a GitHub Actions secret on a repository or one of its environments. "+
				"The value is encrypted to the repository's public key before it leaves this server, and is never echoed back: the result reports only the secret's name. "+
				"Creating and updating share one method, so writing an existing name replaces its value."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ACTIONS_SECRET_WRITE_USER_TITLE", "Write Actions secrets"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create_or_update' or 'delete'",
						Enum:        []any{secretWriteMethodSet, secretWriteMethodDel},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"scope": actionsScopeSchema(false),
					"environment_name": {
						Type:        "string",
						Description: "Environment name. Required when scope is 'environment'.",
					},
					"name": {
						Type:        "string",
						Description: "Secret name, e.g. 'DEPLOY_TOKEN'.",
					},
					"value": {
						Type:        "string",
						Description: "The secret value. Required for 'create_or_update'. It is encrypted before transmission and never returned by any tool.",
					},
				},
				Required: []string{"method", "owner", "repo", "name"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			name, err := RequiredParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			scope, environment, _, errResult := resolveActionsScope(args, false)
			if errResult != nil {
				return errResult, nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case secretWriteMethodSet:
				// RequiredParam rejects an empty string as missing, which is
				// the behaviour wanted here: an empty secret is a mistake.
				value, err := RequiredParam[string](args, "value")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				var (
					publicKey *github.PublicKey
					resp      *github.Response
				)
				if scope == actionsScopeEnvironment {
					publicKey, resp, err = client.Actions.GetEnvPublicKey(ctx, owner, repo, environment)
				} else {
					publicKey, resp, err = client.Actions.GetRepoPublicKey(ctx, owner, repo)
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get the public key needed to encrypt the secret", resp, err), nil, nil
				}
				_ = resp.Body.Close()

				encrypted, err := encryptSecretValue(publicKey.GetKey(), value)
				if err != nil {
					// The error is built from the key, never from the value.
					return utils.NewToolResultError(fmt.Sprintf("failed to encrypt the secret value: %s", err)), nil, nil
				}

				request := github.SecretRequest{
					KeyID:          publicKey.GetKeyID(),
					EncryptedValue: encrypted,
				}
				if scope == actionsScopeEnvironment {
					resp, err = client.Actions.CreateOrUpdateEnvSecret(ctx, owner, repo, environment, name, request)
				} else {
					resp, err = client.Actions.CreateOrUpdateRepoSecret(ctx, owner, repo, name, request)
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to store Actions secret '%s'", name), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(map[string]any{
					"result": "secret_stored",
					"name":   name,
					"scope":  scope,
				}, nil)

			case secretWriteMethodDel:
				var resp *github.Response
				if scope == actionsScopeEnvironment {
					resp, err = client.Actions.DeleteEnvSecret(ctx, owner, repo, environment, name)
				} else {
					resp, err = client.Actions.DeleteRepoSecret(ctx, owner, repo, name)
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to delete Actions secret '%s'", name), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(map[string]any{
					"result": "secret_deleted",
					"name":   name,
					"scope":  scope,
				}, nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create_or_update, delete", method)), nil, nil
			}
		},
	)
}

// encryptSecretValue seals a secret value to GitHub's public key using the
// libsodium sealed box construction the Actions secrets API requires. The
// returned string is the base64 ciphertext; the plaintext never leaves this
// function, and no error it returns mentions the value.
func encryptSecretValue(encodedPublicKey, value string) (string, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(encodedPublicKey)
	if err != nil {
		return "", fmt.Errorf("the public key returned by GitHub is not valid base64")
	}
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("the public key returned by GitHub is %d bytes, expected 32", len(keyBytes))
	}

	var publicKey [32]byte
	copy(publicKey[:], keyBytes)

	sealed, err := box.SealAnonymous(nil, []byte(value), &publicKey, rand.Reader)
	if err != nil {
		return "", fmt.Errorf("sealed box encryption failed")
	}

	return base64.StdEncoding.EncodeToString(sealed), nil
}
