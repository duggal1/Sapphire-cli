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
	dispatchLeaseTimeout         = 2 * time.Minute
	dispatchRetryLimit           = 3
	defaultDispatchQueueCapacity = 50
	defaultDispatchActiveLimit   = 8
	dispatchLeaseOwner           = "sapphire-dispatcher"
	dispatchTargetSubAgent       = "subagent"
	dispatchTargetMailNudge      = "mail_nudge"
	dispatchMailNudgeBackoff     = 5 * time.Second
	dispatchLeaseBatchSlack      = 8
)

type queuedSubAgentDispatchPayload struct {
	Prompt           string   `json:"prompt"`
	Title            string   `json:"title,omitempty"`
	Worktree         bool     `json:"worktree"`
	WorktreePath     string   `json:"worktree_path,omitempty"`
	Branch           string   `json:"branch,omitempty"`
	WriteManifest    []string `json:"write_manifest,omitempty"`
	DefinitionOfDone string   `json:"definition_of_done,omitempty"`
	TestCommand      string   `json:"test_command,omitempty"`
	AgentID          string   `json:"agent,omitempty"`
	Model            string   `json:"model,omitempty"`
	ReasoningEffort  string   `json:"reasoning_effort,omitempty"`
	ForkContext      bool     `json:"fork_context,omitempty"`
}

type queuedAgentNudgePayload struct {
	Recipient string `json:"recipient"`
	Prompt    string `json:"prompt,omitempty"`
}

func mailNudgeDispatchWorkItem(recipient string) string {
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return ""
	}
	return "mail-nudge:" + recipient
}

func (c *coordinator) startOrchestrationServices() {
	if c == nil || c.orchestrationStore == nil || c.orchestrationSvcCancel != nil || c.daemon == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.orchestrationSvcCancel = cancel
	c.orchestrationSvcWG.Add(1)
	go func() {
		defer c.orchestrationSvcWG.Done()
		c.daemon.Start(ctx)
		<-ctx.Done()
		c.daemon.Stop()
	}()
}

func (c *coordinator) stopOrchestrationServices() {
	if c == nil || c.orchestrationSvcCancel == nil {
		return
	}
	c.orchestrationSvcCancel()
	c.orchestrationSvcCancel = nil
	c.orchestrationSvcWG.Wait()
}

