package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const checkpointSnippetLimit = 280

func (c *coordinator) writeSessionCheckpoint(ctx context.Context, sessionID, agentID, workItemID, auditSource string, summary map[string]any) {
	if c == nil || c.orchestrationStore == nil || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(agentID) == "" {
		return
	}
	auditTail := ""
	if auditSource != "" {
		auditTail = truncateForContext(c.GetLongHorizonAuditTail(auditSource, maxLongHorizonChars), maxLongHorizonChars)
	}
	summaryJSON := "{}"
	if summary != nil {
		if data, err := json.Marshal(summary); err == nil {
			summaryJSON = string(data)
		}
	}
	_, _ = c.orchestrationStore.SaveCheckpoint(ctx, orchestrationdb.SessionCheckpoint{
		SessionID:      sessionID,
		AgentID:        agentID,
		WorkItemID:     strings.TrimSpace(workItemID),
		SummaryJSON:    summaryJSON,
		AuditTail:      auditTail,
		MailCursor:     time.Now().UTC().Unix(),
		ActivityCursor: time.Now().UTC().Unix(),
		CreatedAt:      time.Now().UTC(),
	})
}

func buildCheckpointSummary(phase, prompt, result, status string, fields map[string]any) map[string]any {
	summary := map[string]any{
		"phase":  strings.TrimSpace(phase),
		"status": strings.TrimSpace(status),
	}
	if prompt = strings.TrimSpace(prompt); prompt != "" {
		summary["prompt"] = truncateForContext(strings.ReplaceAll(prompt, "\n", " "), checkpointSnippetLimit)
	}
	if result = strings.TrimSpace(result); result != "" {
		summary["result"] = truncateForContext(strings.ReplaceAll(result, "\n", " "), checkpointSnippetLimit)
	}
	for key, value := range fields {
		summary[key] = value
	}
	return summary
}

func (c *coordinator) renderCheckpointContext(ctx context.Context, sessionID, agentID string) string {
	if c == nil || c.orchestrationStore == nil || strings.TrimSpace(sessionID) == "" {
		return ""
	}
	checkpoint, err := c.orchestrationStore.LatestCheckpoint(ctx, sessionID, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ""
		}
		return ""
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(checkpoint.SummaryJSON), &summary); err != nil {
		return ""
	}
	lines := make([]string, 0, 6)
	if phase := firstSummaryString(summary, "phase"); phase != "" {
		lines = append(lines, fmt.Sprintf("- Phase: %s", phase))
	}
	if status := firstSummaryString(summary, "status"); status != "" {
		lines = append(lines, fmt.Sprintf("- Status: %s", status))
	}
	if prompt := firstSummaryString(summary, "prompt"); prompt != "" {
		lines = append(lines, fmt.Sprintf("- Prompt: %s", prompt))
	}
	if result := firstSummaryString(summary, "result"); result != "" {
		lines = append(lines, fmt.Sprintf("- Result: %s", result))
	}
	if workItem := strings.TrimSpace(checkpoint.WorkItemID); workItem != "" {
		lines = append(lines, fmt.Sprintf("- Work item: %s", workItem))
	}
	lines = append(lines, fmt.Sprintf("- Checkpoint age: %s", time.Since(checkpoint.CreatedAt).Truncate(time.Second)))
	if auditTail := strings.TrimSpace(checkpoint.AuditTail); auditTail != "" {
		lines = append(lines, "- Audit tail: "+truncateForContext(strings.ReplaceAll(auditTail, "\n", " "), checkpointSnippetLimit))
	}
	if len(lines) == 0 {
		return ""
	}
	return "### Latest Checkpoint\n" + strings.Join(lines, "\n")
}

func firstSummaryString(summary map[string]any, key string) string {
	if len(summary) == 0 {
		return ""
	}
	value, ok := summary[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
