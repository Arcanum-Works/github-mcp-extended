package github

import (
	"context"
	"encoding/json"
	"fmt"
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
	refMethodCreate = "create"
	refMethodUpdate = "update"
	refMethodDelete = "delete"

	tagMethodCreate = "create"
	tagMethodDelete = "delete"
)

// MinimalRef is the trimmed output type for git references.
type MinimalRef struct {
	Ref        string `json:"ref"`
	SHA        string `json:"sha"`
	ObjectType string `json:"object_type,omitempty"`
}

func convertToMinimalRef(ref *github.Reference) MinimalRef {
	return MinimalRef{
		Ref:        ref.GetRef(),
		SHA:        ref.GetObject().GetSHA(),
		ObjectType: ref.GetObject().GetType(),
	}
}

// MinimalCommitsComparison is the trimmed output type for a two-ref comparison.
type MinimalCommitsComparison struct {
	Status       string          `json:"status"`
	AheadBy      int             `json:"ahead_by"`
	BehindBy     int             `json:"behind_by"`
	TotalCommits int             `json:"total_commits"`
	HTMLURL      string          `json:"html_url,omitempty"`
	MergeBaseSHA string          `json:"merge_base_sha,omitempty"`
	Commits      []MinimalCommit `json:"commits,omitempty"`
	Files        []MinimalPRFile `json:"files,omitempty"`
}

// GitRefWrite creates a tool to create, move and delete git references.
func GitRefWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGit,
		mcp.Tool{
			Name:        "git_ref_write",
			Description: t("TOOL_GIT_REF_WRITE_DESCRIPTION", "Create, move or delete a git reference (branch, tag or any other ref) by its full ref name. For the common cases prefer 'create_branch' and 'tag_write', which resolve the target commit for you."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_GIT_REF_WRITE_USER_TITLE", "Write operations on git references"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create', 'update' (move an existing ref to a different commit), or 'delete'",
						Enum:        []any{refMethodCreate, refMethodUpdate, refMethodDelete},
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
						Description: "Fully qualified ref name, e.g. 'refs/heads/my-branch' or 'refs/tags/v1.2.3'.",
					},
					"sha": {
						Type:        "string",
						Description: "Commit SHA the ref points at. Required for 'create' and 'update'.",
					},
					"force": {
						Type:        "boolean",
						Description: "Allow a non-fast-forward update, discarding commits that only the old ref pointed at. Used by 'update'.",
					},
				},
				Required: []string{"method", "owner", "repo", "ref"},
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
			ref, err := RequiredParam[string](args, "ref")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			sha, err := OptionalParam[string](args, "sha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			force, err := OptionalParam[bool](args, "force")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case refMethodCreate:
				if sha == "" {
					return utils.NewToolResultError("sha is required for create"), nil, nil
				}

				created, resp, err := client.Git.CreateRef(ctx, owner, repo, github.CreateRef{Ref: ref, SHA: sha})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create reference", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalRef(created)

			case refMethodUpdate:
				if sha == "" {
					return utils.NewToolResultError("sha is required for update"), nil, nil
				}

				updated, resp, err := client.Git.UpdateRef(ctx, owner, repo, ref, github.UpdateRef{SHA: sha, Force: github.Ptr(force)})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update reference", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalRef(updated)

			case refMethodDelete:
				resp, err := client.Git.DeleteRef(ctx, owner, repo, ref)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete reference", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return utils.NewToolResultText(fmt.Sprintf("reference '%s' deleted successfully", ref)), nil, nil

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create, update, delete", method)), nil, nil
			}
		},
	)
}

// TagWrite creates a tool to create and delete tags by name.
//
// This is the ergonomic counterpart to git_ref_write: it takes a tag name
// rather than a full ref, resolves a branch name or the default branch to a
// commit SHA, and creates a tag object first when a message is supplied.
func TagWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "tag_write",
			Description: t("TOOL_TAG_WRITE_DESCRIPTION", "Create or delete a tag in a GitHub repository. Supplying a message creates an annotated tag; without one the tag is lightweight. Creating a tag does not create a release - use 'release_write' for that."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_TAG_WRITE_USER_TITLE", "Write operations on tags"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create' or 'delete'",
						Enum:        []any{tagMethodCreate, tagMethodDelete},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"tag": {
						Type:        "string",
						Description: "Tag name, without the 'refs/tags/' prefix (e.g. 'v1.2.3').",
					},
					"from_ref": {
						Type:        "string",
						Description: "Commit SHA, branch name or tag name the new tag points at. Defaults to the head of the repository's default branch. Used by 'create'.",
					},
					"message": {
						Type:        "string",
						Description: "Annotation message. Supplying it creates an annotated tag object instead of a lightweight tag. Used by 'create'.",
					},
				},
				Required: []string{"method", "owner", "repo", "tag"},
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
			tag, err := RequiredParam[string](args, "tag")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			tag = strings.TrimPrefix(tag, "refs/tags/")

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case tagMethodCreate:
				fromRef, err := OptionalParam[string](args, "from_ref")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				message, err := OptionalParam[string](args, "message")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				sha, errResult, err := resolveCommitSHA(ctx, client, owner, repo, fromRef)
				if errResult != nil || err != nil {
					return errResult, nil, err
				}

				// An annotated tag is a real git object that the ref then
				// points at, so it has to exist before the ref is created.
				target := sha
				if message != "" {
					tagObject, resp, err := client.Git.CreateTag(ctx, owner, repo, github.CreateTag{
						Tag:     tag,
						Message: message,
						Object:  sha,
						Type:    "commit",
					})
					if err != nil {
						return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create tag object", resp, err), nil, nil
					}
					defer func() { _ = resp.Body.Close() }()
					target = tagObject.GetSHA()
				}

				created, resp, err := client.Git.CreateRef(ctx, owner, repo, github.CreateRef{
					Ref: "refs/tags/" + tag,
					SHA: target,
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create tag reference", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalRef(created)

			case tagMethodDelete:
				resp, err := client.Git.DeleteRef(ctx, owner, repo, "refs/tags/"+tag)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete tag", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return utils.NewToolResultText(fmt.Sprintf("tag '%s' deleted successfully", tag)), nil, nil

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create, delete", method)), nil, nil
			}
		},
	)
}

