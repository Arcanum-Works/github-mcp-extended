package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	getReposActionsCachesByOwnerByRepo      = "GET /repos/{owner}/{repo}/actions/caches"
	getReposActionsCacheUsageByOwnerByRepo  = "GET /repos/{owner}/{repo}/actions/cache/usage"
	deleteReposActionsCacheByOwnerByRepoID  = "DELETE /repos/{owner}/{repo}/actions/caches/{cache_id}"
	deleteReposActionsCachesByOwnerByRepo   = "DELETE /repos/{owner}/{repo}/actions/caches"
)

func Test_ActionsCacheRead(t *testing.T) {
	t.Parallel()

	serverTool := ActionsCacheRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_cache_read", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	assert.ElementsMatch(t, serverTool.AcceptedScopes, []string{"repo"})

	t.Run("list", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsCachesByOwnerByRepo, &github.ActionsCacheList{
				TotalCount:    1,
				ActionsCaches: []*github.ActionsCache{{ID: github.Ptr(int64(1)), Key: github.Ptr("npm-cache"), SizeInBytes: github.Ptr(int64(1024))}},
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "list", "owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var caches []MinimalActionsCache
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &caches))
		require.Len(t, caches, 1)
		assert.Equal(t, "npm-cache", caches[0].Key)
	})

	t.Run("usage", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsCacheUsageByOwnerByRepo, &github.ActionsCacheUsage{
				FullName: "owner/repo", ActiveCachesCount: 3, ActiveCachesSizeInBytes: 2048,
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "usage", "owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var usage MinimalActionsCacheUsage
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &usage))
		assert.Equal(t, 3, usage.ActiveCachesCount)
		assert.Equal(t, int64(2048), usage.ActiveCachesBytes)
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "nope", "owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method")
	})
}

func Test_ActionsCacheWrite(t *testing.T) {
	t.Parallel()

	serverTool := ActionsCacheWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_cache_write", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	assert.ElementsMatch(t, serverTool.AcceptedScopes, []string{"repo"})

	t.Run("delete_by_id", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteReposActionsCacheByOwnerByRepoID, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "delete_by_id", "owner": "owner", "repo": "repo", "cache_id": 1})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "cache_deleted")
	})

	t.Run("delete_by_key", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteReposActionsCachesByOwnerByRepo, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "npm-cache", r.URL.Query().Get("key"))
				w.WriteHeader(http.StatusOK)
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "delete_by_key", "owner": "owner", "repo": "repo", "key": "npm-cache"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "cache_deleted")
	})

	t.Run("delete_by_id without a cache_id is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "delete_by_id", "owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "cache_id")
	})
}
