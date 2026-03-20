package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const (
	mainAgentMailboxPrefix = "main:"
	mailboxNudgePrompt     = "You have new agent mail. Call `agent_mail_inbox` immediately, incorporate any coordination requests, then continue your assigned task."
	maxOrchestrationMail   = 10
	maxOrchestrationAgents = 8
	maxOrchestrationFeed   = 12
	maxOrchestrationWork   = 8
	maxStructuredEntries   = 6
	maxLongHorizonChars    = 2800
)

func (c *coordinator) Close() error {
	if c == nil {
		return nil
	}
	c.stopOrchestrationServices()
	if c.supervisor != nil {
		c.supervisor.Stop()
	}
	if c.orchestrationStore == nil {
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
	if c == nil || runner == nil {
		return
	}
	runner.mu.Lock()
	lastHeartbeat := runner.lastHeartbeat
	if lastHeartbeat.IsZero() {
		lastHeartbeat = time.Now().UTC()
	}
	state := orchestrationdb.AgentState{
		AgentID:       runner.id,
		Role:          "subagent",
		Status:        string(runner.status),
		SessionID:     runner.sessionID,
		WorktreePath:  runner.workDir,
		Branch:        runner.assignment.Branch,
		HookBeadID:    runner.assignment.ID,
		ParentAgentID: mainAgentMailboxID(runner.parentSession),
		LastHeartbeat: lastHeartbeat,
		CreatedAt:     runner.assignment.CreatedAt,
		UpdatedAt:     time.Now().UTC(),
	}
	workItem := orchestrationdb.WorkItem{
		ID:           runner.assignment.ID,
		Type:         "task",
		Title:        firstNonEmptyString(strings.TrimSpace(runner.assignment.Title), strings.TrimSpace(runner.assignment.TaskKey), runner.id),
		Description:  buildRunnerWorkItemDescription(runner),
		Status:       workItemStatusForRunner(runner.status),
		Assignee:     runner.id,
		Dependencies: marshalPrettyJSON(runner.assignment.Domains),
		CreatedAt:    runner.assignment.CreatedAt,
	}
	if workItem.Status == "closed" {
		workItem.ClosedAt = time.Now().UTC()
	}
	runner.mu.Unlock()
	if c.stateService != nil {
		if err := c.stateService.Register(ctx, state); err != nil {
			slog.Warn("Failed to persist orchestration state", "agent_id", state.AgentID, "error", err)
		}
	} else if c.orchestrationStore != nil {
		if err := c.orchestrationStore.UpsertAgentState(ctx, state); err != nil {
			slog.Warn("Failed to persist orchestration state", "agent_id", state.AgentID, "error", err)
		}
	}
	if c.orchestrationStore != nil && strings.TrimSpace(workItem.ID) != "" {
		if existing, err := c.orchestrationStore.GetWorkItem(ctx, workItem.ID); err == nil {
			if existing.Type != "" {
				workItem.Type = existing.Type
			}
			if existing.Title != "" {
				workItem.Title = existing.Title
			}
			if existing.Description != "" {
				workItem.Description = existing.Description
			}
			if existing.ParentID != "" {
				workItem.ParentID = existing.ParentID
			}
			if existing.ConvoyID != "" {
				workItem.ConvoyID = existing.ConvoyID
			}
			if strings.TrimSpace(existing.Dependencies) != "" && existing.Dependencies != "[]" {
				workItem.Dependencies = existing.Dependencies
			}
			if !existing.CreatedAt.IsZero() {
				workItem.CreatedAt = existing.CreatedAt
			}
		}
		if err := c.orchestrationStore.UpsertWorkItem(ctx, workItem); err != nil {
			slog.Warn("Failed to persist orchestration work item", "agent_id", state.AgentID, "work_item_id", workItem.ID, "error", err)
		}
	}
}

func buildRunnerWorkItemDescription(runner *subAgentRunner) string {
	if runner == nil {
		return ""
	}
	parts := []string{strings.TrimSpace(runner.assignment.Task)}
	if dod := strings.TrimSpace(runner.assignment.DefinitionOfDone); dod != "" {
		parts = append(parts, "Definition of done:\n"+dod)
	}
	if testCommand := strings.TrimSpace(runner.assignment.TestCommand); testCommand != "" {
		parts = append(parts, "Test command:\n"+testCommand)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func workItemStatusForRunner(status subAgentStatus) string {
	switch status {
	case subAgentStatusRunning:
		return "in_progress"
	case subAgentStatusStuck, subAgentStatusError:
		return "blocked"
	case subAgentStatusCompleted, subAgentStatusClosed:
		return "closed"
	default:
		return "open"
	}
}

func (c *coordinator) syncMainAgentOrchestrationState(ctx context.Context, sessionID string) {
	if c == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	state := orchestrationdb.AgentState{
		AgentID:       mainAgentMailboxID(sessionID),
		Role:          "main-agent",
		Status:        "running",
		SessionID:     sessionID,
		WorktreePath:  c.mainWorkingDir(),
		Branch:        c.mainWorktreeBranch,
		ParentAgentID: "",
		LastHeartbeat: time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if c.stateService != nil {
		if err := c.stateService.Register(ctx, state); err != nil {
			slog.Warn("Failed to persist main agent orchestration state", "session_id", sessionID, "error", err)
		}
	} else if c.orchestrationStore != nil {
		if err := c.orchestrationStore.UpsertAgentState(ctx, state); err != nil {
			slog.Warn("Failed to persist main agent orchestration state", "session_id", sessionID, "error", err)
		}
	}
}

func (c *coordinator) recordOrchestrationActivity(ctx context.Context, agentID, eventType string, details any) {
	if c == nil || strings.TrimSpace(agentID) == "" || strings.TrimSpace(eventType) == "" {
		return
	}
	detailsJSON := "{}"
	if c.orchestrationStore != nil {
		detailsJSON = c.orchestrationStore.MarshalDetails(details)
	}
	if c.activityService != nil {
		if err := c.activityService.Log(ctx, agentID, agentactivity.EventType(eventType), detailsJSON); err != nil {
			slog.Warn("Failed to persist orchestration activity", "agent_id", agentID, "event_type", eventType, "error", err)
		}
		return
	}
	if c.orchestrationStore == nil {
		return
	}
	if err := c.orchestrationStore.RecordActivity(ctx, orchestrationdb.AgentActivity{
		AgentID:     agentID,
		EventType:   eventType,
		DetailsJSON: detailsJSON,
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

func (c *coordinator) buildMainOrchestrationMemoryContext(ctx context.Context, sessionID string) string {
	if c == nil || c.orchestrationStore == nil || strings.TrimSpace(sessionID) == "" {
		return ""
	}
	parentID := mainAgentMailboxID(sessionID)
	var sections []string

	if mailbox := c.buildMailboxContext(ctx, parentID, true, maxOrchestrationMail); mailbox != "" {
		sections = append(sections, mailbox)
	}

	agentStates := c.listAgentStateSnapshots(ctx, sessionID, parentID, maxOrchestrationAgents)
	if agentSection := renderAgentStatesContext(agentStates); agentSection != "" {
		sections = append(sections, agentSection)
	}
	if workSection := c.renderWorkItemContext(ctx, uniqueWorkItemIDs(agentStates)); workSection != "" {
		sections = append(sections, workSection)
	}
	if activitySection := c.renderActivityFeedContext(ctx, collectAgentIDs(agentStates, parentID)); activitySection != "" {
		sections = append(sections, activitySection)
	}
	if staleSection := c.renderStaleAgentsContext(ctx); staleSection != "" {
		sections = append(sections, staleSection)
	}
	if checkpointSection := c.renderCheckpointContext(ctx, sessionID, parentID); checkpointSection != "" {
		sections = append(sections, checkpointSection)
	}
	if len(sections) == 0 {
		return ""
	}
	return "## ORCHESTRATION MEMORY\n" + strings.Join(sections, "\n\n")
}

func (c *coordinator) buildSubAgentPersistentMemoryContext(ctx context.Context, runner *subAgentRunner) string {
	if c == nil || runner == nil {
		return ""
	}
	var sections []string

	parentSessionID := strings.TrimSpace(runner.parentSession)
	if parentSessionID != "" {
		if longHorizon := compactLongHorizonContext(c.GetLongHorizonState(parentSessionID)); longHorizon != "" {
			sections = append(sections, "### Long-Horizon State\n"+longHorizon)
		}
		if continuity := c.buildStructuredSummaryContext(ctx, parentSessionID); continuity != "" {
			sections = append(sections, continuity)
		}
	}

	if c.stateService != nil {
		if snapshot, err := c.stateService.Status(ctx, runner.id); err == nil {
			if stateSection := renderRunnerStateContext(snapshot); stateSection != "" {
				sections = append(sections, stateSection)
			}
		}
	} else if c.orchestrationStore != nil {
		if snapshot, err := c.orchestrationStore.GetAgentState(ctx, runner.id); err == nil {
			if stateSection := renderRunnerStateContext(snapshot); stateSection != "" {
				sections = append(sections, stateSection)
			}
		}
	}
	if c.orchestrationStore != nil && strings.TrimSpace(runner.assignment.ID) != "" {
		if workItem, err := c.orchestrationStore.GetWorkItem(ctx, runner.assignment.ID); err == nil {
			if workSection := renderWorkItemsContext([]orchestrationdb.WorkItem{workItem}); workSection != "" {
				sections = append(sections, workSection)
			}
		}
	}
	if activitySection := c.renderRecentAgentActivityContext(ctx, runner.id); activitySection != "" {
		sections = append(sections, activitySection)
	}
	if checkpointSection := c.renderCheckpointContext(ctx, runner.sessionID, runner.id); checkpointSection != "" {
		sections = append(sections, checkpointSection)
	}

	if len(sections) == 0 {
		return ""
	}
	return "## PERSISTENT MEMORY\n" + strings.Join(sections, "\n\n")
}

func (c *coordinator) buildStructuredSummaryContext(ctx context.Context, sessionID string) string {
	if c == nil || c.memory == nil || strings.TrimSpace(sessionID) == "" {
		return ""
	}
	memCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	data, err := c.memory.GetStructuredSummary(memCtx, sessionID)
	if err != nil || data == nil {
		return ""
	}
	trimmed := trimStructuredSummary(data)
	if continuity := buildSessionContinuityInjection(trimmed, nil); continuity != "" {
		return continuity
	}
	return ""
}

func trimStructuredSummary(data *agentmemory.StructuredSummaryData) *agentmemory.StructuredSummaryData {
	if data == nil {
		return nil
	}
	trimmed := *data
	trimmed.Decisions = slices.Clone(limitSlice(data.Decisions, maxStructuredEntries))
	trimmed.FileChanges = slices.Clone(limitSlice(data.FileChanges, maxStructuredEntries))
	trimmed.FailureModes = slices.Clone(limitSlice(data.FailureModes, maxStructuredEntries))
	trimmed.DependencyGraph = slices.Clone(limitSlice(data.DependencyGraph, maxStructuredEntries))
	trimmed.TodoStates = slices.Clone(limitSlice(data.TodoStates, maxStructuredEntries))
	return &trimmed
}

func compactLongHorizonContext(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) <= maxLongHorizonChars {
		return raw
	}
	return strings.TrimSpace(raw[:maxLongHorizonChars]) + "\n..."
}

func (c *coordinator) buildMailboxContext(ctx context.Context, agentID string, unreadOnly bool, limit int) string {
	if c == nil || c.mailbox == nil || strings.TrimSpace(agentID) == "" {
		return ""
	}
	items, err := c.mailbox.Inbox(ctx, agentID, unreadOnly, limit)
	if err != nil || len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		line := fmt.Sprintf("- %s | %s", strings.TrimSpace(item.FromAgent), truncateForContext(item.Subject, 72))
		if body := truncateForContext(strings.ReplaceAll(strings.TrimSpace(item.Body), "\n", " "), 120); body != "" {
			line += " | " + body
		}
		lines = append(lines, line)
	}
	return "### Unread Mail\n" + strings.Join(lines, "\n")
}

func (c *coordinator) listAgentStateSnapshots(ctx context.Context, sessionID, parentID string, limit int) []orchestrationdb.AgentState {
	if c == nil {
		return nil
	}
	if c.stateService != nil {
		if items, err := c.stateService.ListByParent(ctx, parentID, limit); err == nil && len(items) > 0 {
			return items
		}
		if items, err := c.stateService.ListBySession(ctx, sessionID, limit); err == nil {
			return items
		}
		return nil
	}
	if c.orchestrationStore == nil {
		return nil
	}
	if items, err := c.orchestrationStore.ListAgentStatesByParent(ctx, parentID, limit); err == nil && len(items) > 0 {
		return items
	}
	items, _ := c.orchestrationStore.ListAgentStatesBySession(ctx, sessionID, limit)
	return items
}

func renderAgentStatesContext(states []orchestrationdb.AgentState) string {
	if len(states) == 0 {
		return ""
	}
	lines := make([]string, 0, len(states))
	for _, state := range states {
		line := fmt.Sprintf("- %s: %s", state.AgentID, state.Status)
		if taskID := strings.TrimSpace(state.HookBeadID); taskID != "" {
			line += fmt.Sprintf(" | work: %s", taskID)
		}
		if branch := strings.TrimSpace(state.Branch); branch != "" {
			line += fmt.Sprintf(" | branch: %s", branch)
		}
		if worktree := strings.TrimSpace(state.WorktreePath); worktree != "" {
			line += fmt.Sprintf(" | worktree: %s", filepath.Base(worktree))
		}
		if !state.LastHeartbeat.IsZero() {
			line += fmt.Sprintf(" | heartbeat %s ago", time.Since(state.LastHeartbeat).Truncate(time.Second))
		}
		lines = append(lines, line)
	}
	return "### Agent State\n" + strings.Join(lines, "\n")
}

func renderRunnerStateContext(state orchestrationdb.AgentState) string {
	if strings.TrimSpace(state.AgentID) == "" {
		return ""
	}
	lines := []string{
		fmt.Sprintf("- Agent: %s", state.AgentID),
		fmt.Sprintf("- Status: %s", state.Status),
	}
	if workID := strings.TrimSpace(state.HookBeadID); workID != "" {
		lines = append(lines, fmt.Sprintf("- Work item: %s", workID))
	}
	if branch := strings.TrimSpace(state.Branch); branch != "" {
		lines = append(lines, fmt.Sprintf("- Branch: %s", branch))
	}
	if worktree := strings.TrimSpace(state.WorktreePath); worktree != "" {
		lines = append(lines, fmt.Sprintf("- Worktree: %s", filepath.Base(worktree)))
	}
	if !state.LastHeartbeat.IsZero() {
		lines = append(lines, fmt.Sprintf("- Heartbeat age: %s", time.Since(state.LastHeartbeat).Truncate(time.Second)))
	}
	return "### Agent State\n" + strings.Join(lines, "\n")
}

func uniqueWorkItemIDs(states []orchestrationdb.AgentState) []string {
	seen := make(map[string]struct{}, len(states))
	ids := make([]string, 0, len(states))
	for _, state := range states {
		id := strings.TrimSpace(state.HookBeadID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func (c *coordinator) renderWorkItemContext(ctx context.Context, workItemIDs []string) string {
	if c == nil || c.orchestrationStore == nil || len(workItemIDs) == 0 {
		return ""
	}
	items := make([]orchestrationdb.WorkItem, 0, len(workItemIDs))
	for _, workItemID := range limitSlice(workItemIDs, maxOrchestrationWork) {
		item, err := c.orchestrationStore.GetWorkItem(ctx, workItemID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			break
		}
		items = append(items, item)
	}
	return renderWorkItemsContext(items)
}

func renderWorkItemsContext(items []orchestrationdb.WorkItem) string {
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		line := fmt.Sprintf("- %s [%s]", item.Title, item.Status)
		if item.Assignee != "" {
			line += fmt.Sprintf(" | assignee: %s", item.Assignee)
		}
		if item.Description != "" {
			line += fmt.Sprintf(" | %s", truncateForContext(strings.ReplaceAll(item.Description, "\n", " "), 120))
		}
		lines = append(lines, line)
	}
	return "### Work Items\n" + strings.Join(lines, "\n")
}

func collectAgentIDs(states []orchestrationdb.AgentState, extras ...string) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, extra := range extras {
		extra = strings.TrimSpace(extra)
		if extra == "" {
			continue
		}
		if _, ok := seen[extra]; ok {
			continue
		}
		seen[extra] = struct{}{}
		ids = append(ids, extra)
	}
	for _, state := range states {
		if _, ok := seen[state.AgentID]; ok || strings.TrimSpace(state.AgentID) == "" {
			continue
		}
		seen[state.AgentID] = struct{}{}
		ids = append(ids, state.AgentID)
	}
	return ids
}

func (c *coordinator) renderActivityFeedContext(ctx context.Context, agentIDs []string) string {
	if c == nil || len(agentIDs) == 0 {
		return ""
	}
	var (
		items []orchestrationdb.AgentActivity
		err   error
	)
	if c.activityService != nil {
		items, err = c.activityService.Feed(ctx, agentIDs, maxOrchestrationFeed)
	} else if c.orchestrationStore != nil {
		items, err = c.orchestrationStore.ListActivityFeed(ctx, agentIDs, maxOrchestrationFeed)
	}
	if err != nil || len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s | %s | %s ago", item.AgentID, item.EventType, time.Since(item.CreatedAt).Truncate(time.Second)))
	}
	return "### Recent Activity\n" + strings.Join(lines, "\n")
}

func (c *coordinator) renderRecentAgentActivityContext(ctx context.Context, agentID string) string {
	if c == nil || strings.TrimSpace(agentID) == "" {
		return ""
	}
	var (
		items []orchestrationdb.AgentActivity
		err   error
	)
	if c.activityService != nil {
		items, err = c.activityService.Recent(ctx, agentID, maxStructuredEntries)
	} else if c.orchestrationStore != nil {
		items, err = c.orchestrationStore.ListRecentActivity(ctx, agentID, maxStructuredEntries)
	}
	if err != nil || len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s | %s ago", item.EventType, time.Since(item.CreatedAt).Truncate(time.Second)))
	}
	return "### Recent Activity\n" + strings.Join(lines, "\n")
}

func (c *coordinator) renderStaleAgentsContext(ctx context.Context) string {
	if c == nil {
		return ""
	}
	staleBefore := time.Now().UTC().Add(-2 * time.Minute)
	var (
		items []orchestrationdb.AgentState
		err   error
	)
	if c.stateService != nil {
		items, err = c.stateService.ListStale(ctx, staleBefore, maxStructuredEntries)
	} else if c.orchestrationStore != nil {
		items, err = c.orchestrationStore.ListStaleAgentStates(ctx, staleBefore, maxStructuredEntries)
	}
	if err != nil || len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s | %s | last heartbeat %s ago", item.AgentID, item.Status, time.Since(item.LastHeartbeat).Truncate(time.Second)))
	}
	return "### Stale Agents\n" + strings.Join(lines, "\n")
}

func limitSlice[T any](items []T, max int) []T {
	if max <= 0 || len(items) <= max {
		return items
	}
	return items[:max]
}
