package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
	deploymentsMethodList         = "list_deployments"
	deploymentsMethodGet          = "get_deployment"
	deploymentsMethodListStatuses = "list_deployment_statuses"
	deploymentWriteMethodCreate   = "create_deployment"
	deploymentWriteMethodStatus   = "create_deployment_status"
)

// MinimalDeployment is the trimmed output type for a deployment. The SHA is
// kept because it is the link back from an environment to the commit and the
// workflow run that produced it.
type MinimalDeployment struct {
	ID          int64        `json:"id"`
	SHA         string       `json:"sha,omitempty"`
	Ref         string       `json:"ref,omitempty"`
	Task        string       `json:"task,omitempty"`
	Environment string       `json:"environment,omitempty"`
	Description string       `json:"description,omitempty"`
	Creator     *MinimalUser `json:"creator,omitempty"`
	CreatedAt   string       `json:"created_at,omitempty"`
	UpdatedAt   string       `json:"updated_at,omitempty"`
	StatusesURL string       `json:"statuses_url,omitempty"`
}

// MinimalDeploymentStatus is the trimmed output type for a deployment status.
type MinimalDeploymentStatus struct {
	ID             int64        `json:"id"`
	State          string       `json:"state"`
	Environment    string       `json:"environment,omitempty"`
	Description    string       `json:"description,omitempty"`
	EnvironmentURL string       `json:"environment_url,omitempty"`
	LogURL         string       `json:"log_url,omitempty"`
	Creator        *MinimalUser `json:"creator,omitempty"`
	CreatedAt      string       `json:"created_at,omitempty"`
}

func convertToMinimalDeployment(deployment *github.Deployment) MinimalDeployment {
	m := MinimalDeployment{
		ID:          deployment.GetID(),
		SHA:         deployment.GetSHA(),
		Ref:         deployment.GetRef(),
		Task:        deployment.GetTask(),
		Environment: deployment.GetEnvironment(),
		Description: sanitize.Sanitize(deployment.GetDescription()),
		Creator:     convertToMinimalUser(deployment.GetCreator()),
		StatusesURL: deployment.GetStatusesURL(),
	}
	if deployment.CreatedAt != nil {
		m.CreatedAt = deployment.CreatedAt.Format(time.RFC3339)
	}
	if deployment.UpdatedAt != nil {
		m.UpdatedAt = deployment.UpdatedAt.Format(time.RFC3339)
	}
	return m
}

func convertToMinimalDeploymentStatus(status *github.DeploymentStatus) MinimalDeploymentStatus {
	m := MinimalDeploymentStatus{
		ID:             status.GetID(),
		State:          status.GetState(),
		Environment:    status.GetEnvironment(),
		Description:    sanitize.Sanitize(status.GetDescription()),
		EnvironmentURL: status.GetEnvironmentURL(),
		LogURL:         status.GetLogURL(),
		Creator:        convertToMinimalUser(status.GetCreator()),
	}
	if status.CreatedAt != nil {
		m.CreatedAt = status.CreatedAt.Format(time.RFC3339)
	}
	return m
}

