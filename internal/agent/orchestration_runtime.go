package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const (
	mainAgentMailboxPrefix = "main:"
	mailboxNudgePrompt     = "You have new agent mail. Call `agent_mail_inbox` immediately, incorporate any coordination requests, then continue your assigned task."
)

func (c *coordinator) Close() error {
	if c == nil || c.orchestrationStore == nil {
		return nil
	}
	return c.orchestrationStore.Close()
}

func mainAgentMailboxID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	return mainAgentMailboxPrefix + sessionID
}

func (c *coordinator) mailboxIdentityForSession(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	if runner := c.runnerBySessionID(sessionID); runner != nil {
		return runner.id
	}
	return mainAgentMailboxID(sessionID)
}

func (c *coordinator) runnerBySessionID(sessionID string) *subAgentRunner {
	if c == nil || sessionID == "" {
		return nil
	}
	for _, runner := range c.ensureSubAgentRegistry().list() {
		if runner == nil {
			continue
		}
		runner.mu.Lock()
		matches := runner.sessionID == sessionID
		runner.mu.Unlock()
		if matches {
			return runner
		}
	}
	return nil
}

func (c *coordinator) syncRunnerOrchestrationState(ctx context.Context, runner *subAgentRunner) {
	if c == nil || c.orchestrationStore == nil || runner == nil {
		return
	}
	runner.mu.Lock()
	state := orchestrationdb.AgentState{
		AgentID:       runner.id,
		Role:          "subagent",
		Status:        string(runner.status),
		SessionID:     runner.sessionID,
		WorktreePath:  runner.workDir,
		Branch:        runner.assignment.Branch,
		ParentAgentID: mainAgentMailboxID(runner.parentSession),
		LastHeartbeat: time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	runner.mu.Unlock()
	if err := c.orchestrationStore.UpsertAgentState(ctx, state); err != nil {
		slog.Warn("Failed to persist orchestration state", "agent_id", state.AgentID, "error", err)
	}
}

func (c *coordinator) recordOrchestrationActivity(ctx context.Context, agentID, eventType string, details any) {
	if c == nil || c.orchestrationStore == nil || strings.TrimSpace(agentID) == "" || strings.TrimSpace(eventType) == "" {
		return
	}
	if err := c.orchestrationStore.RecordActivity(ctx, orchestrationdb.AgentActivity{
		AgentID:     agentID,
		EventType:   eventType,
		DetailsJSON: c.orchestrationStore.MarshalDetails(details),
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		slog.Warn("Failed to persist orchestration activity", "agent_id", agentID, "event_type", eventType, "error", err)
	}
}

func (c *coordinator) drainRunnerInboxSummary(ctx context.Context, runner *subAgentRunner) string {
	if c == nil || c.mailbox == nil || runner == nil {
		return ""
	}
	items, err := c.mailbox.Inbox(ctx, runner.id, true, 20)
	if err != nil || len(items) == 0 {
		return ""
	}
	var lines []string
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- From: %s | Subject: %s | Body: %s", item.FromAgent, item.Subject, item.Body))
		if markErr := c.mailbox.MarkRead(ctx, runner.id, item.ID); markErr != nil {
			slog.Warn("Failed to mark agent mail as read", "agent_id", runner.id, "message_id", item.ID, "error", markErr)
		}
	}
	c.recordOrchestrationActivity(ctx, runner.id, "mail_received", map[string]any{
		"count": len(items),
	})
	return "Mailbox update:\n" + strings.Join(lines, "\n") + "\n\nUse `agent_mail_inbox` if you need the full thread history. Continue your assigned task after incorporating the messages."
}

func (c *coordinator) nudgeMailboxRecipient(ctx context.Context, recipient string) error {
	if c == nil {
		return nil
	}
	recipient = strings.TrimSpace(recipient)
	if recipient == "" || strings.HasPrefix(recipient, mainAgentMailboxPrefix) {
		return nil
	}
	runner := c.ensureSubAgentRegistry().get(recipient)
	if runner == nil {
		return nil
	}
	runner.mu.Lock()
	closed := runner.closed
	status := runner.status
	pending := runner.pending
	runner.mu.Unlock()
	if closed || status == subAgentStatusRunning || pending > 0 {
		return nil
	}
	if runner.enqueue(mailboxNudgePrompt, nil) == "" {
		return nil
	}
	c.publishSubAgentEvent(SubAgentWaitingEvent, runner, runner.snapshot().LastSubmission, SubAgentStageWaiting, "")
	c.recordOrchestrationActivity(ctx, runner.id, "mail_nudged", map[string]any{"recipient": recipient})
	return nil
}

func (c *coordinator) resolveMailTarget(sessionID, rawTo string) (string, error) {
	to := strings.TrimSpace(rawTo)
	if to == "" {
		return "", fmt.Errorf("recipient is required")
	}
	senderRunner := c.runnerBySessionID(sessionID)
	switch strings.ToLower(to) {
	case "main", "parent":
		if senderRunner == nil || strings.TrimSpace(senderRunner.parentSession) == "" {
			return mainAgentMailboxID(sessionID), nil
		}
		return mainAgentMailboxID(senderRunner.parentSession), nil
	case "self":
		return c.mailboxIdentityForSession(sessionID), nil
	default:
		return to, nil
	}
}

func marshalPrettyJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}
