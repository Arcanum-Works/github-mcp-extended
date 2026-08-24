package github

import (
	"context"
	"encoding/json"
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
	rulesetsMethodList            = "list_rulesets"
	rulesetsMethodGet             = "get_ruleset"
	rulesetsMethodRulesForBranch  = "get_branch_rules"
	rulesetWriteMethodCreate      = "create"
	rulesetWriteMethodUpdate      = "update"
	rulesetWriteMethodDelete      = "delete"
	rulesetWriteMethodSetEnforcem = "set_enforcement"
)

// mergeQueueDefaults are the values GitHub's UI applies when a merge queue is
// switched on without further configuration. The API rejects a partial
// merge_queue object, so an agent that just asks for "merge queue on" needs a
// complete set of parameters from somewhere.
var mergeQueueDefaults = github.MergeQueueRuleParameters{
	CheckResponseTimeoutMinutes:  60,
	GroupingStrategy:             github.MergeGroupingStrategyAllGreen,
	MaxEntriesToBuild:            5,
	MaxEntriesToMerge:            5,
	MergeMethod:                  github.MergeQueueMergeMethodMerge,
	MinEntriesToMerge:            1,
	MinEntriesToMergeWaitMinutes: 5,
}

// MinimalRulesetConditions is the trimmed ref-matching condition of a ruleset.
type MinimalRulesetConditions struct {
	IncludeRefs []string `json:"include_refs,omitempty"`
	ExcludeRefs []string `json:"exclude_refs,omitempty"`
}

// MinimalRulesetMergeQueue is the trimmed merge queue configuration.
type MinimalRulesetMergeQueue struct {
	MergeMethod                  string `json:"merge_method,omitempty"`
	GroupingStrategy             string `json:"grouping_strategy,omitempty"`
	CheckResponseTimeoutMinutes  int    `json:"check_response_timeout_minutes,omitempty"`
	MaxEntriesToBuild            int    `json:"max_entries_to_build,omitempty"`
	MaxEntriesToMerge            int    `json:"max_entries_to_merge,omitempty"`
	MinEntriesToMerge            int    `json:"min_entries_to_merge,omitempty"`
	MinEntriesToMergeWaitMinutes int    `json:"min_entries_to_merge_wait_minutes,omitempty"`
}

// MinimalRulesetRules is the trimmed rule set of a ruleset, flattened into the
// governance questions an agent actually asks: is a PR required, how many
// approvals, which checks must pass, what is blocked outright.
type MinimalRulesetRules struct {
	RequirePullRequest              bool                      `json:"require_pull_request"`
	RequiredApprovingReviewCount    int                       `json:"required_approving_review_count,omitempty"`
	DismissStaleReviewsOnPush       bool                      `json:"dismiss_stale_reviews_on_push,omitempty"`
	RequireCodeOwnerReview          bool                      `json:"require_code_owner_review,omitempty"`
	RequireLastPushApproval         bool                      `json:"require_last_push_approval,omitempty"`
	RequiredReviewThreadResolution  bool                      `json:"required_review_thread_resolution,omitempty"`
	AllowedMergeMethods             []string                  `json:"allowed_merge_methods,omitempty"`
	RequiredStatusChecks            []string                  `json:"required_status_checks,omitempty"`
	StrictRequiredStatusChecks      bool                      `json:"strict_required_status_checks_policy,omitempty"`
	BlockForcePushes                bool                      `json:"block_force_pushes"`
	BlockDeletions                  bool                      `json:"block_deletions"`
	BlockCreations                  bool                      `json:"block_creations,omitempty"`
	RequireSignedCommits            bool                      `json:"require_signed_commits,omitempty"`
	RequireLinearHistory            bool                      `json:"require_linear_history,omitempty"`
	RequiredDeploymentEnvironments  []string                  `json:"required_deployment_environments,omitempty"`
	MergeQueue                      *MinimalRulesetMergeQueue `json:"merge_queue,omitempty"`
	UnrepresentedRulesArePreserved  bool                      `json:"unrepresented_rules_are_preserved,omitempty"`
	UnrepresentedRuleNamesPreserved []string                  `json:"unrepresented_rule_names,omitempty"`
}

// MinimalBypassActor is the trimmed bypass entry of a ruleset.
type MinimalBypassActor struct {
	ActorID    int64  `json:"actor_id,omitempty"`
	ActorType  string `json:"actor_type,omitempty"`
	BypassMode string `json:"bypass_mode,omitempty"`
}

