package github

import (
	"context"
	"fmt"

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

// MinimalRepositorySettings is the trimmed view of the repository settings an
// agent can act on. Visibility and ownership are deliberately absent: they are
// read here for context but never writable through this tool.
type MinimalRepositorySettings struct {
	FullName                 string   `json:"full_name"`
	DefaultBranch            string   `json:"default_branch"`
	Description              string   `json:"description,omitempty"`
	Homepage                 string   `json:"homepage,omitempty"`
	Topics                   []string `json:"topics,omitempty"`
	Visibility               string   `json:"visibility,omitempty"`
	Archived                 bool     `json:"archived"`
	AllowSquashMerge         bool     `json:"allow_squash_merge"`
	AllowMergeCommit         bool     `json:"allow_merge_commit"`
	AllowRebaseMerge         bool     `json:"allow_rebase_merge"`
	AllowAutoMerge           bool     `json:"allow_auto_merge"`
	AllowUpdateBranch        bool     `json:"allow_update_branch"`
	DeleteBranchOnMerge      bool     `json:"delete_branch_on_merge"`
	AllowForking             bool     `json:"allow_forking"`
	WebCommitSignoffRequired bool     `json:"web_commit_signoff_required"`
	SquashMergeCommitTitle   string   `json:"squash_merge_commit_title,omitempty"`
	SquashMergeCommitMessage string   `json:"squash_merge_commit_message,omitempty"`
	MergeCommitTitle         string   `json:"merge_commit_title,omitempty"`
	MergeCommitMessage       string   `json:"merge_commit_message,omitempty"`
	HasIssues                bool     `json:"has_issues"`
	HasProjects              bool     `json:"has_projects"`
	HasWiki                  bool     `json:"has_wiki"`
	HasDiscussions           bool     `json:"has_discussions"`
}

func convertToMinimalRepositorySettings(repo *github.Repository) MinimalRepositorySettings {
	return MinimalRepositorySettings{
		FullName:                 repo.GetFullName(),
		DefaultBranch:            repo.GetDefaultBranch(),
		Description:              sanitize.Sanitize(repo.GetDescription()),
		Homepage:                 sanitize.Sanitize(repo.GetHomepage()),
		Topics:                   repo.Topics,
		Visibility:               repo.GetVisibility(),
		Archived:                 repo.GetArchived(),
		AllowSquashMerge:         repo.GetAllowSquashMerge(),
		AllowMergeCommit:         repo.GetAllowMergeCommit(),
		AllowRebaseMerge:         repo.GetAllowRebaseMerge(),
		AllowAutoMerge:           repo.GetAllowAutoMerge(),
		AllowUpdateBranch:        repo.GetAllowUpdateBranch(),
		DeleteBranchOnMerge:      repo.GetDeleteBranchOnMerge(),
		AllowForking:             repo.GetAllowForking(),
		WebCommitSignoffRequired: repo.GetWebCommitSignoffRequired(),
		SquashMergeCommitTitle:   repo.GetSquashMergeCommitTitle(),
		SquashMergeCommitMessage: repo.GetSquashMergeCommitMessage(),
		MergeCommitTitle:         repo.GetMergeCommitTitle(),
		MergeCommitMessage:       repo.GetMergeCommitMessage(),
		HasIssues:                repo.GetHasIssues(),
		HasProjects:              repo.GetHasProjects(),
		HasWiki:                  repo.GetHasWiki(),
		HasDiscussions:           repo.GetHasDiscussions(),
	}
}

// RepositorySettingsRead creates a tool to read a repository's configuration.
func RepositorySettingsRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepoGovernance,
		mcp.Tool{
			Name:        "repository_settings_read",
			Description: t("TOOL_REPOSITORY_SETTINGS_READ_DESCRIPTION", "Read a repository's configuration: default branch, which merge methods are allowed, branch cleanup and auto-merge behaviour, and which features (issues, projects, wiki, discussions) are enabled."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_REPOSITORY_SETTINGS_READ_USER_TITLE", "Read repository settings"),
				ReadOnlyHint: true,
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
				},
				Required: []string{"owner", "repo"},
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			repository, resp, err := client.Repositories.Get(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository settings", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return marshalGovernanceResult(convertToMinimalRepositorySettings(repository), func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelRepoMetadata)
			})
		},
	)
}