func (c *coordinator) enqueueSubAgentDispatch(ctx context.Context, sessionID, workItemID, targetScope string, opts spawnAgentOptions) (string, error) {
	if c == nil || c.orchestrationStore == nil {
		return "", fmt.Errorf("orchestration store is not initialized")
	}
	if strings.TrimSpace(sessionID) == "" {
		return "", fmt.Errorf("session id is required")
	}
	payload, err := json.Marshal(queuedSubAgentDispatchPayload{
		Prompt:           opts.Prompt,
		Title:            opts.Title,
		Worktree:         opts.Worktree,
		WorktreePath:     opts.WorktreePath,
		Branch:           opts.Branch,
		WriteManifest:    append([]string{}, opts.WriteManifest...),
		DefinitionOfDone: opts.DefinitionOfDone,
		TestCommand:      opts.TestCommand,
		AgentID:          opts.AgentID,
		Model:            opts.Model,
		ReasoningEffort:  opts.ReasoningEffort,
		ForkContext:      opts.ForkContext,
	})
	if err != nil {
		return "", fmt.Errorf("marshal dispatch payload: %w", err)
	}
	item, err := c.orchestrationStore.EnqueueDispatch(ctx, orchestrationdb.DispatchQueueItem{
		SessionID:   sessionID,
		WorkItemID:  strings.TrimSpace(workItemID),
		TargetScope: strings.TrimSpace(targetScope),
		Status:      "queued",
		Priority:    2,
		PayloadJSON: string(payload),
		AvailableAt: time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	c.recordOrchestrationActivity(ctx, mainAgentMailboxID(sessionID), "dispatch_enqueued", map[string]any{
		"dispatch_id": item.ID,
		"work_item":   item.WorkItemID,
		"target":      item.TargetScope,
	})
	c.startOrchestrationServices()
	return item.ID, nil
}

func (c *coordinator) enqueueAgentNudgeDispatch(ctx context.Context, recipient, prompt string) (string, error) {
	if c == nil || c.orchestrationStore == nil {
		return "", fmt.Errorf("orchestration store is not initialized")
	}
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return "", fmt.Errorf("recipient is required")
	}
	sessionID := c.dispatchSessionForAgent(ctx, recipient)
	if sessionID == "" {
		return "", fmt.Errorf("could not resolve dispatch session for %s", recipient)
	}
	workItemID := mailNudgeDispatchWorkItem(recipient)
	existing, err := c.orchestrationStore.ListDispatchesByWorkItem(ctx, workItemID, []string{"queued", "leased", "running"}, 4)
	if err == nil {
		for _, item := range existing {
			if strings.TrimSpace(item.TargetScope) == dispatchTargetMailNudge {
				return item.ID, nil
			}
		}
	}
	payload, err := json.Marshal(queuedAgentNudgePayload{
		Recipient: recipient,
		Prompt:    firstNonEmptyString(strings.TrimSpace(prompt), mailboxNudgePrompt),
	})
	if err != nil {
		return "", fmt.Errorf("marshal mail nudge payload: %w", err)
	}
	item, err := c.orchestrationStore.EnqueueDispatch(ctx, orchestrationdb.DispatchQueueItem{
		SessionID:   sessionID,
		WorkItemID:  workItemID,
		TargetScope: dispatchTargetMailNudge,
		Status:      "queued",
		Priority:    0,
		PayloadJSON: string(payload),
		AvailableAt: time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	c.recordOrchestrationActivity(ctx, mainAgentMailboxID(sessionID), "mail_nudge_enqueued", map[string]any{
		"dispatch_id": item.ID,
		"recipient":   recipient,
	})
	c.startOrchestrationServices()
	return item.ID, nil
}

func (c *coordinator) dispatchSessionForAgent(ctx context.Context, agentID string) string {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return ""
	}
	if runner := c.ensureSubAgentRegistry().get(agentID); runner != nil {
		runner.mu.Lock()
		parentSessionID := strings.TrimSpace(runner.parentSession)
		sessionID := strings.TrimSpace(runner.sessionID)
		runner.mu.Unlock()
		if parentSessionID != "" {
			return parentSessionID
		}
		if sessionID != "" {
			return sessionID
		}
	}
	if c.orchestrationStore == nil {
		return ""
	}
	state, err := c.orchestrationStore.GetAgentState(ctx, agentID)
	if err != nil {
		return ""
	}
	if parentAgentID := strings.TrimSpace(state.ParentAgentID); strings.HasPrefix(parentAgentID, mainAgentMailboxPrefix) {
		if parentSessionID := strings.TrimPrefix(parentAgentID, mainAgentMailboxPrefix); strings.TrimSpace(parentSessionID) != "" {
			return strings.TrimSpace(parentSessionID)
		}
	}
	return strings.TrimSpace(state.SessionID)
}

func (c *coordinator) processDispatchQueue(ctx context.Context) error {
	if c == nil || c.orchestrationStore == nil {
		return nil
	}
	leaseLimit := c.dispatchActiveLimit() + dispatchLeaseBatchSlack
	if leaseLimit < dispatchLeaseBatchSlack {
		leaseLimit = dispatchLeaseBatchSlack
	}
	if leaseLimit > defaultDispatchQueueCapacity {
		leaseLimit = defaultDispatchQueueCapacity
	}
	queued, err := c.orchestrationStore.LeaseDispatch(ctx, dispatchLeaseOwner, leaseLimit)
	if err != nil {
		return err
	}
	for _, item := range queued {
		if err := c.dispatchQueuedItem(ctx, item); err != nil {
			slog.Warn("Failed to dispatch queued item", "dispatch_id", item.ID, "error", err)
		}
	}
	return nil
}

func (c *coordinator) dispatchQueuedItem(ctx context.Context, item orchestrationdb.DispatchQueueItem) error {
	switch strings.TrimSpace(item.TargetScope) {
	case "", dispatchTargetSubAgent:
		return c.dispatchSubAgentItem(ctx, item)
	case dispatchTargetMailNudge:
		return c.dispatchMailNudgeItem(ctx, item)
	default:
		return c.failDispatchItem(ctx, item, fmt.Errorf("unsupported dispatch target scope %q", item.TargetScope))
	}
}

func (c *coordinator) dispatchSubAgentItem(ctx context.Context, item orchestrationdb.DispatchQueueItem) error {
	if strings.TrimSpace(item.SessionID) == "" {
		return c.failDispatchItem(ctx, item, fmt.Errorf("dispatch session id is missing"))
	}
	if c.activeSubAgentCount(item.SessionID) >= c.dispatchActiveLimit() {
		return c.requeueDispatchItem(ctx, item, "", 2*time.Second)
	}

	var payload queuedSubAgentDispatchPayload
	if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
		return c.failDispatchItem(ctx, item, fmt.Errorf("invalid dispatch payload: %w", err))
	}
	if strings.TrimSpace(payload.Prompt) == "" {
		return c.failDispatchItem(ctx, item, fmt.Errorf("dispatch payload prompt is empty"))
	}

	agentID, submissionID, err := c.spawnSubAgent(ctx, item.SessionID, spawnAgentOptions{
		WorkItemID:       item.WorkItemID,
		Prompt:           payload.Prompt,
		Title:            payload.Title,
		Worktree:         payload.Worktree,
		WorktreePath:     payload.WorktreePath,
		Branch:           payload.Branch,
		WriteManifest:    payload.WriteManifest,
		DefinitionOfDone: payload.DefinitionOfDone,
		TestCommand:      payload.TestCommand,
		AgentID:          payload.AgentID,
		Model:            payload.Model,
		ReasoningEffort:  payload.ReasoningEffort,
		ForkContext:      payload.ForkContext,
	})
	if err != nil {
		return c.failDispatchItem(ctx, item, err)
	}

	item.Status = "running"
	item.LeasedBy = ""
	item.LeasedAt = time.Time{}
	item.AssignedAgentID = agentID
	item.SubmissionID = submissionID
	item.LastError = ""
	item.UpdatedAt = time.Now().UTC()
	if err := c.orchestrationStore.UpdateDispatch(ctx, item); err != nil {
		return err
	}
	c.recordOrchestrationActivity(ctx, mainAgentMailboxID(item.SessionID), "dispatch_started", map[string]any{
		"dispatch_id":   item.ID,
		"work_item":     item.WorkItemID,
		"agent_id":      agentID,
		"submission_id": submissionID,
	})
	if strings.TrimSpace(item.WorkItemID) != "" {
		_ = c.syncDispatchWorkItem(ctx, item.WorkItemID, "in_progress", agentID)
	}
	return nil
}

func (c *coordinator) dispatchMailNudgeItem(ctx context.Context, item orchestrationdb.DispatchQueueItem) error {
	if strings.TrimSpace(item.SessionID) == "" {
		return c.failDispatchItem(ctx, item, fmt.Errorf("mail nudge session id is missing"))
	}

	var payload queuedAgentNudgePayload
	if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
		return c.failDispatchItem(ctx, item, fmt.Errorf("invalid mail nudge payload: %w", err))
	}
	payload.Recipient = strings.TrimSpace(payload.Recipient)
	if payload.Recipient == "" {
		return c.failDispatchItem(ctx, item, fmt.Errorf("mail nudge recipient is empty"))
	}

	c.requeueExpiredMailLeases(ctx)
	actionable, err := c.mailbox.Actionable(ctx, payload.Recipient, 1)
	if err != nil {
		return c.failDispatchItem(ctx, item, fmt.Errorf("load actionable mail for %s: %w", payload.Recipient, err))
	}
	if len(actionable) == 0 {
		return c.completeDispatchItem(ctx, item, "completed", "", payload.Recipient)
	}

	runner := c.ensureSubAgentRegistry().get(payload.Recipient)
	if runner == nil {
		if _, _, err := c.resumeSubAgent(ctx, item.SessionID, payload.Recipient, ""); err != nil {
			return c.failDispatchItem(ctx, item, fmt.Errorf("resume nudge recipient %s: %w", payload.Recipient, err))
		}
		runner = c.ensureSubAgentRegistry().get(payload.Recipient)
		if runner == nil {
			return c.failDispatchItem(ctx, item, fmt.Errorf("mail nudge recipient %s is not available", payload.Recipient))
		}
	}

	runner.mu.Lock()
	closed := runner.closed
	status := runner.effectiveStatusLocked()
	outstanding := runner.hasOutstandingWorkLocked()
	runner.mu.Unlock()
	if closed {
		return c.completeDispatchItem(ctx, item, "completed", "", payload.Recipient)
	}
	if outstanding || isSubAgentActiveStatus(status) {
		c.recordOrchestrationActivity(ctx, payload.Recipient, "mail_nudge_requeued", map[string]any{
			"dispatch_id": item.ID,
			"status":      status,
		})
		return c.requeueDispatchItem(ctx, item, "recipient still busy", dispatchMailNudgeBackoff)
	}

	submissionID, err := c.sendSubAgentInput(ctx, payload.Recipient, firstNonEmptyString(payload.Prompt, mailboxNudgePrompt), nil, false)
	if err != nil {
		return c.failDispatchItem(ctx, item, fmt.Errorf("dispatch mail nudge to %s: %w", payload.Recipient, err))
	}

	item.Status = "running"
	item.LeasedBy = ""
	item.LeasedAt = time.Time{}
	item.AssignedAgentID = payload.Recipient
	item.SubmissionID = submissionID
	item.LastError = ""
	item.UpdatedAt = time.Now().UTC()
	if err := c.orchestrationStore.UpdateDispatch(ctx, item); err != nil {
		return err
	}
	c.recordOrchestrationActivity(ctx, payload.Recipient, "mail_nudge_started", map[string]any{
		"dispatch_id":   item.ID,
		"submission_id": submissionID,
	})
	return nil
}