// MinimalRuleset is the trimmed output type for repository rulesets.
type MinimalRuleset struct {
	ID           int64                    `json:"id"`
	Name         string                   `json:"name"`
	Target       string                   `json:"target,omitempty"`
	Enforcement  string                   `json:"enforcement"`
	SourceType   string                   `json:"source_type,omitempty"`
	Source       string                   `json:"source,omitempty"`
	Conditions   MinimalRulesetConditions `json:"conditions"`
	Rules        *MinimalRulesetRules     `json:"rules,omitempty"`
	BypassActors []MinimalBypassActor     `json:"bypass_actors,omitempty"`
	UpdatedAt    string                   `json:"updated_at,omitempty"`
}

func convertToMinimalRuleset(ruleset *github.RepositoryRuleset) MinimalRuleset {
	m := MinimalRuleset{
		ID:          ruleset.GetID(),
		Name:        sanitize.Sanitize(ruleset.Name),
		Enforcement: string(ruleset.Enforcement),
		Source:      ruleset.Source,
	}
	if ruleset.Target != nil {
		m.Target = string(*ruleset.Target)
	}
	if ruleset.SourceType != nil {
		m.SourceType = string(*ruleset.SourceType)
	}
	if ruleset.UpdatedAt != nil {
		m.UpdatedAt = ruleset.UpdatedAt.Format("2006-01-02T15:04:05Z")
	}
	if ruleset.Conditions != nil && ruleset.Conditions.RefName != nil {
		m.Conditions.IncludeRefs = ruleset.Conditions.RefName.Include
		m.Conditions.ExcludeRefs = ruleset.Conditions.RefName.Exclude
	}
	for _, actor := range ruleset.BypassActors {
		if actor == nil {
			continue
		}
		entry := MinimalBypassActor{ActorID: actor.GetActorID()}
		if actor.ActorType != nil {
			entry.ActorType = string(*actor.ActorType)
		}
		if actor.BypassMode != nil {
			entry.BypassMode = string(*actor.BypassMode)
		}
		m.BypassActors = append(m.BypassActors, entry)
	}
	m.Rules = convertToMinimalRulesetRules(ruleset.Rules)
	return m
}

func convertToMinimalRulesetRules(rules *github.RepositoryRulesetRules) *MinimalRulesetRules {
	if rules == nil {
		return nil
	}

	m := &MinimalRulesetRules{
		BlockForcePushes:     rules.NonFastForward != nil,
		BlockDeletions:       rules.Deletion != nil,
		BlockCreations:       rules.Creation != nil,
		RequireSignedCommits: rules.RequiredSignatures != nil,
		RequireLinearHistory: rules.RequiredLinearHistory != nil,
	}

	if pr := rules.PullRequest; pr != nil {
		m.RequirePullRequest = true
		m.RequiredApprovingReviewCount = pr.RequiredApprovingReviewCount
		m.DismissStaleReviewsOnPush = pr.DismissStaleReviewsOnPush
		m.RequireCodeOwnerReview = pr.RequireCodeOwnerReview
		m.RequireLastPushApproval = pr.RequireLastPushApproval
		m.RequiredReviewThreadResolution = pr.RequiredReviewThreadResolution
		for _, method := range pr.AllowedMergeMethods {
			m.AllowedMergeMethods = append(m.AllowedMergeMethods, string(method))
		}
	}

	if checks := rules.RequiredStatusChecks; checks != nil {
		m.StrictRequiredStatusChecks = checks.StrictRequiredStatusChecksPolicy
		for _, check := range checks.RequiredStatusChecks {
			if check != nil {
				m.RequiredStatusChecks = append(m.RequiredStatusChecks, check.Context)
			}
		}
	}

	if deployments := rules.RequiredDeployments; deployments != nil {
		m.RequiredDeploymentEnvironments = deployments.RequiredDeploymentEnvironments
	}

	if queue := rules.MergeQueue; queue != nil {
		m.MergeQueue = &MinimalRulesetMergeQueue{
			MergeMethod:                  string(queue.MergeMethod),
			GroupingStrategy:             string(queue.GroupingStrategy),
			CheckResponseTimeoutMinutes:  queue.CheckResponseTimeoutMinutes,
			MaxEntriesToBuild:            queue.MaxEntriesToBuild,
			MaxEntriesToMerge:            queue.MaxEntriesToMerge,
			MinEntriesToMerge:            queue.MinEntriesToMerge,
			MinEntriesToMergeWaitMinutes: queue.MinEntriesToMergeWaitMinutes,
		}
	}

	// Rules this tool has no vocabulary for still exist on the ruleset and are
	// carried through updates untouched. Naming them keeps the summary honest
	// about being a projection rather than the whole object.
	if names := unrepresentedRuleNames(rules); len(names) > 0 {
		m.UnrepresentedRulesArePreserved = true
		m.UnrepresentedRuleNamesPreserved = names
	}

	return m
}

