package github

import (
	"context"
	"fmt"
	"slices"
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
)

const (
	checksMethodCreateCheckRun = "create_check_run"
	checksMethodUpdateCheckRun = "update_check_run"
)

// checkRunConclusions are the conclusion values the Checks API accepts. A
// conclusion outside this set is rejected locally rather than spending a round
// trip on a 422.
var checkRunConclusions = []string{
	"action_required",
	"cancelled",
	"failure",
	"neutral",
	"skipped",
	"success",
	"timed_out",
}

// ChecksWrite creates a tool to publish check runs against a commit.
//
// This is the write half of the Checks API; commit statuses (the older
// Statuses API) are written by commit_status_write instead. Note that check
// runs can only be created by a GitHub App installation token — a personal
// access token can read check runs but not write them, and GitHub answers such
// a request with 403 "Resource not accessible by integration"/"by personal
// access token".
func ChecksWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "checks_write",
			Description: t("TOOL_CHECKS_WRITE_DESCRIPTION", "Create or update a check run against a commit, to publish verification state that gates a merge. Requires a GitHub App installation token: a personal access token can read check runs but not write them. To post CI state with a personal access token, use commit_status_write instead."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_CHECKS_WRITE_USER_TITLE", "Create or update a check run"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- create_check_run: create a new check run for a commit SHA\n" +
							"- update_check_run: update an existing check run by ID",
						Enum: []any{
							checksMethodCreateCheckRun,
							checksMethodUpdateCheckRun,
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
					"name": {
						Type:        "string",
						Description: "Name of the check run, e.g. 'arcanum/gate'. Required for 'create_check_run'.",
					},
					"head_sha": {
						Type:        "string",
						Description: "Commit SHA the check run reports on. Required for 'create_check_run'.",
					},
					"check_run_id": {
						Type:        "number",
						Description: "Numeric check run ID. Required for 'update_check_run'.",
					},
					"status": {
						Type:        "string",
						Description: "Current state of the check run. Defaults to 'queued' on create.",
						Enum:        []any{"queued", "in_progress", "completed"},
					},
					"conclusion": {
						Type:        "string",
						Description: "Final result of the check run. Required when status is 'completed', and setting it implies completion.",
						Enum:        []any{"action_required", "cancelled", "failure", "neutral", "skipped", "success", "timed_out"},
					},
					"details_url": {
						Type:        "string",
						Description: "URL the check run links to from the GitHub UI, e.g. the build output.",
					},
					"external_id": {
						Type:        "string",
						Description: "Caller-defined identifier for the run, echoed back by GitHub.",
					},
					"started_at": {
						Type:        "string",
						Description: "When the check run began, as an RFC3339 timestamp (e.g. '2026-08-25T10:00:00Z').",
					},
					"completed_at": {
						Type:        "string",
						Description: "When the check run finished, as an RFC3339 timestamp. Only meaningful once the run is completed.",
					},
					"output_title": {
						Type:        "string",
						Description: "Title of the check run output shown in the GitHub UI.",
					},
					"output_summary": {
						Type:        "string",
						Description: "Short summary of the check run output. Supports Markdown.",
					},
					"output_text": {
						Type:        "string",
						Description: "Full detail of the check run output. Supports Markdown.",
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
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if method != checksMethodCreateCheckRun && method != checksMethodUpdateCheckRun {
				return utils.NewToolResultError(fmt.Sprintf("unknown method %q, expected one of: %s, %s", method, checksMethodCreateCheckRun, checksMethodUpdateCheckRun)), nil, nil
			}

			status, err := OptionalParam[string](args, "status")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			conclusion, err := OptionalParam[string](args, "conclusion")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Validate locally so an obviously wrong call does not cost a round
			// trip, and so the agent gets the accepted set back rather than a
			// bare 422.
			if conclusion != "" && !slices.Contains(checkRunConclusions, conclusion) {
				return utils.NewToolResultError(fmt.Sprintf("invalid conclusion %q, expected one of: %s", conclusion, strings.Join(checkRunConclusions, ", "))), nil, nil
			}
			if status == "completed" && conclusion == "" {
				return utils.NewToolResultError("a completed check run requires a conclusion, expected one of: " + strings.Join(checkRunConclusions, ", ")), nil, nil
			}

			detailsURL, err := OptionalParam[string](args, "details_url")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			externalID, err := OptionalParam[string](args, "external_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			startedAt, err := optionalTimestampParam(args, "started_at")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			completedAt, err := optionalTimestampParam(args, "completed_at")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			output, err := optionalCheckRunOutput(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case checksMethodCreateCheckRun:
				name, err := RequiredParam[string](args, "name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				headSHA, err := RequiredParam[string](args, "head_sha")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				opts := github.CreateCheckRunOptions{
					Name:        name,
					HeadSHA:     headSHA,
					Status:      ToStringPtr(status),
					Conclusion:  ToStringPtr(conclusion),
					DetailsURL:  ToStringPtr(detailsURL),
					ExternalID:  ToStringPtr(externalID),
					StartedAt:   startedAt,
					CompletedAt: completedAt,
					Output:      output,
				}

				run, resp, err := client.Checks.CreateCheckRun(ctx, owner, repo, opts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create check run", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalChecksResult(convertToMinimalCheckRun(run), identityChecksLabel)

			default:
				checkRunID, err := RequiredInt(args, "check_run_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				name, err := OptionalParam[string](args, "name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				opts := github.UpdateCheckRunOptions{
					Name:        name,
					Status:      ToStringPtr(status),
					Conclusion:  ToStringPtr(conclusion),
					DetailsURL:  ToStringPtr(detailsURL),
					ExternalID:  ToStringPtr(externalID),
					CompletedAt: completedAt,
					Output:      output,
				}

				run, resp, err := client.Checks.UpdateCheckRun(ctx, owner, repo, int64(checkRunID), opts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update check run", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalChecksResult(convertToMinimalCheckRun(run), identityChecksLabel)
			}
		},
	)
}

// optionalTimestampParam reads an RFC3339 timestamp argument, rejecting a
// malformed value locally rather than sending it to GitHub.
func optionalTimestampParam(args map[string]any, key string) (*github.Timestamp, error) {
	raw, err := OptionalParam[string](args, key)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be an RFC3339 timestamp, e.g. '2026-08-25T10:00:00Z': %w", key, err)
	}

	return &github.Timestamp{Time: parsed}, nil
}

// optionalCheckRunOutput assembles the check run output object, returning nil
// when the caller supplied none of its fields.
func optionalCheckRunOutput(args map[string]any) (*github.CheckRunOutput, error) {
	title, err := OptionalParam[string](args, "output_title")
	if err != nil {
		return nil, err
	}
	summary, err := OptionalParam[string](args, "output_summary")
	if err != nil {
		return nil, err
	}
	text, err := OptionalParam[string](args, "output_text")
	if err != nil {
		return nil, err
	}

	if title == "" && summary == "" && text == "" {
		return nil, nil
	}

	return &github.CheckRunOutput{
		Title:   ToStringPtr(title),
		Summary: ToStringPtr(summary),
		Text:    ToStringPtr(text),
	}, nil
}

// identityChecksLabel is the no-op label used by checks_write, whose results
// need no extra annotation before being returned.
func identityChecksLabel(result *mcp.CallToolResult) *mcp.CallToolResult {
	return result
}
