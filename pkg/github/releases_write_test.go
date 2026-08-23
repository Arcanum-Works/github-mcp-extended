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
	postReposReleasesByOwnerByRepo                = "POST /repos/{owner}/{repo}/releases"
	patchReposReleasesByOwnerByRepoByReleaseID    = "PATCH /repos/{owner}/{repo}/releases/{release_id}"
	deleteReposReleasesByOwnerByRepoByReleaseID   = "DELETE /repos/{owner}/{repo}/releases/{release_id}"
	postReposReleasesGenerateNotesByOwnerByRepo   = "POST /repos/{owner}/{repo}/releases/generate-notes"
	getReposReleasesTagsByOwnerByRepoByTagForTest = "GET /repos/{owner}/{repo}/releases/tags/{tag}"
)

func Test_ReleaseWrite(t *testing.T) {
	t.Parallel()

	serverTool := ReleaseWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "release_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "release_write tool should not be read-only")
	assert.Contains(t, schema.Properties, "tag_name")
	assert.Contains(t, schema.Properties, "release_id")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo"})

	createdRelease := &github.RepositoryRelease{
		ID:      42,
		TagName: "v1.2.3",
		Name:    github.Ptr("v1.2.3"),
		HTMLURL: "https://github.com/owner/repo/releases/tag/v1.2.3",
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
			name: "create release",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(postReposReleasesByOwnerByRepo, createdRelease),
			),
			requestArgs: map[string]any{
				"method":   "create",
				"owner":    "owner",
				"repo":     "repo",
				"tag_name": "v1.2.3",
				"name":     "v1.2.3",
			},
			assertText: func(t *testing.T, text string) {
				var release MinimalRelease
				require.NoError(t, json.Unmarshal([]byte(text), &release))
				assert.Equal(t, int64(42), release.ID)
				assert.Equal(t, "v1.2.3", release.TagName)
			},
		},
		{
			name:         "create without tag_name is rejected before any request",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "tag_name is required for create",
		},
		{
			name: "update resolves the release from its tag",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposReleasesTagsByOwnerByRepoByTagForTest, createdRelease),
				WithRequestMatchHandler(patchReposReleasesByOwnerByRepoByReleaseID,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						assert.Equal(t, "/repos/owner/repo/releases/42", r.URL.Path)

						var payload map[string]any
						require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
						// The tag was the lookup key, so it must not be
						// echoed back as a rename.
						assert.NotContains(t, payload, "tag_name")
						assert.Equal(t, true, payload["prerelease"])

						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(MustMarshal(createdRelease))
					}),
				),
			),
			requestArgs: map[string]any{
				"method":     "update",
				"owner":      "owner",
				"repo":       "repo",
				"tag_name":   "v1.2.3",
				"prerelease": true,
			},
		},
		{
			name: "update by id retags the release",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(patchReposReleasesByOwnerByRepoByReleaseID,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var payload map[string]any
						require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
						assert.Equal(t, "v2.0.0", payload["tag_name"])

						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(MustMarshal(createdRelease))
					}),
				),
			),
			requestArgs: map[string]any{
				"method":     "update",
				"owner":      "owner",
				"repo":       "repo",
				"release_id": float64(42),
				"tag_name":   "v2.0.0",
			},
		},
		{
			name:         "update without an identifier is rejected",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "update",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "either release_id or tag_name is required",
		},
		{
			name: "delete says the tag survives",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(deleteReposReleasesByOwnerByRepoByReleaseID,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"method":     "delete",
				"owner":      "owner",
				"repo":       "repo",
				"release_id": float64(42),
			},
			assertText: func(t *testing.T, text string) {
				assert.Contains(t, text, "release 42 deleted successfully")
				assert.Contains(t, text, "git tag still exists")
			},
		},
		{
			name: "generate notes",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(postReposReleasesGenerateNotesByOwnerByRepo, &github.RepositoryReleaseNotes{
					Name: "v1.2.3",
					Body: "## What's Changed",
				}),
			),
			requestArgs: map[string]any{
				"method":   "generate_notes",
				"owner":    "owner",
				"repo":     "repo",
				"tag_name": "v1.2.3",
			},
			assertText: func(t *testing.T, text string) {
				var notes MinimalReleaseNotes
				require.NoError(t, json.Unmarshal([]byte(text), &notes))
				assert.Equal(t, "v1.2.3", notes.Name)
				// sanitize.Sanitize HTML-escapes the body, so the apostrophe
				// comes back as an entity.
				assert.Contains(t, notes.Body, "What&#39;s Changed")
			},
		},
		{
			name:         "unknown method",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "publish",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "unknown method: publish",
		},
		{
			name: "create failure is surfaced",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(postReposReleasesByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
					}),
				),
			),
			requestArgs: map[string]any{
				"method":   "create",
				"owner":    "owner",
				"repo":     "repo",
				"tag_name": "v1.2.3",
			},
			expectToolError:    true,
			expectedToolErrMsg: "failed to create release",
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
				textContent := getErrorResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
			if tc.assertText != nil {
				tc.assertText(t, getTextResult(t, result).Text)
			}
		})
	}
}

func Test_optionalBoolPointer(t *testing.T) {
	t.Parallel()

	// An absent flag and an explicit false must stay distinguishable, so an
	// update does not silently clear a field the caller never mentioned.
	v, set, err := optionalBoolPointer(map[string]any{}, "draft")
	require.NoError(t, err)
	assert.False(t, set)
	assert.Nil(t, v)

	v, set, err = optionalBoolPointer(map[string]any{"draft": false}, "draft")
	require.NoError(t, err)
	assert.True(t, set)
	require.NotNil(t, v)
	assert.False(t, *v)

	v, set, err = optionalBoolPointer(map[string]any{"draft": nil}, "draft")
	require.NoError(t, err)
	assert.False(t, set)
	assert.Nil(t, v)

	_, _, err = optionalBoolPointer(map[string]any{"draft": "yes"}, "draft")
	require.Error(t, err)
}
