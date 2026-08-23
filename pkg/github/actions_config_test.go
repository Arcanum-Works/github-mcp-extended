package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	getReposActionsPermissionsByOwnerByRepo         = "GET /repos/{owner}/{repo}/actions/permissions"
	putReposActionsPermissionsByOwnerByRepo         = "PUT /repos/{owner}/{repo}/actions/permissions"
	getReposActionsSelectedActionsByOwnerByRepo     = "GET /repos/{owner}/{repo}/actions/permissions/selected-actions"
	putReposActionsSelectedActionsByOwnerByRepo     = "PUT /repos/{owner}/{repo}/actions/permissions/selected-actions"
	getReposActionsWorkflowPermissionsByOwnerByRepo = "GET /repos/{owner}/{repo}/actions/permissions/workflow"
	putReposActionsWorkflowPermissionsByOwnerByRepo = "PUT /repos/{owner}/{repo}/actions/permissions/workflow"
	putReposActionsWorkflowEnableByOwnerByRepoByID  = "PUT /repos/{owner}/{repo}/actions/workflows/{workflow_id}/enable"
	putReposActionsWorkflowDisableByOwnerByRepoByID = "PUT /repos/{owner}/{repo}/actions/workflows/{workflow_id}/disable"
)

func Test_ActionsConfigRead(t *testing.T) {
	t.Parallel()

	serverTool := ActionsConfigRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_config_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "actions_config_read tool should be read-only")

	t.Run("an allow-list is fetched only when the policy is 'selected'", func(t *testing.T) {
		allowListFetched := false
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsPermissionsByOwnerByRepo, `{"enabled":true,"allowed_actions":"selected"}`),
			WithRequestMatchHandler(getReposActionsSelectedActionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					allowListFetched = true
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"github_owned_allowed":true,"verified_allowed":false,"patterns_allowed":["octo-org/*"]}`))
				}),
			),
			WithRequestMatch(getReposActionsWorkflowPermissionsByOwnerByRepo,
				`{"default_workflow_permissions":"read","can_approve_pull_request_reviews":false}`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "get_permissions",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.True(t, allowListFetched)

		var config MinimalActionsConfig
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &config))
		assert.True(t, config.Enabled)
		assert.Equal(t, "selected", config.AllowedActions)
		assert.Equal(t, []string{"octo-org/*"}, config.PatternsAllowed)
		assert.Equal(t, "read", config.DefaultWorkflowPermissions)
	})

	t.Run("no allow-list is fetched when every action is allowed", func(t *testing.T) {
		allowListFetched := false
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsPermissionsByOwnerByRepo, `{"enabled":true,"allowed_actions":"all"}`),
			WithRequestMatchHandler(getReposActionsSelectedActionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					allowListFetched = true
					// Asking for the allow-list under an "all" policy is a 409.
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"message":"Conflict"}`))
				}),
			),
			WithRequestMatch(getReposActionsWorkflowPermissionsByOwnerByRepo,
				`{"default_workflow_permissions":"write","can_approve_pull_request_reviews":true}`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "get_permissions",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.False(t, allowListFetched, "the allow-list endpoint must not be called under an 'all' policy")

		var config MinimalActionsConfig
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &config))
		assert.Equal(t, "all", config.AllowedActions)
		assert.True(t, config.CanApprovePullRequestReviews)
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "get_runners",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method: get_runners")
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposActionsPermissionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Must have admin rights"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "get_permissions",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to get Actions permissions")
	})
}

