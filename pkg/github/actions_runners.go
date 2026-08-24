package github

import (
	"context"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
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
	runnersScopeRepo = "repository"
	runnersScopeOrg  = "organization"

	runnersMethodList = "list"
	runnersMethodGet  = "get"

	runnerWriteMethodRemove = "remove"
)

var runnerScopes = []any{runnersScopeRepo, runnersScopeOrg}

// MinimalRunner is the trimmed output type for a self-hosted Actions runner.
type MinimalRunner struct {
	ID     int64  `json:"id"`
	Name   string `json:"name,omitempty"`
	OS     string `json:"os,omitempty"`
	Status string `json:"status,omitempty"`
	Busy   bool   `json:"busy,omitempty"`
}

func convertToMinimalRunner(r *github.Runner) MinimalRunner {
	return MinimalRunner{
		ID:     r.GetID(),
		Name:   sanitize.Sanitize(r.GetName()),
		OS:     r.GetOS(),
		Status: r.GetStatus(),
		Busy:   r.GetBusy(),
	}
}

// ActionsRunnersRead creates a tool to inspect self-hosted Actions runners.
func ActionsRunnersRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name:        "actions_runners_read",
			Description: t("TOOL_ACTIONS_RUNNERS_READ_DESCRIPTION", "Read self-hosted Actions runners: list them, or get one runner's status. Scoped to a repository or an organization."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ACTIONS_RUNNERS_READ_USER_TITLE", "Read Actions runners"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list: all self-hosted runners in scope\n" +
							"- get: one runner's details",
						Enum: []any{runnersMethodList, runnersMethodGet},
					},
					"scope": {
						Type:        "string",
						Description: "Where the runners are registered. Defaults to 'repository'.",
						Enum:        runnerScopes,
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner. Required when scope is 'repository'.",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name. Required when scope is 'repository'.",
					},
					"org": {
						Type:        "string",
						Description: "Organization login. Required when scope is 'organization'.",
					},
					"runner_id": {
						Type:        "integer",
						Description: "Runner ID. Required for 'get'.",
					},
				},
				Required: []string{"method"},
			}),
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)

			scope, err := OptionalParam[string](args, "scope")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if scope == "" {
				scope = runnersScopeRepo
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case runnersMethodList:
				listOpts := &github.ListRunnersOptions{ListOptions: github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}}

				if scope == runnersScopeOrg {
					org, err := RequiredParam[string](args, "org")
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
					runners, resp, err := client.Actions.ListOrganizationRunners(ctx, org, listOpts)
					if err != nil {
						return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list organization runners", resp, err), nil, nil
					}
					defer func() { _ = resp.Body.Close() }()
					return marshalGovernanceResult(minimalRunners(runners.Runners), nil)
				}

				owner, err := RequiredParam[string](args, "owner")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				repo, err := RequiredParam[string](args, "repo")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				runners, resp, err := client.Actions.ListRunners(ctx, owner, repo, listOpts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list repository runners", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(minimalRunners(runners.Runners), nil)

			case runnersMethodGet:
				runnerID, err := RequiredInt(args, "runner_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				if scope == runnersScopeOrg {
					org, err := RequiredParam[string](args, "org")
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
					runner, resp, err := client.Actions.GetOrganizationRunner(ctx, org, int64(runnerID))
					if err != nil {
						return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get organization runner %d", runnerID), resp, err), nil, nil
					}
					defer func() { _ = resp.Body.Close() }()
					return marshalGovernanceResult(convertToMinimalRunner(runner), nil)
				}

				owner, err := RequiredParam[string](args, "owner")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				repo, err := RequiredParam[string](args, "repo")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				runner, resp, err := client.Actions.GetRunner(ctx, owner, repo, int64(runnerID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get runner %d", runnerID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalRunner(runner), nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list, get", method)), nil, nil
			}
		},
	)
}

func minimalRunners(runners []*github.Runner) []MinimalRunner {
	minimal := make([]MinimalRunner, 0, len(runners))
	for _, r := range runners {
		if r != nil {
			minimal = append(minimal, convertToMinimalRunner(r))
		}
	}
	return minimal
}

// ActionsRunnerWrite creates a tool to deregister a self-hosted Actions runner.
func ActionsRunnerWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name: "actions_runner_write",
			Description: t("TOOL_ACTIONS_RUNNER_WRITE_DESCRIPTION", "Deregister a self-hosted Actions runner from a repository or an organization. "+
				"This removes the runner's registration only; it does not uninstall or stop the runner agent process on its machine."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ACTIONS_RUNNER_WRITE_USER_TITLE", "Manage Actions runners"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The operation to perform:\n- remove: deregister a runner",
						Enum:        []any{runnerWriteMethodRemove},
					},
					"scope": {
						Type:        "string",
						Description: "Where the runner is registered. Defaults to 'repository'.",
						Enum:        runnerScopes,
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner. Required when scope is 'repository'.",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name. Required when scope is 'repository'.",
					},
					"org": {
						Type:        "string",
						Description: "Organization login. Required when scope is 'organization'.",
					},
					"runner_id": {
						Type:        "integer",
						Description: "Runner ID to remove.",
					},
				},
				Required: []string{"method", "runner_id"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)
			if method != runnerWriteMethodRemove {
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: remove", method)), nil, nil
			}

			runnerID, err := RequiredInt(args, "runner_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			scope, err := OptionalParam[string](args, "scope")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if scope == "" {
				scope = runnersScopeRepo
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			if scope == runnersScopeOrg {
				org, err := RequiredParam[string](args, "org")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				resp, err := client.Actions.RemoveOrganizationRunner(ctx, org, int64(runnerID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to remove organization runner %d", runnerID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(map[string]any{"result": "runner_removed", "runner_id": runnerID}, nil)
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			resp, err := client.Actions.RemoveRunner(ctx, owner, repo, int64(runnerID))
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to remove runner %d", runnerID), resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()
			return marshalGovernanceResult(map[string]any{"result": "runner_removed", "runner_id": runnerID}, nil)
		},
	)
}
