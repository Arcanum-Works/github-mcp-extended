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
	getReposDeploymentsByOwnerByRepo               = "GET /repos/{owner}/{repo}/deployments"
	postReposDeploymentsByOwnerByRepo              = "POST /repos/{owner}/{repo}/deployments"
	getReposDeploymentsByOwnerByRepoByID           = "GET /repos/{owner}/{repo}/deployments/{deployment_id}"
	getReposDeploymentStatusesByOwnerByRepoByID    = "GET /repos/{owner}/{repo}/deployments/{deployment_id}/statuses"
	postReposDeploymentStatusesByOwnerByRepoByID   = "POST /repos/{owner}/{repo}/deployments/{deployment_id}/statuses"
	getReposActionsVariablesByOwnerByRepoForDeploy = "GET /repos/{owner}/{repo}/actions/variables"
)

const stagingDeployment = `{
  "id": 42,
  "sha": "abc123",
  "ref": "main",
  "task": "deploy",
  "environment": "staging",
  "description": "deploy 1.2.3",
  "created_at": "2026-08-01T10:00:00Z",
  "statuses_url": "https://api.github.com/repos/owner/repo/deployments/42/statuses"
}`

func Test_DeploymentsRead(t *testing.T) {
	t.Parallel()

	serverTool := DeploymentsRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "deployments_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "deployments_read tool should be read-only")

	t.Run("list deployments filtered by environment and sha", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposDeploymentsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					query := r.URL.Query()
					assert.Equal(t, "staging", query.Get("environment"))
					assert.Equal(t, "abc123", query.Get("sha"))

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[` + stagingDeployment + `]`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":      "list_deployments",
			"owner":       "owner",
			"repo":        "repo",
			"environment": "staging",
			"sha":         "abc123",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var deployments []MinimalDeployment
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &deployments))
		require.Len(t, deployments, 1)
		assert.Equal(t, int64(42), deployments[0].ID)
		// The SHA is what links a deployment back to its commit and run.
		assert.Equal(t, "abc123", deployments[0].SHA)
		assert.Equal(t, "staging", deployments[0].Environment)
	})

	t.Run("get one deployment", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposDeploymentsByOwnerByRepoByID, stagingDeployment),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":        "get_deployment",
			"owner":         "owner",
			"repo":          "repo",
			"deployment_id": float64(42),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var deployment MinimalDeployment
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &deployment))
		assert.Equal(t, "main", deployment.Ref)
	})

	t.Run("list deployment statuses", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposDeploymentStatusesByOwnerByRepoByID, `[
				{"id":2,"state":"success","environment":"staging","environment_url":"https://staging.example.com","log_url":"https://github.com/owner/repo/actions/runs/7"},
				{"id":1,"state":"in_progress","environment":"staging"}
			]`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":        "list_deployment_statuses",
			"owner":         "owner",
			"repo":          "repo",
			"deployment_id": float64(42),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var statuses []MinimalDeploymentStatus
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &statuses))
		require.Len(t, statuses, 2)
		assert.Equal(t, "success", statuses[0].State)
		// Where it was deployed, and where the output can be read.
		assert.Equal(t, "https://staging.example.com", statuses[0].EnvironmentURL)
		assert.Contains(t, statuses[0].LogURL, "actions/runs/7")
	})

	t.Run("statuses without a deployment id are rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list_deployment_statuses",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "deployment_id")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "rollback",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method: rollback")
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposDeploymentsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list_deployments",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to list deployments")
	})
}

func Test_DeploymentWrite(t *testing.T) {
	t.Parallel()

	serverTool := DeploymentWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "deployment_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "deployment_write tool should not be read-only")

	t.Run("create a deployment", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposDeploymentsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "main", payload["ref"])
					assert.Equal(t, "staging", payload["environment"])
					// Deploying the ref as given is the default here, unlike
					// GitHub's own default of merging the base branch in.
					assert.Equal(t, false, payload["auto_merge"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(stagingDeployment))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":      "create_deployment",
			"owner":       "owner",
			"repo":        "repo",
			"ref":         "main",
			"environment": "staging",
			"description": "deploy 1.2.3",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var deployment MinimalDeployment
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &deployment))
		assert.Equal(t, int64(42), deployment.ID)
	})

	t.Run("an empty required_contexts array is sent, an absent one is not", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposDeploymentsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					require.Contains(t, payload, "required_contexts")
					assert.Empty(t, payload["required_contexts"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(stagingDeployment))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":            "create_deployment",
			"owner":             "owner",
			"repo":              "repo",
			"ref":               "main",
			"required_contexts": []any{},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("a declined deployment is reported rather than returned empty", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposDeploymentsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					// GitHub answers 202 with a message when a required check
					// has not passed, and creates nothing.
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte(`{"message":"Conflict merging main into topic"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create_deployment",
			"owner":  "owner",
			"repo":   "repo",
			"ref":    "main",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "created no deployment")
	})

	t.Run("create a deployment status", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposDeploymentStatusesByOwnerByRepoByID,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Contains(t, r.URL.Path, "/deployments/42/statuses")

					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "success", payload["state"])
					assert.Equal(t, "https://staging.example.com", payload["environment_url"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"id":7,"state":"success","environment":"staging","environment_url":"https://staging.example.com"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":          "create_deployment_status",
			"owner":           "owner",
			"repo":            "repo",
			"deployment_id":   float64(42),
			"state":           "success",
			"environment_url": "https://staging.example.com",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var status MinimalDeploymentStatus
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &status))
		assert.Equal(t, "success", status.State)
		assert.Equal(t, "https://staging.example.com", status.EnvironmentURL)
	})

	t.Run("a status without a state is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":        "create_deployment_status",
			"owner":         "owner",
			"repo":          "repo",
			"deployment_id": float64(42),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "state")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "undeploy",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method: undeploy")
	})
}
