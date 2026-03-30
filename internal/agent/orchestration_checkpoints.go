package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const checkpointSnippetLimit = 280

func (c *coordinator) writeSessionCheckpoint(ctx context.Context, sessionID, agentID, workItemID, auditSource string, summary map[string]any) {
	if c == nil || c.checkpointService == nil || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(agentID) == "" {
		return
	}
	c.countSubAgentLaunchMetric("db.checkpoint_write", 1)
	auditTail := ""
	if auditSource != "" {
		auditTail = truncateForContext(c.GetLongHorizonAuditTail(auditSource, maxLongHorizonChars), maxLongHorizonChars)
	}
	phase := firstSummaryString(summary, "phase")
	status := firstSummaryString(summary, "status")
	prompt := firstSummaryString(summary, "prompt")
	result := firstSummaryString(summary, "result")
	mailCursor, activityCursor := c.currentCheckpointCursors(ctx, sessionID, agentID)
	_, _, _ = c.checkpointService.Record(ctx, agentmemory.CheckpointParams{
		SessionID:      sessionID,
		AgentID:        agentID,
		WorkItemID:     strings.TrimSpace(workItemID),
		AuditTail:      auditTail,
		Phase:          phase,
		Prompt:         prompt,
		Result:         result,
		Status:         status,
		Summary:        summary,
		Force:          true,
		MailCursor:     mailCursor,
		ActivityCursor: activityCursor,
	})
	if c.shouldPersistCheckpointHandoff(sessionID, agentID, strings.TrimSpace(workItemID), prompt, status) {
		_ = c.memoryCompiler.PersistHandoff(ctx, agentmemory.CompileRequest{
			SessionID:  sessionID,
			AgentID:    agentID,
			WorkingDir: c.mainWorkingDir(),
			Task:       prompt,
		})
	}
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
	if c == nil || c.checkpointService == nil || strings.TrimSpace(sessionID) == "" {
		return ""
	}
	snapshot, err := c.checkpointService.Resume(ctx, sessionID, agentID)
	if err != nil {
		return ""
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(snapshot.Checkpoint.SummaryJSON), &summary); err != nil {
		return ""
	}
	lines := make([]string, 0, 16)
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
	if summaryText := firstSummaryString(summary, "summary"); summaryText != "" {
		lines = append(lines, fmt.Sprintf("- Summary: %s", summaryText))
	}
	if workItem := strings.TrimSpace(snapshot.Checkpoint.WorkItemID); workItem != "" {
		lines = append(lines, fmt.Sprintf("- Work item: %s", workItem))
	}
	if snapshot.Checkpoint.MessageCount > 0 {
		lines = append(lines, fmt.Sprintf("- Message count: %d", snapshot.Checkpoint.MessageCount))
	}
	if len(snapshot.PendingTasks) > 0 {
		lines = append(lines, "- Pending tasks: "+strings.Join(limitSlice(snapshot.PendingTasks, 3), " | "))
	}
	if len(snapshot.FilesModified) > 0 {
		lines = append(lines, "- Files modified: "+strings.Join(limitSlice(snapshot.FilesModified, 3), " | "))
	}
	if len(snapshot.UserPreferences) > 0 {
		lines = append(lines, "- Preferences: "+renderPreferencesInline(snapshot.UserPreferences, 3))
	}
	if len(snapshot.Decisions) > 0 {
		lines = append(lines, "- Decisions: "+renderDecisionsInline(snapshot.Decisions, 3))
	}
	if len(snapshot.DecisionConflicts) > 0 {
		lines = append(lines, "- Decision conflicts: "+strings.Join(limitSlice(snapshot.DecisionConflicts, 2), " | "))
	}
	lines = append(lines, fmt.Sprintf("- Checkpoint age: %s", time.Since(snapshot.Checkpoint.CreatedAt).Truncate(time.Second)))
	if auditTail := strings.TrimSpace(snapshot.Checkpoint.AuditTail); auditTail != "" {
		lines = append(lines, "- Audit tail: "+truncateForContext(strings.ReplaceAll(auditTail, "\n", " "), checkpointSnippetLimit))
	}
	if retrieval := strings.TrimSpace(snapshot.RetrievalContext); retrieval != "" {
		lines = append(lines, "- Retrieval memory loaded")
	}
	if len(lines) == 0 {
		return ""
	}
	block := "### Latest Checkpoint\n" + strings.Join(lines, "\n")
	if retrieval := strings.TrimSpace(snapshot.RetrievalContext); retrieval != "" {
		block += "\n\n### Retrieved Memory\n" + retrieval
	}
	return block
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

func (c *coordinator) checkpointTurn(ctx context.Context, sessionID, prompt, result, status string, force bool) {
	if c == nil || c.checkpointService == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	agentID := c.mailboxIdentityForSession(sessionID)
	workItemID := ""
	auditTail := c.GetLongHorizonAuditTail(sessionID, maxLongHorizonChars)
	if runner := c.runnerBySessionID(sessionID); runner != nil {
		agentID = runner.id
		workItemID = runner.assignment.ID
		if strings.TrimSpace(runner.parentSession) != "" {
			auditTail = c.GetLongHorizonAuditTail(runner.parentSession, maxLongHorizonChars)
		}
	}
	mailCursor, activityCursor := c.currentCheckpointCursors(ctx, sessionID, agentID)
	_, _, _ = c.checkpointService.Record(ctx, agentmemory.CheckpointParams{
		SessionID:      sessionID,
		AgentID:        agentID,
		WorkItemID:     workItemID,
		AuditTail:      auditTail,
		Phase:          "turn",
		Prompt:         prompt,
		Result:         result,
		Status:         status,
		Force:          force,
		MailCursor:     mailCursor,
		ActivityCursor: activityCursor,
	})
	if c.shouldPersistCheckpointHandoff(sessionID, agentID, strings.TrimSpace(workItemID), prompt, status) {
		workingDir := c.mainWorkingDir()
		task := strings.TrimSpace(prompt)
		if runner := c.runnerBySessionID(sessionID); runner != nil {
			runner.mu.Lock()
			if strings.TrimSpace(runner.workDir) != "" {
				workingDir = runner.workDir
			}
			if task == "" {
				task = strings.TrimSpace(runner.assignment.Task)
				if task == "" {
					task = strings.TrimSpace(runner.assignment.Title)
				}
			}
			runner.mu.Unlock()
		}
		_ = c.memoryCompiler.PersistHandoff(ctx, agentmemory.CompileRequest{
			SessionID:  sessionID,
			AgentID:    agentID,
			WorkingDir: workingDir,
			Task:       task,
		})
	}
}

func (c *coordinator) shouldPersistCheckpointHandoff(sessionID, agentID, workItemID, prompt, status string) bool {
	if c == nil || c.memoryCompiler == nil {
		return false
	}
	if runner := c.runnerBySessionID(strings.TrimSpace(sessionID)); runner != nil {
		return false
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" || status == "running" {
		return false
	}
	if strings.TrimSpace(agentID) != "" && strings.TrimSpace(agentID) != mainAgentMailboxID(sessionID) {
		return false
	}
	if strings.TrimSpace(workItemID) != "" {
		return true
	}
	if strings.TrimSpace(c.GetLongHorizonState(sessionID)) != "" {
		return true
	}
	prompt = strings.TrimSpace(prompt)
	return len(strings.Fields(prompt)) >= 80 || shouldDelegateToSubAgents(prompt)
}

func (c *coordinator) currentCheckpointCursors(ctx context.Context, sessionID, agentID string) (int64, int64) {
	if c == nil || c.orchestrationStore == nil {
		return 0, 0
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return 0, 0
	}
	mailCursor, _ := c.orchestrationStore.LatestMailRowID(ctx, agentID)
	activityAgentIDs := []string{agentID}
	if agentID == mainAgentMailboxID(sessionID) {
		activityAgentIDs = collectAgentIDs(c.listAgentStateSnapshots(ctx, sessionID, agentID, maxOrchestrationAgents), agentID)
	}
	activityCursor, _ := c.orchestrationStore.LatestActivityRowID(ctx, activityAgentIDs)
	return mailCursor, activityCursor
}

func renderPreferencesInline(items []orchestrationdb.UserPreference, max int) string {
	parts := make([]string, 0, max)
	for _, item := range limitSlice(items, max) {
		parts = append(parts, fmt.Sprintf("%s=%s", item.Key, item.Value))
	}
	return strings.Join(parts, " | ")
}

func renderDecisionsInline(items []orchestrationdb.DecisionRecord, max int) string {
	parts := make([]string, 0, max)
	for _, item := range limitSlice(items, max) {
		parts = append(parts, fmt.Sprintf("%s.%s=%s", item.Category, item.Key, item.Value))
	}
	return strings.Join(parts, " | ")
}
