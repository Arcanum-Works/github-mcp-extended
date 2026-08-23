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
	getReposCommitsCheckRunsByOwnerByRepoByRef   = "GET /repos/{owner}/{repo}/commits/{ref}/check-runs"
	getReposCheckRunsByOwnerByRepoByCheckRunID   = "GET /repos/{owner}/{repo}/check-runs/{check_run_id}"
	getReposCommitsCheckSuitesByOwnerByRepoByRef = "GET /repos/{owner}/{repo}/commits/{ref}/check-suites"
	getReposCommitsStatusByOwnerByRepoByRef      = "GET /repos/{owner}/{repo}/commits/{ref}/status"
	getReposCommitsStatusesByOwnerByRepoByRef    = "GET /repos/{owner}/{repo}/commits/{ref}/statuses"
	postReposStatusesByOwnerByRepoBySHA          = "POST /repos/{owner}/{repo}/statuses/{sha}"
)

func Test_ChecksRead(t *testing.T) {
	t.Parallel()

	serverTool := ChecksRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "checks_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "checks_read tool should be read-only")
	assert.Contains(t, schema.Properties, "ref")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo"})

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
		assertText         func(t *testing.T, text string)
	}{
		{
			name: "list check runs for a ref",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposCommitsCheckRunsByOwnerByRepoByRef, &github.ListCheckRunsResults{
					Total: github.Ptr(1),
					CheckRuns: []*github.CheckRun{
						{
							ID:         github.Ptr(int64(9)),
							Name:       github.Ptr("build"),
							Status:     github.Ptr("completed"),
							Conclusion: github.Ptr("success"),
						},
					},
				}),
			),
			requestArgs: map[string]any{
				"method": "list_check_runs",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "main",
			},
			assertText: func(t *testing.T, text string) {
				var result MinimalCheckRunsResult
				require.NoError(t, json.Unmarshal([]byte(text), &result))
				assert.Equal(t, 1, result.TotalCount)
				require.Len(t, result.CheckRuns, 1)
				assert.Equal(t, "success", result.CheckRuns[0].Conclusion)
			},
		},
		{
			name:         "list check runs without a ref is rejected",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "list_check_runs",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "ref is required for list_check_runs",
		},
		{
			name: "get a single check run",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposCheckRunsByOwnerByRepoByCheckRunID, &github.CheckRun{
					ID:     github.Ptr(int64(9)),
					Name:   github.Ptr("build"),
					Status: github.Ptr("in_progress"),
				}),
			),
			requestArgs: map[string]any{
				"method":       "get_check_run",
				"owner":        "owner",
				"repo":         "repo",
				"check_run_id": float64(9),
			},
			assertText: func(t *testing.T, text string) {
				var run MinimalCheckRun
				require.NoError(t, json.Unmarshal([]byte(text), &run))
				assert.Equal(t, int64(9), run.ID)
				assert.Equal(t, "in_progress", run.Status)
			},
		},
		{
			name:         "get check run without an id is rejected",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "get_check_run",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "check_run_id",
		},
		{
			name: "list check suites for a ref",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposCommitsCheckSuitesByOwnerByRepoByRef, &github.ListCheckSuiteResults{
					Total: github.Ptr(1),
					CheckSuites: []*github.CheckSuite{
						{
							ID:         github.Ptr(int64(4)),
							HeadBranch: github.Ptr("main"),
							Status:     github.Ptr("completed"),
							Conclusion: github.Ptr("failure"),
							App:        &github.App{Name: github.Ptr("GitHub Actions")},
						},
					},
				}),
			),
			requestArgs: map[string]any{
				"method": "list_check_suites",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "main",
			},
			assertText: func(t *testing.T, text string) {
				var result MinimalCheckSuitesResult
				require.NoError(t, json.Unmarshal([]byte(text), &result))
				require.Len(t, result.CheckSuites, 1)
				assert.Equal(t, "GitHub Actions", result.CheckSuites[0].AppName)
				assert.Equal(t, "failure", result.CheckSuites[0].Conclusion)
			},
		},
		{
			name: "combined status",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposCommitsStatusByOwnerByRepoByRef, &github.CombinedStatus{
					State:      github.Ptr("success"),
					SHA:        github.Ptr("abc123"),
					TotalCount: github.Ptr(2),
					Statuses: []*github.RepoStatus{
						{State: github.Ptr("success"), Context: github.Ptr("ci/lint")},
					},
				}),
			),
			requestArgs: map[string]any{
				"method": "get_combined_status",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "abc123",
			},
			assertText: func(t *testing.T, text string) {
				var status MinimalCombinedStatus
				require.NoError(t, json.Unmarshal([]byte(text), &status))
				assert.Equal(t, "success", status.State)
				require.Len(t, status.Statuses, 1)
				assert.Equal(t, "ci/lint", status.Statuses[0].Context)
			},
		},
		{
			name: "list individual statuses",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposCommitsStatusesByOwnerByRepoByRef, []*github.RepoStatus{
					{State: github.Ptr("pending"), Context: github.Ptr("ci/build")},
				}),
			),
			requestArgs: map[string]any{
				"method": "list_statuses",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "abc123",
			},
			assertText: func(t *testing.T, text string) {
				var statuses []MinimalRepoStatus
				require.NoError(t, json.Unmarshal([]byte(text), &statuses))
				require.Len(t, statuses, 1)
				assert.Equal(t, "pending", statuses[0].State)
			},
		},
		{
			name:         "unknown method",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "list_everything",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "unknown method: list_everything",
		},
		{
			name: "api failure is surfaced",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(getReposCommitsCheckRunsByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "list_check_runs",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "main",
			},
			expectToolError:    true,
			expectedToolErrMsg: "failed to list check runs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectToolError {
				assert.Contains(t, getErrorResult(t, result).Text, tc.expectedToolErrMsg)
				return
			}

			require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
			if tc.assertText != nil {
				tc.assertText(t, getTextResult(t, result).Text)
			}
		})
	}
}

func Test_CommitStatusWrite(t *testing.T) {
	t.Parallel()

	serverTool := CommitStatusWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "commit_status_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "commit_status_write tool should not be read-only")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "ref", "state"})

	t.Run("posts the status", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposStatusesByOwnerByRepoBySHA,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/repos/owner/repo/statuses/abc123", r.URL.Path)

					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "success", payload["state"])
					assert.Equal(t, "ci/lint", payload["context"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(MustMarshal(&github.RepoStatus{
						State:   github.Ptr("success"),
						Context: github.Ptr("ci/lint"),
					}))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":   "owner",
			"repo":    "repo",
			"ref":     "abc123",
			"state":   "success",
			"context": "ci/lint",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var status MinimalRepoStatus
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &status))
		assert.Equal(t, "success", status.State)
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposStatusesByOwnerByRepoBySHA,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"ref":   "abc123",
			"state": "success",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to create commit status")
	})
}
