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
	postReposGitRefsByOwnerByRepo = "POST /repos/{owner}/{repo}/git/refs"
	// A ref spans several path segments (refs/heads/feature), so these need the
	// mock server's wildcard form rather than a single {ref} placeholder.
	patchReposGitRefsByOwnerByRepoByRef    = "PATCH /repos/{owner}/{repo}/git/refs/{ref:.*}"
	deleteReposGitRefsByOwnerByRepoByRef   = "DELETE /repos/{owner}/{repo}/git/refs/{ref:.*}"
	postReposGitTagsByOwnerByRepo          = "POST /repos/{owner}/{repo}/git/tags"
	getReposCommitsByOwnerByRepoByRef      = "GET /repos/{owner}/{repo}/commits/{ref}"
	getReposByOwnerByRepoForRefs           = "GET /repos/{owner}/{repo}"
	getReposCompareByOwnerByRepoByBasehead = "GET /repos/{owner}/{repo}/compare/{basehead}"
)

func Test_GitRefWrite(t *testing.T) {
	t.Parallel()

	serverTool := GitRefWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "git_ref_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "git_ref_write tool should not be read-only")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "ref"})

	createdRef := &github.Reference{
		Ref: github.Ptr("refs/heads/feature"),
		Object: &github.GitObject{
			SHA:  github.Ptr("abc123"),
			Type: github.Ptr("commit"),
		},
	}

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
		assertText         func(t *testing.T, text string)
	}{
		{
			name: "create a ref",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(postReposGitRefsByOwnerByRepo, createdRef),
			),
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "refs/heads/feature",
				"sha":    "abc123",
			},
			assertText: func(t *testing.T, text string) {
				var ref MinimalRef
				require.NoError(t, json.Unmarshal([]byte(text), &ref))
				assert.Equal(t, "refs/heads/feature", ref.Ref)
				assert.Equal(t, "abc123", ref.SHA)
				assert.Equal(t, "commit", ref.ObjectType)
			},
		},
		{
			name:         "create without a sha is rejected",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "refs/heads/feature",
			},
			expectToolError:    true,
			expectedToolErrMsg: "sha is required for create",
		},
		{
			name: "update forwards force",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(patchReposGitRefsByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var payload map[string]any
						require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
						assert.Equal(t, "def456", payload["sha"])
						assert.Equal(t, true, payload["force"])

						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(MustMarshal(createdRef))
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "update",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "refs/heads/feature",
				"sha":    "def456",
				"force":  true,
			},
		},
		{
			name: "delete a ref",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(deleteReposGitRefsByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "delete",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "refs/heads/feature",
			},
			assertText: func(t *testing.T, text string) {
				assert.Contains(t, text, "deleted successfully")
			},
		},
		{
			name:         "unknown method",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "move",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "refs/heads/feature",
			},
			expectToolError:    true,
			expectedToolErrMsg: "unknown method: move",
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

