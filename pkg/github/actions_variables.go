package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	variablesMethodList     = "list"
	variablesMethodGet      = "get"
	variableWriteMethodSet  = "create_or_update"
	variableWriteMethodDel  = "delete"
	actionsScopeRepository  = "repository"
	actionsScopeEnvironment = "environment"
	actionsScopeOrg         = "organization"
)

// MinimalActionsVariable is the trimmed output type for an Actions variable.
// Variables are not secret, so the value is included.
type MinimalActionsVariable struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	Scope      string `json:"scope"`
	Visibility string `json:"visibility,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

func convertToMinimalActionsVariable(variable *github.ActionsVariable, scope string) MinimalActionsVariable {
	m := MinimalActionsVariable{
		Name:       variable.Name,
		Value:      sanitize.Sanitize(variable.Value),
		Scope:      scope,
		Visibility: variable.GetVisibility(),
	}
	if variable.CreatedAt != nil {
		m.CreatedAt = variable.CreatedAt.Format(time.RFC3339)
	}
	if variable.UpdatedAt != nil {
		m.UpdatedAt = variable.UpdatedAt.Format(time.RFC3339)
	}
	return m
}

// actionsScopeSchema is the scope selector shared by the variable and secret
// tools, so both address repository, environment and organization storage the
// same way.
func actionsScopeSchema(includeOrg bool) *jsonschema.Schema {
	values := []any{actionsScopeRepository, actionsScopeEnvironment}
	description := "Where the value lives: on the repository, or on one of its environments. Defaults to 'repository'."
	if includeOrg {
		values = append(values, actionsScopeOrg)
		description = "Where the value lives: on the repository, on one of its environments, or on the organization. Defaults to 'repository'."
	}
	return &jsonschema.Schema{
		Type:        "string",
		Description: description,
		Enum:        values,
	}
}

// resolveActionsScope reads and validates the scope selector, returning the
// scope along with the environment or organization it names.
func resolveActionsScope(args map[string]any, allowOrg bool) (scope, environment, org string, errResult *mcp.CallToolResult) {
	raw, err := OptionalParam[string](args, "scope")
	if err != nil {
		return "", "", "", utils.NewToolResultError(err.Error())
	}
	scope = strings.ToLower(raw)
	if scope == "" {
		scope = actionsScopeRepository
	}

	switch scope {
	case actionsScopeRepository:
		return scope, "", "", nil
	case actionsScopeEnvironment:
		environment, err = RequiredParam[string](args, "environment_name")
		if err != nil {
			return "", "", "", utils.NewToolResultError("environment_name is required when scope is 'environment'")
		}
		return scope, environment, "", nil
	case actionsScopeOrg:
		if !allowOrg {
			return "", "", "", utils.NewToolResultError("scope 'organization' is not supported by this tool")
		}
		org, err = OptionalParam[string](args, "org")
		if err != nil {
			return "", "", "", utils.NewToolResultError(err.Error())
		}
		if org == "" {
			org, err = RequiredParam[string](args, "owner")
			if err != nil {
				return "", "", "", utils.NewToolResultError("org or owner is required when scope is 'organization'")
			}
		}
		return scope, "", org, nil
	default:
		return "", "", "", utils.NewToolResultError(fmt.Sprintf("unknown scope: %s", scope))
	}
}

// ActionsVariablesRead creates a tool to read Actions variables.
func ActionsVariablesRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name:        "actions_variables_read",
			Description: t("TOOL_ACTIONS_VARIABLES_READ_DESCRIPTION", "Read GitHub Actions variables for a repository, one of its environments, or the organization. Variables are plain configuration, not secrets, so their values are returned."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ACTIONS_VARIABLES_READ_USER_TITLE", "Read Actions variables"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The method to execute: 'list' for every variable in scope, 'get' for one by name",
						Enum:        []any{variablesMethodList, variablesMethodGet},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name. Not needed when scope is 'organization'.",
					},
					"scope": actionsScopeSchema(true),
					"environment_name": {
						Type:        "string",
						Description: "Environment name. Required when scope is 'environment'.",
					},
					"org": {
						Type:        "string",
						Description: "Organization login. Defaults to owner when scope is 'organization'.",
					},
					"name": {
						Type:        "string",
						Description: "Variable name. Required for 'get'.",
					},
				},
				Required: []string{"method", "owner"},
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
			scope, environment, org, errResult := resolveActionsScope(args, true)
			if errResult != nil {
				return errResult, nil, nil
			}

			repo := ""
			if scope != actionsScopeOrg {
				repo, err = RequiredParam[string](args, "repo")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
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

			label := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				if scope == actionsScopeOrg {
					return r
				}
				// Variable values are configuration written by maintainers;
				// confidentiality follows repo visibility.
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelRepoMetadata)
			}

			switch method {
			case variablesMethodList:
				var (
					variables *github.ActionsVariables
					resp      *github.Response
				)
				switch scope {
				case actionsScopeRepository:
					variables, resp, err = client.Actions.ListRepoVariables(ctx, owner, repo, listOpts)
				case actionsScopeEnvironment:
					variables, resp, err = client.Actions.ListEnvVariables(ctx, owner, repo, environment, listOpts)
				case actionsScopeOrg:
					variables, resp, err = client.Actions.ListOrgVariables(ctx, org, listOpts)
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list Actions variables", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalActionsVariable, 0, len(variables.Variables))
				for _, variable := range variables.Variables {
					if variable != nil {
						minimal = append(minimal, convertToMinimalActionsVariable(variable, scope))
					}
				}

				return marshalDeploymentResult(minimal, label)

			case variablesMethodGet:
				name, err := RequiredParam[string](args, "name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				var (
					variable *github.ActionsVariable
					resp     *github.Response
				)
				switch scope {
				case actionsScopeRepository:
					variable, resp, err = client.Actions.GetRepoVariable(ctx, owner, repo, name)
				case actionsScopeEnvironment:
					variable, resp, err = client.Actions.GetEnvVariable(ctx, owner, repo, environment, name)
				case actionsScopeOrg:
					variable, resp, err = client.Actions.GetOrgVariable(ctx, org, name)
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get Actions variable '%s'", name), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(convertToMinimalActionsVariable(variable, scope), label)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list, get", method)), nil, nil
			}
		},
	)
}

// ActionsVariableWrite creates a tool to set and remove Actions variables.
func ActionsVariableWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name: "actions_variable_write",
			Description: t("TOOL_ACTIONS_VARIABLE_WRITE_DESCRIPTION", "Set or remove a GitHub Actions variable on a repository, one of its environments, or the organization. "+
				"Creating and updating share one method: a variable that does not exist is created. Use 'actions_secret_write' for values that must stay secret."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ACTIONS_VARIABLE_WRITE_USER_TITLE", "Write Actions variables"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create_or_update' or 'delete'",
						Enum:        []any{variableWriteMethodSet, variableWriteMethodDel},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name. Not needed when scope is 'organization'.",
					},
					"scope": actionsScopeSchema(false),
					"environment_name": {
						Type:        "string",
						Description: "Environment name. Required when scope is 'environment'.",
					},
					"name": {
						Type:        "string",
						Description: "Variable name.",
					},
					"value": {
						Type:        "string",
						Description: "Variable value. Required for 'create_or_update'.",
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
			case variableWriteMethodSet:
				value, err := RequiredParam[string](args, "value")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				// There is no upsert endpoint: create reports a conflict when
				// the variable already exists, and update reports a 404 when it
				// does not, so one falls back to the other.
				var resp *github.Response
				if scope == actionsScopeEnvironment {
					resp, err = client.Actions.CreateEnvVariable(ctx, owner, repo, environment, github.ActionsCreateVariableRequest{Name: name, Value: value})
				} else {
					resp, err = client.Actions.CreateRepoVariable(ctx, owner, repo, github.ActionsCreateVariableRequest{Name: name, Value: value})
				}
				if resp != nil {
					_ = resp.Body.Close()
				}

				if err != nil {
					update := github.ActionsUpdateVariableRequest{Name: github.Ptr(name), Value: github.Ptr(value)}
					var updateResp *github.Response
					var updateErr error
					if scope == actionsScopeEnvironment {
						updateResp, updateErr = client.Actions.UpdateEnvVariable(ctx, owner, repo, environment, name, update)
					} else {
						updateResp, updateErr = client.Actions.UpdateRepoVariable(ctx, owner, repo, name, update)
					}
					if updateResp != nil {
						_ = updateResp.Body.Close()
					}
					if updateErr != nil {
						return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to set Actions variable '%s'", name), updateResp, updateErr), nil, nil
					}
				}

				return marshalDeploymentResult(map[string]any{
					"result": "variable_set",
					"name":   name,
					"scope":  scope,
				}, nil)

			case variableWriteMethodDel:
				var resp *github.Response
				if scope == actionsScopeEnvironment {
					resp, err = client.Actions.DeleteEnvVariable(ctx, owner, repo, environment, name)
				} else {
					resp, err = client.Actions.DeleteRepoVariable(ctx, owner, repo, name)
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to delete Actions variable '%s'", name), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(map[string]any{
					"result": "variable_deleted",
					"name":   name,
					"scope":  scope,
				}, nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create_or_update, delete", method)), nil, nil
			}
		},
	)
}
