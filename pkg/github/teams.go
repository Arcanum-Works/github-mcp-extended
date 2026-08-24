package github

import (
	"context"
	"fmt"
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
	teamsMethodList       = "list_teams"
	teamsMethodGet        = "get_team"
	teamsMethodMembership = "get_membership"
	teamsMethodListRepos  = "list_repos"

	teamWriteMethodCreate    = "create_team"
	teamWriteMethodUpdate    = "update_team"
	teamWriteMethodDelete    = "delete_team"
	teamWriteMethodAddMember = "add_member"
	teamWriteMethodRmMember  = "remove_member"
)

var teamMemberRoles = []any{"member", "maintainer"}

// MinimalTeam is the trimmed output type for a team.
type MinimalTeam struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	ID          int64  `json:"id,omitempty"`
	Description string `json:"description,omitempty"`
	Privacy     string `json:"privacy,omitempty"`
	Permission  string `json:"permission,omitempty"`
}

func convertToMinimalTeam(team *github.Team) MinimalTeam {
	return MinimalTeam{
		Name:        sanitize.Sanitize(team.GetName()),
		Slug:        team.GetSlug(),
		ID:          team.GetID(),
		Description: sanitize.Sanitize(team.GetDescription()),
		Privacy:     team.GetPrivacy(),
		Permission:  team.GetPermission(),
	}
}

// MinimalTeamMembership reports a single user's role and status on a team.
type MinimalTeamMembership struct {
	Login string `json:"login"`
	Role  string `json:"role,omitempty"`
	State string `json:"state,omitempty"`
}

func convertToMinimalTeamMembership(login string, m *github.Membership) MinimalTeamMembership {
	return MinimalTeamMembership{Login: login, Role: m.GetRole(), State: m.GetState()}
}

// MinimalTeamRepo is the trimmed output type for a repository a team can access.
type MinimalTeamRepo struct {
	FullName   string `json:"full_name"`
	Private    bool   `json:"private"`
	Permission string `json:"permission,omitempty"`
}

// TeamsRead creates a tool to inspect an organization's teams.
func TeamsRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataOrgs,
		mcp.Tool{
			Name: "teams_read",
			Description: t("TOOL_TEAMS_READ_DESCRIPTION", "Read team information: list an organization's teams, get one team's details, look up a user's membership in a team, or list the repositories a team can access."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_TEAMS_READ_USER_TITLE", "Read teams"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list_teams: all teams in the organization\n" +
							"- get_team: one team's details by slug\n" +
							"- get_membership: a user's role and status on a team\n" +
							"- list_repos: repositories the team can access",
						Enum: []any{teamsMethodList, teamsMethodGet, teamsMethodMembership, teamsMethodListRepos},
					},
					"org": {
						Type:        "string",
						Description: "Organization login that contains the team.",
					},
					"team_slug": {
						Type:        "string",
						Description: "Team slug as it appears in the team's URL. Required for all methods except 'list_teams'.",
					},
					"username": {
						Type:        "string",
						Description: "GitHub login. Required for 'get_membership'.",
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
			listOpts := &github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Team rosters and access are maintained by GitHub and visible only
			// to organization members, same reasoning as get_teams/get_team_members.
			label := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachStaticIFCLabel(ctx, deps, r, ifc.LabelTeam())
			}

			switch method {
			case teamsMethodList:
				teams, resp, err := client.Teams.ListTeams(ctx, org, listOpts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list teams", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalTeam, 0, len(teams))
				for _, team := range teams {
					if team != nil {
						minimal = append(minimal, convertToMinimalTeam(team))
					}
				}
				return marshalGovernanceResult(minimal, label)

			case teamsMethodGet:
				teamSlug, err := RequiredParam[string](args, "team_slug")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				team, resp, err := client.Teams.GetTeamBySlug(ctx, org, teamSlug)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get team '%s'", teamSlug), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalTeam(team), label)

			case teamsMethodMembership:
				teamSlug, err := RequiredParam[string](args, "team_slug")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				username, err := RequiredParam[string](args, "username")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				membership, resp, err := client.Teams.GetTeamMembershipBySlug(ctx, org, teamSlug, username)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get membership for '%s'", username), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalTeamMembership(username, membership), label)

			case teamsMethodListRepos:
				teamSlug, err := RequiredParam[string](args, "team_slug")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				repos, resp, err := client.Teams.ListTeamReposBySlug(ctx, org, teamSlug, listOpts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to list repositories for team '%s'", teamSlug), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalTeamRepo, 0, len(repos))
				for _, r := range repos {
					if r != nil {
						minimal = append(minimal, MinimalTeamRepo{
							FullName:   r.GetFullName(),
							Private:    r.GetPrivate(),
							Permission: highestPermission(r.GetPermissions()),
						})
					}
				}
				return marshalGovernanceResult(minimal, label)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list_teams, get_team, get_membership, list_repos", method)), nil, nil
			}
		},
	)
}

