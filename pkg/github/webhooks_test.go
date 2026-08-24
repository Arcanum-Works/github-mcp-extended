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
	getReposHooksByOwnerByRepo             = "GET /repos/{owner}/{repo}/hooks"
	getReposHooksByOwnerByRepoByID         = "GET /repos/{owner}/{repo}/hooks/{hook_id}"
	getReposHooksConfigByOwnerByRepoByID   = "GET /repos/{owner}/{repo}/hooks/{hook_id}/config"
	getReposHooksDeliveriesByID            = "GET /repos/{owner}/{repo}/hooks/{hook_id}/deliveries"
	getReposHooksDeliveryByIDs             = "GET /repos/{owner}/{repo}/hooks/{hook_id}/deliveries/{delivery_id}"
	postReposHooksByOwnerByRepo            = "POST /repos/{owner}/{repo}/hooks"
	patchReposHooksByOwnerByRepoByID       = "PATCH /repos/{owner}/{repo}/hooks/{hook_id}"
	deleteReposHooksByOwnerByRepoByID      = "DELETE /repos/{owner}/{repo}/hooks/{hook_id}"
	postReposHooksPingByOwnerByRepoByID    = "POST /repos/{owner}/{repo}/hooks/{hook_id}/pings"
	postReposHooksRedeliverByOwnerByRepoID = "POST /repos/{owner}/{repo}/hooks/{hook_id}/deliveries/{delivery_id}/attempts"
)

func Test_WebhooksRead(t *testing.T) {
	t.Parallel()

	serverTool := WebhooksRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "webhooks_read", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)

	hookWithSecret := &github.Hook{
		ID:     github.Ptr(int64(42)),
		Active: github.Ptr(true),
		Events: []string{"push"},
		Config: &github.HookConfig{
			URL:    github.Ptr("https://example.com/hook"),
			Secret: github.Ptr("super-secret-value"),
		},
	}

	t.Run("list never surfaces the config secret", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposHooksByOwnerByRepo, []*github.Hook{hookWithSecret}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "list", "owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		text := getTextResult(t, result).Text
		assert.NotContains(t, text, "super-secret-value")
		assert.NotContains(t, text, "secret")

		var hooks []MinimalWebhook
		require.NoError(t, json.Unmarshal([]byte(text), &hooks))
		require.Len(t, hooks, 1)
		assert.Equal(t, int64(42), hooks[0].ID)
		assert.Equal(t, "https://example.com/hook", hooks[0].URL)
	})

	t.Run("get", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposHooksByOwnerByRepoByID, hookWithSecret),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "get", "owner": "owner", "repo": "repo", "hook_id": 42})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.NotContains(t, getTextResult(t, result).Text, "super-secret-value")
	})

	t.Run("get_config never surfaces the secret", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposHooksConfigByOwnerByRepoByID, &github.HookConfig{
				URL:    github.Ptr("https://example.com/hook"),
				Secret: github.Ptr("super-secret-value"),
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "get_config", "owner": "owner", "repo": "repo", "hook_id": 42})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		text := getTextResult(t, result).Text
		assert.NotContains(t, text, "super-secret-value")
		assert.NotContains(t, text, "secret")
		assert.Contains(t, text, "https://example.com/hook")
	})

	t.Run("list_deliveries", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposHooksDeliveriesByID, []*github.HookDelivery{
				{ID: github.Ptr(int64(1)), Event: github.Ptr("push"), Status: github.Ptr("OK"), StatusCode: github.Ptr(200)},
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "list_deliveries", "owner": "owner", "repo": "repo", "hook_id": 42})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var deliveries []MinimalHookDelivery
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &deliveries))
		require.Len(t, deliveries, 1)
		assert.Equal(t, "OK", deliveries[0].Status)
	})

	t.Run("get_delivery", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposHooksDeliveryByIDs, &github.HookDelivery{ID: github.Ptr(int64(9)), Event: github.Ptr("push"), StatusCode: github.Ptr(200)}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "get_delivery", "owner": "owner", "repo": "repo", "hook_id": 42, "delivery_id": 9})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var delivery MinimalHookDelivery
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &delivery))
		assert.Equal(t, int64(9), delivery.ID)
	})

	t.Run("get without a hook_id is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "get", "owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "hook_id")
	})
}

func Test_WebhookWrite(t *testing.T) {
	t.Parallel()

	serverTool := WebhookWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "webhook_write", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)

	t.Run("create sends the secret but never returns it", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposHooksByOwnerByRepo, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				config, ok := payload["config"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "shh-secret", config["secret"])

				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(MustMarshal(&github.Hook{
					ID:     github.Ptr(int64(1)),
					Active: github.Ptr(true),
					Events: []string{"push"},
					Config: &github.HookConfig{URL: github.Ptr("https://example.com/hook"), Secret: github.Ptr("shh-secret")},
				}))
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create",
			"owner":  "owner",
			"repo":   "repo",
			"url":    "https://example.com/hook",
			"secret": "shh-secret",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.NotContains(t, getTextResult(t, result).Text, "shh-secret")
	})

	t.Run("update", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(patchReposHooksByOwnerByRepoByID, &github.Hook{
				ID:     github.Ptr(int64(1)),
				Active: github.Ptr(false),
				Config: &github.HookConfig{URL: github.Ptr("https://example.com/hook")},
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "update", "owner": "owner", "repo": "repo", "hook_id": 1, "active": false})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var hook MinimalWebhook
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &hook))
		assert.False(t, hook.Active)
	})

	t.Run("delete", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteReposHooksByOwnerByRepoByID, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "delete", "owner": "owner", "repo": "repo", "hook_id": 1})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "webhook_deleted")
	})

	t.Run("ping", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposHooksPingByOwnerByRepoByID, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "ping", "owner": "owner", "repo": "repo", "hook_id": 1})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "ping_sent")
	})

	t.Run("redeliver", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(postReposHooksRedeliverByOwnerByRepoID, &github.HookDelivery{ID: github.Ptr(int64(9))}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "redeliver", "owner": "owner", "repo": "repo", "hook_id": 1, "delivery_id": 9})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "redelivery_requested")
	})

	t.Run("create without a url is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "create", "owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "url")
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
