package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	getReposRulesetsByOwnerByRepo             = "GET /repos/{owner}/{repo}/rulesets"
	postReposRulesetsByOwnerByRepo            = "POST /repos/{owner}/{repo}/rulesets"
	getReposRulesetsByOwnerByRepoByID         = "GET /repos/{owner}/{repo}/rulesets/{ruleset_id}"
	putReposRulesetsByOwnerByRepoByID         = "PUT /repos/{owner}/{repo}/rulesets/{ruleset_id}"
	deleteReposRulesetsByOwnerByRepoByID      = "DELETE /repos/{owner}/{repo}/rulesets/{ruleset_id}"
	getReposRulesBranchesByOwnerByRepoByRef   = "GET /repos/{owner}/{repo}/rules/branches/{branch}"
	getReposByOwnerByRepoForGovernanceTesting = "GET /repos/{owner}/{repo}"
)

// protectedMainRuleset is a ruleset in GitHub's wire format: rules arrive as a
// typed array, not as an object, so fixtures have to be written that way for
// the client's custom unmarshaler to see them.
const protectedMainRuleset = `{
  "id": 42,
  "name": "protect main",
  "target": "branch",
  "source_type": "Repository",
  "source": "owner/repo",
  "enforcement": "active",
  "conditions": {"ref_name": {"include": ["~DEFAULT_BRANCH"], "exclude": []}},
  "rules": [
    {"type": "pull_request", "parameters": {
      "allowed_merge_methods": ["squash"],
      "dismiss_stale_reviews_on_push": true,
      "require_code_owner_review": true,
      "require_last_push_approval": false,
      "required_approving_review_count": 2,
      "required_review_thread_resolution": true
    }},
    {"type": "non_fast_forward"},
    {"type": "deletion"},
    {"type": "commit_message_pattern", "parameters": {"operator": "starts_with", "pattern": "JIRA-"}}
  ]
}`

