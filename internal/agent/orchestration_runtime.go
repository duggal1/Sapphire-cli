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
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const (
	mainAgentMailboxPrefix  = "main:"
	mailboxNudgePrompt      = "You have new agent mail. Call `agent_mail_inbox` immediately, handle the coordination request, then call `agent_mail_ack` for completed items before continuing your assigned task."
	maxOrchestrationMail    = 10
	maxOrchestrationAgents  = 8
	maxOrchestrationFeed    = 12
	maxOrchestrationWork    = 8
	maxStructuredEntries    = 6
	maxLongHorizonChars     = 2800
	subAgentLaunchMemoryTTL = 5 * time.Second
)

type subAgentLaunchMemoryCacheEntry struct {
	value     string
	expiresAt time.Time
}

type subAgentLaunchMemoryFlight struct {
	done  chan struct{}
	value string
}

func (c *coordinator) Close() error {
	if c == nil {
		return nil
	}
	c.stopOrchestrationServices()
	if c.supervisor != nil {
		c.supervisor.Stop()
	}
	if c.codeIndex != nil {
		_ = c.codeIndex.Close()
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
		Dependencies: "[]",
		CreatedAt:    runner.assignment.CreatedAt,
	}
	if workItem.Status == "closed" {
		workItem.ClosedAt = time.Now().UTC()
	}
	runner.mu.Unlock()
	if c.stateService != nil {
		c.countSubAgentLaunchMetric("db.agent_state_upsert", 1)
		if err := c.stateService.Register(ctx, state); err != nil {
			slog.Warn("Failed to persist orchestration state", "agent_id", state.AgentID, "error", err)
		}
	} else if c.orchestrationStore != nil {
		c.countSubAgentLaunchMetric("db.agent_state_upsert", 1)
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
		c.countSubAgentLaunchMetric("db.work_item_upsert", 1)
		if err := c.orchestrationStore.UpsertWorkItem(ctx, workItem); err != nil {
			slog.Warn("Failed to persist orchestration work item", "agent_id", state.AgentID, "work_item_id", workItem.ID, "error", err)
		}
	}
	if c.worktreeManager != nil && strings.TrimSpace(state.WorktreePath) != "" {
		c.worktreeManager.MarkStatusByPath(ctx, state.WorktreePath, worktreeLifecycleStatusForRunner(state.Status))
	}
}

func worktreeLifecycleStatusForRunner(status string) string {
	switch strings.TrimSpace(status) {
	case string(subAgentStatusQueued):
		return "queued"
	case string(subAgentStatusStarting), string(subAgentStatusReady), string(subAgentStatusWaitingOnMail), string(subAgentStatusRetrying), string(subAgentStatusRunning), string(subAgentStatusDegraded):
		return "running"
	case string(subAgentStatusCompleted), string(subAgentStatusClosed):
		return "completed"
	case string(subAgentStatusBlocked):
		return "blocked"
	case string(subAgentStatusTimedOut), string(subAgentStatusStuck):
		return "stuck"
	case string(subAgentStatusError):
		return "failed"
	default:
		return "ready"
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
	case subAgentStatusStarting, subAgentStatusReady, subAgentStatusWaitingOnMail, subAgentStatusRetrying, subAgentStatusRunning, subAgentStatusDegraded:
		return "in_progress"
	case subAgentStatusBlocked, subAgentStatusTimedOut, subAgentStatusStuck, subAgentStatusError:
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
	worktreePath, branch, err := c.resolveMainExecutionRoot(ctx, sessionID)
	if err != nil {
		worktreePath = c.mainWorkingDir()
		branch = c.mainWorktreeBranch
	}
	state := orchestrationdb.AgentState{
		AgentID:       mainAgentMailboxID(sessionID),
		Role:          "main-agent",
		Status:        "running",
		SessionID:     sessionID,
		WorktreePath:  worktreePath,
		Branch:        branch,
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
		c.countSubAgentLaunchMetric("db.activity_write", 1)
		if err := c.activityService.Log(ctx, agentID, agentactivity.EventType(eventType), detailsJSON); err != nil {
			slog.Warn("Failed to persist orchestration activity", "agent_id", agentID, "event_type", eventType, "error", err)
		}
		return
	}
	if c.orchestrationStore == nil {
		return
	}
	c.countSubAgentLaunchMetric("db.activity_write", 1)
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
	c.requeueExpiredMailLeases(ctx)
	items, err := c.mailbox.LeaseInbox(ctx, runner.id, runner.id, 20, agentmailbox.DefaultLeaseTTL)
	c.countSubAgentLaunchMetric("mail.inbox_read", 1)
	if err != nil || len(items) == 0 {
		return ""
	}
	var lines []string
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- [%s] From: %s | Subject: %s | Body: %s", item.ID, item.FromAgent, item.Subject, item.Body))
	}
	c.recordOrchestrationActivity(ctx, runner.id, "mail_received", map[string]any{
		"count": len(items),
	})
	return "Mailbox update:\n" + strings.Join(lines, "\n") + "\n\nIf you fully handle any of these items, acknowledge them with `agent_mail_ack`. Use `agent_mail_inbox` if you need the full thread history. Continue your assigned task after incorporating the messages."
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
	status := ""
	closed := false
	if runner != nil {
		runner.mu.Lock()
		closed = runner.closed
		status = string(runner.effectiveStatusLocked())
		runner.mu.Unlock()
		if closed {
			return nil
		}
	} else if c.orchestrationStore != nil {
		if state, err := c.orchestrationStore.GetAgentState(ctx, recipient); err == nil {
			status = strings.TrimSpace(state.Status)
		}
	}
	dispatchID, err := c.enqueueAgentNudgeDispatch(ctx, recipient, mailboxNudgePrompt)
	if err != nil {
		return err
	}
	c.countSubAgentLaunchMetric("mail.nudge_dispatch_enqueue", 1)
	c.recordOrchestrationActivity(ctx, recipient, "mail_pending", map[string]any{
		"recipient":   recipient,
		"status":      status,
		"dispatch_id": dispatchID,
	})
	return nil
}

