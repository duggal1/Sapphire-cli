package agent

import (
	"context"
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
)

type AgentMailSendParams struct {
	To       string `json:"to" description:"Recipient mailbox identity. Use a sub-agent id, 'main', 'parent', or 'self'."`
	Subject  string `json:"subject" description:"Short subject line for the coordination message."`
	Body     string `json:"body" description:"Message body."`
	Priority int    `json:"priority,omitempty" description:"Optional numeric priority. Higher values indicate higher urgency."`
	ThreadID string `json:"thread_id,omitempty" description:"Optional thread id to continue an existing conversation."`
}

type AgentMailInboxParams struct {
	UnreadOnly *bool  `json:"unread_only,omitempty" description:"When true, return only unread mail. Defaults to true."`
	Limit      int    `json:"limit,omitempty" description:"Maximum messages to return. Defaults to 20."`
	ThreadID   string `json:"thread_id,omitempty" description:"Optional thread id to inspect a specific conversation."`
	MarkRead   *bool  `json:"mark_read,omitempty" description:"Mark returned inbox messages as read. Defaults to true for inbox mode."`
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
			to, err := c.resolveMailTarget(sessionID, params.To)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			msg, err := c.mailbox.Send(ctx, to, from, strings.TrimSpace(params.Subject), strings.TrimSpace(params.Body), agentmailbox.SendOptions{
				Priority: params.Priority,
				ThreadID: strings.TrimSpace(params.ThreadID),
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			c.recordOrchestrationActivity(ctx, from, "mail_sent", map[string]any{
				"to":        to,
				"subject":   msg.Subject,
				"thread_id": msg.ThreadID,
				"priority":  msg.Priority,
			})
			response := map[string]any{
				"id":         msg.ID,
				"to":         msg.ToAgent,
				"from":       msg.FromAgent,
				"subject":    msg.Subject,
				"thread_id":  msg.ThreadID,
				"priority":   msg.Priority,
				"created_at": msg.CreatedAt.UTC().Format(time.RFC3339),
			}
			return fantasy.NewTextResponse(marshalPrettyJSON(response)), nil
		},
	), nil
}

func (c *coordinator) agentMailInboxTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		AgentMailInboxToolName,
		"Read persistent coordination messages for the current agent. Supports unread inbox mode and full thread retrieval.",
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
				unreadOnly := params.UnreadOnly == nil || *params.UnreadOnly
				items, err = c.mailbox.Inbox(ctx, agentID, unreadOnly, limit)
			}
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			markRead := params.MarkRead == nil || *params.MarkRead
			if params.ThreadID == "" && markRead {
				for _, item := range items {
					_ = c.mailbox.MarkRead(ctx, agentID, item.ID)
				}
			}
			c.recordOrchestrationActivity(ctx, agentID, "mail_inbox_read", map[string]any{
				"count":       len(items),
				"thread_id":   strings.TrimSpace(params.ThreadID),
				"unread_only": params.ThreadID == "" && (params.UnreadOnly == nil || *params.UnreadOnly),
			})
			if len(items) == 0 {
				return fantasy.NewTextResponse("[]"), nil
			}
			payload := make([]map[string]any, 0, len(items))
			for _, item := range items {
				payload = append(payload, map[string]any{
					"id":         item.ID,
					"to":         item.ToAgent,
					"from":       item.FromAgent,
					"subject":    item.Subject,
					"body":       item.Body,
					"priority":   item.Priority,
					"thread_id":  item.ThreadID,
					"read":       item.Read,
					"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
				})
			}
			return fantasy.NewTextResponse(marshalPrettyJSON(payload)), nil
		},
	), nil
}
