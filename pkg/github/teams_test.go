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
	getOrgsTeamsByOrg                  = "GET /orgs/{org}/teams"
	getOrgsTeamsByOrgBySlug            = "GET /orgs/{org}/teams/{team_slug}"
	getOrgsTeamsMembershipsByOrgBySlug = "GET /orgs/{org}/teams/{team_slug}/memberships/{username}"
	getOrgsTeamsReposByOrgBySlug       = "GET /orgs/{org}/teams/{team_slug}/repos"
	postOrgsTeamsByOrg                 = "POST /orgs/{org}/teams"
	patchOrgsTeamsByOrgBySlug          = "PATCH /orgs/{org}/teams/{team_slug}"
	deleteOrgsTeamsByOrgBySlug         = "DELETE /orgs/{org}/teams/{team_slug}"
	putOrgsTeamsMembershipsByOrgBySlug = "PUT /orgs/{org}/teams/{team_slug}/memberships/{username}"
	deleteOrgsTeamsMembershipsBySlug   = "DELETE /orgs/{org}/teams/{team_slug}/memberships/{username}"
)

func Test_TeamsRead(t *testing.T) {
	t.Parallel()

	serverTool := TeamsRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "teams_read", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)

	tests := []struct {
		name         string
		mockedClient *http.Client
		requestArgs  map[string]any
		expectError  string
		assertText   func(t *testing.T, text string)
	}{
		{
			name: "list_teams",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getOrgsTeamsByOrg, []*github.Team{
					{Name: github.Ptr("Platform"), Slug: github.Ptr("platform"), ID: github.Ptr(int64(1)), Privacy: github.Ptr("closed")},
				}),
			),
			requestArgs: map[string]any{"method": "list_teams", "org": "acme"},
			assertText: func(t *testing.T, text string) {
				var teams []MinimalTeam
				require.NoError(t, json.Unmarshal([]byte(text), &teams))
				require.Len(t, teams, 1)
				assert.Equal(t, "platform", teams[0].Slug)
			},
		},
		{
			name: "get_team",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getOrgsTeamsByOrgBySlug, &github.Team{Name: github.Ptr("Platform"), Slug: github.Ptr("platform"), Privacy: github.Ptr("closed")}),
			),
			requestArgs: map[string]any{"method": "get_team", "org": "acme", "team_slug": "platform"},
			assertText: func(t *testing.T, text string) {
				var team MinimalTeam
				require.NoError(t, json.Unmarshal([]byte(text), &team))
				assert.Equal(t, "platform", team.Slug)
			},
		},
		{
			name: "get_membership",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getOrgsTeamsMembershipsByOrgBySlug, &github.Membership{Role: github.Ptr("maintainer"), State: github.Ptr("active")}),
			),
			requestArgs: map[string]any{"method": "get_membership", "org": "acme", "team_slug": "platform", "username": "octocat"},
			assertText: func(t *testing.T, text string) {
				var m MinimalTeamMembership
				require.NoError(t, json.Unmarshal([]byte(text), &m))
				assert.Equal(t, "octocat", m.Login)
				assert.Equal(t, "maintainer", m.Role)
			},
		},
		{
			name: "list_repos",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getOrgsTeamsReposByOrgBySlug, []*github.Repository{
					{FullName: github.Ptr("acme/widgets"), Private: github.Ptr(true), Permissions: &github.RepositoryPermissions{Pull: github.Ptr(true), Push: github.Ptr(true)}},
				}),
			),
			requestArgs: map[string]any{"method": "list_repos", "org": "acme", "team_slug": "platform"},
			assertText: func(t *testing.T, text string) {
				var repos []MinimalTeamRepo
				require.NoError(t, json.Unmarshal([]byte(text), &repos))
				require.Len(t, repos, 1)
				assert.Equal(t, "acme/widgets", repos[0].FullName)
				assert.Equal(t, "push", repos[0].Permission)
			},
		},
		{
			name:         "get_team without a slug is rejected",
			mockedClient: NewMockedHTTPClient(),
			requestArgs:  map[string]any{"method": "get_team", "org": "acme"},
			expectError:  "team_slug",
		},
		{
			name:         "unknown method",
			mockedClient: NewMockedHTTPClient(),
			requestArgs:  map[string]any{"method": "nope", "org": "acme"},
			expectError:  "unknown method",
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

			if tc.expectError != "" {
				assert.Contains(t, getErrorResult(t, result).Text, tc.expectError)
				return
			}
			require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
			tc.assertText(t, getTextResult(t, result).Text)
		})
	}
}

func Test_TeamWrite(t *testing.T) {
	t.Parallel()

	serverTool := TeamWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "team_write", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)

	t.Run("create_team", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(postOrgsTeamsByOrg, &github.Team{Name: github.Ptr("Platform"), Slug: github.Ptr("platform")}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "create_team", "org": "acme", "name": "Platform"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var team MinimalTeam
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &team))
		assert.Equal(t, "platform", team.Slug)
	})

	t.Run("update_team", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(patchOrgsTeamsByOrgBySlug, &github.Team{Name: github.Ptr("Platform"), Slug: github.Ptr("platform"), Description: github.Ptr("new desc")}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "update_team", "org": "acme", "team_slug": "platform", "name": "Platform", "description": "new desc"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "new desc")
	})

	t.Run("delete_team", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteOrgsTeamsByOrgBySlug, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "delete_team", "org": "acme", "team_slug": "platform"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "team_deleted")
	})

	t.Run("add_member", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(putOrgsTeamsMembershipsByOrgBySlug, &github.Membership{Role: github.Ptr("maintainer"), State: github.Ptr("active")}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "add_member", "org": "acme", "team_slug": "platform", "username": "octocat", "role": "maintainer"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var m MinimalTeamMembership
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &m))
		assert.Equal(t, "maintainer", m.Role)
	})

	t.Run("remove_member", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteOrgsTeamsMembershipsBySlug, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "remove_member", "org": "acme", "team_slug": "platform", "username": "octocat"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "member_removed")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "nope", "org": "acme"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method")
	})
}