func (c *coordinator) reconcileDispatchQueue(ctx context.Context) error {
	if c == nil || c.orchestrationStore == nil {
		return nil
	}
	if err := c.healStaleMailDeliveries(ctx); err != nil {
		slog.Warn("Failed to self-heal stale mail deliveries", "error", err)
	}
	items, err := c.orchestrationStore.ListDispatches(ctx, "", []string{"leased", "running"}, defaultDispatchQueueCapacity)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, item := range items {
		switch item.Status {
		case "leased":
			if !item.LeasedAt.IsZero() && now.Sub(item.LeasedAt) > dispatchLeaseTimeout {
				if err := c.failDispatchItem(ctx, item, fmt.Errorf("dispatch lease expired before worker start")); err != nil {
					slog.Warn("Failed to handle expired dispatch lease", "dispatch_id", item.ID, "error", err)
				}
			}
		case "running":
			if err := c.reconcileRunningDispatch(ctx, item, now); err != nil {
				slog.Warn("Failed to reconcile running dispatch", "dispatch_id", item.ID, "error", err)
			}
		}
	}
	return nil
}

func (c *coordinator) reconcileRunningDispatch(ctx context.Context, item orchestrationdb.DispatchQueueItem, now time.Time) error {
	if strings.TrimSpace(item.AssignedAgentID) == "" {
		return c.failDispatchItem(ctx, item, fmt.Errorf("dispatch running without assigned agent"))
	}
	if runner := c.ensureSubAgentRegistry().get(item.AssignedAgentID); runner != nil {
		if status, errMsg, ok := runner.dispatchCompletionLocked(item.SubmissionID); ok {
			if status == "completed" {
				return c.completeDispatchItem(ctx, item, status, "", item.AssignedAgentID)
			}
			return c.completeDispatchItem(ctx, item, status, errMsg, item.AssignedAgentID)
		}
		return nil
	}

	state, err := c.orchestrationStore.GetAgentState(ctx, item.AssignedAgentID)
	if err == nil {
		if dispatchStatus, errMsg, terminal := dispatchTerminalStatus(subAgentStatus(state.Status), ""); terminal {
			return c.completeDispatchItem(ctx, item, dispatchStatus, errMsg, item.AssignedAgentID)
		}
		if !state.LastHeartbeat.IsZero() && now.Sub(state.LastHeartbeat) > dispatchLeaseTimeout {
			return c.failDispatchItem(ctx, item, fmt.Errorf("agent heartbeat expired during dispatch"))
		}
		return nil
	}
	return c.failDispatchItem(ctx, item, fmt.Errorf("assigned agent is missing from runtime and state store"))
}