// DeploymentsRead creates a tool to read deployments and their statuses.
func DeploymentsRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDeployments,
		mcp.Tool{
			Name: "deployments_read",
			Description: t("TOOL_DEPLOYMENTS_READ_DESCRIPTION", "Read a repository's deployments and their statuses: what was deployed, to which environment, from which commit, and how it ended. "+
				"Each deployment carries the SHA it came from, which is the link back to the commit and the workflow run that produced it."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_DEPLOYMENTS_READ_USER_TITLE", "Read deployments"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list_deployments: deployments, newest first, optionally filtered by environment, ref or SHA\n" +
							"- get_deployment: one deployment by ID\n" +
							"- list_deployment_statuses: the status history of one deployment, newest first",
						Enum: []any{deploymentsMethodList, deploymentsMethodGet, deploymentsMethodListStatuses},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"deployment_id": {
						Type:        "number",
						Description: "Numeric deployment ID. Required for 'get_deployment' and 'list_deployment_statuses'.",
					},
					"environment": {
						Type:        "string",
						Description: "Only list deployments to this environment.",
					},
					"ref": {
						Type:        "string",
						Description: "Only list deployments of this branch or tag.",
					},
					"sha": {
						Type:        "string",
						Description: "Only list deployments of this commit SHA. Use this to go from a commit to what it deployed.",
					},
					"task": {
						Type:        "string",
						Description: "Only list deployments for this task, e.g. 'deploy'.",
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

			label := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				// A deployment record reflects what CI did with the repository's
				// code, so it follows repository visibility like other Actions
				// output.
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelActionsResult)
			}

			switch method {
			case deploymentsMethodList:
				environment, err := OptionalParam[string](args, "environment")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				ref, err := OptionalParam[string](args, "ref")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				sha, err := OptionalParam[string](args, "sha")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				task, err := OptionalParam[string](args, "task")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				deployments, resp, err := client.Repositories.ListDeployments(ctx, owner, repo, &github.DeploymentsListOptions{
					Environment: environment,
					Ref:         ref,
					SHA:         sha,
					Task:        task,
					ListOptions: listOpts,
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list deployments", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalDeployment, 0, len(deployments))
				for _, deployment := range deployments {
					if deployment != nil {
						minimal = append(minimal, convertToMinimalDeployment(deployment))
					}
				}

				return marshalDeploymentResult(minimal, label)

			case deploymentsMethodGet:
				id, err := RequiredInt(args, "deployment_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				deployment, resp, err := client.Repositories.GetDeployment(ctx, owner, repo, int64(id))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get deployment", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(convertToMinimalDeployment(deployment), label)

			case deploymentsMethodListStatuses:
				id, err := RequiredInt(args, "deployment_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				statuses, resp, err := client.Repositories.ListDeploymentStatuses(ctx, owner, repo, int64(id), &listOpts)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list deployment statuses", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalDeploymentStatus, 0, len(statuses))
				for _, status := range statuses {
					if status != nil {
						minimal = append(minimal, convertToMinimalDeploymentStatus(status))
					}
				}

				return marshalDeploymentResult(minimal, label)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list_deployments, get_deployment, list_deployment_statuses", method)), nil, nil
			}
		},
	)
}

// DeploymentWrite creates a tool to record deployments and their outcomes.
func DeploymentWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDeployments,
		mcp.Tool{
			Name: "deployment_write",
			Description: t("TOOL_DEPLOYMENT_WRITE_DESCRIPTION", "Record a deployment, or record what happened to one. "+
				"Creating a deployment asks GitHub to deploy a ref to an environment; recording a status afterwards is what makes the outcome and the deployed URL visible on the commit and in the environment's history."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_DEPLOYMENT_WRITE_USER_TITLE", "Record deployments"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create_deployment' or 'create_deployment_status'",
						Enum:        []any{deploymentWriteMethodCreate, deploymentWriteMethodStatus},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"ref": {
						Type:        "string",
						Description: "Branch, tag or commit SHA to deploy. Required for 'create_deployment'.",
					},
					"environment": {
						Type:        "string",
						Description: "Environment being deployed to, e.g. 'staging'. Defaults to 'production'.",
					},
					"task": {
						Type:        "string",
						Description: "What the deployment does. Defaults to 'deploy'.",
					},
					"description": {
						Type:        "string",
						Description: "Short description of the deployment or of its status.",
					},
					"auto_merge": {
						Type:        "boolean",
						Description: "Let GitHub merge the default branch into the ref first, failing the request if that conflicts. Defaults to false, which deploys the ref exactly as it is.",
					},
					"required_contexts": {
						Type:        "array",
						Description: "Status check contexts that must pass before the deployment is created. Pass an empty array to skip the check entirely; omit it to let GitHub require every context on the ref.",
						Items:       &jsonschema.Schema{Type: "string"},
					},
					"transient_environment": {
						Type:        "boolean",
						Description: "Mark the environment as temporary, e.g. a per-pull-request preview that will disappear.",
					},
					"production_environment": {
						Type:        "boolean",
						Description: "Mark the environment as production. Defaults to GitHub's guess from the environment name.",
					},
					"deployment_id": {
						Type:        "number",
						Description: "Deployment the status belongs to. Required for 'create_deployment_status'.",
					},
					"state": {
						Type:        "string",
						Description: "How the deployment is going. Required for 'create_deployment_status'.",
						Enum:        []any{"queued", "in_progress", "success", "failure", "error", "inactive"},
					},
					"environment_url": {
						Type:        "string",
						Description: "Where the deployed code can be reached. This is what the environment links to once the deployment succeeds.",
					},
					"log_url": {
						Type:        "string",
						Description: "Where the deployment output can be read, e.g. the workflow run.",
					},
					"auto_inactive": {
						Type:        "boolean",
						Description: "On a successful status, mark earlier deployments to the same environment inactive. Defaults to true on GitHub's side.",
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
			environment, err := OptionalParam[string](args, "environment")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			description, err := OptionalParam[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case deploymentWriteMethodCreate:
				ref, err := RequiredParam[string](args, "ref")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				task, err := OptionalParam[string](args, "task")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				request := github.DeploymentRequest{
					Ref:         ref,
					Task:        ToStringPtr(task),
					Environment: ToStringPtr(environment),
					Description: ToStringPtr(description),
				}

				for key, target := range map[string]**bool{
					"auto_merge":             &request.AutoMerge,
					"transient_environment":  &request.TransientEnvironment,
					"production_environment": &request.ProductionEnvironment,
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
				// Deploying a ref exactly as given is the useful default;
				// GitHub's own default merges the base branch in first.
				if request.AutoMerge == nil {
					request.AutoMerge = github.Ptr(false)
				}

				// An empty array and an absent value mean different things
				// here: the first skips the status check, the second requires
				// every context on the ref.
				if contexts, ok, err := optionalStringSlice(args, "required_contexts"); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				} else if ok {
					if contexts == nil {
						contexts = []string{}
					}
					request.RequiredContexts = contexts
				}

				deployment, resp, err := client.Repositories.CreateDeployment(ctx, owner, repo, request)
				if err != nil {
					// A 202 carrying a message instead of a deployment means
					// GitHub declined: a required check has not passed, or the
					// auto-merge it wanted to do conflicts. The client reports
					// that as "job scheduled", which is not what happened.
					var accepted *github.AcceptedError
					if errors.As(err, &accepted) {
						return utils.NewToolResultError(fmt.Sprintf(
							"GitHub accepted the request but created no deployment (%s). This happens when a required status check has not passed, or when auto_merge is on and the ref conflicts with its base. Check the ref's status checks, or pass required_contexts: [] to deploy regardless.",
							strings.TrimSpace(string(accepted.Raw)))), nil, nil
					}
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create deployment", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(convertToMinimalDeployment(deployment), nil)

			case deploymentWriteMethodStatus:
				id, err := RequiredInt(args, "deployment_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				state, err := RequiredParam[string](args, "state")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				environmentURL, err := OptionalParam[string](args, "environment_url")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				logURL, err := OptionalParam[string](args, "log_url")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				request := github.DeploymentStatusRequest{
					State:          state,
					Environment:    ToStringPtr(environment),
					Description:    ToStringPtr(description),
					EnvironmentURL: ToStringPtr(environmentURL),
					LogURL:         ToStringPtr(logURL),
				}
				if raw, ok := args["auto_inactive"]; ok && raw != nil {
					value, ok := raw.(bool)
					if !ok {
						return utils.NewToolResultError("auto_inactive must be a boolean"), nil, nil
					}
					request.AutoInactive = github.Ptr(value)
				}

				status, resp, err := client.Repositories.CreateDeploymentStatus(ctx, owner, repo, int64(id), request)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create deployment status", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				return marshalDeploymentResult(convertToMinimalDeploymentStatus(status), nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create_deployment, create_deployment_status", method)), nil, nil
			}
		},
	)
}

// optionalStringSlice reads an optional array-of-strings argument, reporting
// whether it was present. An explicitly empty array is present but empty, which
// callers use to mean "none" rather than "unspecified".
func optionalStringSlice(args map[string]any, key string) ([]string, bool, error) {
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

func marshalDeploymentResult(payload any, label func(*mcp.CallToolResult) *mcp.CallToolResult) (*mcp.CallToolResult, any, error) {
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
