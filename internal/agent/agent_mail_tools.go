package agent

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"charm.land/fantasy"

	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

const (
	AgentMailSendToolName  = "agent_mail_send"
	AgentMailInboxToolName = "agent_mail_inbox"
	AgentMailAckToolName   = "agent_mail_ack"
)

type AgentMailSendParams struct {
	To       string `json:"to" description:"Recipient mailbox identity. Use a sub-agent id, 'main', 'parent', or 'self'."`
	Subject  string `json:"subject" description:"Short subject line for the coordination message."`
	Body     string `json:"body" description:"Message body."`
	Priority int    `json:"priority,omitempty" description:"Optional numeric priority. Higher values indicate higher urgency."`
	ThreadID string `json:"thread_id,omitempty" description:"Optional thread id to continue an existing conversation."`
}

type AgentMailInboxParams struct {
	UnreadOnly      *bool  `json:"unread_only,omitempty" description:"Accepted for backward compatibility. Delivery leasing is based on ack state, not unread state."`
	Limit           int    `json:"limit,omitempty" description:"Maximum messages to return. Defaults to 20."`
	ThreadID        string `json:"thread_id,omitempty" description:"Optional thread id to inspect a specific conversation."`
	MarkRead        *bool  `json:"mark_read,omitempty" description:"Mark returned messages as read for UI purposes only. Does not acknowledge delivery."`
	LeaseTTLSeconds int    `json:"lease_ttl_seconds,omitempty" description:"Optional lease duration in seconds for leased inbox items."`
}

type AgentMailAckParams struct {
	ID  string   `json:"id,omitempty" description:"Single message id to acknowledge."`
	IDs []string `json:"ids,omitempty" description:"One or more message ids to acknowledge."`
}

func (p AgentMailAckParams) MessageIDs() []string {
	ids := append([]string{}, p.IDs...)
	if strings.TrimSpace(p.ID) != "" {
		ids = append(ids, p.ID)
	}
	return uniqueStrings(ids)
}

func (c *coordinator) agentMailSendTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		AgentMailSendToolName,
		"Send a persistent coordination message between the main agent and sub-agents. Recipients can be sub-agent ids or the aliases 'main', 'parent', and 'self'.",
		func(ctx context.Context, params AgentMailSendParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if c.mailbox == nil {
				return fantasy.NewTextErrorResponse("mailbox service not initialized"), nil
			}
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			from := c.mailboxIdentityForSession(sessionID)
			if from == "" {
				return fantasy.NewTextErrorResponse("unable to resolve sender identity"), nil
			}
			to, err := c.resolveMailTarget(ctx, sessionID, params.To)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			threadID := strings.TrimSpace(params.ThreadID)
			if threadID == "" {
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(params.To)), "work:") {
					threadID = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(params.To), "work:"))
				} else if runner := c.runnerBySessionID(sessionID); runner != nil {
					runner.mu.Lock()
					threadID = strings.TrimSpace(runner.assignment.ID)
					runner.mu.Unlock()
				}
			}
			msg, err := c.mailbox.Send(ctx, to, from, strings.TrimSpace(params.Subject), strings.TrimSpace(params.Body), agentmailbox.SendOptions{
				Address:  strings.TrimSpace(params.To),
				Priority: params.Priority,
				ThreadID: threadID,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			c.recordOrchestrationActivity(ctx, from, "mail_sent", map[string]any{
				"to":                to,
				"address":           strings.TrimSpace(params.To),
				"resolved_to_agent": msg.ResolvedToAgent,
				"subject":           msg.Subject,
				"thread_id":         msg.ThreadID,
				"priority":          msg.Priority,
				"delivery_state":    msg.DeliveryState,
				"delivery_attempts": msg.DeliveryAttempts,
			})
			response := map[string]any{
				"id":                msg.ID,
				"address":           msg.Address,
				"to":                msg.ToAgent,
				"resolved_to_agent": msg.ResolvedToAgent,
				"from":              msg.FromAgent,
				"subject":           msg.Subject,
				"thread_id":         msg.ThreadID,
				"priority":          msg.Priority,
				"delivery_state":    msg.DeliveryState,
				"delivery_attempts": msg.DeliveryAttempts,
				"created_at":        msg.CreatedAt.UTC().Format(time.RFC3339),
			}
			return fantasy.NewTextResponse(marshalPrettyJSON(response)), nil
		},
	), nil
}

