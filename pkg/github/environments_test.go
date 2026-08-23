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
	getReposEnvironmentsByOwnerByRepo                = "GET /repos/{owner}/{repo}/environments"
	getReposEnvironmentsByOwnerByRepoByName          = "GET /repos/{owner}/{repo}/environments/{environment_name}"
	putReposEnvironmentsByOwnerByRepoByName          = "PUT /repos/{owner}/{repo}/environments/{environment_name}"
	deleteReposEnvironmentsByOwnerByRepoByName       = "DELETE /repos/{owner}/{repo}/environments/{environment_name}"
	getReposEnvBranchPoliciesByOwnerByRepoByName     = "GET /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies"
	postReposEnvBranchPoliciesByOwnerByRepoByName    = "POST /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies"
	deleteReposEnvBranchPolicyByOwnerByRepoByNameByI = "DELETE /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies/{branch_policy_id}"
	getUsersByUsername                               = "GET /users/{username}"
	getOrgsTeamsBySlug                               = "GET /orgs/{org}/teams/{slug}"
)

// stagingEnvironment is an environment as the API returns it: the wait timer
// and the reviewer list come back as protection rules rather than as the fields
// they were set with.
const stagingEnvironment = `{
  "id": 161088068,
  "name": "staging",
  "html_url": "https://github.com/owner/repo/deployments/activity_log?environments_filter=staging",
  "can_admins_bypass": false,
  "protection_rules": [
    {"id": 1, "type": "wait_timer", "wait_timer": 15},
    {"id": 2, "type": "required_reviewers", "prevent_self_review": true, "reviewers": [
      {"type": "User", "reviewer": {"id": 7, "login": "octocat"}},
      {"type": "Team", "reviewer": {"id": 9, "slug": "platform", "name": "Platform"}}
    ]},
    {"id": 3, "type": "branch_policy"}
  ],
  "deployment_branch_policy": {"protected_branches": false, "custom_branch_policies": true}
}`

func Test_EnvironmentsRead(t *testing.T) {
	t.Parallel()

	serverTool := EnvironmentsRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "environments_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "environments_read tool should be read-only")

	t.Run("list environments", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposEnvironmentsByOwnerByRepo, `{"total_count":1,"environments":[`+stagingEnvironment+`]}`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list_environments",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var envs []MinimalEnvironment
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &envs))
		require.Len(t, envs, 1)
		assert.Equal(t, "staging", envs[0].Name)
		// The protection rules are unpacked back into the settings that made them.
		assert.Equal(t, 15, envs[0].WaitTimerMinutes)
		assert.True(t, envs[0].PreventSelfReview)
		require.Len(t, envs[0].RequiredReviewers, 2)
		assert.Equal(t, "octocat", envs[0].RequiredReviewers[0].Login)
		assert.Equal(t, "platform", envs[0].RequiredReviewers[1].Slug)
		assert.Equal(t, "custom", envs[0].DeploymentBranchPolicy)
	})

	t.Run("get environment includes its custom branch patterns", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposEnvironmentsByOwnerByRepoByName, stagingEnvironment),
			WithRequestMatch(getReposEnvBranchPoliciesByOwnerByRepoByName, `{"total_count":2,"branch_policies":[{"id":11,"name":"main"},{"id":12,"name":"release/*"}]}`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":           "get_environment",
			"owner":            "owner",
			"repo":             "repo",
			"environment_name": "staging",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var env MinimalEnvironment
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &env))
		assert.Equal(t, []string{"main", "release/*"}, env.CustomBranchPatterns)
	})

	t.Run("get without a name is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "get_environment",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "environment_name")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "describe",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method: describe")
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposEnvironmentsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list_environments",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to list environments")
	})
}

