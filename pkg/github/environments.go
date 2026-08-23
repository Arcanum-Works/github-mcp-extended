package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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
	environmentsMethodList        = "list_environments"
	environmentsMethodGet         = "get_environment"
	environmentWriteMethodSet     = "create_or_update"
	environmentWriteMethodDelete  = "delete"
	environmentBranchPolicyCustom = "custom"
)

// MinimalEnvironmentReviewer is a trimmed required reviewer on an environment.
type MinimalEnvironmentReviewer struct {
	Type  string `json:"type"`
	ID    int64  `json:"id,omitempty"`
	Login string `json:"login,omitempty"`
	Slug  string `json:"slug,omitempty"`
	Name  string `json:"name,omitempty"`
}

// MinimalEnvironment is the trimmed output type for a deployment environment,
// with the protection rules flattened into the settings that produced them.
type MinimalEnvironment struct {
	Name                    string                       `json:"name"`
	ID                      int64                        `json:"id,omitempty"`
	URL                     string                       `json:"html_url,omitempty"`
	WaitTimerMinutes        int                          `json:"wait_timer_minutes,omitempty"`
	RequiredReviewers       []MinimalEnvironmentReviewer `json:"required_reviewers,omitempty"`
	PreventSelfReview       bool                         `json:"prevent_self_review,omitempty"`
	CanAdminsBypass         bool                         `json:"can_admins_bypass"`
	ProtectedBranchesOnly   bool                         `json:"protected_branches_only"`
	CustomBranchPolicies    bool                         `json:"custom_branch_policies"`
	CustomBranchPatterns    []string                     `json:"custom_branch_patterns,omitempty"`
	DeploymentBranchPolicy  string                       `json:"deployment_branch_policy"`
	UpdatedAt               string                       `json:"updated_at,omitempty"`
	ProtectionRuleTypes     []string                     `json:"protection_rule_types,omitempty"`
	HasUnrepresentedRuleset bool                         `json:"has_unrepresented_protection_rules,omitempty"`
}

func convertToMinimalEnvironment(env *github.Environment) MinimalEnvironment {
	m := MinimalEnvironment{
		Name:            env.GetName(),
		ID:              env.GetID(),
		URL:             env.GetHTMLURL(),
		CanAdminsBypass: env.GetCanAdminsBypass(),
	}
	if env.Name == nil {
		m.Name = env.GetEnvironmentName()
	}
	if env.UpdatedAt != nil {
		m.UpdatedAt = env.UpdatedAt.Format(time.RFC3339)
	}

	if policy := env.DeploymentBranchPolicy; policy != nil {
		m.ProtectedBranchesOnly = policy.GetProtectedBranches()
		m.CustomBranchPolicies = policy.GetCustomBranchPolicies()
	}
	switch {
	case m.ProtectedBranchesOnly:
		m.DeploymentBranchPolicy = "protected_branches"
	case m.CustomBranchPolicies:
		m.DeploymentBranchPolicy = environmentBranchPolicyCustom
	default:
		m.DeploymentBranchPolicy = "all_branches"
	}

	// The wait timer and the reviewer list arrive as protection rules rather
	// than as the fields they were configured with, so they are unpacked back
	// into those fields here.
	for _, rule := range env.ProtectionRules {
		if rule == nil {
			continue
		}
		ruleType := rule.GetType()
		m.ProtectionRuleTypes = append(m.ProtectionRuleTypes, ruleType)

		switch ruleType {
		case "wait_timer":
			m.WaitTimerMinutes = rule.GetWaitTimer()
		case "required_reviewers":
			m.PreventSelfReview = rule.GetPreventSelfReview()
			for _, reviewer := range rule.Reviewers {
				if reviewer != nil {
					m.RequiredReviewers = append(m.RequiredReviewers, convertToMinimalEnvironmentReviewer(reviewer))
				}
			}
		case "branch_policy":
			// Already reported through DeploymentBranchPolicy.
		default:
			m.HasUnrepresentedRuleset = true
		}
	}

	return m
}

