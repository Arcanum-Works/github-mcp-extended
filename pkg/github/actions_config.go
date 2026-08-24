package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	actionsConfigMethodPermissions        = "get_permissions"
	actionsConfigMethodSetPermissions     = "set_permissions"
	actionsConfigMethodSetAllowedActions  = "set_allowed_actions"
	actionsConfigMethodSetWorkflowPerms   = "set_workflow_permissions"
	actionsConfigMethodEnableWorkflow     = "enable_workflow"
	actionsConfigMethodDisableWorkflow    = "disable_workflow"
	actionsAllowedActionsSelectedSentinel = "selected"
)

// MinimalActionsConfig is the trimmed view of a repository's Actions
// configuration: whether Actions runs at all, which actions it may use, and
// what the default GITHUB_TOKEN can do.
type MinimalActionsConfig struct {
	Enabled                      bool     `json:"enabled"`
	AllowedActions               string   `json:"allowed_actions,omitempty"`
	SHAPinningRequired           bool     `json:"sha_pinning_required,omitempty"`
	GitHubOwnedAllowed           bool     `json:"github_owned_allowed,omitempty"`
	VerifiedAllowed              bool     `json:"verified_allowed,omitempty"`
	PatternsAllowed              []string `json:"patterns_allowed,omitempty"`
	DefaultWorkflowPermissions   string   `json:"default_workflow_permissions,omitempty"`
	CanApprovePullRequestReviews bool     `json:"can_approve_pull_request_reviews,omitempty"`
}

// ActionsConfigRead creates a tool to read a repository's Actions
// configuration.
func ActionsConfigRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name: "actions_config_read",
			Description: t("TOOL_ACTIONS_CONFIG_READ_DESCRIPTION", "Read a repository's GitHub Actions configuration: whether Actions is enabled, which actions workflows are allowed to use, and what the default GITHUB_TOKEN may do. "+
				"Use 'actions_list' and 'actions_get' for workflows and runs; this reports the policy around them."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ACTIONS_CONFIG_READ_USER_TITLE", "Read Actions configuration"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The method to execute. Only 'get_permissions' is supported; it returns the whole configuration.",
						Enum:        []any{actionsConfigMethodPermissions},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)
			if method != actionsConfigMethodPermissions {
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. The supported method is: get_permissions", method)), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			permissions, resp, err := client.Repositories.GetActionsPermissions(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get Actions permissions", resp, err), nil, nil
			}
			_ = resp.Body.Close()

			config := MinimalActionsConfig{
				Enabled:            permissions.GetEnabled(),
				AllowedActions:     permissions.GetAllowedActions(),
				SHAPinningRequired: permissions.GetSHAPinningRequired(),
			}

			// The allow-list only exists while the policy is "selected", and
			// asking for it otherwise is a 409.
			if config.AllowedActions == actionsAllowedActionsSelectedSentinel {
				allowed, resp, err := client.Repositories.GetActionsAllowed(ctx, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get the allowed actions policy", resp, err), nil, nil
				}
				_ = resp.Body.Close()
				config.GitHubOwnedAllowed = allowed.GetGithubOwnedAllowed()
				config.VerifiedAllowed = allowed.GetVerifiedAllowed()
				config.PatternsAllowed = allowed.PatternsAllowed
			}

			workflowPermissions, resp, err := client.Repositories.GetDefaultWorkflowPermissions(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get the default workflow permissions", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()
			config.DefaultWorkflowPermissions = workflowPermissions.GetDefaultWorkflowPermissions()
			config.CanApprovePullRequestReviews = workflowPermissions.GetCanApprovePullRequestReviews()

			return marshalDeploymentResult(config, func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelRepoMetadata)
			})
		},
	)
}

