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
	t.Run("create_check_run sends started_at, completed_at, external_id and output_text", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposCheckRunsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "gate-run-7", payload["external_id"])
					assert.Equal(t, "2026-08-25T10:00:00Z", payload["started_at"])
					assert.Equal(t, "2026-08-25T10:04:00Z", payload["completed_at"])

					output, ok := payload["output"].(map[string]any)
					require.True(t, ok, "output should be sent as an object")
					assert.Equal(t, "the full detail", output["text"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(MustMarshal(&github.CheckRun{
						ID:     github.Ptr(int64(50)),
						Name:   github.Ptr("arcanum/gate"),
						Status: github.Ptr("completed"),
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
			"status":         "completed",
			"conclusion":     "success",
			"external_id":    "gate-run-7",
			"started_at":     "2026-08-25T10:00:00Z",
			"completed_at":   "2026-08-25T10:04:00Z",
			"output_summary": "green",
			"output_text":    "the full detail",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("a malformed timestamp is rejected before any API call", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "create_check_run",
			"owner":      "owner",
			"repo":       "repo",
			"name":       "arcanum/gate",
			"head_sha":   "abc123",
			"started_at": "yesterday",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "started_at must be an RFC3339 timestamp")
	})

	t.Run("update_check_run rejects started_at rather than silently dropping it", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":       "update_check_run",
			"owner":        "owner",
			"repo":         "repo",
			"check_run_id": float64(42),
			"started_at":   "2026-08-25T10:00:00Z",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "started_at is only accepted by create_check_run")
	})

	t.Run("create_check_run sends annotations", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposCheckRunsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					output, ok := payload["output"].(map[string]any)
					require.True(t, ok, "output should be sent as an object")
					annotations, ok := output["annotations"].([]any)
					require.True(t, ok, "annotations should be sent as an array")
					require.Len(t, annotations, 2)

					first, ok := annotations[0].(map[string]any)
					require.True(t, ok)
					assert.Equal(t, "pkg/github/checks_write.go", first["path"])
					assert.Equal(t, float64(214), first["start_line"])
					assert.Equal(t, float64(214), first["end_line"])
					assert.Equal(t, float64(5), first["start_column"])
					assert.Equal(t, float64(18), first["end_column"])
					assert.Equal(t, "failure", first["annotation_level"])
					assert.Equal(t, "started_at is dropped here", first["message"])
					assert.Equal(t, "silent parameter loss", first["title"])
					assert.Equal(t, "UpdateCheckRunOptions has no StartedAt field", first["raw_details"])

					second, ok := annotations[1].(map[string]any)
					require.True(t, ok)
					assert.Equal(t, float64(1), second["start_line"])
					assert.Equal(t, float64(9), second["end_line"])
					assert.Equal(t, "notice", second["annotation_level"])
					assert.NotContains(t, second, "start_column", "a multi-line annotation must not carry columns")

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(MustMarshal(&github.CheckRun{
						ID:     github.Ptr(int64(51)),
						Name:   github.Ptr("arcanum/gate"),
						Status: github.Ptr("completed"),
					}))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "create_check_run",
			"owner":      "owner",
			"repo":       "repo",
			"name":       "arcanum/gate",
			"head_sha":   "abc123",
			"conclusion": "failure",
			"output_annotations": []any{
				map[string]any{
					"path":             "pkg/github/checks_write.go",
					"start_line":       float64(214),
					"end_line":         float64(214),
					"start_column":     float64(5),
					"end_column":       float64(18),
					"annotation_level": "failure",
					"message":          "started_at is dropped here",
					"title":            "silent parameter loss",
					"raw_details":      "UpdateCheckRunOptions has no StartedAt field",
				},
				map[string]any{
					"path":             "README.md",
					"start_line":       float64(1),
					"end_line":         float64(9),
					"annotation_level": "notice",
					"message":          "documented",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("annotations are accepted on update_check_run too", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(patchReposCheckRunsByOwnerByRepoByCheckRunID,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					output, ok := payload["output"].(map[string]any)
					require.True(t, ok)
					annotations, ok := output["annotations"].([]any)
					require.True(t, ok)
					require.Len(t, annotations, 1)

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(MustMarshal(&github.CheckRun{
						ID:     github.Ptr(int64(42)),
						Status: github.Ptr("completed"),
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
			"output_annotations": []any{
				map[string]any{
					"path":             "main.go",
					"start_line":       float64(3),
					"end_line":         float64(3),
					"annotation_level": "warning",
					"message":          "still here",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("an invalid annotation_level is rejected before any API call", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "create_check_run",
			"owner":    "owner",
			"repo":     "repo",
			"name":     "arcanum/gate",
			"head_sha": "abc123",
			"output_annotations": []any{
				map[string]any{
					"path":             "main.go",
					"start_line":       float64(1),
					"end_line":         float64(1),
					"annotation_level": "catastrophe",
					"message":          "boom",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		text := getErrorResult(t, result).Text
		assert.Contains(t, text, "catastrophe")
		assert.Contains(t, text, "failure, notice, warning")
	})

	t.Run("an annotation missing a required field is rejected before any API call", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "create_check_run",
			"owner":    "owner",
			"repo":     "repo",
			"name":     "arcanum/gate",
			"head_sha": "abc123",
			"output_annotations": []any{
				map[string]any{
					"path":             "main.go",
					"end_line":         float64(1),
					"annotation_level": "warning",
					"message":          "boom",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "output_annotations[0].start_line is required")
	})

	t.Run("a column on a multi-line annotation is rejected before any API call", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "create_check_run",
			"owner":    "owner",
			"repo":     "repo",
			"name":     "arcanum/gate",
			"head_sha": "abc123",
			"output_annotations": []any{
				map[string]any{
					"path":             "main.go",
					"start_line":       float64(1),
					"end_line":         float64(4),
					"start_column":     float64(2),
					"annotation_level": "warning",
					"message":          "boom",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "only when start_line equals end_line")
	})

	t.Run("an end_line before start_line is rejected before any API call", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "create_check_run",
			"owner":    "owner",
			"repo":     "repo",
			"name":     "arcanum/gate",
			"head_sha": "abc123",
			"output_annotations": []any{
				map[string]any{
					"path":             "main.go",
					"start_line":       float64(9),
					"end_line":         float64(4),
					"annotation_level": "warning",
					"message":          "boom",
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "end_line (4) is before start_line (9)")
	})

	t.Run("more annotations than GitHub accepts in one call are rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		entries := make([]any, 0, checkRunAnnotationsPerRequest+1)
		for i := 0; i <= checkRunAnnotationsPerRequest; i++ {
			entries = append(entries, map[string]any{
				"path":             "main.go",
				"start_line":       float64(i + 1),
				"end_line":         float64(i + 1),
				"annotation_level": "notice",
				"message":          "line noted",
			})
		}

		request := createMCPRequest(map[string]any{
			"method":             "create_check_run",
			"owner":              "owner",
			"repo":               "repo",
			"name":               "arcanum/gate",
			"head_sha":           "abc123",
			"output_annotations": entries,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "at most 50 entries per call, got 51")
	})
}
