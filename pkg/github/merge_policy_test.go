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

func Test_MergePolicyRead(t *testing.T) {
	t.Parallel()

	serverTool := MergePolicyRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "merge_policy_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "merge_policy_read tool should be read-only")

	t.Run("defaults to the default branch and intersects merge methods", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposByOwnerByRepoForGovernanceTesting, &github.Repository{
				DefaultBranch:       github.Ptr("develop"),
				AllowSquashMerge:    github.Ptr(true),
				AllowMergeCommit:    github.Ptr(true),
				AllowRebaseMerge:    github.Ptr(true),
				DeleteBranchOnMerge: github.Ptr(true),
			}),
			WithRequestMatchHandler(getReposRulesBranchesByOwnerByRepoByRef,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// The branch was not given, so the default branch is used.
					assert.Equal(t, "/repos/owner/repo/rules/branches/develop", r.URL.Path)

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[
						{"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":42,
						 "parameters":{"required_approving_review_count":2,"required_review_thread_resolution":true,"allowed_merge_methods":["squash","rebase"]}},
						{"type":"required_status_checks","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":42,
						 "parameters":{"required_status_checks":[{"context":"ci/lint"},{"context":"ci/test"}],"strict_required_status_checks_policy":true}},
						{"type":"non_fast_forward","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":42},
						{"type":"deletion","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":42}
					]`))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var policy MinimalMergePolicy
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &policy))

		assert.Equal(t, "develop", policy.Branch)
		assert.True(t, policy.IsDefaultBranch)
		assert.ElementsMatch(t, []string{"merge", "squash", "rebase"}, policy.RepositoryAllowedMergeMethods)
		// The ruleset narrows the repository's three methods to two.
		assert.ElementsMatch(t, []string{"squash", "rebase"}, policy.EffectiveAllowedMergeMethods)
		assert.True(t, policy.DeleteBranchOnMerge)

		rules := policy.BranchRules
		assert.True(t, rules.RequirePullRequest)
		assert.Equal(t, 2, rules.RequiredApprovingReviewCount)
		assert.True(t, rules.RequiredReviewThreadResolution)
		assert.ElementsMatch(t, []string{"ci/lint", "ci/test"}, rules.RequiredStatusChecks)
		assert.True(t, rules.StrictRequiredStatusChecks)
		assert.True(t, rules.BlockForcePushes)
		assert.True(t, rules.BlockDeletions)
		assert.Empty(t, policy.Notes)
	})

	t.Run("an unsatisfiable merge method combination is called out", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposByOwnerByRepoForGovernanceTesting, &github.Repository{
				DefaultBranch:    github.Ptr("main"),
				AllowMergeCommit: github.Ptr(true),
			}),
			WithRequestMatch(getReposRulesBranchesByOwnerByRepoByRef, `[
				{"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":1,
				 "parameters":{"allowed_merge_methods":["squash"]}}
			]`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "branch": "main"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var policy MinimalMergePolicy
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &policy))
		assert.Empty(t, policy.EffectiveAllowedMergeMethods)
		require.NotEmpty(t, policy.Notes)
		assert.Contains(t, policy.Notes[0], "no merge method is available")
	})

	// Several rulesets can govern one branch at once and a pull request has to
	// satisfy all of them, so their merge-method restrictions intersect rather
	// than accumulate. A ruleset that names no methods restricts nothing.
	t.Run("merge methods intersect across every applicable ruleset", func(t *testing.T) {
		tests := []struct {
			name                   string
			repository             *github.Repository
			branchRules            string
			expectedRulesetMethods []string
			expectedRestriction    bool
			expectedEffective      []string
			expectedSourceIDs      []int64
			expectNoMethodNote     bool
		}{
			{
				// Case A: the two rulesets overlap on a single method.
				name: "two restricting rulesets are intersected, not unioned",
				repository: &github.Repository{
					DefaultBranch:    github.Ptr("main"),
					AllowMergeCommit: github.Ptr(true),
					AllowSquashMerge: github.Ptr(true),
					AllowRebaseMerge: github.Ptr(true),
				},
				branchRules: `[
					{"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":1,
					 "parameters":{"allowed_merge_methods":["merge","squash"]}},
					{"type":"pull_request","ruleset_source_type":"Organization","ruleset_source":"owner","ruleset_id":2,
					 "parameters":{"allowed_merge_methods":["squash","rebase"]}}
				]`,
				expectedRulesetMethods: []string{"squash"},
				expectedRestriction:    true,
				expectedEffective:      []string{"squash"},
				expectedSourceIDs:      []int64{1, 2},
			},
			{
				// Case B: the two rulesets share no method at all.
				name: "rulesets that overlap on nothing leave no merge method",
				repository: &github.Repository{
					DefaultBranch:    github.Ptr("main"),
					AllowMergeCommit: github.Ptr(true),
					AllowSquashMerge: github.Ptr(true),
					AllowRebaseMerge: github.Ptr(true),
				},
				branchRules: `[
					{"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":1,
					 "parameters":{"allowed_merge_methods":["merge"]}},
					{"type":"pull_request","ruleset_source_type":"Organization","ruleset_source":"owner","ruleset_id":2,
					 "parameters":{"allowed_merge_methods":["squash"]}}
				]`,
				expectedRulesetMethods: nil,
				expectedRestriction:    true,
				expectedEffective:      nil,
				expectedSourceIDs:      []int64{1, 2},
				expectNoMethodNote:     true,
			},
			{
				// Case C: silence is not a restriction, so the one ruleset that
				// does restrict decides on its own.
				name: "a ruleset that names no merge methods restricts none",
				repository: &github.Repository{
					DefaultBranch:    github.Ptr("main"),
					AllowMergeCommit: github.Ptr(true),
					AllowSquashMerge: github.Ptr(true),
					AllowRebaseMerge: github.Ptr(true),
				},
				branchRules: `[
					{"type":"pull_request","ruleset_source_type":"Organization","ruleset_source":"owner","ruleset_id":7,
					 "parameters":{"required_approving_review_count":1}},
					{"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":1,
					 "parameters":{"allowed_merge_methods":["squash"]}}
				]`,
				expectedRulesetMethods: []string{"squash"},
				expectedRestriction:    true,
				expectedEffective:      []string{"squash"},
				expectedSourceIDs:      []int64{1, 7},
			},
			{
				// Case D: the repository settings are a separate layer and
				// narrow the ruleset's methods further.
				name: "the repository settings narrow the ruleset's methods",
				repository: &github.Repository{
					DefaultBranch:    github.Ptr("main"),
					AllowSquashMerge: github.Ptr(true),
				},
				branchRules: `[
					{"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":1,
					 "parameters":{"allowed_merge_methods":["merge","squash"]}}
				]`,
				expectedRulesetMethods: []string{"merge", "squash"},
				expectedRestriction:    true,
				expectedEffective:      []string{"squash"},
				expectedSourceIDs:      []int64{1},
			},
			{
				// Case E: a governed branch whose rules say nothing about merge
				// methods keeps every method the repository allows.
				name: "an unrestricting ruleset leaves the repository's methods intact",
				repository: &github.Repository{
					DefaultBranch:    github.Ptr("main"),
					AllowMergeCommit: github.Ptr(true),
					AllowSquashMerge: github.Ptr(true),
				},
				branchRules: `[
					{"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":1,
					 "parameters":{"required_approving_review_count":1}}
				]`,
				expectedRulesetMethods: nil,
				expectedRestriction:    false,
				expectedEffective:      []string{"merge", "squash"},
				expectedSourceIDs:      []int64{1},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				client := mustNewGHClient(t, NewMockedHTTPClient(
					WithRequestMatch(getReposByOwnerByRepoForGovernanceTesting, tc.repository),
					WithRequestMatch(getReposRulesBranchesByOwnerByRepoByRef, tc.branchRules),
				))
				deps := BaseDeps{Client: client}
				handler := serverTool.Handler(deps)

				request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "branch": "main"})
				result, err := handler(ContextWithDeps(context.Background(), deps), &request)
				require.NoError(t, err)
				require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

				var policy MinimalMergePolicy
				require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &policy))

				assert.Equal(t, tc.expectedRestriction, policy.BranchRules.MergeMethodsRestricted)
				assert.ElementsMatch(t, tc.expectedRulesetMethods, policy.BranchRules.AllowedMergeMethods)
				assert.ElementsMatch(t, tc.expectedEffective, policy.EffectiveAllowedMergeMethods)

				// Every contributing ruleset stays named whatever the merge
				// methods worked out to.
				sourceIDs := make([]int64, 0, len(policy.BranchRules.Sources))
				for _, source := range policy.BranchRules.Sources {
					sourceIDs = append(sourceIDs, source.RulesetID)
				}
				assert.ElementsMatch(t, tc.expectedSourceIDs, sourceIDs)

				if tc.expectNoMethodNote {
					require.NotEmpty(t, policy.Notes)
					assert.Contains(t, policy.Notes[0], "no merge method is available")
				} else {
					for _, note := range policy.Notes {
						assert.NotContains(t, note, "no merge method is available")
					}
				}
			})
		}
	})

	t.Run("an ungoverned branch reports the repository settings alone", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposByOwnerByRepoForGovernanceTesting, &github.Repository{
				DefaultBranch:    github.Ptr("main"),
				AllowSquashMerge: github.Ptr(true),
			}),
			WithRequestMatch(getReposRulesBranchesByOwnerByRepoByRef, `[]`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "branch": "feature"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)

		var policy MinimalMergePolicy
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &policy))
		assert.Equal(t, "feature", policy.Branch)
		assert.False(t, policy.IsDefaultBranch)
		assert.False(t, policy.BranchRules.RequirePullRequest)
		assert.Equal(t, []string{"squash"}, policy.EffectiveAllowedMergeMethods)
	})

	t.Run("an archived repository is called out", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposByOwnerByRepoForGovernanceTesting, &github.Repository{
				DefaultBranch:    github.Ptr("main"),
				AllowSquashMerge: github.Ptr(true),
				Archived:         github.Ptr(true),
			}),
			WithRequestMatch(getReposRulesBranchesByOwnerByRepoByRef, `[]`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var policy MinimalMergePolicy
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &policy))
		assert.True(t, policy.Archived)
		require.NotEmpty(t, policy.Notes)
		assert.Contains(t, policy.Notes[0], "archived")
	})

	t.Run("api failure is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(getReposByOwnerByRepoForGovernanceTesting,
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
		assert.Contains(t, getErrorResult(t, result).Text, "failed to get repository settings")
	})
}