func (c *coordinator) agentMailInboxTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		AgentMailInboxToolName,
		"Lease actionable coordination mail for processing, or inspect a full thread. Read status is UI metadata only; delivery is completed by explicit ack.",
		func(ctx context.Context, params AgentMailInboxParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if c.mailbox == nil {
				return fantasy.NewTextErrorResponse("mailbox service not initialized"), nil
			}
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			agentID := c.mailboxIdentityForSession(sessionID)
			if agentID == "" {
				return fantasy.NewTextErrorResponse("unable to resolve mailbox identity"), nil
			}

			limit := params.Limit
			if limit <= 0 || limit > 100 {
				limit = 20
			}

			var (
				items []agentmailbox.Message
				err   error
			)
			if threadID := strings.TrimSpace(params.ThreadID); threadID != "" {
				items, err = c.mailbox.Thread(ctx, agentID, threadID, limit)
			} else {
				c.requeueExpiredMailLeases(ctx)
				leaseTTL := agentmailbox.DefaultLeaseTTL
				if params.LeaseTTLSeconds > 0 {
					leaseTTL = time.Duration(params.LeaseTTLSeconds) * time.Second
				}
				items, err = c.mailbox.LeaseInbox(ctx, agentID, agentID, limit, leaseTTL)
			}
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			markRead := params.MarkRead == nil || *params.MarkRead
			if markRead {
				for _, item := range items {
					_ = c.mailbox.MarkRead(ctx, agentID, item.ID)
				}
			}
			c.recordOrchestrationActivity(ctx, agentID, "mail_inbox_read", map[string]any{
				"count":     len(items),
				"thread_id": strings.TrimSpace(params.ThreadID),
				"leased":    params.ThreadID == "",
				"mark_read": markRead,
			})
			if len(items) == 0 {
				return fantasy.NewTextResponse("[]"), nil
			}
			payload := make([]map[string]any, 0, len(items))
			for _, item := range items {
				payload = append(payload, map[string]any{
					"id":                item.ID,
					"address":           item.Address,
					"to":                item.ToAgent,
					"resolved_to_agent": item.ResolvedToAgent,
					"from":              item.FromAgent,
					"subject":           item.Subject,
					"body":              item.Body,
					"priority":          item.Priority,
					"thread_id":         item.ThreadID,
					"delivery_state":    item.DeliveryState,
					"delivery_attempts": item.DeliveryAttempts,
					"lease_owner":       item.LeaseOwner,
					"lease_expires_at":  formatOptionalTime(item.LeaseExpiresAt),
					"read":              item.Read,
					"created_at":        item.CreatedAt.UTC().Format(time.RFC3339),
					"acked_at":          formatOptionalTime(item.AckedAt),
				})
			}
			return fantasy.NewTextResponse(marshalPrettyJSON(payload)), nil
		},
	), nil
}

func (c *coordinator) agentMailAckTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		AgentMailAckToolName,
		"Acknowledge leased or pending coordination mail after the current agent has handled it. This completes durable delivery.",
		func(ctx context.Context, params AgentMailAckParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if c.mailbox == nil {
				return fantasy.NewTextErrorResponse("mailbox service not initialized"), nil
			}
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			agentID := c.mailboxIdentityForSession(sessionID)
			if agentID == "" {
				return fantasy.NewTextErrorResponse("unable to resolve mailbox identity"), nil
			}

			ids := params.MessageIDs()
			if len(ids) == 0 {
				return fantasy.NewTextErrorResponse("id or ids is required"), nil
			}

			acked := make([]map[string]any, 0, len(ids))
			failed := make([]map[string]any, 0)
			for _, id := range ids {
				msg, err := c.mailbox.Ack(ctx, agentID, id)
				if err != nil {
					entry := map[string]any{"id": id, "error": err.Error()}
					if errors.Is(err, sql.ErrNoRows) {
						entry["error"] = "message not found or not ackable"
					}
					failed = append(failed, entry)
					continue
				}
				acked = append(acked, map[string]any{
					"id":             msg.ID,
					"thread_id":      msg.ThreadID,
					"delivery_state": msg.DeliveryState,
					"acked_at":       formatOptionalTime(msg.AckedAt),
				})
			}
			c.recordOrchestrationActivity(ctx, agentID, "mail_acknowledged", map[string]any{
				"acked_count": len(acked),
				"failed":      len(failed),
			})
			return fantasy.NewTextResponse(marshalPrettyJSON(map[string]any{
				"acked":  acked,
				"errors": failed,
			})), nil
		},
	), nil
}

func formatOptionalTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}
