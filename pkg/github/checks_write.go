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

// checkRunAnnotationLevels are the annotation_level values the Checks API
// accepts, validated locally for the same reason as the conclusion set.
var checkRunAnnotationLevels = []string{
	"failure",
	"notice",
	"warning",
}

// checkRunAnnotationsPerRequest is GitHub's cap on how many annotations one
// create/update call may carry. Longer runs are published by updating the same
// check run repeatedly, 50 at a time.
const checkRunAnnotationsPerRequest = 50

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
						Description: "When the check run began, as an RFC3339 timestamp (e.g. '2026-08-25T10:00:00Z'). Accepted by 'create_check_run' only — GitHub's update endpoint does not take it, and passing it to 'update_check_run' is rejected rather than silently dropped.",
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
					"output_annotations": {
						Type:        "array",
						Description: "Line-level annotations attached to the check run output, shown inline on the diff. At most 50 per call; publish more by updating the same check run again with the next batch.",
						Items: &jsonschema.Schema{
							Type: "object",
							Properties: map[string]*jsonschema.Schema{
								"path": {
									Type:        "string",
									Description: "Repository-relative path of the annotated file, e.g. 'pkg/github/checks_write.go'.",
								},
								"start_line": {
									Type:        "number",
									Description: "First line of the annotated range, within the file's diff.",
								},
								"end_line": {
									Type:        "number",
									Description: "Last line of the annotated range. Equal to start_line for a single line.",
								},
								"start_column": {
									Type:        "number",
									Description: "First column of the annotated range. Only accepted when start_line and end_line are the same line.",
								},
								"end_column": {
									Type:        "number",
									Description: "Last column of the annotated range. Only accepted when start_line and end_line are the same line.",
								},
								"annotation_level": {
									Type:        "string",
									Description: "Severity of the annotation.",
									Enum:        []any{"failure", "notice", "warning"},
								},
								"message": {
									Type:        "string",
									Description: "The annotation text shown on the line.",
								},
								"title": {
									Type:        "string",
									Description: "Short heading for the annotation.",
								},
								"raw_details": {
									Type:        "string",
									Description: "Verbatim tool output for the annotation, shown when it is expanded.",
								},
							},
							Required: []string{"path", "start_line", "end_line", "annotation_level", "message"},
						},
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
				// github.UpdateCheckRunOptions has no StartedAt field because
				// GitHub's PATCH endpoint does not accept one. Accepting the
				// argument here and dropping it would report success for a
				// change that never happened.
				if startedAt != nil {
					return utils.NewToolResultError("started_at is only accepted by create_check_run; GitHub's update endpoint does not change a run's start time"), nil, nil
				}

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

	annotations, err := optionalCheckRunAnnotations(args)
	if err != nil {
		return nil, err
	}

	if title == "" && summary == "" && text == "" && len(annotations) == 0 {
		return nil, nil
	}

	return &github.CheckRunOutput{
		Title:       ToStringPtr(title),
		Summary:     ToStringPtr(summary),
		Text:        ToStringPtr(text),
		Annotations: annotations,
	}, nil
}

// optionalCheckRunAnnotations reads the output_annotations array, validating
// each entry against the Checks API's rules locally: the required fields, the
// accepted annotation levels, the per-request cap, and the API's restriction
// that columns may only be given for a single-line annotation.
func optionalCheckRunAnnotations(args map[string]any) ([]*github.CheckRunAnnotation, error) {
	raw, ok := args["output_annotations"]
	if !ok || raw == nil {
		return nil, nil
	}

	entries, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("output_annotations must be an array")
	}
	if len(entries) > checkRunAnnotationsPerRequest {
		return nil, fmt.Errorf("output_annotations accepts at most %d entries per call, got %d; publish the rest by updating the same check run again", checkRunAnnotationsPerRequest, len(entries))
	}

	annotations := make([]*github.CheckRunAnnotation, 0, len(entries))
	for i, entry := range entries {
		fields, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("output_annotations[%d] must be an object", i)
		}

		path, ok := fields["path"].(string)
		if !ok || path == "" {
			return nil, fmt.Errorf("output_annotations[%d].path is required", i)
		}
		message, ok := fields["message"].(string)
		if !ok || message == "" {
			return nil, fmt.Errorf("output_annotations[%d].message is required", i)
		}
		level, ok := fields["annotation_level"].(string)
		if !ok || level == "" {
			return nil, fmt.Errorf("output_annotations[%d].annotation_level is required", i)
		}
		if !slices.Contains(checkRunAnnotationLevels, level) {
			return nil, fmt.Errorf("output_annotations[%d].annotation_level is %q, expected one of: %s", i, level, strings.Join(checkRunAnnotationLevels, ", "))
		}

		startLine, err := requiredAnnotationInt(fields, i, "start_line")
		if err != nil {
			return nil, err
		}
		endLine, err := requiredAnnotationInt(fields, i, "end_line")
		if err != nil {
			return nil, err
		}
		if endLine < startLine {
			return nil, fmt.Errorf("output_annotations[%d].end_line (%d) is before start_line (%d)", i, endLine, startLine)
		}

		annotation := &github.CheckRunAnnotation{
			Path:            github.Ptr(path),
			StartLine:       github.Ptr(startLine),
			EndLine:         github.Ptr(endLine),
			AnnotationLevel: github.Ptr(level),
			Message:         github.Ptr(message),
			Title:           ToStringPtr(stringField(fields, "title")),
			RawDetails:      ToStringPtr(stringField(fields, "raw_details")),
		}

		startColumn, hasStartColumn, err := optionalAnnotationInt(fields, i, "start_column")
		if err != nil {
			return nil, err
		}
		endColumn, hasEndColumn, err := optionalAnnotationInt(fields, i, "end_column")
		if err != nil {
			return nil, err
		}
		if (hasStartColumn || hasEndColumn) && startLine != endLine {
			return nil, fmt.Errorf("output_annotations[%d] gives a column on a multi-line annotation; GitHub accepts start_column/end_column only when start_line equals end_line", i)
		}
		if hasStartColumn {
			annotation.StartColumn = github.Ptr(startColumn)
		}
		if hasEndColumn {
			annotation.EndColumn = github.Ptr(endColumn)
		}

		annotations = append(annotations, annotation)
	}

	return annotations, nil
}

// requiredAnnotationInt reads a numeric annotation field that must be present.
func requiredAnnotationInt(fields map[string]any, index int, key string) (int, error) {
	value, ok, err := optionalAnnotationInt(fields, index, key)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("output_annotations[%d].%s is required", index, key)
	}
	return value, nil
}

// optionalAnnotationInt reads a numeric annotation field, reporting whether it
// was supplied at all so a missing field and an explicit zero stay distinct.
func optionalAnnotationInt(fields map[string]any, index int, key string) (int, bool, error) {
	raw, ok := fields[key]
	if !ok || raw == nil {
		return 0, false, nil
	}
	value, err := toInt(raw)
	if err != nil {
		return 0, false, fmt.Errorf("output_annotations[%d].%s must be a number", index, key)
	}
	return value, true, nil
}

// stringField reads an optional string field, treating a wrong type as absent
// rather than failing: every field read through it is cosmetic.
func stringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

// identityChecksLabel is the no-op label used by checks_write, whose results
// need no extra annotation before being returned.
func identityChecksLabel(result *mcp.CallToolResult) *mcp.CallToolResult {
	return result
}
