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
	getOrgsMembersByOrg           = "GET /orgs/{org}/members"
	getOrgsMembershipsByOrgByUser = "GET /orgs/{org}/memberships/{username}"
	putOrgsMembershipsByOrgByUser = "PUT /orgs/{org}/memberships/{username}"
	deleteOrgsMembersByOrgByUser  = "DELETE /orgs/{org}/members/{username}"
)

func Test_OrgMembersRead(t *testing.T) {
	t.Parallel()

	serverTool := OrgMembersRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "org_members_read", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)

	t.Run("list_members", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getOrgsMembersByOrg, []*github.User{{Login: github.Ptr("octocat"), ID: github.Ptr(int64(1))}}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "list_members", "org": "acme"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var members []MinimalOrgMember
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &members))
		require.Len(t, members, 1)
		assert.Equal(t, "octocat", members[0].Login)
	})

	t.Run("get_membership", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getOrgsMembershipsByOrgByUser, &github.Membership{Role: github.Ptr("admin"), State: github.Ptr("active")}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "get_membership", "org": "acme", "username": "octocat"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var m MinimalOrgMembership
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &m))
		assert.Equal(t, "admin", m.Role)
	})

	t.Run("get_membership without a username is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "get_membership", "org": "acme"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "username")
	})
}

func Test_OrgMemberWrite(t *testing.T) {
	t.Parallel()

	serverTool := OrgMemberWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "org_member_write", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)

	t.Run("update_membership", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(putOrgsMembershipsByOrgByUser, &github.Membership{Role: github.Ptr("admin"), State: github.Ptr("active")}),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "update_membership", "org": "acme", "username": "octocat", "role": "admin"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "admin")
	})

	t.Run("update_membership without a role is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "update_membership", "org": "acme", "username": "octocat"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "role")
	})

	t.Run("remove_member", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteOrgsMembersByOrgByUser, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"method": "remove_member", "org": "acme", "username": "octocat"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
		assert.Contains(t, getTextResult(t, result).Text, "member_removed")
	})
}