// ActionsConfigWrite creates a tool to change a repository's Actions
// configuration and to enable or disable individual workflows.
func ActionsConfigWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name: "actions_config_write",
			Description: t("TOOL_ACTIONS_CONFIG_WRITE_DESCRIPTION", "Change a repository's GitHub Actions configuration: enable or disable Actions, restrict which actions workflows may use, set what the default GITHUB_TOKEN can do, and enable or disable an individual workflow. "+
				"Use 'actions_run_trigger' to run, re-run or cancel workflows; this changes the policy around them."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ACTIONS_CONFIG_WRITE_USER_TITLE", "Change Actions configuration"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The operation to perform:\n" +
							"- set_permissions: enable or disable Actions, and choose which actions may be used\n" +
							"- set_allowed_actions: the allow-list applied when allowed_actions is 'selected'\n" +
							"- set_workflow_permissions: what the default GITHUB_TOKEN can do\n" +
							"- enable_workflow / disable_workflow: switch one workflow on or off",
						Enum: []any{
							actionsConfigMethodSetPermissions,
							actionsConfigMethodSetAllowedActions,
							actionsConfigMethodSetWorkflowPerms,
							actionsConfigMethodEnableWorkflow,
							actionsConfigMethodDisableWorkflow,
						},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"enabled": {
						Type:        "boolean",
						Description: "Whether GitHub Actions runs in this repository at all. Used by 'set_permissions'.",
					},
					"allowed_actions": {
						Type:        "string",
						Description: "Which actions workflows may use: 'all', 'local_only' for actions defined in this repository, or 'selected' for the allow-list set by 'set_allowed_actions'. Used by 'set_permissions'.",
						Enum:        []any{"all", "local_only", actionsAllowedActionsSelectedSentinel},
					},
					"github_owned_allowed": {
						Type:        "boolean",
						Description: "Allow actions published by GitHub. Used by 'set_allowed_actions'.",
					},
					"verified_allowed": {
						Type:        "boolean",
						Description: "Allow actions from verified creators. Used by 'set_allowed_actions'.",
					},
					"patterns_allowed": {
						Type:        "array",
						Description: "Specific actions to allow, e.g. ['octo-org/*', 'actions/checkout@v4']. Pass an empty array to allow no third-party actions. Used by 'set_allowed_actions'.",
						Items:       &jsonschema.Schema{Type: "string"},
					},
					"default_workflow_permissions": {
						Type:        "string",
						Description: "What the default GITHUB_TOKEN may do in a workflow. Used by 'set_workflow_permissions'.",
						Enum:        []any{"read", "write"},
					},
					"can_approve_pull_request_reviews": {
						Type:        "boolean",
						Description: "Allow GitHub Actions to approve pull requests. Used by 'set_workflow_permissions'.",
					},
					"workflow": {
						Type:        "string",
						Description: "Workflow file name, e.g. 'deploy.yml', or its numeric ID. Required for 'enable_workflow' and 'disable_workflow'.",
					},
				},
				Required: []string{"method", "owner", "repo"},
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case actionsConfigMethodSetPermissions:
				update := github.ActionsPermissionsRepository{}
				changed := false

				if raw, ok := args["enabled"]; ok && raw != nil {
					value, ok := raw.(bool)
					if !ok {
						return utils.NewToolResultError("enabled must be a boolean"), nil, nil
					}
					update.Enabled = github.Ptr(value)
					changed = true
				}
				if allowed, err := OptionalParam[string](args, "allowed_actions"); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				} else if allowed != "" {
					update.AllowedActions = github.Ptr(allowed)
					changed = true
				}
				if !changed {
					return utils.NewToolResultError("provide enabled, allowed_actions, or both"), nil, nil
				}
				// The API rejects a request that sets allowed_actions without
				// saying Actions is enabled.
				if update.AllowedActions != nil && update.Enabled == nil {
					update.Enabled = github.Ptr(true)
				}

				permissions, resp, err := client.Repositories.UpdateActionsPermissions(ctx, owner, repo, update)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update Actions permissions", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(MinimalActionsConfig{
					Enabled:            permissions.GetEnabled(),
					AllowedActions:     permissions.GetAllowedActions(),
					SHAPinningRequired: permissions.GetSHAPinningRequired(),
				}, nil)

			case actionsConfigMethodSetAllowedActions:
				// This endpoint replaces the policy wholesale, so the current
				// one is read first and only the named fields are changed. A
				// read that fails aborts the whole operation: a policy
				// assembled from nothing would silently clear whatever the
				// repository has configured today.
				current, resp, err := client.Repositories.GetActionsAllowed(ctx, owner, repo)
				if err != nil {
					if resp != nil && resp.StatusCode == http.StatusConflict {
						// GitHub only serves this policy while allowed_actions
						// is "selected"; say so rather than passing on a bare
						// conflict.
						return utils.NewToolResultError("cannot read the current allowed actions policy because allowed_actions is not 'selected', so nothing was changed. Run set_permissions with allowed_actions 'selected' first, then set the allow-list."), nil, nil
					}
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to read the current allowed actions policy, so it was left unchanged", resp, err), nil, nil
				}
				_ = resp.Body.Close()

				// Start from the state that was actually read back, then layer
				// the caller's explicit changes on top of it.
				allowed := github.ActionsAllowed{
					GithubOwnedAllowed: current.GithubOwnedAllowed,
					VerifiedAllowed:    current.VerifiedAllowed,
					PatternsAllowed:    current.PatternsAllowed,
				}

				for key, target := range map[string]**bool{
					"github_owned_allowed": &allowed.GithubOwnedAllowed,
					"verified_allowed":     &allowed.VerifiedAllowed,
				} {
					raw, ok := args[key]
					if !ok || raw == nil {
						continue
					}
					value, ok := raw.(bool)
					if !ok {
						return utils.NewToolResultError(fmt.Sprintf("%s must be a boolean", key)), nil, nil
					}
					*target = github.Ptr(value)
				}
				if patterns, ok, err := optionalStringSlice(args, "patterns_allowed"); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				} else if ok {
					if patterns == nil {
						patterns = []string{}
					}
					allowed.PatternsAllowed = patterns
				}

				result, resp, err := client.Repositories.EditActionsAllowed(ctx, owner, repo, allowed)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update the allowed actions policy", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(MinimalActionsConfig{
					AllowedActions:     actionsAllowedActionsSelectedSentinel,
					GitHubOwnedAllowed: result.GetGithubOwnedAllowed(),
					VerifiedAllowed:    result.GetVerifiedAllowed(),
					PatternsAllowed:    result.PatternsAllowed,
				}, nil)

			case actionsConfigMethodSetWorkflowPerms:
				update := github.DefaultWorkflowPermissionRepository{}
				changed := false

				if permissions, err := OptionalParam[string](args, "default_workflow_permissions"); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				} else if permissions != "" {
					update.DefaultWorkflowPermissions = github.Ptr(permissions)
					changed = true
				}
				if raw, ok := args["can_approve_pull_request_reviews"]; ok && raw != nil {
					value, ok := raw.(bool)
					if !ok {
						return utils.NewToolResultError("can_approve_pull_request_reviews must be a boolean"), nil, nil
					}
					update.CanApprovePullRequestReviews = github.Ptr(value)
					changed = true
				}
				if !changed {
					return utils.NewToolResultError("provide default_workflow_permissions, can_approve_pull_request_reviews, or both"), nil, nil
				}

				result, resp, err := client.Repositories.UpdateDefaultWorkflowPermissions(ctx, owner, repo, update)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update the default workflow permissions", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(MinimalActionsConfig{
					DefaultWorkflowPermissions:   result.GetDefaultWorkflowPermissions(),
					CanApprovePullRequestReviews: result.GetCanApprovePullRequestReviews(),
				}, nil)

			case actionsConfigMethodEnableWorkflow, actionsConfigMethodDisableWorkflow:
				workflow, err := RequiredParam[string](args, "workflow")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				enable := method == actionsConfigMethodEnableWorkflow
				var resp *github.Response
				if id, convErr := toInt(workflow); convErr == nil {
					if enable {
						resp, err = client.Actions.EnableWorkflowByID(ctx, owner, repo, int64(id))
					} else {
						resp, err = client.Actions.DisableWorkflowByID(ctx, owner, repo, int64(id))
					}
				} else if enable {
					resp, err = client.Actions.EnableWorkflowByFileName(ctx, owner, repo, workflow)
				} else {
					resp, err = client.Actions.DisableWorkflowByFileName(ctx, owner, repo, workflow)
				}
				if err != nil {
					action := "enable"
					if !enable {
						action = "disable"
					}
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to %s workflow '%s'", action, workflow), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				state := "workflow_enabled"
				if !enable {
					state = "workflow_disabled"
				}
				return marshalDeploymentResult(map[string]any{
					"result":   state,
					"workflow": workflow,
				}, nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: set_permissions, set_allowed_actions, set_workflow_permissions, enable_workflow, disable_workflow", method)), nil, nil
			}
		},
	)
}