// unrepresentedRuleNames lists the rules present on a ruleset that
// MinimalRulesetRules does not model.
func unrepresentedRuleNames(rules *github.RepositoryRulesetRules) []string {
	var names []string
	add := func(present bool, name string) {
		if present {
			names = append(names, name)
		}
	}
	add(rules.Update != nil, "update")
	add(rules.CommitMessagePattern != nil, "commit_message_pattern")
	add(rules.CommitAuthorEmailPattern != nil, "commit_author_email_pattern")
	add(rules.CommitterEmailPattern != nil, "committer_email_pattern")
	add(rules.BranchNamePattern != nil, "branch_name_pattern")
	add(rules.TagNamePattern != nil, "tag_name_pattern")
	add(rules.Workflows != nil, "workflows")
	add(rules.CodeScanning != nil, "code_scanning")
	add(rules.CopilotCodeReview != nil, "copilot_code_review")
	add(rules.FileExtensionRestriction != nil, "file_extension_restriction")
	add(rules.FilePathRestriction != nil, "file_path_restriction")
	add(rules.MaxFilePathLength != nil, "max_file_path_length")
	add(rules.MaxFileSize != nil, "max_file_size")
	return names
}

// RulesetsRead creates a tool to read repository rulesets and the rules in
// force on a given branch.
func RulesetsRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepoGovernance,
		mcp.Tool{
			Name:        "rulesets_read",
			Description: t("TOOL_RULESETS_READ_DESCRIPTION", "Read the rulesets governing a repository: list them, get one by ID or name, or ask which rules are actually in force on a branch (including rules inherited from organization or enterprise rulesets)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_RULESETS_READ_USER_TITLE", "Read repository rulesets"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list_rulesets: every ruleset applying to the repository\n" +
							"- get_ruleset: one ruleset in full, by ruleset_id or by name\n" +
							"- get_branch_rules: the rules in force on a branch, whatever ruleset they come from",
						Enum: []any{rulesetsMethodList, rulesetsMethodGet, rulesetsMethodRulesForBranch},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"ruleset_id": {
						Type:        "number",
						Description: "Numeric ruleset ID. Identifies the ruleset for 'get_ruleset'; takes precedence over name.",
					},
					"name": {
						Type:        "string",
						Description: "Ruleset name. Identifies the ruleset for 'get_ruleset' when ruleset_id is not given.",
					},
					"branch": {
						Type:        "string",
						Description: "Branch name. Required for 'get_branch_rules'.",
					},
					"includes_parents": {
						Type:        "boolean",
						Description: "Include rulesets inherited from the organization or enterprise. Defaults to true. Used by 'list_rulesets' and 'get_ruleset'.",
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
			includesParents, err := OptionalBoolParamWithDefault(args, "includes_parents", true)
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
				// Rulesets are governance metadata written by admins;
				// confidentiality follows repo visibility.
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelRepoMetadata)
			}

			switch method {
			case rulesetsMethodList:
				rulesets, resp, err := client.Repositories.GetAllRulesets(ctx, owner, repo, &github.RepositoryListRulesetsOptions{
					IncludesParents: github.Ptr(includesParents),
					ListOptions:     github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage},
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list rulesets", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalRuleset, 0, len(rulesets))
				for _, ruleset := range rulesets {
					if ruleset != nil {
						minimal = append(minimal, convertToMinimalRuleset(ruleset))
					}
				}

				return marshalGovernanceResult(minimal, label)

			case rulesetsMethodGet:
				id, errResult, err := resolveRulesetID(ctx, client, owner, repo, args)
				if errResult != nil || err != nil {
					return errResult, nil, err
				}

				ruleset, resp, err := client.Repositories.GetRuleset(ctx, owner, repo, id, includesParents)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get ruleset", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalGovernanceResult(convertToMinimalRuleset(ruleset), label)

			case rulesetsMethodRulesForBranch:
				branch, err := RequiredParam[string](args, "branch")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				rules, resp, err := client.Repositories.ListRulesForBranch(ctx, owner, repo, branch, &github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get rules for branch", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalGovernanceResult(convertToMinimalBranchRules(branch, rules), label)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list_rulesets, get_ruleset, get_branch_rules", method)), nil, nil
			}
		},
	)
}