func (c *coordinator) requeueExpiredMailLeases(ctx context.Context) {
	if c == nil || c.mailbox == nil {
		return
	}
	requeued, deadLetters, err := c.mailbox.RequeueExpiredLeases(ctx, agentmailbox.DefaultMaxLeaseAttempts)
	if err != nil {
		slog.Warn("Failed to requeue expired mail leases", "error", err)
		return
	}
	for _, item := range requeued {
		c.recordOrchestrationActivity(ctx, item.ResolvedToAgent, "mail_requeued", map[string]any{
			"id":                item.ID,
			"thread_id":         item.ThreadID,
			"delivery_attempts": item.DeliveryAttempts,
		})
		if err := c.nudgeMailboxRecipient(ctx, item.ResolvedToAgent); err != nil {
			slog.Warn("Failed to nudge requeued mail recipient", "recipient", item.ResolvedToAgent, "error", err)
		}
	}
	for _, item := range deadLetters {
		c.recordOrchestrationActivity(ctx, item.ResolvedToAgent, "mail_dead_letter", map[string]any{
			"id":                item.ID,
			"thread_id":         item.ThreadID,
			"delivery_attempts": item.DeliveryAttempts,
		})
	}
}

func (c *coordinator) resolveMailTarget(ctx context.Context, sessionID, rawTo string) (string, error) {
	to := strings.TrimSpace(rawTo)
	if to == "" {
		return "", fmt.Errorf("recipient is required")
	}
	senderRunner := c.runnerBySessionID(sessionID)
	lower := strings.ToLower(to)
	if strings.HasPrefix(lower, "agent:") {
		agentID := strings.TrimSpace(strings.TrimPrefix(to, "agent:"))
		if agentID == "" {
			return "", fmt.Errorf("agent recipient is required")
		}
		return agentID, nil
	}
	if strings.HasPrefix(lower, "work:") {
		workItemID := strings.TrimSpace(strings.TrimPrefix(to, "work:"))
		if workItemID == "" {
			return "", fmt.Errorf("work item recipient is required")
		}
		if c == nil || c.orchestrationStore == nil {
			return "", fmt.Errorf("orchestration store not initialized")
		}
		item, err := c.orchestrationStore.GetWorkItem(ctx, workItemID)
		if err != nil {
			return "", fmt.Errorf("resolve work recipient %s: %w", workItemID, err)
		}
		if strings.TrimSpace(item.Assignee) == "" {
			return "", fmt.Errorf("work item %s has no assignee", workItemID)
		}
		return strings.TrimSpace(item.Assignee), nil
	}
	switch lower {
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
	if c.memoryCompiler != nil {
		if compiled := c.memoryCompiler.RenderCachedPromptInjection(ctx, agentmemory.CompileRequest{
			SessionID:  sessionID,
			AgentID:    parentID,
			WorkingDir: c.mainWorkingDir(),
		}); compiled != "" {
			sections = append(sections, compiled)
		}
	}

	if mailbox := c.buildMailboxContext(ctx, parentID, true, maxOrchestrationMail); mailbox != "" {
		sections = append(sections, mailbox)
	}
	if directorySection := c.buildAgentDirectoryContext(ctx, sessionID, maxOrchestrationAgents); directorySection != "" {
		sections = append(sections, directorySection)
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

	runner.mu.Lock()
	sessionID := runner.sessionID
	parentSessionID := strings.TrimSpace(runner.parentSession)
	task := strings.TrimSpace(runner.assignment.Task)
	if task == "" {
		task = strings.TrimSpace(runner.assignment.Title)
	}
	workDir := runner.workDir
	agentID := runner.id
	workItemID := strings.TrimSpace(runner.assignment.ID)
	freshLaunch := runner.freshLaunch && !runner.hasPriorTurnHistoryLocked()
	if freshLaunch {
		runner.freshLaunch = false
	}
	runner.mu.Unlock()

	rootSessionID := firstNonEmptyString(parentSessionID, sessionID)
	if freshLaunch {
		if shared := c.buildSharedSubAgentLaunchMemoryContext(ctx, rootSessionID, workDir); shared != "" {
			sections = append(sections, shared)
		}
		if c.orchestrationStore != nil && workItemID != "" {
			if workItem, err := c.orchestrationStore.GetWorkItem(ctx, workItemID); err == nil {
				if workSection := renderWorkItemsContext([]orchestrationdb.WorkItem{workItem}); workSection != "" {
					sections = append(sections, workSection)
				}
			}
		}
		if mailboxSection := c.buildMailboxContext(ctx, runner.id, true, maxOrchestrationMail); mailboxSection != "" {
			sections = append(sections, mailboxSection)
		}
		if len(sections) == 0 {
			return ""
		}
		c.countSubAgentLaunchMetric("subagent_memory.launch_lightweight", 1)
		return "## PERSISTENT MEMORY\n" + strings.Join(sections, "\n\n")
	}

	if c.memoryCompiler != nil {
		if compiled := c.memoryCompiler.RenderPromptInjection(ctx, agentmemory.CompileRequest{
			SessionID:  sessionID,
			AgentID:    agentID,
			WorkingDir: workDir,
			Task:       task,
		}); compiled != "" {
			sections = append(sections, compiled)
		}
	}

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
	if mailboxSection := c.buildMailboxContext(ctx, runner.id, true, maxOrchestrationMail); mailboxSection != "" {
		sections = append(sections, mailboxSection)
	}
	if directorySection := c.buildAgentDirectoryContext(ctx, sessionID, maxOrchestrationAgents); directorySection != "" {
		sections = append(sections, directorySection)
	}
	if activitySection := c.renderRecentAgentActivityContext(ctx, runner.id); activitySection != "" {
		sections = append(sections, activitySection)
	}
	if checkpointSection := c.renderCheckpointContext(ctx, sessionID, agentID); checkpointSection != "" {
		sections = append(sections, checkpointSection)
	}

	if len(sections) == 0 {
		return ""
	}
	c.countSubAgentLaunchMetric("subagent_memory.launch_full", 1)
	return "## PERSISTENT MEMORY\n" + strings.Join(sections, "\n\n")
}

func (r *subAgentRunner) hasPriorTurnHistoryLocked() bool {
	if r == nil {
		return false
	}
	if strings.TrimSpace(r.lastResult) != "" || strings.TrimSpace(r.lastError) != "" || strings.TrimSpace(r.lastProgress) != "" {
		return true
	}
	for _, submission := range r.submissions {
		if submission == nil {
			continue
		}
		if !submission.EndedAt.IsZero() {
			return true
		}
	}
	return false
}

func (c *coordinator) buildSharedSubAgentLaunchMemoryContext(ctx context.Context, sessionID, workDir string) string {
	if c == nil {
		return ""
	}
	sessionID = strings.TrimSpace(sessionID)
	workDir = strings.TrimSpace(workDir)
	if sessionID == "" && workDir == "" {
		return ""
	}
	key := sessionID + "|" + filepath.Clean(workDir)
	if value, ok := c.cachedSubAgentLaunchMemory(key); ok {
		c.countSubAgentLaunchMetric("subagent_memory.launch_context_cache_hit", 1)
		return value
	}
	flight, leader := c.startSubAgentLaunchMemoryFlight(key)
	if !leader {
		c.countSubAgentLaunchMetric("subagent_memory.launch_context_flight_wait", 1)
		select {
		case <-ctx.Done():
			return ""
		case <-flight.done:
			return flight.value
		}
	}

	var value string
	defer func() {
		c.finishSubAgentLaunchMemoryFlight(key, value)
	}()

	var sections []string
	if c.memoryCompiler != nil {
		req := agentmemory.CompileRequest{
			SessionID:           sessionID,
			AgentID:             mainAgentMailboxID(sessionID),
			WorkingDir:          workDir,
			IncludeMistakesRead: true,
		}
		compiled := c.memoryCompiler.RenderCachedPromptInjection(ctx, req)
		if compiled == "" {
			compiled = c.memoryCompiler.RenderPromptInjection(ctx, req)
		}
		if compiled != "" {
			sections = append(sections, compiled)
		}
	}
	if sessionID != "" {
		if longHorizon := compactLongHorizonContext(c.GetLongHorizonState(sessionID)); longHorizon != "" {
			sections = append(sections, "### Long-Horizon State\n"+longHorizon)
		}
		if continuity := c.buildStructuredSummaryContext(ctx, sessionID); continuity != "" {
			sections = append(sections, continuity)
		}
		if directorySection := c.buildAgentDirectoryContext(ctx, sessionID, maxOrchestrationAgents); directorySection != "" {
			sections = append(sections, directorySection)
		}
	}

	value = strings.Join(sections, "\n\n")
	c.storeSubAgentLaunchMemory(key, value)
	c.countSubAgentLaunchMetric("subagent_memory.launch_context_cache_miss", 1)
	return value
}

func (c *coordinator) cachedSubAgentLaunchMemory(key string) (string, bool) {
	if c == nil || strings.TrimSpace(key) == "" {
		return "", false
	}
	now := time.Now().UTC()
	c.subAgentLaunchMemoryMu.Lock()
	defer c.subAgentLaunchMemoryMu.Unlock()
	entry, ok := c.subAgentLaunchMemoryCache[key]
	if !ok {
		return "", false
	}
	if !entry.expiresAt.After(now) {
		delete(c.subAgentLaunchMemoryCache, key)
		return "", false
	}
	return entry.value, true
}

func (c *coordinator) storeSubAgentLaunchMemory(key, value string) {
	if c == nil || strings.TrimSpace(key) == "" {
		return
	}
	c.subAgentLaunchMemoryMu.Lock()
	if c.subAgentLaunchMemoryCache == nil {
		c.subAgentLaunchMemoryCache = make(map[string]subAgentLaunchMemoryCacheEntry)
	}
	c.subAgentLaunchMemoryCache[key] = subAgentLaunchMemoryCacheEntry{
		value:     value,
		expiresAt: time.Now().UTC().Add(subAgentLaunchMemoryTTL),
	}
	c.subAgentLaunchMemoryMu.Unlock()
}

func (c *coordinator) startSubAgentLaunchMemoryFlight(key string) (*subAgentLaunchMemoryFlight, bool) {
	c.subAgentLaunchMemoryMu.Lock()
	defer c.subAgentLaunchMemoryMu.Unlock()
	if c.subAgentLaunchMemoryWork == nil {
		c.subAgentLaunchMemoryWork = make(map[string]*subAgentLaunchMemoryFlight)
	}
	if flight := c.subAgentLaunchMemoryWork[key]; flight != nil {
		return flight, false
	}
	flight := &subAgentLaunchMemoryFlight{done: make(chan struct{})}
	c.subAgentLaunchMemoryWork[key] = flight
	return flight, true
}

func (c *coordinator) finishSubAgentLaunchMemoryFlight(key, value string) {
	c.subAgentLaunchMemoryMu.Lock()
	flight := c.subAgentLaunchMemoryWork[key]
	delete(c.subAgentLaunchMemoryWork, key)
	c.subAgentLaunchMemoryMu.Unlock()
	if flight == nil {
		return
	}
	flight.value = value
	close(flight.done)
}

func (c *coordinator) reportSubAgentOutcomeToParent(ctx context.Context, runner *subAgentRunner, submissionID string, report subAgentReport, rawResult string) {
	if c == nil || c.mailbox == nil || runner == nil {
		return
	}

	runner.mu.Lock()
	parentSessionID := strings.TrimSpace(runner.parentSession)
	assignmentID := strings.TrimSpace(runner.assignment.ID)
	agentID := strings.TrimSpace(runner.id)
	title := strings.TrimSpace(runner.assignment.Title)
	task := strings.TrimSpace(runner.assignment.Task)
	runner.mu.Unlock()

	if parentSessionID == "" || agentID == "" {
		return
	}

	mainMailboxID := mainAgentMailboxID(parentSessionID)
	if mainMailboxID == "" {
		return
	}

	reportStatus := strings.ToUpper(firstNonEmptyString(strings.TrimSpace(report.Status), "done"))
	subject := "SUBAGENT_DONE"
	switch strings.ToLower(strings.TrimSpace(report.Status)) {
	case "blocked":
		subject = "SUBAGENT_BLOCKED"
	case "needs_followup":
		subject = "SUBAGENT_NEEDS_FOLLOWUP"
	}

	bodyLines := []string{
		fmt.Sprintf("Agent: %s", agentID),
		fmt.Sprintf("Assignment: %s", firstNonEmptyString(title, assignmentID, "sub-agent task")),
		fmt.Sprintf("Submission: %s", strings.TrimSpace(submissionID)),
		fmt.Sprintf("Status: %s", reportStatus),
		fmt.Sprintf("Task: %s", firstNonEmptyString(title, task)),
		fmt.Sprintf("Summary: %s", firstNonEmptyString(strings.TrimSpace(report.Summary), strings.TrimSpace(report.Progress), truncateForContext(strings.TrimSpace(rawResult), 280), "none")),
		fmt.Sprintf("Progress: %s", firstNonEmptyString(strings.TrimSpace(report.Progress), "none")),
		fmt.Sprintf("Files: %s", joinOrNone(report.Files)),
		fmt.Sprintf("Commands: %s", joinOrNone(report.Commands)),
		fmt.Sprintf("Risks: %s", firstNonEmptyString(strings.TrimSpace(report.Risks), "none")),
		fmt.Sprintf("Next: %s", firstNonEmptyString(strings.TrimSpace(report.Next), "none")),
		fmt.Sprintf("Blockers: %s", firstNonEmptyString(strings.TrimSpace(report.Blockers), "none")),
	}

	if _, err := c.mailbox.Send(ctx, mainMailboxID, agentID, subject, strings.Join(bodyLines, "\n"), agentmailbox.SendOptions{
		Priority:  1,
		ThreadID:  assignmentID,
		SkipNudge: true,
	}); err != nil {
		slog.Warn("Failed to report sub-agent outcome to parent", "agent_id", agentID, "parent_session_id", parentSessionID, "error", err)
		return
	}
	c.countSubAgentLaunchMetric("mail.outcome_report", 1)

	c.recordOrchestrationActivity(ctx, agentID, "reported_to_parent", map[string]any{
		"subject":       subject,
		"submission_id": submissionID,
		"thread_id":     assignmentID,
	})
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	trimmed := make([]string, 0, len(items))
	for _, item := range items {
		if v := strings.TrimSpace(item); v != "" {
			trimmed = append(trimmed, v)
		}
	}
	if len(trimmed) == 0 {
		return "none"
	}
	return strings.Join(trimmed, ", ")
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
	items, err := c.mailbox.Actionable(ctx, agentID, limit)
	if err != nil || len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		line := fmt.Sprintf("- [%s] %s | %s | %s", item.ID, item.DeliveryState, strings.TrimSpace(item.FromAgent), truncateForContext(item.Subject, 72))
		if body := truncateForContext(strings.ReplaceAll(strings.TrimSpace(item.Body), "\n", " "), 120); body != "" {
			line += " | " + body
		}
		if !item.LeaseExpiresAt.IsZero() {
			line += fmt.Sprintf(" | lease %s", time.Until(item.LeaseExpiresAt).Truncate(time.Second))
		}
		lines = append(lines, line)
	}
	return "### Actionable Mail\n" + strings.Join(lines, "\n")
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
