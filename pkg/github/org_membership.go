package github

import (
	"context"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	orgMembersMethodList       = "list_members"
	orgMembersMethodMembership = "get_membership"

	orgMemberWriteMethodUpdate = "update_membership"
	orgMemberWriteMethodRemove = "remove_member"
)

var orgMemberRoles = []any{"member", "admin"}

// MinimalOrgMember is the trimmed output type for an organization member.
type MinimalOrgMember struct {
	Login string `json:"login"`
	ID    int64  `json:"id,omitempty"`
}

// MinimalOrgMembership reports a single user's role and status in an organization.
type MinimalOrgMembership struct {
	Login string `json:"login"`
	Role  string `json:"role,omitempty"`
	State string `json:"state,omitempty"`
}

func convertToMinimalOrgMembership(login string, m *github.Membership) MinimalOrgMembership {
	return MinimalOrgMembership{Login: login, Role: m.GetRole(), State: m.GetState()}
}

// OrgMembersRead creates a tool to inspect organization membership.
func OrgMembersRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataOrgs,
		mcp.Tool{
			Name:        "org_members_read",
			Description: t("TOOL_ORG_MEMBERS_READ_DESCRIPTION", "Read organization membership: list members, or get one user's role and status."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ORG_MEMBERS_READ_USER_TITLE", "Read organization members"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list_members: users who belong to the organization\n" +
							"- get_membership: one user's role and status",
						Enum: []any{orgMembersMethodList, orgMembersMethodMembership},
					},
					"org": {
						Type:        "string",
						Description: "Organization login.",
					},
					"username": {
						Type:        "string",
						Description: "GitHub login. Required for 'get_membership'.",
					},
					"role": {
						Type:        "string",
						Description: "Only list members holding this role. Used by 'list_members'. Defaults to 'all'.",
						Enum:        []any{"all", "admin", "member"},
					},
				},
				Required: []string{"method", "org"},
			}),
		},
		[]scopes.Scope{scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)

			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Org membership rosters are visible only to organization members,
			// same reasoning as team rosters.
			label := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachStaticIFCLabel(ctx, deps, r, ifc.LabelTeam())
			}

			switch method {
			case orgMembersMethodList:
				role, err := OptionalParam[string](args, "role")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				members, resp, err := client.Organizations.ListMembers(ctx, org, &github.ListMembersOptions{
					Role:        role,
					ListOptions: github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage},
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list organization members", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalOrgMember, 0, len(members))
				for _, m := range members {
					if m != nil {
						minimal = append(minimal, MinimalOrgMember{Login: m.GetLogin(), ID: m.GetID()})
					}
				}
				return marshalGovernanceResult(minimal, label)

			case orgMembersMethodMembership:
				username, err := RequiredParam[string](args, "username")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				membership, resp, err := client.Organizations.GetOrgMembership(ctx, username, org)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get membership for '%s'", username), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalOrgMembership(username, membership), label)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list_members, get_membership", method)), nil, nil
			}
		},
	)
}

// OrgMemberWrite creates a tool to change a member's role or remove them from the organization.
func OrgMemberWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataOrgs,
		mcp.Tool{
			Name: "org_member_write",
			Description: t("TOOL_ORG_MEMBER_WRITE_DESCRIPTION", "Change an organization member's role, or remove them from the organization. "+
				"Removal revokes access to every repository and team in the organization."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ORG_MEMBER_WRITE_USER_TITLE", "Manage organization members"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The operation to perform:\n" +
							"- update_membership: change a member's role\n" +
							"- remove_member: remove a user from the organization entirely",
						Enum: []any{orgMemberWriteMethodUpdate, orgMemberWriteMethodRemove},
					},
					"org": {
						Type:        "string",
						Description: "Organization login.",
					},
					"username": {
						Type:        "string",
						Description: "GitHub login.",
					},
					"role": {
						Type:        "string",
						Description: "Role to grant. Required for 'update_membership'.",
						Enum:        orgMemberRoles,
					},
				},
				Required: []string{"method", "org", "username"},
			},
		},
		[]scopes.Scope{scopes.WriteOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)

			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			username, err := RequiredParam[string](args, "username")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case orgMemberWriteMethodUpdate:
				role, err := RequiredParam[string](args, "role")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				membership, resp, err := client.Organizations.EditOrgMembership(ctx, username, org, &github.Membership{Role: &role})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to update membership for '%s'", username), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalOrgMembership(username, membership), nil)

			case orgMemberWriteMethodRemove:
				resp, err := client.Organizations.RemoveMember(ctx, org, username)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to remove '%s' from the organization", username), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(MinimalCollaboratorChange{Result: "member_removed", Login: username}, nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: update_membership, remove_member", method)), nil, nil
			}
		},
	)
}
