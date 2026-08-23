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
	getReposCollaboratorsByOwnerByRepo                = "GET /repos/{owner}/{repo}/collaborators"
	getReposCollaboratorPermissionByOwnerByRepoByUser = "GET /repos/{owner}/{repo}/collaborators/{username}/permission"
	putReposCollaboratorsByOwnerByRepoByUser          = "PUT /repos/{owner}/{repo}/collaborators/{username}"
	deleteReposCollaboratorsByOwnerByRepoByUser       = "DELETE /repos/{owner}/{repo}/collaborators/{username}"
	getReposTeamsByOwnerByRepo                        = "GET /repos/{owner}/{repo}/teams"
	putOrgsTeamsReposByOrgBySlugByOwnerByRepo         = "PUT /orgs/{org}/teams/{slug}/repos/{owner}/{repo}"
	getOrgsTeamsReposByOrgBySlugByOwnerByRepo         = "GET /orgs/{org}/teams/{slug}/repos/{owner}/{repo}"
	deleteOrgsTeamsReposByOrgBySlugByOwnerByRepo      = "DELETE /orgs/{org}/teams/{slug}/repos/{owner}/{repo}"
)

func Test_CollaboratorsRead(t *testing.T) {
	t.Parallel()

	serverTool := CollaboratorsRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "collaborators_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "collaborators_read tool should be read-only")

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
		assertText         func(t *testing.T, text string)
	}{
		{
			name: "list collaborators reports the role each holds",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposCollaboratorsByOwnerByRepo, []*github.User{
					{
						Login:       github.Ptr("octocat"),
						ID:          github.Ptr(int64(1)),
						Type:        github.Ptr("User"),
						RoleName:    github.Ptr("maintain"),
						Permissions: &github.RepositoryPermissions{Pull: github.Ptr(true), Triage: github.Ptr(true), Push: github.Ptr(true), Maintain: github.Ptr(true)},
					},
				}),
			),
			requestArgs: map[string]any{
				"method": "list_collaborators",
				"owner":  "owner",
				"repo":   "repo",
			},
			assertText: func(t *testing.T, text string) {
				var collaborators []MinimalRepositoryAccess
				require.NoError(t, json.Unmarshal([]byte(text), &collaborators))
				require.Len(t, collaborators, 1)
				assert.Equal(t, "octocat", collaborators[0].Login)
				// The permission flags are cumulative; the highest one wins.
				assert.Equal(t, "maintain", collaborators[0].Permission)
			},
		},
		{
			name: "get_permission resolves one user",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposCollaboratorPermissionByOwnerByRepoByUser, &github.RepositoryPermissionLevel{
					Permission: github.Ptr("write"),
					RoleName:   github.Ptr("push"),
					User:       &github.User{Login: github.Ptr("octocat"), ID: github.Ptr(int64(1))},
				}),
			),
			requestArgs: map[string]any{
				"method":   "get_permission",
				"owner":    "owner",
				"repo":     "repo",
				"username": "octocat",
			},
			assertText: func(t *testing.T, text string) {
				var access MinimalRepositoryAccess
				require.NoError(t, json.Unmarshal([]byte(text), &access))
				assert.Equal(t, "octocat", access.Login)
				assert.Equal(t, "write", access.Permission)
				assert.Equal(t, "push", access.RoleName)
			},
		},
		{
			name:         "get_permission without a username is rejected",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "get_permission",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "username",
		},
		{
			name: "list_teams reports team access",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposTeamsByOwnerByRepo, []*github.Team{
					{
						Name:       github.Ptr("Platform"),
						Slug:       github.Ptr("platform"),
						ID:         github.Ptr(int64(5)),
						Permission: github.Ptr("push"),
					},
				}),
			),
			requestArgs: map[string]any{
				"method": "list_teams",
				"owner":  "owner",
				"repo":   "repo",
			},
			assertText: func(t *testing.T, text string) {
				var teams []MinimalRepositoryTeam
				require.NoError(t, json.Unmarshal([]byte(text), &teams))
				require.Len(t, teams, 1)
				assert.Equal(t, "platform", teams[0].Slug)
				assert.Equal(t, "push", teams[0].Permission)
			},
		},
		{
			name:         "unknown method",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "list_everyone",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "unknown method: list_everyone",
		},
		{
			name: "api failure is surfaced",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(getReposCollaboratorsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"message": "Must have push access"}`))
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "list_collaborators",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "failed to list collaborators",
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

func Test_CollaboratorWrite(t *testing.T) {
	t.Parallel()

	serverTool := CollaboratorWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "collaborator_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "collaborator_write tool should not be read-only")

	t.Run("adding a new user reports the pending invitation", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(putReposCollaboratorsByOwnerByRepoByUser,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "maintain", payload["permission"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(MustMarshal(&github.CollaboratorInvitation{
						ID:          github.Ptr(int64(11)),
						Permissions: github.Ptr("maintain"),
					}))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "add_user",
			"owner":      "owner",
			"repo":       "repo",
			"username":   "octocat",
			"permission": "maintain",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var change MinimalCollaboratorChange
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &change))
		assert.Equal(t, "invitation_sent", change.Result)
		assert.True(t, change.Pending)
		assert.Equal(t, "maintain", change.Permission)
	})

	t.Run("changing an existing collaborator reads the effective role back", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(putReposCollaboratorsByOwnerByRepoByUser,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					// 204 means the user was already a collaborator.
					w.WriteHeader(http.StatusNoContent)
				}),
			),
			WithRequestMatch(getReposCollaboratorPermissionByOwnerByRepoByUser, &github.RepositoryPermissionLevel{
				Permission: github.Ptr("admin"),
				RoleName:   github.Ptr("admin"),
				User:       &github.User{Login: github.Ptr("octocat")},
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "add_user",
			"owner":      "owner",
			"repo":       "repo",
			"username":   "octocat",
			"permission": "admin",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var change MinimalCollaboratorChange
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &change))
		assert.Equal(t, "permission_updated", change.Result)
		assert.False(t, change.Pending)
		assert.Equal(t, "admin", change.Permission)
	})

	t.Run("removing a user", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteReposCollaboratorsByOwnerByRepoByUser,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/repos/owner/repo/collaborators/octocat", r.URL.Path)
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "remove_user",
			"owner":    "owner",
			"repo":     "repo",
			"username": "octocat",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "collaborator_removed")
	})

	t.Run("granting a team access defaults the org to the repository owner", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(putOrgsTeamsReposByOrgBySlugByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/orgs/owner/teams/platform/repos/owner/repo", r.URL.Path)

					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "push", payload["permission"])

					w.WriteHeader(http.StatusNoContent)
				}),
			),
			WithRequestMatch(getOrgsTeamsReposByOrgBySlugByOwnerByRepo, &github.Repository{
				Permissions: &github.RepositoryPermissions{Pull: github.Ptr(true), Push: github.Ptr(true)},
				RoleName:    github.Ptr("push"),
			}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "set_team",
			"owner":      "owner",
			"repo":       "repo",
			"team_slug":  "platform",
			"permission": "push",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var change MinimalCollaboratorChange
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &change))
		assert.Equal(t, "team_access_set", change.Result)
		assert.Equal(t, "platform", change.TeamSlug)
		assert.Equal(t, "push", change.Permission)
	})

	t.Run("removing a team", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteOrgsTeamsReposByOrgBySlugByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/orgs/acme/teams/platform/repos/owner/repo", r.URL.Path)
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":    "remove_team",
			"owner":     "owner",
			"repo":      "repo",
			"org":       "acme",
			"team_slug": "platform",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "team_access_removed")
	})

	t.Run("set_team without a slug is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "set_team",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "team_slug")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "promote",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method: promote")
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(putReposCollaboratorsByOwnerByRepoByUser,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Must have admin rights"}`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":   "add_user",
			"owner":    "owner",
			"repo":     "repo",
			"username": "octocat",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to add 'octocat' as a collaborator")
	})
}