func dispatchTerminalStatus(status subAgentStatus, errMsg string) (string, string, bool) {
	switch status {
	case subAgentStatusCompleted, subAgentStatusClosed:
		return "completed", "", true
	case subAgentStatusBlocked, subAgentStatusTimedOut, subAgentStatusStuck, subAgentStatusError:
		return "blocked", firstNonEmptyString(strings.TrimSpace(errMsg), string(status)), true
	default:
		return "", "", false
	}
}

func (c *coordinator) completeDispatchItem(ctx context.Context, item orchestrationdb.DispatchQueueItem, status, errMsg, assignee string) error {
	item.Status = status
	item.LastError = strings.TrimSpace(errMsg)
	item.LeasedBy = ""
	item.LeasedAt = time.Time{}
	if assignee != "" {
		item.AssignedAgentID = assignee
	}
	item.UpdatedAt = time.Now().UTC()
	if err := c.orchestrationStore.UpdateDispatch(ctx, item); err != nil {
		return err
	}
	c.recordOrchestrationActivity(ctx, mainAgentMailboxID(item.SessionID), "dispatch_completed", map[string]any{
		"dispatch_id": item.ID,
		"work_item":   item.WorkItemID,
		"status":      status,
		"error":       item.LastError,
	})
	if status == "completed" && strings.TrimSpace(item.AssignedAgentID) != "" && strings.TrimSpace(item.WorkItemID) != "" && c.hookService != nil {
		_ = c.hookService.ClearHook(ctx, item.AssignedAgentID, item.WorkItemID)
	}
	if strings.TrimSpace(item.WorkItemID) != "" {
		workStatus := "closed"
		if status != "completed" {
			workStatus = "blocked"
		}
		return c.syncDispatchWorkItem(ctx, item.WorkItemID, workStatus, item.AssignedAgentID)
	}
	return nil
}