// convertToMinimalEnvironmentReviewer unpacks a required reviewer, whose
// concrete shape the API leaves as an untyped object.
func convertToMinimalEnvironmentReviewer(reviewer *github.RequiredReviewer) MinimalEnvironmentReviewer {
	m := MinimalEnvironmentReviewer{Type: reviewer.GetType()}

	// Reviewer is typed as any; re-marshaling is the least fragile way to read
	// whichever of the two shapes came back.
	raw, err := json.Marshal(reviewer.Reviewer)
	if err != nil {
		return m
	}
	var fields struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Slug  string `json:"slug"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return m
	}

	m.ID = fields.ID
	m.Login = fields.Login
	m.Slug = fields.Slug
	m.Name = fields.Name
	return m
}

// EnvironmentsRead creates a tool to read deployment environments.
func EnvironmentsRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDeployments,
		mcp.Tool{
			Name:        "environments_read",
			Description: t("TOOL_ENVIRONMENTS_READ_DESCRIPTION", "Read a repository's deployment environments and the protection rules guarding them: the wait timer, required reviewers, and which branches may deploy."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ENVIRONMENTS_READ_USER_TITLE", "Read deployment environments"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list_environments: every environment in the repository\n" +
							"- get_environment: one environment in full, including its custom branch patterns",
						Enum: []any{environmentsMethodList, environmentsMethodGet},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"environment_name": {
						Type:        "string",
						Description: "Environment name, e.g. 'staging'. Required for 'get_environment'.",
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			label := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelRepoMetadata)
			}

			switch method {
			case environmentsMethodList:
				envs, resp, err := client.Repositories.ListEnvironments(ctx, owner, repo, &github.EnvironmentListOptions{
					ListOptions: github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage},
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list environments", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalEnvironment, 0, len(envs.Environments))
				for _, env := range envs.Environments {
					if env != nil {
						minimal = append(minimal, convertToMinimalEnvironment(env))
					}
				}

				return marshalDeploymentResult(minimal, label)

			case environmentsMethodGet:
				name, err := RequiredParam[string](args, "environment_name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				env, resp, err := client.Repositories.GetEnvironment(ctx, owner, repo, name)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get environment '%s'", name), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := convertToMinimalEnvironment(env)
				if minimal.CustomBranchPolicies {
					// Custom patterns live behind their own endpoint, so the
					// environment alone would not say which branches qualify.
					patterns, errResult := listDeploymentBranchPatterns(ctx, client, owner, repo, name)
					if errResult != nil {
						return errResult, nil, nil
					}
					minimal.CustomBranchPatterns = patterns
				}

				return marshalDeploymentResult(minimal, label)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list_environments, get_environment", method)), nil, nil
			}
		},
	)
}

// EnvironmentWrite creates a tool to create, configure and delete deployment
// environments.
func EnvironmentWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDeployments,
		mcp.Tool{
			Name: "environment_write",
			Description: t("TOOL_ENVIRONMENT_WRITE_DESCRIPTION", "Create, configure or delete a deployment environment. "+
				"Creating and updating share one method: naming an environment that does not exist creates it. "+
				"Only the settings you name are changed; every omitted setting keeps its current value."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ENVIRONMENT_WRITE_USER_TITLE", "Configure deployment environments"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create_or_update' or 'delete'",
						Enum:        []any{environmentWriteMethodSet, environmentWriteMethodDelete},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"environment_name": {
						Type:        "string",
						Description: "Environment name, e.g. 'staging'. Created if it does not exist.",
					},
					"wait_timer_minutes": {
						Type:        "number",
						Description: "Minutes to delay before a deployment to this environment can proceed. 0 removes the delay.",
					},
					"prevent_self_review": {
						Type:        "boolean",
						Description: "Prevent the user who triggered a deployment from approving it.",
					},
					"can_admins_bypass": {
						Type:        "boolean",
						Description: "Allow repository admins to bypass the protection rules.",
					},
					"reviewers": {
						Type:        "array",
						Description: "Users and teams that must approve a deployment. Replaces the current list; an empty array removes the review requirement. Identify each reviewer by login, team slug, or numeric id.",
						Items: &jsonschema.Schema{
							Type: "object",
							Properties: map[string]*jsonschema.Schema{
								"type": {
									Type:        "string",
									Description: "Whether the reviewer is a user or a team.",
									Enum:        []any{"User", "Team"},
								},
								"login": {Type: "string", Description: "GitHub login, when type is 'User'."},
								"slug":  {Type: "string", Description: "Team slug, when type is 'Team'."},
								"id":    {Type: "number", Description: "Numeric user or team id, if already known."},
							},
							Required: []string{"type"},
						},
					},
					"deployment_branch_policy": {
						Type:        "string",
						Description: "Which branches may deploy: 'all_branches', 'protected_branches', or 'custom' to restrict to the patterns in custom_branch_patterns.",
						Enum:        []any{"all_branches", "protected_branches", environmentBranchPolicyCustom},
					},
					"custom_branch_patterns": {
						Type:        "array",
						Description: "Branch name patterns allowed to deploy, e.g. ['main', 'release/*']. Replaces the current patterns. Requires deployment_branch_policy 'custom'.",
						Items:       &jsonschema.Schema{Type: "string"},
					},
				},
				Required: []string{"method", "owner", "repo", "environment_name"},
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
			name, err := RequiredParam[string](args, "environment_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case environmentWriteMethodDelete:
				resp, err := client.Repositories.DeleteEnvironment(ctx, owner, repo, name)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to delete environment '%s'", name), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return utils.NewToolResultText(fmt.Sprintf("environment '%s' deleted successfully", name)), nil, nil

			case environmentWriteMethodSet:
				// The update endpoint clears any field sent as null, so the
				// current configuration is the starting point rather than an
				// empty request.
				update := &github.CreateUpdateEnvironment{}
				existing, resp, err := client.Repositories.GetEnvironment(ctx, owner, repo, name)
				// A 404 means the environment does not exist yet and this call
				// creates it, so there is nothing to preserve. Any other
				// failure, including one that produced no response at all,
				// leaves the current configuration unknown, and an update
				// built on that would clear every setting not named here.
				if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to read environment '%s' before updating it, so it was left unchanged", name), resp, err), nil, nil
				}
				if err == nil {
					_ = resp.Body.Close()
					current := convertToMinimalEnvironment(existing)
					update.WaitTimer = github.Ptr(current.WaitTimerMinutes)
					update.CanAdminsBypass = github.Ptr(current.CanAdminsBypass)
					update.PreventSelfReview = github.Ptr(current.PreventSelfReview)
					update.Reviewers = []*github.EnvReviewers{}
					for _, reviewer := range current.RequiredReviewers {
						update.Reviewers = append(update.Reviewers, &github.EnvReviewers{
							Type: github.Ptr(reviewer.Type),
							ID:   github.Ptr(reviewer.ID),
						})
					}
					if current.ProtectedBranchesOnly || current.CustomBranchPolicies {
						update.DeploymentBranchPolicy = &github.BranchPolicy{
							ProtectedBranches:    github.Ptr(current.ProtectedBranchesOnly),
							CustomBranchPolicies: github.Ptr(current.CustomBranchPolicies),
						}
					}
				}

				if raw, ok := args["wait_timer_minutes"]; ok && raw != nil {
					minutes, err := toInt(raw)
					if err != nil {
						return utils.NewToolResultError("wait_timer_minutes must be a number"), nil, nil
					}
					update.WaitTimer = github.Ptr(minutes)
				}
				for key, target := range map[string]**bool{
					"prevent_self_review": &update.PreventSelfReview,
					"can_admins_bypass":   &update.CanAdminsBypass,
				} {
					raw, ok := args[key]
					if !ok || raw == nil {
						continue
					}
					value, ok := raw.(bool)
					if !ok {
						return utils.NewToolResultError(fmt.Sprintf("%s must be a boolean", key)), nil, nil
					}
					*target = github.Ptr(value)
				}

				if raw, ok := args["reviewers"]; ok && raw != nil {
					reviewers, errResult := resolveEnvironmentReviewers(ctx, client, owner, raw)
					if errResult != nil {
						return errResult, nil, nil
					}
					update.Reviewers = reviewers
				}

				policy, err := OptionalParam[string](args, "deployment_branch_policy")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				patterns, patternsSet, err := optionalStringSlice(args, "custom_branch_patterns")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				if patternsSet && policy == "" {
					policy = environmentBranchPolicyCustom
				}
				switch policy {
				case "":
					// Left as read back from the existing environment.
				case "all_branches":
					update.DeploymentBranchPolicy = nil
				case "protected_branches":
					update.DeploymentBranchPolicy = &github.BranchPolicy{
						ProtectedBranches:    github.Ptr(true),
						CustomBranchPolicies: github.Ptr(false),
					}
				case environmentBranchPolicyCustom:
					update.DeploymentBranchPolicy = &github.BranchPolicy{
						ProtectedBranches:    github.Ptr(false),
						CustomBranchPolicies: github.Ptr(true),
					}
				default:
					return utils.NewToolResultError(fmt.Sprintf("unknown deployment_branch_policy: %s", policy)), nil, nil
				}

				env, resp, err := client.Repositories.CreateUpdateEnvironment(ctx, owner, repo, name, update)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to configure environment '%s'", name), resp, err), nil, nil
				}
				_ = resp.Body.Close()

				minimal := convertToMinimalEnvironment(env)
				if minimal.Name == "" {
					minimal.Name = name
				}

				if patternsSet {
					applied, errResult := replaceDeploymentBranchPatterns(ctx, client, owner, repo, name, patterns)
					if errResult != nil {
						return errResult, nil, nil
					}
					minimal.CustomBranchPatterns = applied
				} else if minimal.CustomBranchPolicies {
					existingPatterns, errResult := listDeploymentBranchPatterns(ctx, client, owner, repo, name)
					if errResult != nil {
						return errResult, nil, nil
					}
					minimal.CustomBranchPatterns = existingPatterns
				}

				return marshalDeploymentResult(minimal, nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create_or_update, delete", method)), nil, nil
			}
		},
	)
}

// resolveEnvironmentReviewers turns reviewer descriptions into the numeric IDs
// the API requires, looking up a login or team slug when no ID was given.
func resolveEnvironmentReviewers(ctx context.Context, client *github.Client, owner string, raw any) ([]*github.EnvReviewers, *mcp.CallToolResult) {
	entries, ok := raw.([]any)
	if !ok {
		return nil, utils.NewToolResultError("reviewers must be an array")
	}

	reviewers := make([]*github.EnvReviewers, 0, len(entries))
	for i, entry := range entries {
		fields, ok := entry.(map[string]any)
		if !ok {
			return nil, utils.NewToolResultError(fmt.Sprintf("reviewers[%d] must be an object", i))
		}
		reviewerType, ok := fields["type"].(string)
		if !ok || reviewerType == "" {
			return nil, utils.NewToolResultError(fmt.Sprintf("reviewers[%d].type is required", i))
		}

		if rawID, ok := fields["id"]; ok && rawID != nil {
			id, err := toInt(rawID)
			if err != nil {
				return nil, utils.NewToolResultError(fmt.Sprintf("reviewers[%d].id must be a number", i))
			}
			reviewers = append(reviewers, &github.EnvReviewers{
				Type: github.Ptr(reviewerType),
				ID:   github.Ptr(int64(id)),
			})
			continue
		}

		switch reviewerType {
		case "User":
			login, ok := fields["login"].(string)
			if !ok || login == "" {
				return nil, utils.NewToolResultError(fmt.Sprintf("reviewers[%d] needs a login or an id", i))
			}
			user, resp, err := client.Users.Get(ctx, login)
			if err != nil {
				return nil, ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to resolve reviewer '%s'", login), resp, err)
			}
			_ = resp.Body.Close()
			reviewers = append(reviewers, &github.EnvReviewers{
				Type: github.Ptr("User"),
				ID:   github.Ptr(user.GetID()),
			})
		case "Team":
			slug, ok := fields["slug"].(string)
			if !ok || slug == "" {
				return nil, utils.NewToolResultError(fmt.Sprintf("reviewers[%d] needs a slug or an id", i))
			}
			team, resp, err := client.Teams.GetTeamBySlug(ctx, owner, slug)
			if err != nil {
				return nil, ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to resolve reviewer team '%s'", slug), resp, err)
			}
			_ = resp.Body.Close()
			reviewers = append(reviewers, &github.EnvReviewers{
				Type: github.Ptr("Team"),
				ID:   github.Ptr(team.GetID()),
			})
		default:
			return nil, utils.NewToolResultError(fmt.Sprintf("reviewers[%d].type must be 'User' or 'Team'", i))
		}
	}

	return reviewers, nil
}

// deploymentBranchPattern is one custom branch policy reduced to what callers
// here need: the pattern itself, and the id that removes it.
type deploymentBranchPattern struct {
	name string
	id   int64
}

// listDeploymentBranchPolicies reads every custom branch policy on an
// environment. The endpoint is paginated, so stopping at the first page would
// leave later patterns invisible: reconciliation would recreate the ones it
// could not see, and keep the ones it should have removed.
func listDeploymentBranchPolicies(ctx context.Context, client *github.Client, owner, repo, environment string) ([]deploymentBranchPattern, *mcp.CallToolResult) {
	opts := &github.ListOptions{PerPage: 100}
	var all []deploymentBranchPattern
	currentPage := 1

	for {
		policies, resp, err := client.Repositories.ListDeploymentBranchPolicies(ctx, owner, repo, environment, opts)
		if err != nil {
			return nil, ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list deployment branch policies", resp, err)
		}
		for _, policy := range policies.BranchPolicies {
			if policy != nil {
				all = append(all, deploymentBranchPattern{name: policy.GetName(), id: policy.GetID()})
			}
		}

		nextPage := 0
		if resp != nil {
			nextPage = resp.NextPage
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
		}
		// Truncating here would reintroduce exactly the bug this loop exists
		// to fix, so the only stop is the end of the pages. A page number that
		// does not advance would be the API contradicting itself; stop rather
		// than spin.
		if nextPage == 0 || nextPage <= currentPage {
			break
		}
		currentPage = nextPage
		opts.Page = nextPage
	}

	return all, nil
}

func listDeploymentBranchPatterns(ctx context.Context, client *github.Client, owner, repo, environment string) ([]string, *mcp.CallToolResult) {
	policies, errResult := listDeploymentBranchPolicies(ctx, client, owner, repo, environment)
	if errResult != nil {
		return nil, errResult
	}

	patterns := make([]string, 0, len(policies))
	for _, policy := range policies {
		patterns = append(patterns, policy.name)
	}
	return patterns, nil
}

// replaceDeploymentBranchPatterns makes the environment's custom branch
// patterns match the requested set, adding what is missing and removing what is
// no longer wanted. The API has no bulk form, so this is a diff rather than a
// wholesale replace.
func replaceDeploymentBranchPatterns(ctx context.Context, client *github.Client, owner, repo, environment string, wanted []string) ([]string, *mcp.CallToolResult) {
	// Reading every page first is what makes this a diff: a policy that was
	// not read looks like one that does not exist, and would be created a
	// second time or left behind when it should have been removed.
	policies, errResult := listDeploymentBranchPolicies(ctx, client, owner, repo, environment)
	if errResult != nil {
		return nil, errResult
	}

	existing := map[string]bool{}
	for _, policy := range policies {
		existing[policy.name] = true
	}

	keep := map[string]bool{}
	applied := make([]string, 0, len(wanted))
	for _, pattern := range wanted {
		if keep[pattern] {
			// The same pattern named twice is one pattern.
			continue
		}
		keep[pattern] = true
		applied = append(applied, pattern)
		if existing[pattern] {
			continue
		}
		_, resp, err := client.Repositories.CreateDeploymentBranchPolicy(ctx, owner, repo, environment, github.CreateDeploymentBranchPolicyRequest{
			Name: pattern,
		})
		if err != nil {
			return nil, ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to allow branch pattern '%s'", pattern), resp, err)
		}
		_ = resp.Body.Close()
	}

	// Iterating the read order rather than a map keeps removals deterministic.
	for _, policy := range policies {
		if keep[policy.name] {
			continue
		}
		resp, err := client.Repositories.DeleteDeploymentBranchPolicy(ctx, owner, repo, environment, policy.id)
		if err != nil {
			return nil, ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to remove branch pattern '%s'", policy.name), resp, err)
		}
		_ = resp.Body.Close()
	}

	return applied, nil
}
