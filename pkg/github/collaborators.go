package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	collaboratorsMethodList        = "list_collaborators"
	collaboratorsMethodPermission  = "get_permission"
	collaboratorsMethodListTeams   = "list_teams"
	collaboratorWriteMethodAddUser = "add_user"
	collaboratorWriteMethodRmUser  = "remove_user"
	collaboratorWriteMethodSetTeam = "set_team"
	collaboratorWriteMethodRmTeam  = "remove_team"
)

// repositoryRoles are the roles GitHub accepts when granting repository access.
var repositoryRoles = []any{"pull", "triage", "push", "maintain", "admin"}

// MinimalRepositoryAccess is the trimmed output type for a repository collaborator.
type MinimalRepositoryAccess struct {
	Login      string `json:"login"`
	ID         int64  `json:"id,omitempty"`
	Type       string `json:"type,omitempty"`
	RoleName   string `json:"role_name,omitempty"`
	Permission string `json:"permission,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`
}

// MinimalRepositoryTeam is the trimmed output type for a team with repository
// access.
type MinimalRepositoryTeam struct {
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	ID         int64  `json:"id,omitempty"`
	Permission string `json:"permission,omitempty"`
	Privacy    string `json:"privacy,omitempty"`
}

// MinimalCollaboratorChange reports the state of an access grant after a write,
// so the caller sees what the repository now says rather than what was asked
// for.
type MinimalCollaboratorChange struct {
	Result     string `json:"result"`
	Login      string `json:"login,omitempty"`
	TeamSlug   string `json:"team_slug,omitempty"`
	Permission string `json:"permission,omitempty"`
	RoleName   string `json:"role_name,omitempty"`
	Pending    bool   `json:"pending_invitation,omitempty"`
}

// highestPermission collapses GitHub's permission flags into the single role
// they represent. The flags are cumulative — an admin also has push — so they
// are checked from most to least privileged and the first hit wins.
func highestPermission(permissions *github.RepositoryPermissions) string {
	if permissions == nil {
		return ""
	}
	switch {
	case permissions.GetAdmin():
		return "admin"
	case permissions.GetMaintain():
		return "maintain"
	case permissions.GetPush():
		return "push"
	case permissions.GetTriage():
		return "triage"
	case permissions.GetPull():
		return "pull"
	default:
		return ""
	}
}

func convertToMinimalRepositoryAccess(user *github.User) MinimalRepositoryAccess {
	return MinimalRepositoryAccess{
		Login:      user.GetLogin(),
		ID:         user.GetID(),
		Type:       user.GetType(),
		RoleName:   user.GetRoleName(),
		Permission: highestPermission(user.GetPermissions()),
		ProfileURL: user.GetHTMLURL(),
	}
}

func convertToMinimalRepositoryTeam(team *github.Team) MinimalRepositoryTeam {
	return MinimalRepositoryTeam{
		Name:       sanitize.Sanitize(team.GetName()),
		Slug:       team.GetSlug(),
		ID:         team.GetID(),
		Permission: team.GetPermission(),
		Privacy:    team.GetPrivacy(),
	}
}

// CollaboratorsRead creates a tool to inspect who has access to a repository.
func CollaboratorsRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepoGovernance,
		mcp.Tool{
			Name:        "collaborators_read",
			Description: t("TOOL_COLLABORATORS_READ_DESCRIPTION", "Read who has access to a repository: list collaborators, resolve one user's effective permission, or list the teams with repository access."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_COLLABORATORS_READ_USER_TITLE", "Read repository access"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list_collaborators: users with access, and the role each holds\n" +
							"- get_permission: one user's effective permission, including access inherited from a team or from organization membership\n" +
							"- list_teams: teams granted access to the repository",
						Enum: []any{collaboratorsMethodList, collaboratorsMethodPermission, collaboratorsMethodListTeams},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"username": {
						Type:        "string",
						Description: "GitHub login. Required for 'get_permission'.",
					},
					"affiliation": {
						Type:        "string",
						Description: "Which collaborators to list: 'direct' for those granted access on the repository itself, 'outside' for collaborators who are not organization members, 'all' (default) for everyone visible.",
						Enum:        []any{"outside", "direct", "all"},
					},
					"permission": {
						Type:        "string",
						Description: "Only list collaborators holding this role. Used by 'list_collaborators'.",
						Enum:        repositoryRoles,
					},
				},
				Required: []string{"method", "owner", "repo"},
			}),
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			listOpts := github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Who can reach a repository is roster data about real people; it
			// is never public, whatever the repository's visibility.
			label := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachStaticIFCLabel(ctx, deps, r, ifc.LabelCollaboratorRoster())
			}

			switch method {
			case collaboratorsMethodList:
				affiliation, err := OptionalParam[string](args, "affiliation")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				permission, err := OptionalParam[string](args, "permission")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				collaborators, resp, err := client.Repositories.ListCollaborators(ctx, owner, repo, &github.ListCollaboratorsOptions{
					Affiliation: affiliation,
					Permission:  permission,
					ListOptions: listOpts,
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list collaborators", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalRepositoryAccess, 0, len(collaborators))
				for _, user := range collaborators {
					if user != nil {
						minimal = append(minimal, convertToMinimalRepositoryAccess(user))
					}
				}

				return marshalGovernanceResult(minimal, label)

			case collaboratorsMethodPermission:
				username, err := RequiredParam[string](args, "username")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				level, resp, err := client.Repositories.GetPermissionLevel(ctx, owner, repo, username)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get the repository permission for '%s'", username), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalGovernanceResult(MinimalRepositoryAccess{
					Login:      level.GetUser().GetLogin(),
					ID:         level.GetUser().GetID(),
					Type:       level.GetUser().GetType(),
					Permission: level.GetPermission(),
					RoleName:   level.GetRoleName(),
					ProfileURL: level.GetUser().GetHTMLURL(),
				}, label)

			case collaboratorsMethodListTeams:
				teams, resp, err := client.Repositories.ListTeams(ctx, owner, repo, &listOpts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list repository teams", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalRepositoryTeam, 0, len(teams))
				for _, team := range teams {
					if team != nil {
						minimal = append(minimal, convertToMinimalRepositoryTeam(team))
					}
				}

				return marshalGovernanceResult(minimal, label)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list_collaborators, get_permission, list_teams", method)), nil, nil
			}
		},
	)
}