// RepositorySettingsWrite creates a tool to change a repository's
// configuration.
//
// Every setting is optional and an omitted setting is left alone: the update
// sends only the fields the caller named, so a request that turns on
// auto-merge cannot accidentally switch off the wiki.
func RepositorySettingsWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepoGovernance,
		mcp.Tool{
			Name: "repository_settings_write",
			Description: t("TOOL_REPOSITORY_SETTINGS_WRITE_DESCRIPTION", "Change a repository's configuration: default branch, allowed merge methods, branch cleanup, auto-merge, and which features are enabled. "+
				"Only the settings you name are changed; every omitted setting keeps its current value. Visibility and ownership transfers are not exposed by this tool."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REPOSITORY_SETTINGS_WRITE_USER_TITLE", "Change repository settings"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
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
					"default_branch": {
						Type:        "string",
						Description: "Branch to use as the default. The branch must already exist.",
					},
					"description": {Type: "string", Description: "Short repository description."},
					"homepage":    {Type: "string", Description: "Homepage URL."},
					"topics": {
						Type:        "array",
						Description: "Repository topics. Replaces the current list; an empty array clears it.",
						Items:       &jsonschema.Schema{Type: "string"},
					},
					"allow_squash_merge":  {Type: "boolean", Description: "Allow squash merging pull requests."},
					"allow_merge_commit":  {Type: "boolean", Description: "Allow merging pull requests with a merge commit."},
					"allow_rebase_merge":  {Type: "boolean", Description: "Allow rebase merging pull requests."},
					"allow_auto_merge":    {Type: "boolean", Description: "Allow pull requests to be queued for automatic merge once requirements pass."},
					"allow_update_branch": {Type: "boolean", Description: "Offer to update a pull request branch that is behind its base."},
					"delete_branch_on_merge": {
						Type:        "boolean",
						Description: "Delete the head branch automatically after a pull request merges.",
					},
					"allow_forking":               {Type: "boolean", Description: "Allow the repository to be forked."},
					"web_commit_signoff_required": {Type: "boolean", Description: "Require a sign-off on web-based commits."},
					"squash_merge_commit_title": {
						Type:        "string",
						Description: "Default title for squash merge commits.",
						Enum:        []any{"PR_TITLE", "COMMIT_OR_PR_TITLE"},
					},
					"squash_merge_commit_message": {
						Type:        "string",
						Description: "Default message body for squash merge commits.",
						Enum:        []any{"PR_BODY", "COMMIT_MESSAGES", "BLANK"},
					},
					"merge_commit_title": {
						Type:        "string",
						Description: "Default title for merge commits.",
						Enum:        []any{"PR_TITLE", "MERGE_MESSAGE"},
					},
					"merge_commit_message": {
						Type:        "string",
						Description: "Default message body for merge commits.",
						Enum:        []any{"PR_BODY", "PR_TITLE", "BLANK"},
					},
					"has_issues":      {Type: "boolean", Description: "Enable the Issues tab."},
					"has_projects":    {Type: "boolean", Description: "Enable the Projects tab."},
					"has_wiki":        {Type: "boolean", Description: "Enable the Wiki tab."},
					"has_discussions": {Type: "boolean", Description: "Enable Discussions."},
					"archived": {
						Type:        "boolean",
						Description: "Archive the repository, making it read-only. Reversible by setting it back to false, but while archived no other write tool will succeed against this repository.",
					},
				},
				Required: []string{"owner", "repo"},
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

			update := &github.Repository{}
			changed := false

			for key, target := range map[string]**string{
				"default_branch":              &update.DefaultBranch,
				"description":                 &update.Description,
				"homepage":                    &update.Homepage,
				"squash_merge_commit_title":   &update.SquashMergeCommitTitle,
				"squash_merge_commit_message": &update.SquashMergeCommitMessage,
				"merge_commit_title":          &update.MergeCommitTitle,
				"merge_commit_message":        &update.MergeCommitMessage,
			} {
				raw, ok := args[key]
				if !ok || raw == nil {
					continue
				}
				value, ok := raw.(string)
				if !ok {
					return utils.NewToolResultError(fmt.Sprintf("%s must be a string", key)), nil, nil
				}
				*target = github.Ptr(value)
				changed = true
			}

			for key, target := range map[string]**bool{
				"allow_squash_merge":          &update.AllowSquashMerge,
				"allow_merge_commit":          &update.AllowMergeCommit,
				"allow_rebase_merge":          &update.AllowRebaseMerge,
				"allow_auto_merge":            &update.AllowAutoMerge,
				"allow_update_branch":         &update.AllowUpdateBranch,
				"delete_branch_on_merge":      &update.DeleteBranchOnMerge,
				"allow_forking":               &update.AllowForking,
				"web_commit_signoff_required": &update.WebCommitSignoffRequired,
				"has_issues":                  &update.HasIssues,
				"has_projects":                &update.HasProjects,
				"has_wiki":                    &update.HasWiki,
				"has_discussions":             &update.HasDiscussions,
				"archived":                    &update.Archived,
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
				changed = true
			}

			topics, topicsSet, err := optionalStringArray(args, "topics")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if !changed && !topicsSet {
				return utils.NewToolResultError("no settings were provided; name at least one setting to change"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			repository := (*github.Repository)(nil)
			if changed {
				edited, resp, err := client.Repositories.Edit(ctx, owner, repo, update)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update repository settings", resp, err), nil, nil
				}
				_ = resp.Body.Close()
				repository = edited
			}

			// Topics live behind their own endpoint, so they are a second call
			// rather than a field on the update.
			if topicsSet {
				updatedTopics, resp, err := client.Repositories.ReplaceAllTopics(ctx, owner, repo, topics)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update repository topics", resp, err), nil, nil
				}
				_ = resp.Body.Close()
				if repository == nil {
					fetched, resp, err := client.Repositories.Get(ctx, owner, repo)
					if err != nil {
						return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to read repository settings back", resp, err), nil, nil
					}
					_ = resp.Body.Close()
					repository = fetched
				}
				repository.Topics = updatedTopics
			}

			return marshalGovernanceResult(convertToMinimalRepositorySettings(repository), nil)
		},
	)
}