func Test_ActionsConfigWrite(t *testing.T) {
	t.Parallel()

	serverTool := ActionsConfigWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "actions_config_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "actions_config_write tool should not be read-only")

	t.Run("setting allowed_actions implies Actions is enabled", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(putReposActionsPermissionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "selected", payload["allowed_actions"])
					// The API rejects an allowed_actions change that does not
					// also say Actions is on.
					assert.Equal(t, true, payload["enabled"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"enabled":true,"allowed_actions":"selected"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":          "set_permissions",
			"owner":           "owner",
			"repo":            "repo",
			"allowed_actions": "selected",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("set_permissions with nothing to set is refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "set_permissions",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "provide enabled, allowed_actions, or both")
	})

	t.Run("the allow-list keeps the flags it was not asked to change", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposActionsSelectedActionsByOwnerByRepo,
				`{"github_owned_allowed":true,"verified_allowed":true,"patterns_allowed":["old/*"]}`),
			WithRequestMatchHandler(putReposActionsSelectedActionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					// Only the patterns were named; the two flags must survive.
					assert.Equal(t, true, payload["github_owned_allowed"])
					assert.Equal(t, true, payload["verified_allowed"])
					assert.Equal(t, []any{"octo-org/*"}, payload["patterns_allowed"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"github_owned_allowed":true,"verified_allowed":true,"patterns_allowed":["octo-org/*"]}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":           "set_allowed_actions",
			"owner":            "owner",
			"repo":             "repo",
			"patterns_allowed": []any{"octo-org/*"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("a failed allow-list read aborts before anything is written", func(t *testing.T) {
		putCalled := false
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposActionsSelectedActionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"message": "Server Error"}`))
				}),
			),
			WithRequestMatchHandler(putReposActionsSelectedActionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					// Reaching here means the tool wrote a policy it had
					// assembled without knowing the current one, which would
					// clear the flags it failed to read.
					putCalled = true
					t.Error("the allowed actions policy must not be written when the current one could not be read")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":           "set_allowed_actions",
			"owner":            "owner",
			"repo":             "repo",
			"patterns_allowed": []any{"octo-org/*"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to read the current allowed actions policy")
		assert.False(t, putCalled, "no write may follow a failed prerequisite read")
	})

	t.Run("a conflict on the allow-list read explains why nothing was written", func(t *testing.T) {
		putCalled := false
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposActionsSelectedActionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					// GitHub serves this policy only while allowed_actions is
					// "selected".
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"message": "Conflict"}`))
				}),
			),
			WithRequestMatchHandler(putReposActionsSelectedActionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					putCalled = true
					t.Error("the allowed actions policy must not be written after a conflicting read")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":               "set_allowed_actions",
			"owner":                "owner",
			"repo":                 "repo",
			"github_owned_allowed": true,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "allowed_actions is not 'selected'")
		assert.False(t, putCalled, "no write may follow a failed prerequisite read")
	})

	t.Run("set workflow permissions", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(putReposActionsWorkflowPermissionsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "read", payload["default_workflow_permissions"])
					assert.NotContains(t, payload, "can_approve_pull_request_reviews")

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"default_workflow_permissions":"read","can_approve_pull_request_reviews":false}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":                       "set_workflow_permissions",
			"owner":                        "owner",
			"repo":                         "repo",
			"default_workflow_permissions": "read",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var config MinimalActionsConfig
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &config))
		assert.Equal(t, "read", config.DefaultWorkflowPermissions)
	})

	t.Run("a workflow is addressed by file name", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(putReposActionsWorkflowDisableByOwnerByRepoByID,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Contains(t, r.URL.Path, "/workflows/deploy.yml/disable")
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "disable_workflow",
			"owner":    "owner",
			"repo":     "repo",
			"workflow": "deploy.yml",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "workflow_disabled")
	})

	t.Run("a workflow is addressed by numeric id", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(putReposActionsWorkflowEnableByOwnerByRepoByID,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Contains(t, r.URL.Path, "/workflows/12345/enable")
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "enable_workflow",
			"owner":    "owner",
			"repo":     "repo",
			"workflow": "12345",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "workflow_enabled")
	})

	t.Run("enable_workflow without a workflow is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "enable_workflow",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "workflow")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "add_runner",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method: add_runner")
	})
}