// TeamWrite creates a tool to create, update, delete a team, and manage its membership.
func TeamWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataOrgs,
		mcp.Tool{
			Name: "team_write",
			Description: t("TOOL_TEAM_WRITE_DESCRIPTION", "Create, update or delete a team, or manage its membership. "+
				"Use collaborator_write's set_team/remove_team methods to grant or revoke a team's access to a repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_TEAM_WRITE_USER_TITLE", "Manage teams"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The operation to perform:\n" +
							"- create_team: create a new team\n" +
							"- update_team: change a team's name, description or privacy\n" +
							"- delete_team: permanently delete a team\n" +
							"- add_member: add a user to a team, or change their role\n" +
							"- remove_member: remove a user from a team",
						Enum: []any{
							teamWriteMethodCreate, teamWriteMethodUpdate, teamWriteMethodDelete,
							teamWriteMethodAddMember, teamWriteMethodRmMember,
						},
					},
					"org": {
						Type:        "string",
						Description: "Organization login that contains the team.",
					},
					"team_slug": {
						Type:        "string",
						Description: "Team slug. Required for all methods except 'create_team'.",
					},
					"name": {
						Type:        "string",
						Description: "Team name. Required for 'create_team'; optional for 'update_team'.",
					},
					"description": {
						Type:        "string",
						Description: "Team description. Used by 'create_team' and 'update_team'.",
					},
					"privacy": {
						Type:        "string",
						Description: "'secret' (visible only to org owners and team members) or 'closed' (visible to the whole org). Used by 'create_team' and 'update_team'.",
						Enum:        []any{"secret", "closed"},
					},
					"username": {
						Type:        "string",
						Description: "GitHub login. Required for 'add_member' and 'remove_member'.",
					},
					"role": {
						Type:        "string",
						Description: "Role to grant a team member. Used by 'add_member'. Defaults to 'member'.",
						Enum:        teamMemberRoles,
					},
				},
				Required: []string{"method", "org"},
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case teamWriteMethodCreate:
				name, err := RequiredParam[string](args, "name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				description, err := OptionalParam[string](args, "description")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				privacy, err := OptionalParam[string](args, "privacy")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				newTeam := github.NewTeam{Name: name}
				if description != "" {
					newTeam.Description = &description
				}
				if privacy != "" {
					newTeam.Privacy = &privacy
				}

				team, resp, err := client.Teams.CreateTeam(ctx, org, newTeam)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to create team '%s'", name), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalTeam(team), nil)

			case teamWriteMethodUpdate:
				teamSlug, err := RequiredParam[string](args, "team_slug")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				name, err := OptionalParam[string](args, "name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				description, err := OptionalParam[string](args, "description")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				privacy, err := OptionalParam[string](args, "privacy")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				if name == "" {
					// EditTeamBySlug requires a Name even when unchanged.
					current, resp, err := client.Teams.GetTeamBySlug(ctx, org, teamSlug)
					if err != nil {
						return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to read team '%s' before update", teamSlug), resp, err), nil, nil
					}
					_ = resp.Body.Close()
					name = current.GetName()
				}
				newTeam := github.NewTeam{Name: name}
				if description != "" {
					newTeam.Description = &description
				}
				if privacy != "" {
					newTeam.Privacy = &privacy
				}

				team, resp, err := client.Teams.EditTeamBySlug(ctx, org, teamSlug, newTeam, false)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to update team '%s'", teamSlug), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalTeam(team), nil)

			case teamWriteMethodDelete:
				teamSlug, err := RequiredParam[string](args, "team_slug")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				resp, err := client.Teams.DeleteTeamBySlug(ctx, org, teamSlug)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to delete team '%s'", teamSlug), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(MinimalCollaboratorChange{Result: "team_deleted", TeamSlug: teamSlug}, nil)

			case teamWriteMethodAddMember:
				teamSlug, err := RequiredParam[string](args, "team_slug")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				username, err := RequiredParam[string](args, "username")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				role, err := OptionalParam[string](args, "role")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				membership, resp, err := client.Teams.AddTeamMembershipBySlug(ctx, org, teamSlug, username, &github.TeamAddTeamMembershipOptions{Role: role})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to add '%s' to team '%s'", username, teamSlug), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalTeamMembership(username, membership), nil)

			case teamWriteMethodRmMember:
				teamSlug, err := RequiredParam[string](args, "team_slug")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				username, err := RequiredParam[string](args, "username")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				resp, err := client.Teams.RemoveTeamMembershipBySlug(ctx, org, teamSlug, username)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to remove '%s' from team '%s'", username, teamSlug), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(MinimalCollaboratorChange{Result: "member_removed", Login: username, TeamSlug: teamSlug}, nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create_team, update_team, delete_team, add_member, remove_member", method)), nil, nil
			}
		},
	)
}
