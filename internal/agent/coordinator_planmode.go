package agent

import (
	"context"
	"strings"

	"charm.land/fantasy"
	agentformula "github.com/duggal1/Sapphire-cli/internal/agent/formula"
	"github.com/duggal1/Sapphire-cli/internal/message"
)

func (c *coordinator) initPlanMode() error {
	if c == nil {
		return nil
	}
	c.toolRegistry = nil
	c.formulaExecutor = nil
	return nil
}

func (c *coordinator) RunPlanMode(ctx context.Context, sessionID, task, _ string) (*agentformula.ExecutionState, error) {
	if c == nil {
		return nil, nil
	}
	_, err := c.Run(ctx, sessionID, task)
	return nil, err
}

func (c *coordinator) ResolvePlanApproval(_ context.Context, sessionID string, approved bool) error {
	if c == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	c.planApprovalMu.Lock()
	waiter, ok := c.planApprovalWaiters[sessionID]
	if ok {
		delete(c.planApprovalWaiters, sessionID)
	}
	c.planApprovalMu.Unlock()
	if ok {
		select {
		case waiter <- approved:
		default:
		}
	}
	return nil
}

func extractAgentResultText(result *fantasy.AgentResult) string {
	if result == nil {
		return ""
	}
	return strings.TrimSpace(result.Response.Content.Text())
}

func (c *coordinator) extractSingularityResultText(ctx context.Context, sessionID string, result *fantasy.AgentResult) string {
	text := strings.TrimSpace(extractAgentResultText(result))
	if c == nil || c.messages == nil || strings.TrimSpace(sessionID) == "" {
		return text
	}
	msgs, err := c.messages.List(ctx, sessionID)
	if err != nil {
		return text
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg.SessionID != sessionID || msg.Role != message.Assistant || msg.IsSummaryMessage {
			continue
		}
		candidate := strings.TrimSpace(msg.Content().Text)
		if candidate == "" {
			continue
		}
		if len(candidate) >= len(text) {
			return candidate
		}
		break
	}
	return text
}