func Test_EnvironmentWrite(t *testing.T) {
	t.Parallel()

	serverTool := EnvironmentWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "environment_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "environment_write tool should not be read-only")

	t.Run("update keeps the settings it was not asked to change", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposEnvironmentsByOwnerByRepoByName, stagingEnvironment),
			WithRequestMatch(getReposEnvBranchPoliciesByOwnerByRepoByName, `{"total_count":0,"branch_policies":[]}`),
			WithRequestMatchHandler(putReposEnvironmentsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					// Only can_admins_bypass was named; the wait timer and the
					// reviewers must be sent back or the API would clear them.
					assert.Equal(t, true, payload["can_admins_bypass"])
					assert.Equal(t, float64(15), payload["wait_timer"])
					assert.Equal(t, true, payload["prevent_self_review"])

					reviewers := payload["reviewers"].([]any)
					require.Len(t, reviewers, 2)
					assert.Equal(t, float64(7), reviewers[0].(map[string]any)["id"])
					assert.Equal(t, "Team", reviewers[1].(map[string]any)["type"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(stagingEnvironment))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":            "create_or_update",
			"owner":             "owner",
			"repo":              "repo",
			"environment_name":  "staging",
			"can_admins_bypass": true,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("creating a new environment does not need it to exist first", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposEnvironmentsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			),
			WithRequestMatchHandler(putReposEnvironmentsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, float64(30), payload["wait_timer"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"id":1,"name":"staging","protection_rules":[{"id":1,"type":"wait_timer","wait_timer":30}]}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":             "create_or_update",
			"owner":              "owner",
			"repo":               "repo",
			"environment_name":   "staging",
			"wait_timer_minutes": float64(30),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var env MinimalEnvironment
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &env))
		assert.Equal(t, "staging", env.Name)
		assert.Equal(t, 30, env.WaitTimerMinutes)
	})

	t.Run("reviewers are resolved from logins and slugs", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposEnvironmentsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			),
			WithRequestMatch(getUsersByUsername, &github.User{ID: github.Ptr(int64(7)), Login: github.Ptr("octocat")}),
			WithRequestMatch(getOrgsTeamsBySlug, &github.Team{ID: github.Ptr(int64(9)), Slug: github.Ptr("platform")}),
			WithRequestMatchHandler(putReposEnvironmentsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					reviewers := payload["reviewers"].([]any)
					require.Len(t, reviewers, 2)
					assert.Equal(t, float64(7), reviewers[0].(map[string]any)["id"])
					assert.Equal(t, float64(9), reviewers[1].(map[string]any)["id"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"id":1,"name":"staging"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":           "create_or_update",
			"owner":            "owner",
			"repo":             "repo",
			"environment_name": "staging",
			"reviewers": []any{
				map[string]any{"type": "User", "login": "octocat"},
				map[string]any{"type": "Team", "slug": "platform"},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("a reviewer with neither an id nor a name is refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposEnvironmentsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":           "create_or_update",
			"owner":            "owner",
			"repo":             "repo",
			"environment_name": "staging",
			"reviewers":        []any{map[string]any{"type": "User"}},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "needs a login or an id")
	})

	t.Run("custom branch patterns are reconciled", func(t *testing.T) {
		created := []string{}
		deleted := []string{}

		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposEnvironmentsByOwnerByRepoByName, stagingEnvironment),
			WithRequestMatch(putReposEnvironmentsByOwnerByRepoByName, stagingEnvironment),
			WithRequestMatch(getReposEnvBranchPoliciesByOwnerByRepoByName,
				`{"total_count":2,"branch_policies":[{"id":11,"name":"main"},{"id":12,"name":"legacy/*"}]}`),
			WithRequestMatchHandler(postReposEnvBranchPoliciesByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					created = append(created, payload["name"].(string))

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"id":13,"name":"release/*"}`))
				}),
			),
			WithRequestMatchHandler(deleteReposEnvBranchPolicyByOwnerByRepoByNameByI,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					deleted = append(deleted, r.URL.Path)
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":                 "create_or_update",
			"owner":                  "owner",
			"repo":                   "repo",
			"environment_name":       "staging",
			"custom_branch_patterns": []any{"main", "release/*"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		// main already existed and is left alone, release/* is added, and
		// legacy/* is removed because it was not asked for.
		assert.Equal(t, []string{"release/*"}, created)
		require.Len(t, deleted, 1)
		assert.Contains(t, deleted[0], "/12")

		var env MinimalEnvironment
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &env))
		assert.Equal(t, []string{"main", "release/*"}, env.CustomBranchPatterns)
	})

	t.Run("delete an environment", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteReposEnvironmentsByOwnerByRepoByName,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":           "delete",
			"owner":            "owner",
			"repo":             "repo",
			"environment_name": "staging",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "environment 'staging' deleted successfully")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":           "archive",
			"owner":            "owner",
			"repo":             "repo",
			"environment_name": "staging",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method: archive")
	})
}