// RulesetWrite creates a tool to create, update and delete repository rulesets.
func RulesetWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepoGovernance,
		mcp.Tool{
			Name: "ruleset_write",
			Description: t("TOOL_RULESET_WRITE_DESCRIPTION", "Create, update or delete a repository ruleset, or switch its enforcement on and off. "+
				"Updates are additive: the current ruleset is read first and only the fields you name are changed, so an update cannot silently drop protections it did not mention. "+
				"Rulesets are the modern replacement for branch protection; prefer them."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_RULESET_WRITE_USER_TITLE", "Write repository rulesets"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create', 'update', 'delete', or 'set_enforcement'",
						Enum: []any{
							rulesetWriteMethodCreate,
							rulesetWriteMethodUpdate,
							rulesetWriteMethodDelete,
							rulesetWriteMethodSetEnforcem,
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
					"ruleset_id": {
						Type:        "number",
						Description: "Numeric ruleset ID. Identifies the ruleset for 'update', 'delete' and 'set_enforcement'; takes precedence over name.",
					},
					"name": {
						Type:        "string",
						Description: "Ruleset name. Required for 'create'. For the other methods it identifies the ruleset when ruleset_id is not given; combined with ruleset_id on 'update' it renames the ruleset.",
					},
					"target": {
						Type:        "string",
						Description: "What the ruleset applies to. Defaults to 'branch' on create.",
						Enum:        []any{"branch", "tag", "push"},
					},
					"enforcement": {
						Type:        "string",
						Description: "Whether the ruleset is enforced. 'evaluate' reports violations without blocking (available on some plans). Defaults to 'active' on create. Required for 'set_enforcement'.",
						Enum:        []any{"active", "evaluate", "disabled"},
					},
					"include_refs": {
						Type:        "array",
						Description: "Ref patterns the ruleset applies to, e.g. ['refs/heads/main', 'refs/heads/release/*'] or the special value '~DEFAULT_BRANCH' or '~ALL'. Replaces the current include list when given.",
						Items:       &jsonschema.Schema{Type: "string"},
					},
					"exclude_refs": {
						Type:        "array",
						Description: "Ref patterns exempted from the ruleset. Replaces the current exclude list when given.",
						Items:       &jsonschema.Schema{Type: "string"},
					},
					"rules": {
						Type:        "object",
						Description: "The rules to enforce. Only the keys you provide are changed; every other rule on the ruleset, including rules this tool does not model, is preserved.",
						Properties: map[string]*jsonschema.Schema{
							"require_pull_request": {
								Type:        "boolean",
								Description: "Require changes to go through a pull request. Setting false removes the pull request rule and every review requirement with it.",
							},
							"required_approving_review_count": {
								Type:        "number",
								Description: "Approving reviews needed before merge. Implies require_pull_request.",
							},
							"dismiss_stale_reviews_on_push": {
								Type:        "boolean",
								Description: "Dismiss existing approvals when new commits are pushed. Implies require_pull_request.",
							},
							"require_code_owner_review": {
								Type:        "boolean",
								Description: "Require review from a CODEOWNERS owner. Implies require_pull_request.",
							},
							"require_last_push_approval": {
								Type:        "boolean",
								Description: "Require the most recent push to be approved by someone other than its author. Implies require_pull_request.",
							},
							"required_review_thread_resolution": {
								Type:        "boolean",
								Description: "Require all review conversations to be resolved before merge. Implies require_pull_request.",
							},
							"allowed_merge_methods": {
								Type:        "array",
								Description: "Merge methods permitted for pull requests under this rule. Implies require_pull_request.",
								Items:       &jsonschema.Schema{Type: "string", Enum: []any{"merge", "squash", "rebase"}},
							},
							"required_status_checks": {
								Type:        "array",
								Description: "Status check contexts that must pass. An empty array removes the required-checks rule entirely.",
								Items:       &jsonschema.Schema{Type: "string"},
							},
							"strict_required_status_checks_policy": {
								Type:        "boolean",
								Description: "Require the branch to be up to date with its base before merging.",
							},
							"block_force_pushes": {
								Type:        "boolean",
								Description: "Block force pushes to matching refs.",
							},
							"block_deletions": {
								Type:        "boolean",
								Description: "Block deletion of matching refs.",
							},
							"block_creations": {
								Type:        "boolean",
								Description: "Block creation of matching refs.",
							},
							"require_signed_commits": {
								Type:        "boolean",
								Description: "Require commits to be signed.",
							},
							"require_linear_history": {
								Type:        "boolean",
								Description: "Require a linear history, blocking merge commits.",
							},
							"required_deployment_environments": {
								Type:        "array",
								Description: "Environments that must have a successful deployment before merge. An empty array removes the rule.",
								Items:       &jsonschema.Schema{Type: "string"},
							},
							"merge_queue": {
								Type:        "object",
								Description: "Merge queue configuration. Pass {\"enabled\": false} to remove it; any omitted setting falls back to GitHub's default rather than being left unset, because the API rejects a partial merge queue.",
								Properties: map[string]*jsonschema.Schema{
									"enabled": {
										Type:        "boolean",
										Description: "Whether the merge queue is required. Defaults to true when the object is present.",
									},
									"merge_method": {
										Type:        "string",
										Description: "How the queue merges entries.",
										Enum:        []any{"MERGE", "SQUASH", "REBASE"},
									},
									"grouping_strategy": {
										Type:        "string",
										Description: "ALLGREEN requires every entry in the group to pass; HEADGREEN only the newest.",
										Enum:        []any{"ALLGREEN", "HEADGREEN"},
									},
									"check_response_timeout_minutes": {Type: "number", Description: "Minutes to wait for checks to report before the entry is dropped."},
									"max_entries_to_build":           {Type: "number", Description: "Maximum entries built at once."},
									"max_entries_to_merge":           {Type: "number", Description: "Maximum entries merged at once."},
									"min_entries_to_merge":           {Type: "number", Description: "Minimum entries before a merge is attempted."},
									"min_entries_to_merge_wait_minutes": {
										Type:        "number",
										Description: "Minutes to wait for min_entries_to_merge before merging anyway.",
									},
								},
							},
						},
					},
					"bypass_actors": {
						Type:        "array",
						Description: "Actors allowed to bypass the ruleset. Replaces the current list when given; pass an empty array to remove every bypass.",
						Items: &jsonschema.Schema{
							Type: "object",
							Properties: map[string]*jsonschema.Schema{
								"actor_id": {
									Type:        "number",
									Description: "ID of the team, app or role. Omit for OrganizationAdmin.",
								},
								"actor_type": {
									Type:        "string",
									Description: "What kind of actor this is.",
									Enum:        []any{"Integration", "OrganizationAdmin", "RepositoryRole", "Team", "DeployKey"},
								},
								"bypass_mode": {
									Type:        "string",
									Description: "'always' bypasses everywhere, 'pull_request' only via a pull request.",
									Enum:        []any{"always", "pull_request"},
								},
							},
							Required: []string{"actor_type"},
						},
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case rulesetWriteMethodCreate:
				name, err := RequiredParam[string](args, "name")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				ruleset := github.RepositoryRuleset{
					Name:        name,
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Rules:       &github.RepositoryRulesetRules{},
				}

				if errResult, err := applyRulesetArgs(&ruleset, args); errResult != nil || err != nil {
					return errResult, nil, err
				}

				created, resp, err := client.Repositories.CreateRuleset(ctx, owner, repo, ruleset)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create ruleset", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalGovernanceResult(convertToMinimalRuleset(created), nil)

			case rulesetWriteMethodUpdate, rulesetWriteMethodSetEnforcem:
				if method == rulesetWriteMethodSetEnforcem {
					if _, ok := args["enforcement"]; !ok {
						return utils.NewToolResultError("enforcement is required for set_enforcement"), nil, nil
					}
				}

				id, errResult, err := resolveRulesetID(ctx, client, owner, repo, args)
				if errResult != nil || err != nil {
					return errResult, nil, err
				}

				// Read before write: the update endpoint replaces the ruleset
				// wholesale, so anything not sent back would be dropped.
				current, resp, err := client.Repositories.GetRuleset(ctx, owner, repo, id, false)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to read the ruleset before updating it", resp, err), nil, nil
				}
				_ = resp.Body.Close()

				updated := *current
				updated.ID = nil
				updated.SourceType = nil
				updated.CurrentUserCanBypass = nil
				updated.Links = nil
				updated.CreatedAt = nil
				updated.UpdatedAt = nil
				if updated.Rules == nil {
					updated.Rules = &github.RepositoryRulesetRules{}
				}

				if method == rulesetWriteMethodUpdate {
					if errResult, err := applyRulesetArgs(&updated, args); errResult != nil || err != nil {
						return errResult, nil, err
					}
				} else if enforcement, err := OptionalParam[string](args, "enforcement"); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				} else {
					updated.Enforcement = github.RulesetEnforcement(enforcement)
				}

				result, resp, err := client.Repositories.UpdateRuleset(ctx, owner, repo, id, updated)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update ruleset", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalGovernanceResult(convertToMinimalRuleset(result), nil)

			case rulesetWriteMethodDelete:
				id, errResult, err := resolveRulesetID(ctx, client, owner, repo, args)
				if errResult != nil || err != nil {
					return errResult, nil, err
				}

				resp, err := client.Repositories.DeleteRuleset(ctx, owner, repo, id)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete ruleset", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return utils.NewToolResultText(fmt.Sprintf("ruleset %d deleted successfully", id)), nil, nil

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create, update, delete, set_enforcement", method)), nil, nil
			}
		},
	)
}

