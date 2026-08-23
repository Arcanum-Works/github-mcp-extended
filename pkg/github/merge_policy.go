package github

import (
	"context"
	"fmt"
	"slices"
	"sort"

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

// MinimalRulesetSource identifies a ruleset that contributes rules to a branch.
type MinimalRulesetSource struct {
	RulesetID  int64  `json:"ruleset_id"`
	SourceType string `json:"source_type,omitempty"`
	Source     string `json:"source,omitempty"`
}

// MinimalBranchRules is the effective rule set for one branch, flattened
// across every ruleset that targets it.
//
// GitHub applies the union of all matching rulesets, and the strictest value
// wins where they disagree, so this collapses them the same way: booleans are
// OR-ed, review counts take the maximum, and status check contexts are unioned.
type MinimalBranchRules struct {
	Branch                         string                 `json:"branch"`
	RequirePullRequest             bool                   `json:"require_pull_request"`
	RequiredApprovingReviewCount   int                    `json:"required_approving_review_count"`
	DismissStaleReviewsOnPush      bool                   `json:"dismiss_stale_reviews_on_push"`
	RequireCodeOwnerReview         bool                   `json:"require_code_owner_review"`
	RequireLastPushApproval        bool                   `json:"require_last_push_approval"`
	RequiredReviewThreadResolution bool                   `json:"required_review_thread_resolution"`
	AllowedMergeMethods            []string               `json:"allowed_merge_methods,omitempty"`
	RequiredStatusChecks           []string               `json:"required_status_checks,omitempty"`
	StrictRequiredStatusChecks     bool                   `json:"strict_required_status_checks_policy"`
	BlockForcePushes               bool                   `json:"block_force_pushes"`
	BlockDeletions                 bool                   `json:"block_deletions"`
	BlockCreations                 bool                   `json:"block_creations"`
	RequireSignedCommits           bool                   `json:"require_signed_commits"`
	RequireLinearHistory           bool                   `json:"require_linear_history"`
	RequiredDeploymentEnvironments []string               `json:"required_deployment_environments,omitempty"`
	MergeQueueRequired             bool                   `json:"merge_queue_required"`
	Sources                        []MinimalRulesetSource `json:"sources,omitempty"`
}

// MinimalMergePolicy answers "can a pull request into this branch merge, and
// what governs that?" by pairing the repository's merge configuration with the
// rules in force on the target branch.
type MinimalMergePolicy struct {
	Branch                        string             `json:"branch"`
	IsDefaultBranch               bool               `json:"is_default_branch"`
	RepositoryAllowedMergeMethods []string           `json:"repository_allowed_merge_methods"`
	EffectiveAllowedMergeMethods  []string           `json:"effective_allowed_merge_methods"`
	DeleteBranchOnMerge           bool               `json:"delete_branch_on_merge"`
	AllowAutoMerge                bool               `json:"allow_auto_merge"`
	AllowUpdateBranch             bool               `json:"allow_update_branch"`
	Archived                      bool               `json:"archived"`
	BranchRules                   MinimalBranchRules `json:"branch_rules"`
	Notes                         []string           `json:"notes,omitempty"`
}

func convertToMinimalBranchRules(branch string, rules *github.BranchRules) MinimalBranchRules {
	m := MinimalBranchRules{Branch: branch}
	if rules == nil {
		return m
	}

	sources := map[int64]MinimalRulesetSource{}
	noteSource := func(meta github.BranchRuleMetadata) {
		sources[meta.RulesetID] = MinimalRulesetSource{
			RulesetID:  meta.RulesetID,
			SourceType: string(meta.RulesetSourceType),
			Source:     meta.RulesetSource,
		}
	}

	for _, rule := range rules.PullRequest {
		if rule == nil {
			continue
		}
		noteSource(rule.BranchRuleMetadata)
		m.RequirePullRequest = true
		m.RequiredApprovingReviewCount = max(m.RequiredApprovingReviewCount, rule.Parameters.RequiredApprovingReviewCount)
		m.DismissStaleReviewsOnPush = m.DismissStaleReviewsOnPush || rule.Parameters.DismissStaleReviewsOnPush
		m.RequireCodeOwnerReview = m.RequireCodeOwnerReview || rule.Parameters.RequireCodeOwnerReview
		m.RequireLastPushApproval = m.RequireLastPushApproval || rule.Parameters.RequireLastPushApproval
		m.RequiredReviewThreadResolution = m.RequiredReviewThreadResolution || rule.Parameters.RequiredReviewThreadResolution
		for _, method := range rule.Parameters.AllowedMergeMethods {
			if !slices.Contains(m.AllowedMergeMethods, string(method)) {
				m.AllowedMergeMethods = append(m.AllowedMergeMethods, string(method))
			}
		}
	}

	for _, rule := range rules.RequiredStatusChecks {
		if rule == nil {
			continue
		}
		noteSource(rule.BranchRuleMetadata)
		m.StrictRequiredStatusChecks = m.StrictRequiredStatusChecks || rule.Parameters.StrictRequiredStatusChecksPolicy
		for _, check := range rule.Parameters.RequiredStatusChecks {
			if check != nil && !slices.Contains(m.RequiredStatusChecks, check.Context) {
				m.RequiredStatusChecks = append(m.RequiredStatusChecks, check.Context)
			}
		}
	}

	for _, rule := range rules.RequiredDeployments {
		if rule == nil {
			continue
		}
		noteSource(rule.BranchRuleMetadata)
		for _, environment := range rule.Parameters.RequiredDeploymentEnvironments {
			if !slices.Contains(m.RequiredDeploymentEnvironments, environment) {
				m.RequiredDeploymentEnvironments = append(m.RequiredDeploymentEnvironments, environment)
			}
		}
	}

	for _, rule := range rules.MergeQueue {
		if rule == nil {
			continue
		}
		noteSource(rule.BranchRuleMetadata)
		m.MergeQueueRequired = true
	}

	flag := func(entries []*github.BranchRuleMetadata, target *bool) {
		for _, meta := range entries {
			if meta == nil {
				continue
			}
			noteSource(*meta)
			*target = true
		}
	}
	flag(rules.NonFastForward, &m.BlockForcePushes)
	flag(rules.Deletion, &m.BlockDeletions)
	flag(rules.Creation, &m.BlockCreations)
	flag(rules.RequiredSignatures, &m.RequireSignedCommits)
	flag(rules.RequiredLinearHistory, &m.RequireLinearHistory)

	for _, source := range sources {
		m.Sources = append(m.Sources, source)
	}
	sort.Slice(m.Sources, func(i, j int) bool { return m.Sources[i].RulesetID < m.Sources[j].RulesetID })

	return m
}

// MergePolicyRead creates a tool that answers whether and how a pull request
// into a given branch is allowed to merge.
//
// It is an aggregation, not a policy engine: every value it returns comes
// straight from the repository settings or the branch's rules, in one call
// instead of two plus the flattening work.
func MergePolicyRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepoGovernance,
		mcp.Tool{
			Name: "merge_policy_read",
			Description: t("TOOL_MERGE_POLICY_READ_DESCRIPTION", "Read the governance policy that decides whether a pull request into a branch can merge: which merge methods the repository and its rulesets both allow, how many approvals are required, whether conversations must be resolved, which status checks must pass, and what is blocked outright. "+
				"This reports policy, not the state of any particular pull request; use 'pull_request_read' for that."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_MERGE_POLICY_READ_USER_TITLE", "Read merge policy for a branch"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"branch": {
						Type:        "string",
						Description: "The branch a pull request would merge into. Defaults to the repository's default branch.",
					},
				},
				Required: []string{"owner", "repo"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			branch, err := OptionalParam[string](args, "branch")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			repository, resp, err := client.Repositories.Get(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository settings", resp, err), nil, nil
			}
			_ = resp.Body.Close()

			if branch == "" {
				branch = repository.GetDefaultBranch()
			}

			rules, resp, err := client.Repositories.ListRulesForBranch(ctx, owner, repo, branch, nil)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get rules for branch", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			settings := convertToMinimalRepositorySettings(repository)
			branchRules := convertToMinimalBranchRules(branch, rules)

			policy := MinimalMergePolicy{
				Branch:              branch,
				IsDefaultBranch:     branch == repository.GetDefaultBranch(),
				DeleteBranchOnMerge: settings.DeleteBranchOnMerge,
				AllowAutoMerge:      settings.AllowAutoMerge,
				AllowUpdateBranch:   settings.AllowUpdateBranch,
				Archived:            settings.Archived,
				BranchRules:         branchRules,
			}

			if settings.AllowMergeCommit {
				policy.RepositoryAllowedMergeMethods = append(policy.RepositoryAllowedMergeMethods, "merge")
			}
			if settings.AllowSquashMerge {
				policy.RepositoryAllowedMergeMethods = append(policy.RepositoryAllowedMergeMethods, "squash")
			}
			if settings.AllowRebaseMerge {
				policy.RepositoryAllowedMergeMethods = append(policy.RepositoryAllowedMergeMethods, "rebase")
			}
			if policy.RepositoryAllowedMergeMethods == nil {
				policy.RepositoryAllowedMergeMethods = []string{}
			}

			// A ruleset can only narrow what the repository already allows, so
			// the methods actually available are the intersection.
			if len(branchRules.AllowedMergeMethods) == 0 {
				policy.EffectiveAllowedMergeMethods = policy.RepositoryAllowedMergeMethods
			} else {
				policy.EffectiveAllowedMergeMethods = []string{}
				for _, method := range policy.RepositoryAllowedMergeMethods {
					if slices.Contains(branchRules.AllowedMergeMethods, method) {
						policy.EffectiveAllowedMergeMethods = append(policy.EffectiveAllowedMergeMethods, method)
					}
				}
			}

			if len(policy.EffectiveAllowedMergeMethods) == 0 {
				policy.Notes = append(policy.Notes, "no merge method is available: the repository settings and the branch rules do not agree on one")
			}
			if policy.Archived {
				policy.Notes = append(policy.Notes, "the repository is archived and is read-only; nothing can merge")
			}
			if branchRules.MergeQueueRequired {
				policy.Notes = append(policy.Notes, "a merge queue is required: pull requests are merged by the queue rather than directly")
			}

			return marshalGovernanceResult(policy, func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelRepoMetadata)
			})
		},
	)
}
