package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	milestoneMethodCreate = "create"
	milestoneMethodUpdate = "update"
	milestoneMethodDelete = "delete"
)

// milestoneDueOnLayout is the date format accepted for due dates. The REST API
// wants an RFC 3339 timestamp, but milestones are day-granular in the UI, so
// the tool takes a plain date and pins it to the end of that day in UTC —
// otherwise "due 2026-09-01" silently becomes due at midnight, a day early for
// anyone west of UTC.
const milestoneDueOnLayout = "2006-01-02"

// MinimalMilestone is the trimmed output type for milestone objects.
type MinimalMilestone struct {
	Number       int          `json:"number"`
	Title        string       `json:"title"`
	Description  string       `json:"description,omitempty"`
	State        string       `json:"state"`
	HTMLURL      string       `json:"html_url,omitempty"`
	OpenIssues   int          `json:"open_issues"`
	ClosedIssues int          `json:"closed_issues"`
	DueOn        string       `json:"due_on,omitempty"`
	ClosedAt     string       `json:"closed_at,omitempty"`
	Creator      *MinimalUser `json:"creator,omitempty"`
}

func convertToMinimalMilestone(milestone *github.Milestone) MinimalMilestone {
	m := MinimalMilestone{
		Number:       milestone.GetNumber(),
		Title:        sanitize.Sanitize(milestone.GetTitle()),
		Description:  sanitize.Sanitize(milestone.GetDescription()),
		State:        milestone.GetState(),
		HTMLURL:      milestone.GetHTMLURL(),
		OpenIssues:   milestone.GetOpenIssues(),
		ClosedIssues: milestone.GetClosedIssues(),
		Creator:      convertToMinimalUser(milestone.GetCreator()),
	}

	if milestone.DueOn != nil {
		m.DueOn = milestone.DueOn.Format(time.RFC3339)
	}
	if milestone.ClosedAt != nil {
		m.ClosedAt = milestone.ClosedAt.Format(time.RFC3339)
	}

	return m
}

// ListMilestones creates a tool to list the milestones of a repository.
func ListMilestones(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "list_milestones",
			Description: t("TOOL_LIST_MILESTONES_DESCRIPTION", "List milestones in a GitHub repository. Use this to discover milestone numbers before assigning issues to a milestone."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_MILESTONES_USER_TITLE", "List milestones"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
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
					"state": {
						Type:        "string",
						Description: "Filter by milestone state. Defaults to 'open'.",
						Enum:        []any{"open", "closed", "all"},
					},
					"sort": {
						Type:        "string",
						Description: "Sort field. Defaults to 'due_on'.",
						Enum:        []any{"due_on", "completeness"},
					},
					"direction": {
						Type:        "string",
						Description: "Sort direction. Defaults to 'asc'.",
						Enum:        []any{"asc", "desc"},
					},
				},
				Required: []string{"owner", "repo"},
			}),
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
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			sort, err := OptionalParam[string](args, "sort")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			direction, err := OptionalParam[string](args, "direction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			opts := &github.MilestoneListOptions{
				State:     state,
				Sort:      sort,
				Direction: direction,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			milestones, resp, err := client.Issues.ListMilestones(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list milestones", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			minimalMilestones := make([]MinimalMilestone, 0, len(milestones))
			for _, milestone := range milestones {
				if milestone != nil {
					minimalMilestones = append(minimalMilestones, convertToMinimalMilestone(milestone))
				}
			}

			r, err := json.Marshal(minimalMilestones)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			result := utils.NewToolResultText(string(r))
			// Milestones are planning metadata written by collaborators
			// (trusted); confidentiality follows repo visibility.
			result = attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelRepoMetadata)
			return result, nil, nil
		},
	)
}

