package github

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	cacheMethodList  = "list"
	cacheMethodUsage = "usage"

	cacheWriteMethodDeleteByID  = "delete_by_id"
	cacheWriteMethodDeleteByKey = "delete_by_key"
)

// MinimalActionsCache is the trimmed output type for an Actions cache entry.
type MinimalActionsCache struct {
	ID             int64  `json:"id"`
	Key            string `json:"key,omitempty"`
	Ref            string `json:"ref,omitempty"`
	SizeInBytes    int64  `json:"size_in_bytes,omitempty"`
	LastAccessedAt string `json:"last_accessed_at,omitempty"`
}

// MinimalActionsCacheUsage is the trimmed output type for a repository's
// Actions cache storage usage.
type MinimalActionsCacheUsage struct {
	FullName          string `json:"full_name"`
	ActiveCachesCount int    `json:"active_caches_count"`
	ActiveCachesBytes int64  `json:"active_caches_size_in_bytes"`
}

func convertToMinimalActionsCacheUsage(u *github.ActionsCacheUsage) MinimalActionsCacheUsage {
	return MinimalActionsCacheUsage{
		FullName:          u.GetFullName(),
		ActiveCachesCount: u.GetActiveCachesCount(),
		ActiveCachesBytes: u.GetActiveCachesSizeInBytes(),
	}
}

func convertToMinimalActionsCache(c *github.ActionsCache) MinimalActionsCache {
	m := MinimalActionsCache{
		ID:          c.GetID(),
		Key:         sanitize.Sanitize(c.GetKey()),
		Ref:         c.GetRef(),
		SizeInBytes: c.GetSizeInBytes(),
	}
	if c.LastAccessedAt != nil {
		m.LastAccessedAt = c.LastAccessedAt.Format(time.RFC3339)
	}
	return m
}

// ActionsCacheRead creates a tool to inspect a repository's Actions caches.
func ActionsCacheRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name:        "actions_cache_read",
			Description: t("TOOL_ACTIONS_CACHE_READ_DESCRIPTION", "Read a repository's GitHub Actions caches: list them, optionally filtered by key or ref, or get overall cache storage usage."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ACTIONS_CACHE_READ_USER_TITLE", "Read Actions caches"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list: caches stored for the repository\n" +
							"- usage: total cache count and size for the repository",
						Enum: []any{cacheMethodList, cacheMethodUsage},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"key": {
						Type:        "string",
						Description: "Only list caches with this key. Used by 'list'.",
					},
					"ref": {
						Type:        "string",
						Description: "Only list caches for this Git ref, e.g. 'refs/heads/main'. Used by 'list'.",
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
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case cacheMethodList:
				key, err := OptionalParam[string](args, "key")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				ref, err := OptionalParam[string](args, "ref")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				opts := &github.ActionsCacheListOptions{ListOptions: github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}}
				if key != "" {
					opts.Key = &key
				}
				if ref != "" {
					opts.Ref = &ref
				}

				list, resp, err := client.Actions.ListCaches(ctx, owner, repo, opts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list Actions caches", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalActionsCache, 0, len(list.ActionsCaches))
				for _, c := range list.ActionsCaches {
					if c != nil {
						minimal = append(minimal, convertToMinimalActionsCache(c))
					}
				}
				return marshalGovernanceResult(minimal, nil)

			case cacheMethodUsage:
				usage, resp, err := client.Actions.GetCacheUsageForRepo(ctx, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get Actions cache usage", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalActionsCacheUsage(usage), nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list, usage", method)), nil, nil
			}
		},
	)
}

// ActionsCacheWrite creates a tool to delete a repository's Actions caches.
func ActionsCacheWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataActionsAdmin,
		mcp.Tool{
			Name: "actions_cache_write",
			Description: t("TOOL_ACTIONS_CACHE_WRITE_DESCRIPTION", "Delete a repository's GitHub Actions cache entries, either by cache ID or by key."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ACTIONS_CACHE_WRITE_USER_TITLE", "Manage Actions caches"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The operation to perform:\n" +
							"- delete_by_id: delete one cache entry by its ID\n" +
							"- delete_by_key: delete all cache entries matching a key, optionally scoped to a ref",
						Enum: []any{cacheWriteMethodDeleteByID, cacheWriteMethodDeleteByKey},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"cache_id": {
						Type:        "integer",
						Description: "Cache ID. Required for 'delete_by_id'.",
					},
					"key": {
						Type:        "string",
						Description: "Cache key. Required for 'delete_by_key'.",
					},
					"ref": {
						Type:        "string",
						Description: "Only delete cache entries for this Git ref, e.g. 'refs/heads/main'. Used by 'delete_by_key'.",
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
			case cacheWriteMethodDeleteByID:
				cacheID, err := RequiredInt(args, "cache_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				resp, err := client.Actions.DeleteCachesByID(ctx, owner, repo, int64(cacheID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to delete cache %d", cacheID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(map[string]any{"result": "cache_deleted", "cache_id": cacheID}, nil)

			case cacheWriteMethodDeleteByKey:
				key, err := RequiredParam[string](args, "key")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				ref, err := OptionalParam[string](args, "ref")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				var refPtr *string
				if ref != "" {
					refPtr = &ref
				}
				resp, err := client.Actions.DeleteCachesByKey(ctx, owner, repo, key, refPtr)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to delete caches with key '%s'", key), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(map[string]any{"result": "cache_deleted", "key": key}, nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: delete_by_id, delete_by_key", method)), nil, nil
			}
		},
	)
}
