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
	getReposActionsRunnersByOwnerByRepo     = "GET /repos/{owner}/{repo}/actions/runners"
	getReposActionsRunnerByOwnerByRepoByID  = "GET /repos/{owner}/{repo}/actions/runners/{runner_id}"
	deleteReposActionsRunnerByOwnerByRepoID = "DELETE /repos/{owner}/{repo}/actions/runners/{runner_id}"
	getOrgsActionsRunnersByOrg              = "GET /orgs/{org}/actions/runners"
	getOrgsActionsRunnerByOrgByID           = "GET /orgs/{org}/actions/runners/{runner_id}"
	deleteOrgsActionsRunnerByOrgByID        = "DELETE /orgs/{org}/actions/runners/{runner_id}"
)

func Test_ActionsRunnersRead(t *testing.T) {
	t.Parallel()

	serverTool := ActionsRunnersRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_runners_read", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	assert.ElementsMatch(t, serverTool.AcceptedScopes, []string{"repo", "read:org", "write:org", "admin:org"})

	t.Run("list defaults to repository scope", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsRunnersByOwnerByRepo, &github.Runners{
				TotalCount: 1,
				Runners:    []*github.Runner{{ID: github.Ptr(int64(1)), Name: github.Ptr("runner-1"), Status: github.Ptr("online")}},
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "list", "owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var runners []MinimalRunner
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &runners))
		require.Len(t, runners, 1)
		assert.Equal(t, "runner-1", runners[0].Name)
	})

	t.Run("list organization scope", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getOrgsActionsRunnersByOrg, &github.Runners{
				TotalCount: 1,
				Runners:    []*github.Runner{{ID: github.Ptr(int64(2)), Name: github.Ptr("org-runner"), Status: github.Ptr("offline")}},
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "list", "scope": "organization", "org": "acme"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var runners []MinimalRunner
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &runners))
		require.Len(t, runners, 1)
		assert.Equal(t, "org-runner", runners[0].Name)
	})

	t.Run("get", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsRunnerByOwnerByRepoByID, &github.Runner{ID: github.Ptr(int64(1)), Name: github.Ptr("runner-1"), Busy: github.Ptr(true)}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "get", "owner": "owner", "repo": "repo", "runner_id": 1})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var runner MinimalRunner
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &runner))
		assert.True(t, runner.Busy)
	})

	t.Run("get organization scope", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getOrgsActionsRunnerByOrgByID, &github.Runner{ID: github.Ptr(int64(2)), Name: github.Ptr("org-runner")}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "get", "scope": "organization", "org": "acme", "runner_id": 2})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "org-runner")
	})

	t.Run("get without a runner_id is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "get", "owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "runner_id")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "nope"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method")
	})

	t.Run("unknown scope is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "list", "scope": "nope"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown scope")
	})

	t.Run("scope is case-insensitive", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getOrgsActionsRunnersByOrg, &github.Runners{
				TotalCount: 1,
				Runners:    []*github.Runner{{ID: github.Ptr(int64(2)), Name: github.Ptr("org-runner")}},
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "list", "scope": "Organization", "org": "acme"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "org-runner")
	})
}

func Test_ActionsRunnerWrite(t *testing.T) {
	t.Parallel()

	serverTool := ActionsRunnerWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_runner_write", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	assert.ElementsMatch(t, serverTool.AcceptedScopes, []string{"repo", "write:org", "admin:org"})

	t.Run("remove repository runner", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteReposActionsRunnerByOwnerByRepoID, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "remove", "owner": "owner", "repo": "repo", "runner_id": 1})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "runner_removed")
	})

	t.Run("remove organization runner", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteOrgsActionsRunnerByOrgByID, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "remove", "scope": "organization", "org": "acme", "runner_id": 2})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "runner_removed")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "nope", "runner_id": 1})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method")
	})
}