// MilestoneWrite creates a tool to create, update and delete milestones.
//
// Milestones are addressable by title as well as by number: agents know the
// milestone they mean by name, and nothing else in the tool surface hands them
// a number to use.
func MilestoneWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "milestone_write",
			Description: t("TOOL_MILESTONE_WRITE_DESCRIPTION", "Create, update or delete a milestone in a GitHub repository. To assign an issue to a milestone, use the 'issue_write' tool instead."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_MILESTONE_WRITE_USER_TITLE", "Write operations on milestones"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create', 'update', or 'delete'",
						Enum:        []any{milestoneMethodCreate, milestoneMethodUpdate, milestoneMethodDelete},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"milestone_number": {
						Type:        "number",
						Description: "Milestone number. Identifies the milestone for 'update' and 'delete'; takes precedence over title.",
					},
					"title": {
						Type:        "string",
						Description: "Milestone title. Required for 'create'. For 'update' and 'delete' it identifies the milestone when milestone_number is not given; for 'update' it also renames the milestone when milestone_number is given.",
					},
					"description": {
						Type:        "string",
						Description: "Milestone description. On 'update', pass an empty string to clear it; omit it to leave the current description unchanged.",
					},
					"state": {
						Type:        "string",
						Description: "Milestone state.",
						Enum:        []any{"open", "closed"},
					},
					"due_on": {
						Type:        "string",
						Description: "Due date as YYYY-MM-DD. Interpreted as the end of that day in UTC. On 'update', pass an empty string to clear the due date; omit it to leave the current one unchanged.",
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
			number, err := OptionalIntParam(args, "milestone_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			title, err := OptionalParam[string](args, "title")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			// description and due_on are the milestone fields an update can
			// clear, so they are read with a presence flag: OptionalParam
			// collapses "omitted" and "explicitly empty" into the same empty
			// string, which would turn `"description": ""` into a no-op instead
			// of a clear.
			description, descriptionSet, err := OptionalParamOK[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			dueOn, dueOnSet, err := OptionalParamOK[string](args, "due_on")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var dueOnTimestamp *github.Timestamp
			if dueOn != "" {
				parsed, err := time.Parse(milestoneDueOnLayout, dueOn)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("due_on must be a date in YYYY-MM-DD format, got '%s'", dueOn)), nil, nil
				}
				endOfDay := parsed.Add(24*time.Hour - time.Second)
				dueOnTimestamp = &github.Timestamp{Time: endOfDay}
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case milestoneMethodCreate:
				if title == "" {
					return utils.NewToolResultError("title is required for create"), nil, nil
				}

				request := &github.Milestone{
					Title:       github.Ptr(title),
					Description: ToStringPtr(description),
					State:       ToStringPtr(state),
					DueOn:       dueOnTimestamp,
				}

				milestone, resp, err := client.Issues.CreateMilestone(ctx, owner, repo, request)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create milestone", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalMilestone(milestone)

			case milestoneMethodUpdate:
				resolved, errResult, err := resolveMilestoneNumber(ctx, client, owner, repo, number, title)
				if errResult != nil || err != nil {
					return errResult, nil, err
				}

				// The payload is a raw map rather than a *github.Milestone
				// because every field on that struct is tagged omitempty: a nil
				// DueOn is dropped, so the struct cannot express the explicit
				// null that clears a due date.
				payload := map[string]any{}
				// As with releases, a title that served as the lookup key is
				// not also a rename. A title cannot be cleared either — the API
				// requires a non-empty one — so an empty title is dropped.
				if number != 0 && title != "" {
					payload["title"] = title
				}
				// state is an enum with no empty member, so it is only sent
				// when the caller named a state.
				if state != "" {
					payload["state"] = state
				}
				// An explicitly empty description or due_on is a clear and has
				// to reach the API, as "" and null respectively; an omitted one
				// stays out of the payload so the stored value survives.
				if descriptionSet {
					payload["description"] = description
				}
				if dueOnSet {
					if dueOnTimestamp != nil {
						payload["due_on"] = dueOnTimestamp
					} else {
						payload["due_on"] = nil
					}
				}

				apiURL := fmt.Sprintf("repos/%s/%s/milestones/%d", owner, repo, resolved)
				req, err := client.NewRequest(ctx, http.MethodPatch, apiURL, payload)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
				}

				milestone := &github.Milestone{}
				resp, err := client.Do(req, milestone)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update milestone", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalMilestone(milestone)

			case milestoneMethodDelete:
				resolved, errResult, err := resolveMilestoneNumber(ctx, client, owner, repo, number, title)
				if errResult != nil || err != nil {
					return errResult, nil, err
				}

				resp, err := client.Issues.DeleteMilestone(ctx, owner, repo, resolved)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete milestone", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return utils.NewToolResultText(fmt.Sprintf("milestone %d deleted successfully", resolved)), nil, nil

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create, update, delete", method)), nil, nil
			}
		},
	)
}

// resolveMilestoneNumber turns either an explicit milestone number or a title
// into a milestone number. Titles are matched case-insensitively across open
// and closed milestones; an ambiguous title is an error rather than a guess.
func resolveMilestoneNumber(ctx context.Context, client *github.Client, owner, repo string, number int, title string) (int, *mcp.CallToolResult, error) {
	if number != 0 {
		return number, nil, nil
	}
	if title == "" {
		return 0, utils.NewToolResultError("either milestone_number or title is required"), nil
	}

	opts := &github.MilestoneListOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var matches []*github.Milestone
	for {
		milestones, resp, err := client.Issues.ListMilestones(ctx, owner, repo, opts)
		if err != nil {
			return 0, ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list milestones", resp, err), nil
		}
		nextPage := 0
		if resp != nil {
			nextPage = resp.NextPage
			_ = resp.Body.Close()
		}

		for _, milestone := range milestones {
			if milestone != nil && strings.EqualFold(milestone.GetTitle(), title) {
				matches = append(matches, milestone)
			}
		}

		if nextPage == 0 {
			break
		}
		opts.Page = nextPage
	}

	switch len(matches) {
	case 0:
		return 0, utils.NewToolResultError(fmt.Sprintf("milestone '%s' not found in %s/%s", title, owner, repo)), nil
	case 1:
		return matches[0].GetNumber(), nil, nil
	default:
		numbers := make([]string, 0, len(matches))
		for _, milestone := range matches {
			numbers = append(numbers, fmt.Sprintf("%d (%s)", milestone.GetNumber(), milestone.GetState()))
		}
		return 0, utils.NewToolResultError(fmt.Sprintf("milestone title '%s' is ambiguous in %s/%s: matches %s. Pass milestone_number instead.", title, owner, repo, strings.Join(numbers, ", "))), nil
	}
}

func marshalMilestone(milestone *github.Milestone) (*mcp.CallToolResult, any, error) {
	r, err := json.Marshal(convertToMinimalMilestone(milestone))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(r)), nil, nil
}