// applyRulesetArgs layers the caller's arguments onto a ruleset in place. Only
// the arguments actually present are applied, which is what keeps an update
// from weakening rules the caller never mentioned.
func applyRulesetArgs(ruleset *github.RepositoryRuleset, args map[string]any) (*mcp.CallToolResult, error) {
	if name, ok := args["name"].(string); ok && name != "" {
		ruleset.Name = name
	}
	if target, ok := args["target"].(string); ok && target != "" {
		ruleset.Target = github.Ptr(github.RulesetTarget(target))
	}
	if enforcement, ok := args["enforcement"].(string); ok && enforcement != "" {
		ruleset.Enforcement = github.RulesetEnforcement(enforcement)
	}

	includeRefs, includeSet, err := optionalStringArray(args, "include_refs")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	excludeRefs, excludeSet, err := optionalStringArray(args, "exclude_refs")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	if includeSet || excludeSet {
		if ruleset.Conditions == nil {
			ruleset.Conditions = &github.RepositoryRulesetConditions{}
		}
		if ruleset.Conditions.RefName == nil {
			ruleset.Conditions.RefName = &github.RepositoryRulesetRefConditionParameters{}
		}
		if includeSet {
			ruleset.Conditions.RefName.Include = includeRefs
		}
		if excludeSet {
			ruleset.Conditions.RefName.Exclude = excludeRefs
		}
		if ruleset.Conditions.RefName.Include == nil {
			ruleset.Conditions.RefName.Include = []string{}
		}
		if ruleset.Conditions.RefName.Exclude == nil {
			ruleset.Conditions.RefName.Exclude = []string{}
		}
	}

	if raw, ok := args["bypass_actors"]; ok {
		actors, err := parseBypassActors(raw)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil
		}
		ruleset.BypassActors = actors
	}

	if raw, ok := args["rules"]; ok && raw != nil {
		rules, ok := raw.(map[string]any)
		if !ok {
			return utils.NewToolResultError("rules must be an object"), nil
		}
		if ruleset.Rules == nil {
			ruleset.Rules = &github.RepositoryRulesetRules{}
		}
		if errResult, err := applyRulesetRules(ruleset.Rules, rules); errResult != nil || err != nil {
			return errResult, err
		}
	}

	return nil, nil
}

