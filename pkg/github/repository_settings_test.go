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
	patchReposByOwnerByRepo       = "PATCH /repos/{owner}/{repo}"
	putReposTopicsByOwnerByRepo   = "PUT /repos/{owner}/{repo}/topics"
	getReposTopicsByOwnerByRepoNA = "GET /repos/{owner}/{repo}/topics"
)

func governedRepository() *github.Repository {
	return &github.Repository{
		FullName:            github.Ptr("owner/repo"),
		DefaultBranch:       github.Ptr("develop"),
		Visibility:          github.Ptr("private"),
		AllowSquashMerge:    github.Ptr(true),
		AllowMergeCommit:    github.Ptr(false),
		AllowRebaseMerge:    github.Ptr(false),
		AllowAutoMerge:      github.Ptr(true),
		DeleteBranchOnMerge: github.Ptr(true),
		HasIssues:           github.Ptr(true),
		HasWiki:             github.Ptr(false),
		Topics:              []string{"platform"},
	}
}

func Test_RepositorySettingsRead(t *testing.T) {
	t.Parallel()

	serverTool := RepositorySettingsRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "repository_settings_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "repository_settings_read tool should be read-only")

	t.Run("reads the settings", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposByOwnerByRepoForGovernanceTesting, governedRepository()),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var settings MinimalRepositorySettings
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &settings))
		assert.Equal(t, "develop", settings.DefaultBranch)
		assert.True(t, settings.AllowSquashMerge)
		assert.False(t, settings.AllowMergeCommit)
		assert.True(t, settings.DeleteBranchOnMerge)
		assert.Equal(t, []string{"platform"}, settings.Topics)
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposByOwnerByRepoForGovernanceTesting,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to get repository settings")
	})
}

func Test_RepositorySettingsWrite(t *testing.T) {
	t.Parallel()

	serverTool := RepositorySettingsWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "repository_settings_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "repository_settings_write tool should not be read-only")

	t.Run("sends only the settings that were named", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(patchReposByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					assert.Equal(t, "develop", payload["default_branch"])
					assert.Equal(t, true, payload["delete_branch_on_merge"])
					// An explicit false must reach the API rather than being
					// dropped as a zero value.
					assert.Contains(t, payload, "allow_merge_commit")
					assert.Equal(t, false, payload["allow_merge_commit"])
					// Nothing else may be sent, or an unrelated setting would
					// be silently reset.
					assert.NotContains(t, payload, "has_wiki")
					assert.NotContains(t, payload, "has_issues")
					assert.NotContains(t, payload, "allow_squash_merge")
					assert.NotContains(t, payload, "archived")
					assert.NotContains(t, payload, "private")
					assert.NotContains(t, payload, "visibility")

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(MustMarshal(governedRepository()))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":                  "owner",
			"repo":                   "repo",
			"default_branch":         "develop",
			"delete_branch_on_merge": true,
			"allow_merge_commit":     false,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var settings MinimalRepositorySettings
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &settings))
		assert.Equal(t, "develop", settings.DefaultBranch)
	})

	t.Run("topics go through their own endpoint", func(t *testing.T) {
		patched := false
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(patchReposByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					patched = true
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(MustMarshal(governedRepository()))
				}),
			),
			WithRequestMatchHandler(putReposTopicsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, []any{"platform", "hq"}, payload["names"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"names":["platform","hq"]}`))
				}),
			),
			WithRequestMatch(getReposByOwnerByRepoForGovernanceTesting, governedRepository()),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":  "owner",
			"repo":   "repo",
			"topics": []any{"platform", "hq"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		// Topics alone must not trigger a repository patch.
		assert.False(t, patched, "topics-only update should not PATCH the repository")

		var settings MinimalRepositorySettings
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &settings))
		assert.Equal(t, []string{"platform", "hq"}, settings.Topics)
	})

	t.Run("a request that names no setting is refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "no settings were provided")
	})

	t.Run("a wrongly typed setting is refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":              "owner",
			"repo":               "repo",
			"allow_merge_commit": "yes",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "allow_merge_commit must be a boolean")
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(patchReposByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Must have admin rights"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":          "owner",
			"repo":           "repo",
			"default_branch": "develop",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to update repository settings")
	})
}