func (c *coordinator) requeueDispatchItem(ctx context.Context, item orchestrationdb.DispatchQueueItem, errMsg string, backoff time.Duration) error {
	if backoff <= 0 {
		backoff = 5 * time.Second
	}
	item.Status = "queued"
	item.LastError = strings.TrimSpace(errMsg)
	item.LeasedBy = ""
	item.LeasedAt = time.Time{}
	item.AvailableAt = time.Now().UTC().Add(backoff)
	item.UpdatedAt = time.Now().UTC()
	return c.orchestrationStore.UpdateDispatch(ctx, item)
}

func (c *coordinator) failDispatchItem(ctx context.Context, item orchestrationdb.DispatchQueueItem, dispatchErr error) error {
	errMsg := ""
	if dispatchErr != nil {
		errMsg = dispatchErr.Error()
	}
	item.RetryCount++
	if item.RetryCount >= dispatchRetryLimit {
		return c.completeDispatchItem(ctx, orchestrationdb.DispatchQueueItem{
			ID:              item.ID,
			SessionID:       item.SessionID,
			WorkItemID:      item.WorkItemID,
			AssignedAgentID: item.AssignedAgentID,
			SubmissionID:    item.SubmissionID,
			RetryCount:      item.RetryCount,
		}, "blocked", errMsg, item.AssignedAgentID)
	}
	if err := c.requeueDispatchItem(ctx, item, errMsg, time.Duration(item.RetryCount)*15*time.Second); err != nil {
		return err
	}
	c.recordOrchestrationActivity(ctx, mainAgentMailboxID(item.SessionID), "dispatch_requeued", map[string]any{
		"dispatch_id": item.ID,
		"work_item":   item.WorkItemID,
		"retry_count": item.RetryCount,
		"error":       errMsg,
	})
	if strings.TrimSpace(item.WorkItemID) != "" {
		_ = c.syncDispatchWorkItem(ctx, item.WorkItemID, "blocked", item.AssignedAgentID)
	}
	return nil
}

func (c *coordinator) syncDispatchWorkItem(ctx context.Context, workItemID, status, assignee string) error {
	if c == nil || c.orchestrationStore == nil || strings.TrimSpace(workItemID) == "" {
		return nil
	}
	item, err := c.orchestrationStore.GetWorkItem(ctx, workItemID)
	if err != nil {
		return nil
	}
	item.Status = status
	if strings.TrimSpace(assignee) != "" {
		item.Assignee = assignee
	}
	if status == "closed" {
		item.ClosedAt = time.Now().UTC()
	}
	if err := c.orchestrationStore.UpsertWorkItem(ctx, item); err != nil {
		return err
	}
	if status == "closed" && strings.TrimSpace(item.ConvoyID) != "" && c.convoyService != nil {
		_, err := c.convoyService.CheckConvoyCompletion(ctx, item.ConvoyID)
		return err
	}
	return nil
}

func (c *coordinator) dispatchActiveLimit() int {
	limit := c.subAgentThreadLimit()
	if limit <= 0 {
		limit = defaultDispatchActiveLimit
	}
	if limit > defaultDispatchActiveLimit {
		return defaultDispatchActiveLimit
	}
	return limit
}

func (c *coordinator) activeSubAgentCountAll() int {
	if c == nil {
		return 0
	}
	count := 0
	for _, runner := range c.ensureSubAgentRegistry().list() {
		if runner == nil {
			continue
		}
		runner.mu.Lock()
		active := isSubAgentActiveStatus(runner.status)
		runner.mu.Unlock()
		if active {
			count++
		}
	}
	return count
}
