package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	postReposCheckRunsByOwnerByRepo              = "POST /repos/{owner}/{repo}/check-runs"
	patchReposCheckRunsByOwnerByRepoByCheckRunID = "PATCH /repos/{owner}/{repo}/check-runs/{check_run_id}"
)

func Test_ChecksWrite(t *testing.T) {
	t.Parallel()

	serverTool := ChecksWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "checks_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "checks_write tool should not be read-only")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo"})

	t.Run("create_check_run posts a queued run for a head SHA", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposCheckRunsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/repos/owner/repo/check-runs", r.URL.Path)

					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "arcanum/gate", payload["name"])
					assert.Equal(t, "abc123", payload["head_sha"])
					assert.Equal(t, "queued", payload["status"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(MustMarshal(&github.CheckRun{
						ID:     github.Ptr(int64(42)),
						Name:   github.Ptr("arcanum/gate"),
						Status: github.Ptr("queued"),
					}))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "create_check_run",
			"owner":    "owner",
			"repo":     "repo",
			"name":     "arcanum/gate",
			"head_sha": "abc123",
			"status":   "queued",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var run MinimalCheckRun
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &run))
		assert.Equal(t, int64(42), run.ID)
		assert.Equal(t, "queued", run.Status)
	})

	t.Run("create_check_run sends output and details URL", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposCheckRunsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "https://ci.example/run/1", payload["details_url"])

					output, ok := payload["output"].(map[string]any)
					require.True(t, ok, "output should be sent as an object")
					assert.Equal(t, "Gate report", output["title"])
					assert.Equal(t, "3 channels green", output["summary"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(MustMarshal(&github.CheckRun{
						ID:         github.Ptr(int64(43)),
						Name:       github.Ptr("arcanum/gate"),
						Status:     github.Ptr("queued"),
						DetailsURL: github.Ptr("https://ci.example/run/1"),
					}))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":         "create_check_run",
			"owner":          "owner",
			"repo":           "repo",
			"name":           "arcanum/gate",
			"head_sha":       "abc123",
			"details_url":    "https://ci.example/run/1",
			"output_title":   "Gate report",
			"output_summary": "3 channels green",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var run MinimalCheckRun
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &run))
		assert.Equal(t, "https://ci.example/run/1", run.DetailsURL)
	})

	t.Run("update_check_run completes a run with a conclusion", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(patchReposCheckRunsByOwnerByRepoByCheckRunID,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, http.MethodPatch, r.Method)
					assert.Equal(t, "/repos/owner/repo/check-runs/42", r.URL.Path)

					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "completed", payload["status"])
					assert.Equal(t, "success", payload["conclusion"])
					assert.Equal(t, "2026-08-25T10:00:00Z", payload["completed_at"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(MustMarshal(&github.CheckRun{
						ID:         github.Ptr(int64(42)),
						Name:       github.Ptr("arcanum/gate"),
						Status:     github.Ptr("completed"),
						Conclusion: github.Ptr("success"),
					}))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":       "update_check_run",
			"owner":        "owner",
			"repo":         "repo",
			"check_run_id": float64(42),
			"status":       "completed",
			"conclusion":   "success",
			"completed_at": "2026-08-25T10:00:00Z",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var run MinimalCheckRun
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &run))
		assert.Equal(t, "completed", run.Status)
		assert.Equal(t, "success", run.Conclusion)
	})

	t.Run("create_check_run requires head_sha", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create_check_run",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "arcanum/gate",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "head_sha")
	})

	t.Run("update_check_run requires check_run_id", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "update_check_run",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "check_run_id")
	})

	t.Run("an invalid conclusion is rejected before any API call", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":       "update_check_run",
			"owner":        "owner",
			"repo":         "repo",
			"check_run_id": float64(42),
			"status":       "completed",
			"conclusion":   "mostly_fine",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "mostly_fine")
	})

	t.Run("completing a run without a conclusion is rejected before any API call", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":       "update_check_run",
			"owner":        "owner",
			"repo":         "repo",
			"check_run_id": float64(42),
			"status":       "completed",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "conclusion")
	})

	t.Run("an unknown method is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "delete_check_run",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "delete_check_run")
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposCheckRunsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Resource not accessible by integration"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "create_check_run",
			"owner":    "owner",
			"repo":     "repo",
			"name":     "arcanum/gate",
			"head_sha": "abc123",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to create check run")
	})
}
