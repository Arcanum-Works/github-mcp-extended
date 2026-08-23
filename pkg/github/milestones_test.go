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
	getReposMilestonesByOwnerByRepo                            = "GET /repos/{owner}/{repo}/milestones"
	postReposMilestonesByOwnerByRepo                           = "POST /repos/{owner}/{repo}/milestones"
	patchReposMilestonesByOwnerByRepoByMilestoneNumber         = "PATCH /repos/{owner}/{repo}/milestones/{milestone_number}"
	deleteReposMilestonesByOwnerByRepoByMilestoneNumberForTest = "DELETE /repos/{owner}/{repo}/milestones/{milestone_number}"
)

func Test_ListMilestones(t *testing.T) {
	t.Parallel()

	serverTool := ListMilestones(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "list_milestones", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "list_milestones tool should be read-only")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	mockMilestones := []*github.Milestone{
		{
			Number:      github.Ptr(1),
			Title:       github.Ptr("v1.0"),
			State:       github.Ptr("open"),
			Description: github.Ptr("First release"),
			OpenIssues:  github.Ptr(3),
		},
	}

	t.Run("successful listing", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposMilestonesByOwnerByRepo, mockMilestones),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var milestones []MinimalMilestone
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &milestones))
		require.Len(t, milestones, 1)
		assert.Equal(t, 1, milestones[0].Number)
		assert.Equal(t, "v1.0", milestones[0].Title)
		assert.Equal(t, 3, milestones[0].OpenIssues)
	})

	t.Run("listing failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposMilestonesByOwnerByRepo,
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
		assert.Contains(t, getErrorResult(t, result).Text, "failed to list milestones")
	})
}

func Test_MilestoneWrite(t *testing.T) {
	t.Parallel()

	serverTool := MilestoneWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "milestone_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "milestone_write tool should not be read-only")

	created := &github.Milestone{
		Number: github.Ptr(7),
		Title:  github.Ptr("v1.0"),
		State:  github.Ptr("open"),
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
			name: "create milestone with a due date",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(postReposMilestonesByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var payload map[string]any
						require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
						assert.Equal(t, "v1.0", payload["title"])
						// End of the given day in UTC, not midnight.
						assert.Equal(t, "2026-09-01T23:59:59Z", payload["due_on"])

						w.WriteHeader(http.StatusCreated)
						_, _ = w.Write(MustMarshal(created))
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
				"title":  "v1.0",
				"due_on": "2026-09-01",
			},
			assertText: func(t *testing.T, text string) {
				var milestone MinimalMilestone
				require.NoError(t, json.Unmarshal([]byte(text), &milestone))
				assert.Equal(t, 7, milestone.Number)
			},
		},
		{
			name:         "create without title is rejected",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "title is required for create",
		},
		{
			name:         "a malformed due date is rejected before any request",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
				"title":  "v1.0",
				"due_on": "01/09/2026",
			},
			expectToolError:    true,
			expectedToolErrMsg: "due_on must be a date in YYYY-MM-DD format",
		},
		{
			name: "update resolves the milestone from its title",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposMilestonesByOwnerByRepo, []*github.Milestone{
					{Number: github.Ptr(3), Title: github.Ptr("other"), State: github.Ptr("open")},
					{Number: github.Ptr(7), Title: github.Ptr("V1.0"), State: github.Ptr("open")},
				}),
				WithRequestMatchHandler(patchReposMilestonesByOwnerByRepoByMilestoneNumber,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// Matched case-insensitively against "V1.0".
						assert.Equal(t, "/repos/owner/repo/milestones/7", r.URL.Path)

						var payload map[string]any
						require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
						assert.NotContains(t, payload, "title", "the lookup title is not a rename")
						assert.Equal(t, "closed", payload["state"])

						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(MustMarshal(created))
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "update",
				"owner":  "owner",
				"repo":   "repo",
				"title":  "v1.0",
				"state":  "closed",
			},
		},
		{
			name: "an ambiguous title is an error, not a guess",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposMilestonesByOwnerByRepo, []*github.Milestone{
					{Number: github.Ptr(3), Title: github.Ptr("v1.0"), State: github.Ptr("closed")},
					{Number: github.Ptr(7), Title: github.Ptr("v1.0"), State: github.Ptr("open")},
				}),
			),
			requestArgs: map[string]any{
				"method": "delete",
				"owner":  "owner",
				"repo":   "repo",
				"title":  "v1.0",
			},
			expectToolError:    true,
			expectedToolErrMsg: "is ambiguous",
		},
		{
			name: "an unknown title reports not found",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposMilestonesByOwnerByRepo, []*github.Milestone{}),
			),
			requestArgs: map[string]any{
				"method": "delete",
				"owner":  "owner",
				"repo":   "repo",
				"title":  "nope",
			},
			expectToolError:    true,
			expectedToolErrMsg: "milestone 'nope' not found in owner/repo",
		},
		{
			name: "delete by number",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(deleteReposMilestonesByOwnerByRepoByMilestoneNumberForTest,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"method":           "delete",
				"owner":            "owner",
				"repo":             "repo",
				"milestone_number": float64(7),
			},
			assertText: func(t *testing.T, text string) {
				assert.Contains(t, text, "milestone 7 deleted successfully")
			},
		},
		{
			name:         "neither number nor title is an error",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "update",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "either milestone_number or title is required",
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
