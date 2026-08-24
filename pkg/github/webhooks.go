package github

import (
	"context"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	webhooksMethodList           = "list"
	webhooksMethodGet            = "get"
	webhooksMethodGetConfig      = "get_config"
	webhooksMethodListDeliveries = "list_deliveries"
	webhooksMethodGetDelivery    = "get_delivery"

	webhookWriteMethodCreate    = "create"
	webhookWriteMethodUpdate    = "update"
	webhookWriteMethodDelete    = "delete"
	webhookWriteMethodPing      = "ping"
	webhookWriteMethodRedeliver = "redeliver"
)

// MinimalWebhook is the trimmed output type for a repository webhook.
//
// It deliberately never carries the config secret: GitHub itself never
// returns the plaintext value, and this type has no field for it, so a
// secret cannot leak into a tool result or a log line by accident.
type MinimalWebhook struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name,omitempty"`
	Active      bool     `json:"active"`
	Events      []string `json:"events,omitempty"`
	URL         string   `json:"url,omitempty"`
	ContentType string   `json:"content_type,omitempty"`
	InsecureSSL string   `json:"insecure_ssl,omitempty"`
	PingURL     string   `json:"ping_url,omitempty"`
}

func convertToMinimalWebhook(h *github.Hook) MinimalWebhook {
	cfg := h.GetConfig()
	return MinimalWebhook{
		ID:          h.GetID(),
		Name:        h.GetName(),
		Active:      h.GetActive(),
		Events:      h.Events,
		URL:         cfg.GetURL(),
		ContentType: cfg.GetContentType(),
		InsecureSSL: cfg.GetInsecureSSL(),
		PingURL:     h.GetPingURL(),
	}
}