// CollaboratorWrite creates a tool to grant, change and revoke repository
// access for users and teams.
func CollaboratorWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepoGovernance,
		mcp.Tool{
			Name: "collaborator_write",
			Description: t("TOOL_COLLABORATOR_WRITE_DESCRIPTION", "Grant, change or revoke repository access for a user or a team. "+
				"Adding a user who is not already a member sends an invitation, which they must accept before the access takes effect. "+
				"Changing the role of an existing collaborator or team takes effect immediately."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_COLLABORATOR_WRITE_USER_TITLE", "Manage repository access"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The operation to perform:\n" +
							"- add_user: add a collaborator, or change the role of an existing one\n" +
							"- remove_user: revoke a user's direct access\n" +
							"- set_team: grant a team access, or change the role it holds\n" +
							"- remove_team: revoke a team's access",
						Enum: []any{
							collaboratorWriteMethodAddUser,
							collaboratorWriteMethodRmUser,
							collaboratorWriteMethodSetTeam,
							collaboratorWriteMethodRmTeam,
						},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"username": {
						Type:        "string",
						Description: "GitHub login. Required for 'add_user' and 'remove_user'.",
					},
					"team_slug": {
						Type:        "string",
						Description: "Team slug as it appears in the team's URL, not its display name. Required for 'set_team' and 'remove_team'.",
					},
					"org": {
						Type:        "string",
						Description: "Organization that owns the team. Defaults to the repository owner.",
					},
					"permission": {
						Type:        "string",
						Description: "Role to grant: 'pull' read, 'triage' manage issues and pull requests, 'push' write, 'maintain' manage without destructive actions, 'admin' full control. Defaults to 'push'.",
						Enum:        repositoryRoles,
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			permission, err := OptionalParam[string](args, "permission")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			org, err := OptionalParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if org == "" {
				org = owner
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case collaboratorWriteMethodAddUser:
				username, err := RequiredParam[string](args, "username")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				invitation, resp, err := client.Repositories.AddCollaborator(ctx, owner, repo, username, &github.RepositoryAddCollaboratorOptions{
					Permission: permission,
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to add '%s' as a collaborator", username), resp, err), nil, nil
				}
				statusCode := 0
				if resp != nil {
					statusCode = resp.StatusCode
					_ = resp.Body.Close()
				}

				// A 201 means an invitation was created; a 204 means the user
				// was already a collaborator and the role was updated in place.
				change := MinimalCollaboratorChange{Login: username}
				if statusCode == http.StatusCreated || invitation.GetID() != 0 {
					change.Result = "invitation_sent"
					change.Pending = true
					change.Permission = invitation.GetPermissions()
					return marshalGovernanceResult(change, nil)
				}

				change.Result = "permission_updated"
				level, resp, err := client.Repositories.GetPermissionLevel(ctx, owner, repo, username)
				if err != nil {
					// The change landed; only the read-back failed, so report
					// what was asked for rather than an error.
					change.Permission = permission
					return marshalGovernanceResult(change, nil)
				}
				defer func() { _ = resp.Body.Close() }()
				change.Permission = level.GetPermission()
				change.RoleName = level.GetRoleName()

				return marshalGovernanceResult(change, nil)

			case collaboratorWriteMethodRmUser:
				username, err := RequiredParam[string](args, "username")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				resp, err := client.Repositories.RemoveCollaborator(ctx, owner, repo, username)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to remove collaborator '%s'", username), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalGovernanceResult(MinimalCollaboratorChange{
					Result: "collaborator_removed",
					Login:  username,
				}, nil)

			case collaboratorWriteMethodSetTeam:
				teamSlug, err := RequiredParam[string](args, "team_slug")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				resp, err := client.Teams.AddTeamRepoBySlug(ctx, org, teamSlug, owner, repo, &github.TeamAddTeamRepoOptions{
					Permission: permission,
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to grant team '%s' access", teamSlug), resp, err), nil, nil
				}
				_ = resp.Body.Close()

				change := MinimalCollaboratorChange{
					Result:     "team_access_set",
					TeamSlug:   teamSlug,
					Permission: permission,
				}

				// Read the grant back so the caller sees the role the
				// repository actually recorded, including the default that
				// applies when no permission was named.
				repository, resp, err := client.Teams.IsTeamRepoBySlug(ctx, org, teamSlug, owner, repo)
				if err == nil {
					defer func() { _ = resp.Body.Close() }()
					if effective := highestPermission(repository.GetPermissions()); effective != "" {
						change.Permission = effective
					}
					change.RoleName = repository.GetRoleName()
				}

				return marshalGovernanceResult(change, nil)

			case collaboratorWriteMethodRmTeam:
				teamSlug, err := RequiredParam[string](args, "team_slug")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				resp, err := client.Teams.RemoveTeamRepoBySlug(ctx, org, teamSlug, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to revoke team '%s' access", teamSlug), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalGovernanceResult(MinimalCollaboratorChange{
					Result:   "team_access_removed",
					TeamSlug: teamSlug,
				}, nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: add_user, remove_user, set_team, remove_team", method)), nil, nil
			}
		},
	)
}