func Test_RulesetsRead(t *testing.T) {
	t.Parallel()

	serverTool := RulesetsRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "rulesets_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "rulesets_read tool should be read-only")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo"})

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
		assertText         func(t *testing.T, text string)
	}{
		{
			name: "list rulesets",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposRulesetsByOwnerByRepo, "["+protectedMainRuleset+"]"),
			),
			requestArgs: map[string]any{
				"method": "list_rulesets",
				"owner":  "owner",
				"repo":   "repo",
			},
			assertText: func(t *testing.T, text string) {
				var rulesets []MinimalRuleset
				require.NoError(t, json.Unmarshal([]byte(text), &rulesets))
				require.Len(t, rulesets, 1)
				assert.Equal(t, "protect main", rulesets[0].Name)
				assert.Equal(t, "active", rulesets[0].Enforcement)
				assert.Equal(t, []string{"~DEFAULT_BRANCH"}, rulesets[0].Conditions.IncludeRefs)

				rules := rulesets[0].Rules
				require.NotNil(t, rules)
				assert.True(t, rules.RequirePullRequest)
				assert.Equal(t, 2, rules.RequiredApprovingReviewCount)
				assert.True(t, rules.RequiredReviewThreadResolution)
				assert.True(t, rules.BlockForcePushes)
				assert.True(t, rules.BlockDeletions)
				// The commit message pattern has no place in the summary, so
				// the summary says so rather than implying it is gone.
				assert.True(t, rules.UnrepresentedRulesArePreserved)
				assert.Contains(t, rules.UnrepresentedRuleNamesPreserved, "commit_message_pattern")
			},
		},
		{
			name: "get ruleset resolves the name to an id",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposRulesetsByOwnerByRepo, "["+protectedMainRuleset+"]"),
				WithRequestMatchHandler(getReposRulesetsByOwnerByRepoByID,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						assert.Equal(t, "/repos/owner/repo/rulesets/42", r.URL.Path)
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(protectedMainRuleset))
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "get_ruleset",
				"owner":  "owner",
				"repo":   "repo",
				"name":   "PROTECT MAIN",
			},
			assertText: func(t *testing.T, text string) {
				var ruleset MinimalRuleset
				require.NoError(t, json.Unmarshal([]byte(text), &ruleset))
				assert.Equal(t, int64(42), ruleset.ID)
			},
		},
		{
			name: "an unknown ruleset name is reported",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposRulesetsByOwnerByRepo, "[]"),
			),
			requestArgs: map[string]any{
				"method": "get_ruleset",
				"owner":  "owner",
				"repo":   "repo",
				"name":   "nope",
			},
			expectToolError:    true,
			expectedToolErrMsg: "no repository ruleset named 'nope'",
		},
		{
			name: "branch rules are flattened across rulesets",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatch(getReposRulesBranchesByOwnerByRepoByRef, `[
					{"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":42,
					 "parameters":{"required_approving_review_count":1,"required_review_thread_resolution":true,"allowed_merge_methods":["squash"]}},
					{"type":"pull_request","ruleset_source_type":"Organization","ruleset_source":"owner","ruleset_id":7,
					 "parameters":{"required_approving_review_count":3,"require_code_owner_review":true}},
					{"type":"required_status_checks","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":42,
					 "parameters":{"required_status_checks":[{"context":"ci/lint"}],"strict_required_status_checks_policy":true}},
					{"type":"non_fast_forward","ruleset_source_type":"Repository","ruleset_source":"owner/repo","ruleset_id":42}
				]`),
			),
			requestArgs: map[string]any{
				"method": "get_branch_rules",
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
			},
			assertText: func(t *testing.T, text string) {
				var rules MinimalBranchRules
				require.NoError(t, json.Unmarshal([]byte(text), &rules))
				assert.Equal(t, "main", rules.Branch)
				assert.True(t, rules.RequirePullRequest)
				// The strictest of the two pull request rules wins.
				assert.Equal(t, 3, rules.RequiredApprovingReviewCount)
				assert.True(t, rules.RequireCodeOwnerReview)
				assert.True(t, rules.RequiredReviewThreadResolution)
				assert.Equal(t, []string{"ci/lint"}, rules.RequiredStatusChecks)
				assert.True(t, rules.StrictRequiredStatusChecks)
				assert.True(t, rules.BlockForcePushes)
				// Both the repository and the organization ruleset are named.
				require.Len(t, rules.Sources, 2)
				assert.Equal(t, int64(7), rules.Sources[0].RulesetID)
				assert.Equal(t, "Organization", rules.Sources[0].SourceType)
			},
		},
		{
			name:         "get_branch_rules without a branch is rejected",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "get_branch_rules",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "branch",
		},
		{
			name:         "unknown method",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method": "list_everything",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "unknown method: list_everything",
		},
		{
			name: "api failure is surfaced",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(getReposRulesetsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"message": "Resource not accessible"}`))
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "list_rulesets",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "failed to list rulesets",
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

// rulesByType indexes a marshaled ruleset's rule array by rule type, which is
// how the wire format identifies each rule.
func rulesByType(t *testing.T, payload map[string]any) map[string]map[string]any {
	t.Helper()

	rawRules, ok := payload["rules"].([]any)
	require.True(t, ok, "expected a rules array in the request body")

	byType := map[string]map[string]any{}
	for _, raw := range rawRules {
		rule, ok := raw.(map[string]any)
		require.True(t, ok)
		ruleType, ok := rule["type"].(string)
		require.True(t, ok)
		byType[ruleType] = rule
	}
	return byType
}

func Test_RulesetWrite(t *testing.T) {
	t.Parallel()

	serverTool := RulesetWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "ruleset_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "ruleset_write tool should not be read-only")

	t.Run("create builds the requested rules", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposRulesetsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "protect main", payload["name"])
					assert.Equal(t, "branch", payload["target"])
					assert.Equal(t, "active", payload["enforcement"])

					conditions := payload["conditions"].(map[string]any)["ref_name"].(map[string]any)
					assert.Equal(t, []any{"~DEFAULT_BRANCH"}, conditions["include"])

					rules := rulesByType(t, payload)
					require.Contains(t, rules, "pull_request")
					params := rules["pull_request"]["parameters"].(map[string]any)
					assert.Equal(t, float64(2), params["required_approving_review_count"])
					assert.Equal(t, true, params["required_review_thread_resolution"])
					assert.Equal(t, []any{"squash"}, params["allowed_merge_methods"])

					require.Contains(t, rules, "required_status_checks")
					checkParams := rules["required_status_checks"]["parameters"].(map[string]any)
					assert.Equal(t, true, checkParams["strict_required_status_checks_policy"])

					assert.Contains(t, rules, "non_fast_forward")
					assert.Contains(t, rules, "deletion")

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(protectedMainRuleset))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":       "create",
			"owner":        "owner",
			"repo":         "repo",
			"name":         "protect main",
			"include_refs": []any{"~DEFAULT_BRANCH"},
			"rules": map[string]any{
				"required_approving_review_count":      float64(2),
				"required_review_thread_resolution":    true,
				"allowed_merge_methods":                []any{"squash"},
				"required_status_checks":               []any{"ci/lint"},
				"strict_required_status_checks_policy": true,
				"block_force_pushes":                   true,
				"block_deletions":                      true,
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("update preserves rules it was not asked to change", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposRulesetsByOwnerByRepo, "["+protectedMainRuleset+"]"),
			WithRequestMatch(getReposRulesetsByOwnerByRepoByID, protectedMainRuleset),
			WithRequestMatchHandler(putReposRulesetsByOwnerByRepoByID,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					rules := rulesByType(t, payload)

					// The call only added a status check. Everything else the
					// ruleset already enforced must still be in the body,
					// including a rule this tool has no vocabulary for.
					require.Contains(t, rules, "pull_request")
					params := rules["pull_request"]["parameters"].(map[string]any)
					assert.Equal(t, float64(2), params["required_approving_review_count"])
					assert.Equal(t, true, params["require_code_owner_review"])
					assert.Equal(t, true, params["dismiss_stale_reviews_on_push"])
					assert.Contains(t, rules, "non_fast_forward")
					assert.Contains(t, rules, "deletion")
					assert.Contains(t, rules, "commit_message_pattern")

					require.Contains(t, rules, "required_status_checks")
					checks := rules["required_status_checks"]["parameters"].(map[string]any)["required_status_checks"].([]any)
					require.Len(t, checks, 1)
					assert.Equal(t, "ci/build", checks[0].(map[string]any)["context"])

					// Conditions were not mentioned either.
					conditions := payload["conditions"].(map[string]any)["ref_name"].(map[string]any)
					assert.Equal(t, []any{"~DEFAULT_BRANCH"}, conditions["include"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(protectedMainRuleset))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "update",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "protect main",
			"rules": map[string]any{
				"required_status_checks": []any{"ci/build"},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("turning off the pull request rule removes its review settings", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposRulesetsByOwnerByRepoByID, protectedMainRuleset),
			WithRequestMatchHandler(putReposRulesetsByOwnerByRepoByID,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					rules := rulesByType(t, payload)
					assert.NotContains(t, rules, "pull_request")
					// Unrelated protections survive the removal.
					assert.Contains(t, rules, "non_fast_forward")

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(protectedMainRuleset))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "update",
			"owner":      "owner",
			"repo":       "repo",
			"ruleset_id": float64(42),
			"rules":      map[string]any{"require_pull_request": false},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("review settings alongside require_pull_request false are refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposRulesetsByOwnerByRepoByID, `{"id":42,"name":"empty","source":"owner/repo","enforcement":"active","rules":[]}`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "update",
			"owner":      "owner",
			"repo":       "repo",
			"ruleset_id": float64(42),
			"rules": map[string]any{
				"require_pull_request":            false,
				"required_approving_review_count": float64(2),
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "cannot set pull request review settings while require_pull_request is false")
	})

	t.Run("strict checks without a required checks rule are refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposRulesetsByOwnerByRepoByID, `{"id":42,"name":"empty","source":"owner/repo","enforcement":"active","rules":[]}`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "update",
			"owner":      "owner",
			"repo":       "repo",
			"ruleset_id": float64(42),
			"rules":      map[string]any{"strict_required_status_checks_policy": true},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "needs a required_status_checks rule")
	})

	t.Run("set_enforcement only changes enforcement", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposRulesetsByOwnerByRepoByID, protectedMainRuleset),
			WithRequestMatchHandler(putReposRulesetsByOwnerByRepoByID,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					assert.Equal(t, "disabled", payload["enforcement"])
					assert.Equal(t, "protect main", payload["name"])
					assert.Contains(t, rulesByType(t, payload), "pull_request")

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(protectedMainRuleset))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":      "set_enforcement",
			"owner":       "owner",
			"repo":        "repo",
			"ruleset_id":  float64(42),
			"enforcement": "disabled",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("set_enforcement without an enforcement value is refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "set_enforcement",
			"owner":      "owner",
			"repo":       "repo",
			"ruleset_id": float64(42),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "enforcement is required for set_enforcement")
	})

	t.Run("an ambiguous ruleset name is an error", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposRulesetsByOwnerByRepo, `[
				{"id":1,"name":"protect main","source":"owner/repo","source_type":"Repository","enforcement":"active"},
				{"id":2,"name":"protect main","source":"owner/repo","source_type":"Repository","enforcement":"active"}
			]`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "delete",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "protect main",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "is ambiguous")
	})

	t.Run("an inherited organization ruleset is not a candidate", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatch(getReposRulesetsByOwnerByRepo, `[
				{"id":9,"name":"org policy","source":"owner","source_type":"Organization","enforcement":"active"}
			]`),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "delete",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "org policy",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "no repository ruleset named 'org policy'")
	})

	t.Run("delete by id", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(deleteReposRulesetsByOwnerByRepoByID,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":     "delete",
			"owner":      "owner",
			"repo":       "repo",
			"ruleset_id": float64(42),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "ruleset 42 deleted successfully")
	})

	t.Run("merge queue defaults fill in the settings the API demands", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposRulesetsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					rules := rulesByType(t, payload)
					require.Contains(t, rules, "merge_queue")
					params := rules["merge_queue"]["parameters"].(map[string]any)
					assert.Equal(t, "SQUASH", params["merge_method"])
					assert.Equal(t, "ALLGREEN", params["grouping_strategy"])
					assert.Equal(t, float64(60), params["check_response_timeout_minutes"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(protectedMainRuleset))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "with queue",
			"rules": map[string]any{
				"merge_queue": map[string]any{"merge_method": "SQUASH"},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("bypass actors are replaced wholesale", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(postReposRulesetsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					actors := payload["bypass_actors"].([]any)
					require.Len(t, actors, 1)
					actor := actors[0].(map[string]any)
					assert.Equal(t, "Team", actor["actor_type"])
					assert.Equal(t, float64(99), actor["actor_id"])
					assert.Equal(t, "pull_request", actor["bypass_mode"])

					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(protectedMainRuleset))
				}),
			),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "create",
			"owner":  "owner",
			"repo":   "repo",
			"name":   "with bypass",
			"bypass_actors": []any{
				map[string]any{"actor_type": "Team", "actor_id": float64(99), "bypass_mode": "pull_request"},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "unexpected tool error: %v", result.Content)
	})

	t.Run("a bypass actor without a type is refused", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":        "create",
			"owner":         "owner",
			"repo":          "repo",
			"name":          "bad bypass",
			"bypass_actors": []any{map[string]any{"actor_id": float64(1)}},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		assert.Contains(t, getErrorResult(t, result).Text, "actor_type is required")
	})
}