// applyRulesetRules layers rule arguments onto an existing rule set in place.
func applyRulesetRules(rules *github.RepositoryRulesetRules, args map[string]any) (*mcp.CallToolResult, error) {
	toggle := func(key string, target **github.EmptyRuleParameters) error {
		raw, ok := args[key]
		if !ok {
			return nil
		}
		enabled, ok := raw.(bool)
		if !ok {
			return fmt.Errorf("rules.%s must be a boolean", key)
		}
		if enabled {
			*target = &github.EmptyRuleParameters{}
		} else {
			*target = nil
		}
		return nil
	}

	for key, target := range map[string]**github.EmptyRuleParameters{
		"block_force_pushes":     &rules.NonFastForward,
		"block_deletions":        &rules.Deletion,
		"block_creations":        &rules.Creation,
		"require_signed_commits": &rules.RequiredSignatures,
		"require_linear_history": &rules.RequiredLinearHistory,
	} {
		if err := toggle(key, target); err != nil {
			return utils.NewToolResultError(err.Error()), nil
		}
	}

	// The pull request rule is the parent of every review setting, so any
	// review argument implies it; only an explicit false removes it.
	if raw, ok := args["require_pull_request"]; ok {
		enabled, ok := raw.(bool)
		if !ok {
			return utils.NewToolResultError("rules.require_pull_request must be a boolean"), nil
		}
		if enabled {
			if rules.PullRequest == nil {
				rules.PullRequest = &github.PullRequestRuleParameters{}
			}
		} else {
			rules.PullRequest = nil
		}
	}

	prKeys := []string{
		"required_approving_review_count",
		"dismiss_stale_reviews_on_push",
		"require_code_owner_review",
		"require_last_push_approval",
		"required_review_thread_resolution",
		"allowed_merge_methods",
	}
	wantsPRSettings := false
	for _, key := range prKeys {
		if _, ok := args[key]; ok {
			wantsPRSettings = true
			break
		}
	}
	if wantsPRSettings {
		if rules.PullRequest == nil {
			// Explicitly turning the rule off in the same call wins.
			if enabled, ok := args["require_pull_request"].(bool); ok && !enabled {
				return utils.NewToolResultError("cannot set pull request review settings while require_pull_request is false"), nil
			}
			rules.PullRequest = &github.PullRequestRuleParameters{}
		}

		pr := rules.PullRequest
		if raw, ok := args["required_approving_review_count"]; ok {
			count, err := toInt(raw)
			if err != nil {
				return utils.NewToolResultError("rules.required_approving_review_count must be a number"), nil
			}
			pr.RequiredApprovingReviewCount = count
		}
		for key, target := range map[string]*bool{
			"dismiss_stale_reviews_on_push":     &pr.DismissStaleReviewsOnPush,
			"require_code_owner_review":         &pr.RequireCodeOwnerReview,
			"require_last_push_approval":        &pr.RequireLastPushApproval,
			"required_review_thread_resolution": &pr.RequiredReviewThreadResolution,
		} {
			raw, ok := args[key]
			if !ok {
				continue
			}
			value, ok := raw.(bool)
			if !ok {
				return utils.NewToolResultError(fmt.Sprintf("rules.%s must be a boolean", key)), nil
			}
			*target = value
		}
		if methods, ok, err := optionalStringArray(args, "allowed_merge_methods"); err != nil {
			return utils.NewToolResultError(err.Error()), nil
		} else if ok {
			pr.AllowedMergeMethods = nil
			for _, method := range methods {
				pr.AllowedMergeMethods = append(pr.AllowedMergeMethods, github.PullRequestMergeMethod(method))
			}
		}
	}

	if contexts, ok, err := optionalStringArray(args, "required_status_checks"); err != nil {
		return utils.NewToolResultError(err.Error()), nil
	} else if ok {
		if len(contexts) == 0 {
			rules.RequiredStatusChecks = nil
		} else {
			if rules.RequiredStatusChecks == nil {
				rules.RequiredStatusChecks = &github.RequiredStatusChecksRuleParameters{}
			}
			checks := make([]*github.RuleStatusCheck, 0, len(contexts))
			for _, context := range contexts {
				checks = append(checks, &github.RuleStatusCheck{Context: context})
			}
			rules.RequiredStatusChecks.RequiredStatusChecks = checks
		}
	}
	if raw, ok := args["strict_required_status_checks_policy"]; ok {
		strict, ok := raw.(bool)
		if !ok {
			return utils.NewToolResultError("rules.strict_required_status_checks_policy must be a boolean"), nil
		}
		if rules.RequiredStatusChecks == nil {
			return utils.NewToolResultError("strict_required_status_checks_policy needs a required_status_checks rule; provide required_status_checks in the same call"), nil
		}
		rules.RequiredStatusChecks.StrictRequiredStatusChecksPolicy = strict
	}

	if environments, ok, err := optionalStringArray(args, "required_deployment_environments"); err != nil {
		return utils.NewToolResultError(err.Error()), nil
	} else if ok {
		if len(environments) == 0 {
			rules.RequiredDeployments = nil
		} else {
			rules.RequiredDeployments = &github.RequiredDeploymentsRuleParameters{
				RequiredDeploymentEnvironments: environments,
			}
		}
	}

	if raw, ok := args["merge_queue"]; ok && raw != nil {
		queueArgs, ok := raw.(map[string]any)
		if !ok {
			return utils.NewToolResultError("rules.merge_queue must be an object"), nil
		}
		if enabled, ok := queueArgs["enabled"].(bool); ok && !enabled {
			rules.MergeQueue = nil
		} else {
			queue := mergeQueueDefaults
			if rules.MergeQueue != nil {
				queue = *rules.MergeQueue
			}
			if method, ok := queueArgs["merge_method"].(string); ok && method != "" {
				queue.MergeMethod = github.MergeQueueMergeMethod(method)
			}
			if strategy, ok := queueArgs["grouping_strategy"].(string); ok && strategy != "" {
				queue.GroupingStrategy = github.MergeGroupingStrategy(strategy)
			}
			for key, target := range map[string]*int{
				"check_response_timeout_minutes":    &queue.CheckResponseTimeoutMinutes,
				"max_entries_to_build":              &queue.MaxEntriesToBuild,
				"max_entries_to_merge":              &queue.MaxEntriesToMerge,
				"min_entries_to_merge":              &queue.MinEntriesToMerge,
				"min_entries_to_merge_wait_minutes": &queue.MinEntriesToMergeWaitMinutes,
			} {
				raw, ok := queueArgs[key]
				if !ok {
					continue
				}
				value, err := toInt(raw)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("rules.merge_queue.%s must be a number", key)), nil
				}
				*target = value
			}
			rules.MergeQueue = &queue
		}
	}

	return nil, nil
}

