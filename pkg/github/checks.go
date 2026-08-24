package github

import (
	"context"
	"encoding/json"
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
	checksMethodListCheckRuns    = "list_check_runs"
	checksMethodGetCheckRun      = "get_check_run"
	checksMethodListCheckSuites  = "list_check_suites"
	checksMethodGetCombinedState = "get_combined_status"
	checksMethodListStatuses     = "list_statuses"
)

// MinimalCheckSuite is the trimmed output type for check suite objects.
type MinimalCheckSuite struct {
	ID         int64  `json:"id"`
	AppName    string `json:"app_name,omitempty"`
	HeadBranch string `json:"head_branch,omitempty"`
	HeadSHA    string `json:"head_sha,omitempty"`
	Status     string `json:"status,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// MinimalCheckSuitesResult is the trimmed output type for check suite list results.
type MinimalCheckSuitesResult struct {
	TotalCount  int                 `json:"total_count"`
	CheckSuites []MinimalCheckSuite `json:"check_suites"`
}

func convertToMinimalCheckSuite(suite *github.CheckSuite) MinimalCheckSuite {
	m := MinimalCheckSuite{
		ID:         suite.GetID(),
		AppName:    sanitize.Sanitize(suite.GetApp().GetName()),
		HeadBranch: suite.GetHeadBranch(),
		HeadSHA:    suite.GetHeadSHA(),
		Status:     suite.GetStatus(),
		Conclusion: suite.GetConclusion(),
	}

	if suite.UpdatedAt != nil {
		m.UpdatedAt = suite.UpdatedAt.Format(time.RFC3339)
	}

	return m
}

// ChecksRead creates a tool to read check runs, check suites and commit
// statuses for a ref.
//
// Check runs (the Checks API) and commit statuses (the older Statuses API) are
// separate systems that both gate a merge, so both are exposed here: an agent
// asking "is this commit green?" has to look at each.
func ChecksRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "checks_read",
			Description: t("TOOL_CHECKS_READ_DESCRIPTION", "Read CI results for a commit, branch or tag: check runs and check suites from the Checks API, and commit statuses from the Statuses API. Both gate a merge, so a fully green ref needs both to pass."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CHECKS_READ_USER_TITLE", "Read check runs and commit statuses"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list_check_runs: check runs for a ref (Checks API)\n" +
							"- get_check_run: a single check run by ID\n" +
							"- list_check_suites: check suites for a ref\n" +
							"- get_combined_status: the rolled-up commit status for a ref (Statuses API)\n" +
							"- list_statuses: individual commit statuses for a ref, most recent first",
						Enum: []any{
							checksMethodListCheckRuns,
							checksMethodGetCheckRun,
							checksMethodListCheckSuites,
							checksMethodGetCombinedState,
							checksMethodListStatuses,
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
					"ref": {
						Type:        "string",
						Description: "Commit SHA, branch name or tag name. Required for all methods except 'get_check_run'.",
					},
					"check_run_id": {
						Type:        "number",
						Description: "Numeric check run ID. Required for 'get_check_run'.",
					},
					"check_name": {
						Type:        "string",
						Description: "Only return check runs or suites with this name. Used by 'list_check_runs' and 'list_check_suites'.",
					},
					"status": {
						Type:        "string",
						Description: "Only return check runs in this status. Used by 'list_check_runs'.",
						Enum:        []any{"queued", "in_progress", "completed"},
					},
					"filter": {
						Type:        "string",
						Description: "Whether to return the latest check run per name or every run, including reruns. Defaults to 'latest'. Used by 'list_check_runs'.",
						Enum:        []any{"latest", "all"},
					},
					"app_id": {
						Type:        "number",
						Description: "Only return check runs or suites produced by this GitHub App ID.",
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
			ref, err := OptionalParam[string](args, "ref")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			listOpts := github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// CI results are produced by whatever the repo's checks are wired
			// to, which on a public repo means world-readable output that an
			// attacker can influence via a PR; confidentiality still follows
			// repo visibility.
			labelResult := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelActionsResult)
			}

			switch method {
			case checksMethodListCheckRuns:
				if ref == "" {
					return utils.NewToolResultError("ref is required for list_check_runs"), nil, nil
				}

				checkName, err := OptionalParam[string](args, "check_name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				status, err := OptionalParam[string](args, "status")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				filter, err := OptionalParam[string](args, "filter")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				appID, err := OptionalIntParam(args, "app_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				opts := &github.ListCheckRunsOptions{
					CheckName:   ToStringPtr(checkName),
					Status:      ToStringPtr(status),
					Filter:      ToStringPtr(filter),
					ListOptions: listOpts,
				}
				if appID != 0 {
					opts.AppID = github.Ptr(int64(appID))
				}

				results, resp, err := client.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, opts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list check runs", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := MinimalCheckRunsResult{
					TotalCount: results.GetTotal(),
					CheckRuns:  make([]MinimalCheckRun, 0, len(results.CheckRuns)),
				}
				for _, run := range results.CheckRuns {
					if run != nil {
						minimal.CheckRuns = append(minimal.CheckRuns, convertToMinimalCheckRun(run))
					}
				}

				return marshalChecksResult(minimal, labelResult)

			case checksMethodGetCheckRun:
				checkRunID, err := RequiredInt(args, "check_run_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				run, resp, err := client.Checks.GetCheckRun(ctx, owner, repo, int64(checkRunID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get check run", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalChecksResult(convertToMinimalCheckRun(run), labelResult)

			case checksMethodListCheckSuites:
				if ref == "" {
					return utils.NewToolResultError("ref is required for list_check_suites"), nil, nil
				}

				checkName, err := OptionalParam[string](args, "check_name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				appID, err := OptionalIntParam(args, "app_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				opts := &github.ListCheckSuiteOptions{
					CheckName:   ToStringPtr(checkName),
					ListOptions: listOpts,
				}
				if appID != 0 {
					opts.AppID = github.Ptr(int64(appID))
				}

				results, resp, err := client.Checks.ListCheckSuitesForRef(ctx, owner, repo, ref, opts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list check suites", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := MinimalCheckSuitesResult{
					TotalCount:  results.GetTotal(),
					CheckSuites: make([]MinimalCheckSuite, 0, len(results.CheckSuites)),
				}
				for _, suite := range results.CheckSuites {
					if suite != nil {
						minimal.CheckSuites = append(minimal.CheckSuites, convertToMinimalCheckSuite(suite))
					}
				}

				return marshalChecksResult(minimal, labelResult)

			case checksMethodGetCombinedState:
				if ref == "" {
					return utils.NewToolResultError("ref is required for get_combined_status"), nil, nil
				}

				status, resp, err := client.Repositories.GetCombinedStatus(ctx, owner, repo, ref, &listOpts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get combined status", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalChecksResult(convertToMinimalCombinedStatus(status), labelResult)

			case checksMethodListStatuses:
				if ref == "" {
					return utils.NewToolResultError("ref is required for list_statuses"), nil, nil
				}

				statuses, resp, err := client.Repositories.ListStatuses(ctx, owner, repo, ref, &listOpts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list commit statuses", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalRepoStatus, 0, len(statuses))
				for _, status := range statuses {
					if status != nil {
						minimal = append(minimal, convertToMinimalRepoStatus(status))
					}
				}

				return marshalChecksResult(minimal, labelResult)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list_check_runs, get_check_run, list_check_suites, get_combined_status, list_statuses", method)), nil, nil
			}
		},
	)
}

// CommitStatusWrite creates a tool to post a commit status against a ref.
func CommitStatusWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "commit_status_write",
			Description: t("TOOL_COMMIT_STATUS_WRITE_DESCRIPTION", "Post a commit status against a commit, branch or tag. Statuses are additive: posting a new status for an existing context supersedes the previous one rather than replacing it, and there is no way to delete one."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_COMMIT_STATUS_WRITE_USER_TITLE", "Post a commit status"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"ref": {
						Type:        "string",
						Description: "Commit SHA, branch name or tag name to attach the status to.",
					},
					"state": {
						Type:        "string",
						Description: "The status state.",
						Enum:        []any{"error", "failure", "pending", "success"},
					},
					"context": {
						Type:        "string",
						Description: "Label that distinguishes this status from other systems' statuses (e.g. 'ci/lint'). Defaults to 'default'.",
					},
					"description": {
						Type:        "string",
						Description: "Short human-readable summary of the status.",
					},
					"target_url": {
						Type:        "string",
						Description: "URL the status links to from the GitHub UI, e.g. the build output.",
					},
				},
				Required: []string{"owner", "repo", "ref", "state"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ref, err := RequiredParam[string](args, "ref")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state, err := RequiredParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			statusContext, err := OptionalParam[string](args, "context")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			description, err := OptionalParam[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			targetURL, err := OptionalParam[string](args, "target_url")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			status := github.RepoStatus{
				State:       github.Ptr(state),
				Context:     ToStringPtr(statusContext),
				Description: ToStringPtr(description),
				TargetURL:   ToStringPtr(targetURL),
			}

			created, resp, err := client.Repositories.CreateStatus(ctx, owner, repo, ref, status)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create commit status", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(convertToMinimalRepoStatus(created))
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func marshalChecksResult(payload any, label func(*mcp.CallToolResult) *mcp.CallToolResult) (*mcp.CallToolResult, any, error) {
	r, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return label(utils.NewToolResultText(string(r))), nil, nil
}