// MinimalHookDelivery is the trimmed output type for a webhook delivery attempt.
type MinimalHookDelivery struct {
	ID         int64  `json:"id"`
	GUID       string `json:"guid,omitempty"`
	Event      string `json:"event,omitempty"`
	Action     string `json:"action,omitempty"`
	Status     string `json:"status,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Redelivery bool   `json:"redelivery,omitempty"`
}

func convertToMinimalHookDelivery(d *github.HookDelivery) MinimalHookDelivery {
	return MinimalHookDelivery{
		ID:         d.GetID(),
		GUID:       d.GetGUID(),
		Event:      d.GetEvent(),
		Action:     d.GetAction(),
		Status:     d.GetStatus(),
		StatusCode: d.GetStatusCode(),
		Redelivery: d.GetRedelivery(),
	}
}

// WebhooksRead creates a tool to inspect a repository's webhooks and their delivery history.
func WebhooksRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepoGovernance,
		mcp.Tool{
			Name: "webhooks_read",
			Description: t("TOOL_WEBHOOKS_READ_DESCRIPTION", "Read a repository's webhooks: list them, get one's details or delivery config, or inspect its delivery history. "+
				"The config secret is never returned by the GitHub API and is never included in the result."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_WEBHOOKS_READ_USER_TITLE", "Read webhooks"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The method to execute:\n" +
							"- list: all webhooks configured on the repository\n" +
							"- get: one webhook's details\n" +
							"- get_config: one webhook's delivery config (URL, content type, SSL verification)\n" +
							"- list_deliveries: recent delivery attempts for a webhook\n" +
							"- get_delivery: one delivery's status",
						Enum: []any{webhooksMethodList, webhooksMethodGet, webhooksMethodGetConfig, webhooksMethodListDeliveries, webhooksMethodGetDelivery},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"hook_id": {
						Type:        "integer",
						Description: "Webhook ID. Required for all methods except 'list'.",
					},
					"delivery_id": {
						Type:        "integer",
						Description: "Delivery ID. Required for 'get_delivery'.",
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

			switch method {
			case webhooksMethodList:
				hooks, resp, err := client.Repositories.ListHooks(ctx, owner, repo, &github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list webhooks", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalWebhook, 0, len(hooks))
				for _, h := range hooks {
					if h != nil {
						minimal = append(minimal, convertToMinimalWebhook(h))
					}
				}
				return marshalGovernanceResult(minimal, nil)

			case webhooksMethodGet:
				hookID, err := RequiredInt(args, "hook_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				hook, resp, err := client.Repositories.GetHook(ctx, owner, repo, int64(hookID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get webhook %d", hookID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalWebhook(hook), nil)

			case webhooksMethodGetConfig:
				hookID, err := RequiredInt(args, "hook_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				cfg, resp, err := client.Repositories.GetHookConfiguration(ctx, owner, repo, int64(hookID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get config for webhook %d", hookID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				// The secret field is deliberately never read: GitHub redacts
				// it in this response, and it is left out here on purpose.
				return marshalGovernanceResult(map[string]any{
					"url":          cfg.GetURL(),
					"content_type": cfg.GetContentType(),
					"insecure_ssl": cfg.GetInsecureSSL(),
				}, nil)

			case webhooksMethodListDeliveries:
				hookID, err := RequiredInt(args, "hook_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				deliveries, resp, err := client.Repositories.ListHookDeliveries(ctx, owner, repo, int64(hookID), &github.ListCursorOptions{PerPage: pagination.PerPage})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to list deliveries for webhook %d", hookID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				minimal := make([]MinimalHookDelivery, 0, len(deliveries))
				for _, d := range deliveries {
					if d != nil {
						minimal = append(minimal, convertToMinimalHookDelivery(d))
					}
				}
				return marshalGovernanceResult(minimal, nil)

			case webhooksMethodGetDelivery:
				hookID, err := RequiredInt(args, "hook_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				deliveryID, err := RequiredInt(args, "delivery_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				delivery, resp, err := client.Repositories.GetHookDelivery(ctx, owner, repo, int64(hookID), int64(deliveryID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to get delivery %d for webhook %d", deliveryID, hookID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalHookDelivery(delivery), nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: list, get, get_config, list_deliveries, get_delivery", method)), nil, nil
			}
		},
	)
}

// WebhookWrite creates a tool to create, update, delete, ping and redeliver a repository webhook.
func WebhookWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepoGovernance,
		mcp.Tool{
			Name: "webhook_write",
			Description: t("TOOL_WEBHOOK_WRITE_DESCRIPTION", "Create, update or delete a repository webhook, send it a ping, or redeliver a past delivery. "+
				"The secret, if set, is write-only: it is sent to GitHub but never returned or logged by this tool."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_WEBHOOK_WRITE_USER_TITLE", "Manage webhooks"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: "The operation to perform:\n" +
							"- create: create a new webhook\n" +
							"- update: change an existing webhook's URL, secret, content type, SSL verification, events or active state\n" +
							"- delete: permanently remove a webhook\n" +
							"- ping: send a ping event to test delivery\n" +
							"- redeliver: resend a past delivery",
						Enum: []any{webhookWriteMethodCreate, webhookWriteMethodUpdate, webhookWriteMethodDelete, webhookWriteMethodPing, webhookWriteMethodRedeliver},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"hook_id": {
						Type:        "integer",
						Description: "Webhook ID. Required for all methods except 'create'.",
					},
					"delivery_id": {
						Type:        "integer",
						Description: "Delivery ID. Required for 'redeliver'.",
					},
					"url": {
						Type:        "string",
						Description: "Payload delivery URL. Required for 'create'; optional for 'update'.",
					},
					"secret": {
						Type:        "string",
						Description: "Shared secret GitHub uses to sign payloads. Optional. Never returned by any read tool once set.",
					},
					"content_type": {
						Type:        "string",
						Description: "Payload format. Defaults to 'form'.",
						Enum:        []any{"json", "form"},
					},
					"insecure_ssl": {
						Type:        "string",
						Description: "'0' to verify SSL certs (default), '1' to skip verification.",
						Enum:        []any{"0", "1"},
					},
					"events": {
						Type:        "array",
						Items:       &jsonschema.Schema{Type: "string"},
						Description: "GitHub event names that trigger this webhook, e.g. 'push', 'pull_request'. Defaults to ['push'] on create.",
					},
					"active": {
						Type:        "boolean",
						Description: "Whether the webhook is active. Defaults to true on create.",
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
			case webhookWriteMethodCreate:
				url, err := RequiredParam[string](args, "url")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				secret, err := OptionalParam[string](args, "secret")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				contentType, err := OptionalParam[string](args, "content_type")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				insecureSSL, err := OptionalParam[string](args, "insecure_ssl")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				events, hasEvents, err := optionalStringSlice(args, "events")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				active, err := OptionalBoolParamWithDefault(args, "active", true)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				if !hasEvents || len(events) == 0 {
					events = []string{"push"}
				}

				cfg := &github.HookConfig{URL: &url}
				if secret != "" {
					cfg.Secret = &secret
				}
				if contentType != "" {
					cfg.ContentType = &contentType
				}
				if insecureSSL != "" {
					cfg.InsecureSSL = &insecureSSL
				}

				created, resp, err := client.Repositories.CreateHook(ctx, owner, repo, &github.Hook{
					Config: cfg,
					Events: events,
					Active: &active,
				})
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create webhook", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalWebhook(created), nil)

			case webhookWriteMethodUpdate:
				hookID, err := RequiredInt(args, "hook_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				url, err := OptionalParam[string](args, "url")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				secret, err := OptionalParam[string](args, "secret")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				contentType, err := OptionalParam[string](args, "content_type")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				insecureSSL, err := OptionalParam[string](args, "insecure_ssl")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				events, hasEvents, err := optionalStringSlice(args, "events")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				_, hasActive := args["active"]
				active, err := OptionalParam[bool](args, "active")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				hook := &github.Hook{}
				if hasEvents {
					hook.Events = events
				}
				if hasActive {
					hook.Active = &active
				}
				if url != "" || secret != "" || contentType != "" || insecureSSL != "" {
					cfg := &github.HookConfig{}
					if url != "" {
						cfg.URL = &url
					}
					if secret != "" {
						cfg.Secret = &secret
					}
					if contentType != "" {
						cfg.ContentType = &contentType
					}
					if insecureSSL != "" {
						cfg.InsecureSSL = &insecureSSL
					}
					hook.Config = cfg
				}

				updated, resp, err := client.Repositories.EditHook(ctx, owner, repo, int64(hookID), hook)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to update webhook %d", hookID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(convertToMinimalWebhook(updated), nil)

			case webhookWriteMethodDelete:
				hookID, err := RequiredInt(args, "hook_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				resp, err := client.Repositories.DeleteHook(ctx, owner, repo, int64(hookID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to delete webhook %d", hookID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(map[string]any{"result": "webhook_deleted", "hook_id": hookID}, nil)

			case webhookWriteMethodPing:
				hookID, err := RequiredInt(args, "hook_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				resp, err := client.Repositories.PingHook(ctx, owner, repo, int64(hookID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to ping webhook %d", hookID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(map[string]any{"result": "ping_sent", "hook_id": hookID}, nil)

			case webhookWriteMethodRedeliver:
				hookID, err := RequiredInt(args, "hook_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				deliveryID, err := RequiredInt(args, "delivery_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				_, resp, err := client.Repositories.RedeliverHookDelivery(ctx, owner, repo, int64(hookID), int64(deliveryID))
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to redeliver delivery %d for webhook %d", deliveryID, hookID), resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()
				return marshalGovernanceResult(map[string]any{"result": "redelivery_requested", "hook_id": hookID, "delivery_id": deliveryID}, nil)

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create, update, delete, ping, redeliver", method)), nil, nil
			}
		},
	)
}