func parseBypassActors(raw any) ([]*github.BypassActor, error) {
	entries, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("bypass_actors must be an array")
	}

	actors := make([]*github.BypassActor, 0, len(entries))
	for i, entry := range entries {
		fields, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("bypass_actors[%d] must be an object", i)
		}
		actorType, ok := fields["actor_type"].(string)
		if !ok || actorType == "" {
			return nil, fmt.Errorf("bypass_actors[%d].actor_type is required", i)
		}

		actor := &github.BypassActor{
			ActorType:  github.Ptr(github.BypassActorType(actorType)),
			BypassMode: github.Ptr(github.BypassModeAlways),
		}
		if mode, ok := fields["bypass_mode"].(string); ok && mode != "" {
			actor.BypassMode = github.Ptr(github.BypassMode(mode))
		}
		if rawID, ok := fields["actor_id"]; ok && rawID != nil {
			id, err := toInt(rawID)
			if err != nil {
				return nil, fmt.Errorf("bypass_actors[%d].actor_id must be a number", i)
			}
			actor.ActorID = github.Ptr(int64(id))
		}
		actors = append(actors, actor)
	}

	return actors, nil
}

// resolveRulesetID turns either an explicit ruleset ID or a ruleset name into
// an ID. Only rulesets defined on this repository are candidates: the
// repository endpoints cannot modify one inherited from an organization, and
// matching such a name would produce a confusing 404 later.
func resolveRulesetID(ctx context.Context, client *github.Client, owner, repo string, args map[string]any) (int64, *mcp.CallToolResult, error) {
	id, err := OptionalIntParam(args, "ruleset_id")
	if err != nil {
		return 0, utils.NewToolResultError(err.Error()), nil
	}
	if id != 0 {
		return int64(id), nil, nil
	}

	name, err := OptionalParam[string](args, "name")
	if err != nil {
		return 0, utils.NewToolResultError(err.Error()), nil
	}
	if name == "" {
		return 0, utils.NewToolResultError("either ruleset_id or name is required"), nil
	}

	opts := &github.RepositoryListRulesetsOptions{
		IncludesParents: github.Ptr(false),
		ListOptions:     github.ListOptions{PerPage: 100},
	}

	var matches []*github.RepositoryRuleset
	for {
		rulesets, resp, err := client.Repositories.GetAllRulesets(ctx, owner, repo, opts)
		if err != nil {
			return 0, ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list rulesets", resp, err), nil
		}
		nextPage := 0
		if resp != nil {
			nextPage = resp.NextPage
			_ = resp.Body.Close()
		}

		for _, ruleset := range rulesets {
			if ruleset == nil || !strings.EqualFold(ruleset.Name, name) {
				continue
			}
			if ruleset.SourceType != nil && *ruleset.SourceType != github.RulesetSourceTypeRepository {
				continue
			}
			matches = append(matches, ruleset)
		}

		if nextPage == 0 {
			break
		}
		opts.Page = nextPage
	}

	switch len(matches) {
	case 0:
		return 0, utils.NewToolResultError(fmt.Sprintf("no repository ruleset named '%s' in %s/%s", name, owner, repo)), nil
	case 1:
		return matches[0].GetID(), nil, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, ruleset := range matches {
			ids = append(ids, fmt.Sprintf("%d", ruleset.GetID()))
		}
		return 0, utils.NewToolResultError(fmt.Sprintf("ruleset name '%s' is ambiguous in %s/%s: matches IDs %s. Pass ruleset_id instead.", name, owner, repo, strings.Join(ids, ", "))), nil
	}
}

// optionalStringArray reads an optional array-of-strings argument, reporting
// whether it was present. An explicitly empty array is present but empty,
// which callers use to mean "remove this".
func optionalStringArray(args map[string]any, key string) ([]string, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, false, nil
	}
	entries, ok := raw.([]any)
	if !ok {
		return nil, false, fmt.Errorf("%s must be an array of strings", key)
	}
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		value, ok := entry.(string)
		if !ok {
			return nil, false, fmt.Errorf("%s must be an array of strings", key)
		}
		values = append(values, value)
	}
	return values, true, nil
}

func marshalGovernanceResult(payload any, label func(*mcp.CallToolResult) *mcp.CallToolResult) (*mcp.CallToolResult, any, error) {
	r, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	result := utils.NewToolResultText(string(r))
	if label != nil {
		result = label(result)
	}
	return result, nil, nil
}