func Test_TagWrite(t *testing.T) {
	t.Parallel()

	serverTool := TagWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "tag_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "tag_write tool should not be read-only")

	t.Run("lightweight tag from an explicit ref", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposCommitsByOwnerByRepoByRef, &github.RepositoryCommit{
				SHA: github.Ptr("abc123"),
			}),
			WithRequestMatchHandler(postReposGitRefsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "refs/tags/v1.2.3", payload["ref"])
					// Without a message the ref points straight at the commit.
					assert.Equal(t, "abc123", payload["sha"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(MustMarshal(&github.Reference{
						Ref:    github.Ptr("refs/tags/v1.2.3"),
						Object: &github.GitObject{SHA: github.Ptr("abc123"), Type: github.Ptr("commit")},
					}))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "create",
			"owner":    "owner",
			"repo":     "repo",
			"tag":      "v1.2.3",
			"from_ref": "main",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var ref MinimalRef
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &ref))
		assert.Equal(t, "refs/tags/v1.2.3", ref.Ref)
	})

	t.Run("annotated tag creates the tag object first", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposCommitsByOwnerByRepoByRef, &github.RepositoryCommit{
				SHA: github.Ptr("abc123"),
			}),
			WithRequestMatchHandler(postReposGitTagsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "v1.2.3", payload["tag"])
					assert.Equal(t, "release 1.2.3", payload["message"])
					assert.Equal(t, "abc123", payload["object"])
					assert.Equal(t, "commit", payload["type"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(MustMarshal(&github.Tag{SHA: github.Ptr("tagobj789")}))
				}),
			),
			WithRequestMatchHandler(postReposGitRefsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					// The ref points at the tag object, not the commit.
					assert.Equal(t, "tagobj789", payload["sha"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(MustMarshal(&github.Reference{
						Ref:    github.Ptr("refs/tags/v1.2.3"),
						Object: &github.GitObject{SHA: github.Ptr("tagobj789"), Type: github.Ptr("tag")},
					}))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "create",
			"owner":    "owner",
			"repo":     "repo",
			"tag":      "refs/tags/v1.2.3",
			"from_ref": "main",
			"message":  "release 1.2.3",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("without from_ref the default branch head is used", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposByOwnerByRepoForRefs, &github.Repository{
				DefaultBranch: github.Ptr("trunk"),
			}),
			WithRequestMatchHandler(getReposCommitsByOwnerByRepoByRef,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/repos/owner/repo/commits/trunk", r.URL.Path)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(MustMarshal(&github.RepositoryCommit{SHA: github.Ptr("head999")}))
				}),
			),
			WithRequestMatch(postReposGitRefsByOwnerByRepo, &github.Reference{
				Ref:    github.Ptr("refs/tags/v2"),
				Object: &github.GitObject{SHA: github.Ptr("head999"), Type: github.Ptr("commit")},
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create",
			"owner":  "owner",
			"repo":   "repo",
			"tag":    "v2",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("delete a tag", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteReposGitRefsByOwnerByRepoByRef,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Contains(t, r.URL.Path, "tags/v1.2.3")
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "delete",
			"owner":  "owner",
			"repo":   "repo",
			"tag":    "v1.2.3",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "tag 'v1.2.3' deleted successfully")
	})

	t.Run("an unresolvable from_ref is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposCommitsByOwnerByRepoByRef,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "No commit found"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "create",
			"owner":    "owner",
			"repo":     "repo",
			"tag":      "v1.2.3",
			"from_ref": "nope",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to resolve 'nope' to a commit")
	})
}

func Test_CompareCommits(t *testing.T) {
	t.Parallel()

	serverTool := CompareCommits(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "compare_commits", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "compare_commits tool should be read-only")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "base", "head"})

	comparison := &github.CommitsComparison{
		Status:       github.Ptr("ahead"),
		AheadBy:      github.Ptr(2),
		BehindBy:     github.Ptr(0),
		TotalCommits: github.Ptr(2),
		HTMLURL:      github.Ptr("https://github.com/owner/repo/compare/main...feature"),
		MergeBaseCommit: &github.RepositoryCommit{
			SHA: github.Ptr("base000"),
		},
		Commits: []*github.RepositoryCommit{
			{SHA: github.Ptr("c1"), Commit: &github.Commit{Message: github.Ptr("first")}},
			{SHA: github.Ptr("c2"), Commit: &github.Commit{Message: github.Ptr("second")}},
		},
		Files: []*github.CommitFile{
			{
				Filename:  github.Ptr("main.go"),
				Status:    github.Ptr("modified"),
				Additions: github.Ptr(3),
				Patch:     github.Ptr("@@ -1 +1 @@"),
			},
		},
	}

	t.Run("stats detail omits patches", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposCompareByOwnerByRepoByBasehead, comparison),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"base":  "main",
			"head":  "feature",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var got MinimalCommitsComparison
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &got))
		assert.Equal(t, "ahead", got.Status)
		assert.Equal(t, 2, got.AheadBy)
		assert.Equal(t, "base000", got.MergeBaseSHA)
		assert.Len(t, got.Commits, 2)
		require.Len(t, got.Files, 1)
		assert.Equal(t, "main.go", got.Files[0].Filename)
		assert.Empty(t, got.Files[0].Patch, "stats detail should not carry patch text")
	})

	t.Run("full_patch detail keeps patches", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposCompareByOwnerByRepoByBasehead, comparison),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":  "owner",
			"repo":   "repo",
			"base":   "main",
			"head":   "feature",
			"detail": "full_patch",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var got MinimalCommitsComparison
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &got))
		require.Len(t, got.Files, 1)
		assert.NotEmpty(t, got.Files[0].Patch)
	})

	t.Run("none detail omits the file list", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposCompareByOwnerByRepoByBasehead, comparison),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":  "owner",
			"repo":   "repo",
			"base":   "main",
			"head":   "feature",
			"detail": "none",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var got MinimalCommitsComparison
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &got))
		assert.Empty(t, got.Files)
		assert.Len(t, got.Commits, 2)
	})

	t.Run("an invalid detail value is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":  "owner",
			"repo":   "repo",
			"base":   "main",
			"head":   "feature",
			"detail": "everything",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "invalid detail")
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposCompareByOwnerByRepoByBasehead,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"base":  "main",
			"head":  "feature",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to compare commits")
	})
}