// CompareCommits creates a tool to compare two refs.
func CompareCommits(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "compare_commits",
			Description: t("TOOL_COMPARE_COMMITS_DESCRIPTION", "Compare two commits, branches or tags in a GitHub repository, returning how far apart they are and what changed between them. Use this to diff arbitrary refs; for the diff of an open pull request use 'pull_request_read' instead."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_COMPARE_COMMITS_USER_TITLE", "Compare two refs"),
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
					"base": {
						Type:        "string",
						Description: "The ref to compare from: a commit SHA, branch name or tag name. To compare across forks, use 'owner:branch'.",
					},
					"head": {
						Type:        "string",
						Description: "The ref to compare to: a commit SHA, branch name or tag name. To compare across forks, use 'owner:branch'.",
					},
					"detail": {
						Type:        "string",
						Description: "How much per-file detail to include: 'none' omits the file list, 'stats' (default) lists changed files with counts, 'full_patch' includes the diff for each file.",
						Enum:        []any{"none", "stats", "full_patch"},
					},
				},
				Required: []string{"owner", "repo", "base", "head"},
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
			base, err := RequiredParam[string](args, "base")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			head, err := RequiredParam[string](args, "head")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			detailArg, err := OptionalParam[string](args, "detail")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			detail, err := parseCommitDetail(detailArg)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			comparison, resp, err := client.Repositories.CompareCommits(ctx, owner, repo, base, head, &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to compare commits", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			minimal := MinimalCommitsComparison{
				Status:       comparison.GetStatus(),
				AheadBy:      comparison.GetAheadBy(),
				BehindBy:     comparison.GetBehindBy(),
				TotalCommits: comparison.GetTotalCommits(),
				HTMLURL:      comparison.GetHTMLURL(),
				MergeBaseSHA: comparison.GetMergeBaseCommit().GetSHA(),
			}

			minimal.Commits = make([]MinimalCommit, 0, len(comparison.Commits))
			for _, commit := range comparison.Commits {
				if commit != nil {
					// Per-commit files would repeat the comparison-level file
					// list once per commit, so the detail level only governs
					// the latter.
					minimal.Commits = append(minimal.Commits, convertToMinimalCommit(commit, commitDetailNone))
				}
			}

			if detail != commitDetailNone {
				files := convertToMinimalPRFiles(comparison.Files)
				if detail != commitDetailFullPatch {
					for i := range files {
						files[i].Patch = ""
					}
				}
				minimal.Files = files
			}

			r, err := json.Marshal(minimal)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			result := utils.NewToolResultText(string(r))
			// A comparison carries commit messages and diff text, which are
			// attacker-influenceable on a public repo.
			result = attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelCommitContents)
			return result, nil, nil
		},
	)
}

// resolveCommitSHA turns a commit SHA, branch name or tag name into a commit
// SHA, defaulting to the head of the repository's default branch. It returns a
// tool-level error result when the ref cannot be resolved.
func resolveCommitSHA(ctx context.Context, client *github.Client, owner, repo, ref string) (string, *mcp.CallToolResult, error) {
	if ref == "" {
		repository, resp, err := client.Repositories.Get(ctx, owner, repo)
		if err != nil {
			return "", ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository", resp, err), nil
		}
		defer func() { _ = resp.Body.Close() }()
		ref = repository.GetDefaultBranch()
	}

	// Commits.Get resolves SHAs, branch names and tag names alike, so a single
	// call covers every form from_ref can take.
	commit, resp, err := client.Repositories.GetCommit(ctx, owner, repo, ref, &github.ListOptions{PerPage: 1})
	if err != nil {
		return "", ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to resolve '%s' to a commit", ref), resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	return commit.GetSHA(), nil, nil
}

func marshalRef(ref *github.Reference) (*mcp.CallToolResult, any, error) {
	r, err := json.Marshal(convertToMinimalRef(ref))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(r)), nil, nil
}
