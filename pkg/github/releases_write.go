package github

import (
	"context"
	"encoding/json"
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
	releaseMethodCreate        = "create"
	releaseMethodUpdate        = "update"
	releaseMethodDelete        = "delete"
	releaseMethodGenerateNotes = "generate_notes"
)

// MinimalReleaseNotes is the trimmed output type for generated release notes.
type MinimalReleaseNotes struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// ReleaseWrite creates a tool to create, update and delete GitHub releases, and
// to generate release notes without publishing anything.
//
// Releases are identified either by release_id or by tag_name: agents usually
// know the tag they just pushed, not the numeric release ID, so resolving the
// tag here saves a round trip through get_release_by_tag.
func ReleaseWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "release_write",
			Description: t("TOOL_RELEASE_WRITE_DESCRIPTION", "Create, update or delete a release in a GitHub repository, or generate release notes for a tag without publishing. Creating a release also creates its tag if it does not already exist."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_RELEASE_WRITE_USER_TITLE", "Write operations on releases"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create', 'update', 'delete', or 'generate_notes'",
						Enum: []any{
							releaseMethodCreate,
							releaseMethodUpdate,
							releaseMethodDelete,
							releaseMethodGenerateNotes,
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
					"tag_name": {
						Type:        "string",
						Description: "Tag the release points at. Required for 'create' and 'generate_notes'. For 'update' and 'delete' it identifies the release when release_id is not given; for 'update' it also retags the release when release_id is given.",
					},
					"release_id": {
						Type:        "number",
						Description: "Numeric release ID. Identifies the release for 'update' and 'delete'; takes precedence over tag_name.",
					},
					"target_commitish": {
						Type:        "string",
						Description: "Branch or commit SHA the tag is created from when it does not exist yet. Defaults to the repository's default branch.",
					},
					"name": {
						Type:        "string",
						Description: "Release title. On 'update', pass an empty string to clear it; omit it to leave the current title unchanged.",
					},
					"body": {
						Type:        "string",
						Description: "Release notes in markdown. On 'update', pass an empty string to clear them; omit it to leave the current notes unchanged.",
					},
					"draft": {
						Type:        "boolean",
						Description: "Whether the release is a draft (unpublished).",
					},
					"prerelease": {
						Type:        "boolean",
						Description: "Whether the release is a prerelease.",
					},
					"make_latest": {
						Type:        "string",
						Description: "Whether to mark this release as the repository's latest. 'legacy' defers to GitHub's date-and-semver heuristic.",
						Enum:        []any{"true", "false", "legacy"},
					},
					"discussion_category_name": {
						Type:        "string",
						Description: "Create a discussion of this category for the release. The category must already exist in the repository.",
					},
					"generate_release_notes": {
						Type:        "boolean",
						Description: "Auto-generate release notes on 'create'. When body is also given, the generated notes are appended to it.",
					},
					"previous_tag_name": {
						Type:        "string",
						Description: "Tag to generate notes against for 'generate_notes'. Defaults to the previous release's tag.",
					},
					"configuration_file_path": {
						Type:        "string",
						Description: "Path to a release notes configuration file for 'generate_notes' (defaults to .github/release.yml).",
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

			tagName, err := OptionalParam[string](args, "tag_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			releaseID, err := OptionalIntParam(args, "release_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			targetCommitish, err := OptionalParam[string](args, "target_commitish")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			// name and body are the only release fields GitHub lets an update
			// clear, so they are read with a presence flag: OptionalParam
			// collapses "omitted" and "explicitly empty" into the same empty
			// string, which would turn `"body": ""` into a no-op instead of a
			// clear.
			name, nameSet, err := OptionalParamOK[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			body, bodySet, err := OptionalParamOK[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			makeLatest, err := OptionalParam[string](args, "make_latest")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			discussionCategory, err := OptionalParam[string](args, "discussion_category_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// draft, prerelease and generate_release_notes are tri-state on
			// update: an absent flag must leave the stored value alone rather
			// than reset it to false.
			draft, draftSet, err := optionalBoolPointer(args, "draft")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			prerelease, prereleaseSet, err := optionalBoolPointer(args, "prerelease")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			generateNotes, generateNotesSet, err := optionalBoolPointer(args, "generate_release_notes")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case releaseMethodCreate:
				if tagName == "" {
					return utils.NewToolResultError("tag_name is required for create"), nil, nil
				}

				req := github.CreateReleaseRequest{
					TagName:                tagName,
					TargetCommitish:        ToStringPtr(targetCommitish),
					Name:                   ToStringPtr(name),
					Body:                   ToStringPtr(body),
					MakeLatest:             ToStringPtr(makeLatest),
					DiscussionCategoryName: ToStringPtr(discussionCategory),
				}
				if draftSet {
					req.Draft = draft
				}
				if prereleaseSet {
					req.Prerelease = prerelease
				}
				if generateNotesSet {
					req.GenerateReleaseNotes = generateNotes
				}

				release, resp, err := client.Repositories.CreateRelease(ctx, owner, repo, req)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create release", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalRelease(release)

			case releaseMethodUpdate:
				id, errResult, err := resolveReleaseID(ctx, client, owner, repo, int64(releaseID), tagName)
				if errResult != nil || err != nil {
					return errResult, nil, err
				}

				req := github.UpdateReleaseRequest{
					TargetCommitish:        ToStringPtr(targetCommitish),
					MakeLatest:             ToStringPtr(makeLatest),
					DiscussionCategoryName: ToStringPtr(discussionCategory),
				}
				// An explicitly empty name or body is a clear and has to reach
				// the API; an omitted one stays out of the request so the
				// stored value survives. target_commitish, make_latest and
				// discussion_category_name have no empty form the API accepts,
				// so they stay omit-when-empty.
				if nameSet {
					req.Name = github.Ptr(name)
				}
				if bodySet {
					req.Body = github.Ptr(body)
				}
				// Retagging is only meaningful when the caller addressed the
				// release by ID; when tag_name was the lookup key, resending it
				// would be a no-op.
				if releaseID != 0 {
					req.TagName = ToStringPtr(tagName)
				}
				if draftSet {
					req.Draft = draft
				}
				if prereleaseSet {
					req.Prerelease = prerelease
				}

				release, resp, err := client.Repositories.UpdateRelease(ctx, owner, repo, id, req)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update release", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalRelease(release)

			case releaseMethodDelete:
				id, errResult, err := resolveReleaseID(ctx, client, owner, repo, int64(releaseID), tagName)
				if errResult != nil || err != nil {
					return errResult, nil, err
				}

				resp, err := client.Repositories.DeleteRelease(ctx, owner, repo, id)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete release", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				// Deleting a release leaves its git tag in place; say so rather
				// than let the agent assume the tag is gone too.
				return utils.NewToolResultText(fmt.Sprintf("release %d deleted successfully. Its git tag still exists; use tag_write to remove it.", id)), nil, nil

			case releaseMethodGenerateNotes:
				if tagName == "" {
					return utils.NewToolResultError("tag_name is required for generate_notes"), nil, nil
				}

				previousTag, err := OptionalParam[string](args, "previous_tag_name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				configPath, err := OptionalParam[string](args, "configuration_file_path")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				req := github.GenerateNotesRequest{
					TagName:               tagName,
					TargetCommitish:       ToStringPtr(targetCommitish),
					PreviousTagName:       ToStringPtr(previousTag),
					ConfigurationFilePath: ToStringPtr(configPath),
				}

				notes, resp, err := client.Repositories.GenerateReleaseNotes(ctx, owner, repo, req)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to generate release notes", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := MinimalReleaseNotes{
					Name: sanitize.Sanitize(notes.Name),
					Body: sanitize.Sanitize(notes.Body),
				}
				r, err := json.Marshal(minimal)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
				}
				return utils.NewToolResultText(string(r)), nil, nil

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create, update, delete, generate_notes", method)), nil, nil
			}
		},
	)
}

// resolveReleaseID turns either an explicit release ID or a tag name into a
// release ID. It returns a tool-level error result when neither was supplied or
// when the tag has no release, so callers can return it verbatim.
func resolveReleaseID(ctx context.Context, client *github.Client, owner, repo string, releaseID int64, tagName string) (int64, *mcp.CallToolResult, error) {
	if releaseID != 0 {
		return releaseID, nil, nil
	}
	if tagName == "" {
		return 0, utils.NewToolResultError("either release_id or tag_name is required"), nil
	}

	release, resp, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tagName)
	if err != nil {
		return 0, ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to find a release for tag '%s'", tagName), resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	return release.GetID(), nil, nil
}

func marshalRelease(release *github.RepositoryRelease) (*mcp.CallToolResult, any, error) {
	r, err := json.Marshal(convertToMinimalRelease(release))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(r)), nil, nil
}

// optionalBoolPointer reads an optional boolean argument, reporting whether it
// was present at all. Write tools need that distinction to leave unspecified
// fields untouched instead of overwriting them with the zero value.
func optionalBoolPointer(args map[string]any, p string) (*bool, bool, error) {
	if v, ok := args[p]; !ok || v == nil {
		return nil, false, nil
	}
	v, err := OptionalParam[bool](args, p)
	if err != nil {
		return nil, false, err
	}
	return &v, true, nil
}
