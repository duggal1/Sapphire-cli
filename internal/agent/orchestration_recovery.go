package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agentsupervisor "github.com/duggal1/Sapphire-cli/internal/agent/supervisor"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const startupRecoveryResumePrompt = "Startup recovery: the prior Sapphire process stopped before this sub-agent finished. Resume the assigned work from persisted context, inspect the current repository state before changing anything, and continue without restarting from scratch."

const (
	mailPendingRecoveryThreshold = 45 * time.Second
	maxMailRecoveryItems         = 20
	startupRecoveryResumeWindow  = 30 * time.Minute
)

func (c *coordinator) recoverOrchestrationState(ctx context.Context) error {
	if c == nil || c.orchestrationStore == nil {
		return nil
	}
	c.requeueExpiredMailLeases(ctx)

	var errs []error
	if err := c.reclaimLeasedDispatches(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := c.recoverRunningDispatches(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := c.healStaleMailDeliveries(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := c.rehydrateSupervisorTrackers(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (c *coordinator) reclaimLeasedDispatches(ctx context.Context) error {
	if c == nil || c.orchestrationStore == nil {
		return nil
	}
	items, err := c.orchestrationStore.ListDispatches(ctx, "", []string{"leased"}, defaultDispatchQueueCapacity)
	if err != nil {
		return err
	}
	var errs []error
	now := time.Now().UTC()
	for _, item := range items {
		item.Status = "queued"
		item.LeasedBy = ""
		item.LeasedAt = time.Time{}
		item.AssignedAgentID = ""
		item.SubmissionID = ""
		item.AvailableAt = now
		item.LastError = firstNonEmptyString(strings.TrimSpace(item.LastError), "startup recovery reclaimed dispatcher lease")
		item.UpdatedAt = now
		if err := c.orchestrationStore.UpdateDispatch(ctx, item); err != nil {
			errs = append(errs, fmt.Errorf("requeue leased dispatch %s: %w", item.ID, err))
			continue
		}
		c.recordOrchestrationActivity(ctx, mainAgentMailboxID(item.SessionID), "dispatch_recovered", map[string]any{
			"dispatch_id": item.ID,
			"work_item":   item.WorkItemID,
			"status":      "queued",
			"reason":      "leased_dispatch_reclaimed",
		})
	}
	return errors.Join(errs...)
}

func (c *coordinator) recoverRunningDispatches(ctx context.Context) error {
	if c == nil || c.orchestrationStore == nil {
		return nil
	}
	items, err := c.orchestrationStore.ListDispatches(ctx, "", []string{"running"}, defaultDispatchQueueCapacity)
	if err != nil {
		return err
	}
	var errs []error
	for _, item := range items {
		if err := c.recoverRunningDispatch(ctx, item); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (c *coordinator) recoverRunningDispatch(ctx context.Context, item orchestrationdb.DispatchQueueItem) error {
	if strings.TrimSpace(item.AssignedAgentID) == "" {
		return c.requeueRunningDispatchAfterResumeFailure(ctx, item, fmt.Errorf("running dispatch is missing assigned agent"))
	}
	if reason, ok := c.skipRunningDispatchResumeReason(ctx, item); ok {
		return c.blockRunningDispatchAfterStartupSkip(ctx, item, reason)
	}

	submissionID, _, err := c.resumeSubAgent(ctx, item.SessionID, item.AssignedAgentID, c.buildDispatchRecoveryPrompt(ctx, item))
	if err != nil {
		return c.requeueRunningDispatchAfterResumeFailure(ctx, item, err)
	}

	if runner := c.ensureSubAgentRegistry().get(item.AssignedAgentID); runner != nil && strings.TrimSpace(item.WorkItemID) != "" {
		runner.mu.Lock()
		runner.assignment.ID = item.WorkItemID
		if strings.TrimSpace(runner.parentSession) == "" {
			runner.parentSession = item.SessionID
		}
		runner.mu.Unlock()
		c.syncRunnerOrchestrationState(ctx, runner)
	}

	item.Status = "running"
	item.LeasedBy = ""
	item.LeasedAt = time.Time{}
	item.LastError = ""
	if strings.TrimSpace(submissionID) != "" {
		item.SubmissionID = submissionID
	}
	item.UpdatedAt = time.Now().UTC()
	if err := c.orchestrationStore.UpdateDispatch(ctx, item); err != nil {
		return fmt.Errorf("update recovered running dispatch %s: %w", item.ID, err)
	}
	c.recordOrchestrationActivity(ctx, mainAgentMailboxID(item.SessionID), "dispatch_resumed", map[string]any{
		"dispatch_id":   item.ID,
		"work_item":     item.WorkItemID,
		"agent_id":      item.AssignedAgentID,
		"submission_id": item.SubmissionID,
	})
	return nil
}

func (c *coordinator) skipRunningDispatchResumeReason(ctx context.Context, item orchestrationdb.DispatchQueueItem) (string, bool) {
	if c == nil || c.orchestrationStore == nil {
		return "", false
	}
	state, err := c.orchestrationStore.GetAgentState(ctx, item.AssignedAgentID)
	if err != nil {
		return "", false
	}
	status := strings.ToLower(strings.TrimSpace(state.Status))
	switch status {
	case string(subAgentStatusBlocked), string(subAgentStatusClosed), string(subAgentStatusCompleted), string(subAgentStatusError), string(subAgentStatusTimedOut):
		return fmt.Sprintf("startup recovery skipped dispatch for agent in %s state", status), true
	}

	cutoff := time.Now().UTC().Add(-startupRecoveryResumeWindow)
	lastSeen := newestNonZeroTime(item.UpdatedAt, item.LeasedAt, state.UpdatedAt, state.LastHeartbeat)
	if !lastSeen.IsZero() && lastSeen.Before(cutoff) {
		return fmt.Sprintf("startup recovery skipped stale dispatch last active at %s", lastSeen.Format(time.RFC3339)), true
	}
	return "", false
}

func (c *coordinator) buildDispatchRecoveryPrompt(ctx context.Context, item orchestrationdb.DispatchQueueItem) string {
	lines := []string{startupRecoveryResumePrompt}
	if strings.TrimSpace(item.WorkItemID) != "" && c.orchestrationStore != nil {
		if workItem, err := c.orchestrationStore.GetWorkItem(ctx, item.WorkItemID); err == nil {
			if title := strings.TrimSpace(workItem.Title); title != "" {
				lines = append(lines, "Work item: "+title)
			}
			if desc := strings.TrimSpace(workItem.Description); desc != "" {
				lines = append(lines, desc)
			}
		}
	}
	return strings.Join(lines, "\n\n")
}

func (c *coordinator) requeueRunningDispatchAfterResumeFailure(ctx context.Context, item orchestrationdb.DispatchQueueItem, resumeErr error) error {
	reason := "startup recovery could not resume prior sub-agent"
	if resumeErr != nil && strings.TrimSpace(resumeErr.Error()) != "" {
		reason = strings.TrimSpace(resumeErr.Error())
	}

	if strings.TrimSpace(item.AssignedAgentID) != "" {
		c.markRecoveredAgentStateFailed(ctx, item.AssignedAgentID, reason)
	}
	if strings.TrimSpace(item.WorkItemID) != "" {
		if err := c.reopenWorkItemForRedispatch(ctx, item.WorkItemID); err != nil {
			return err
		}
	}

	item.Status = "queued"
	item.LeasedBy = ""
	item.LeasedAt = time.Time{}
	item.AssignedAgentID = ""
	item.SubmissionID = ""
	item.AvailableAt = time.Now().UTC()
	item.LastError = reason
	item.UpdatedAt = time.Now().UTC()
	if err := c.orchestrationStore.UpdateDispatch(ctx, item); err != nil {
		return fmt.Errorf("requeue unrecoverable running dispatch %s: %w", item.ID, err)
	}
	c.recordOrchestrationActivity(ctx, mainAgentMailboxID(item.SessionID), "dispatch_requeued", map[string]any{
		"dispatch_id": item.ID,
		"work_item":   item.WorkItemID,
		"retry_count": item.RetryCount,
		"error":       reason,
		"reason":      "startup_resume_failed",
	})
	return nil
}

func (c *coordinator) blockRunningDispatchAfterStartupSkip(ctx context.Context, item orchestrationdb.DispatchQueueItem, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "startup recovery skipped running dispatch"
	}

	if strings.TrimSpace(item.AssignedAgentID) != "" {
		c.markRecoveredAgentStateBlocked(ctx, item.AssignedAgentID, reason)
	}
	if strings.TrimSpace(item.WorkItemID) != "" {
		if err := c.blockWorkItemAfterStartupSkip(ctx, item.WorkItemID, reason); err != nil {
			return err
		}
	}

	item.Status = "blocked"
	item.LeasedBy = ""
	item.LeasedAt = time.Time{}
	item.AssignedAgentID = ""
	item.SubmissionID = ""
	item.LastError = reason
	item.UpdatedAt = time.Now().UTC()
	if err := c.orchestrationStore.UpdateDispatch(ctx, item); err != nil {
		return fmt.Errorf("block stale startup dispatch %s: %w", item.ID, err)
	}
	c.recordOrchestrationActivity(ctx, mainAgentMailboxID(item.SessionID), "dispatch_blocked", map[string]any{
		"dispatch_id": item.ID,
		"work_item":   item.WorkItemID,
		"error":       reason,
		"reason":      "startup_skip",
	})
	return nil
}

func (c *coordinator) reopenWorkItemForRedispatch(ctx context.Context, workItemID string) error {
	if c == nil || c.orchestrationStore == nil || strings.TrimSpace(workItemID) == "" {
		return nil
	}
	item, err := c.orchestrationStore.GetWorkItem(ctx, workItemID)
	if err != nil {
		return nil
	}
	item.Status = "open"
	item.Assignee = ""
	item.ClosedAt = time.Time{}
	if err := c.orchestrationStore.UpsertWorkItem(ctx, item); err != nil {
		return fmt.Errorf("reopen work item %s: %w", workItemID, err)
	}
	return nil
}

func (c *coordinator) blockWorkItemAfterStartupSkip(ctx context.Context, workItemID, reason string) error {
	if c == nil || c.orchestrationStore == nil || strings.TrimSpace(workItemID) == "" {
		return nil
	}
	item, err := c.orchestrationStore.GetWorkItem(ctx, workItemID)
	if err != nil {
		return nil
	}
	item.Status = "blocked"
	item.Assignee = ""
	item.ClosedAt = time.Time{}
	if strings.TrimSpace(item.Description) == "" {
		item.Description = reason
	}
	if err := c.orchestrationStore.UpsertWorkItem(ctx, item); err != nil {
		return fmt.Errorf("block work item %s: %w", workItemID, err)
	}
	return nil
}

func (c *coordinator) markRecoveredAgentStateFailed(ctx context.Context, agentID, reason string) {
	if c == nil || c.orchestrationStore == nil || strings.TrimSpace(agentID) == "" {
		return
	}
	state, err := c.orchestrationStore.GetAgentState(ctx, agentID)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	state.Status = string(subAgentStatusError)
	state.UpdatedAt = now
	if state.LastHeartbeat.IsZero() {
		state.LastHeartbeat = now
	}
	if err := c.orchestrationStore.UpsertAgentState(ctx, state); err == nil {
		c.recordOrchestrationActivity(ctx, agentID, "recovery_resume_failed", map[string]any{
			"reason": reason,
		})
	}
}

func (c *coordinator) markRecoveredAgentStateBlocked(ctx context.Context, agentID, reason string) {
	if c == nil || c.orchestrationStore == nil || strings.TrimSpace(agentID) == "" {
		return
	}
	state, err := c.orchestrationStore.GetAgentState(ctx, agentID)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	state.Status = string(subAgentStatusBlocked)
	state.UpdatedAt = now
	if state.LastHeartbeat.IsZero() {
		state.LastHeartbeat = now
	}
	if err := c.orchestrationStore.UpsertAgentState(ctx, state); err == nil {
		c.recordOrchestrationActivity(ctx, agentID, "recovery_resume_skipped", map[string]any{
			"reason": reason,
		})
	}
}

func (c *coordinator) rehydrateSupervisorTrackers(ctx context.Context) error {
	if c == nil || c.supervisor == nil || c.orchestrationStore == nil {
		return nil
	}
	states, err := c.orchestrationStore.ListAgentStates(ctx, 200)
	if err != nil {
		return err
	}
	for _, state := range states {
		if strings.TrimSpace(state.AgentID) == "" || strings.TrimSpace(state.Role) != "subagent" {
			continue
		}
		if snapshot, ok := c.supervisorRuntimeSnapshot(state.AgentID); ok {
			c.supervisor.TrackAgent(snapshot)
			continue
		}
		parentSessionID := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(state.ParentAgentID), mainAgentMailboxPrefix))
		c.supervisor.TrackAgent(agentsupervisor.AgentRuntimeSnapshot{
			AgentID:         state.AgentID,
			SessionID:       strings.TrimSpace(state.SessionID),
			ParentSessionID: parentSessionID,
			WorkItemID:      strings.TrimSpace(state.HookBeadID),
			Status:          strings.TrimSpace(state.Status),
			Branch:          strings.TrimSpace(state.Branch),
			LastHeartbeat:   state.LastHeartbeat,
		})
	}
	return nil
}

func (c *coordinator) healStaleMailDeliveries(ctx context.Context) error {
	if c == nil || c.orchestrationStore == nil {
		return nil
	}
	items, err := c.orchestrationStore.ListStalePendingMail(ctx, time.Now().UTC().Add(-mailPendingRecoveryThreshold), maxMailRecoveryItems)
	if err != nil {
		return err
	}
	var errs []error
	for _, item := range items {
		if err := c.recoverPendingMailDelivery(ctx, item); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (c *coordinator) recoverPendingMailDelivery(ctx context.Context, item orchestrationdb.AgentMail) error {
	recipient := strings.TrimSpace(item.ResolvedToAgent)
	if recipient == "" || strings.HasPrefix(recipient, mainAgentMailboxPrefix) {
		return nil
	}
	if recipient == "supervisor" {
		_, err := c.orchestrationStore.DeadLetterMail(ctx, item.ID)
		return err
	}
	dispatchID, action, err := c.ensureMailRecipientAvailable(ctx, recipient)
	if err != nil {
		return fmt.Errorf("recover pending mail delivery %s for %s: %w", item.ID, recipient, err)
	}
	if dispatchID == "" {
		return nil
	}
	c.recordOrchestrationActivity(ctx, recipient, "mail_self_healed", map[string]any{
		"id":                item.ID,
		"thread_id":         item.ThreadID,
		"delivery_state":    item.DeliveryState,
		"delivery_attempts": item.DeliveryAttempts,
		"action":            action,
		"dispatch_id":       dispatchID,
	})
	return nil
}

func (c *coordinator) ensureMailRecipientAvailable(ctx context.Context, recipient string) (string, string, error) {
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return "", "", nil
	}
	if runner := c.ensureSubAgentRegistry().get(recipient); runner != nil {
		dispatchID, err := c.enqueueAgentNudgeDispatch(ctx, recipient, mailboxNudgePrompt)
		return dispatchID, "nudge", err
	}
	if c.orchestrationStore == nil {
		return "", "", fmt.Errorf("orchestration store is not initialized")
	}

	state, err := c.orchestrationStore.GetAgentState(ctx, recipient)
	if err == nil {
		workItemID := strings.TrimSpace(state.HookBeadID)
		if workItemID != "" {
			workItem, workErr := c.orchestrationStore.GetWorkItem(ctx, workItemID)
			if workErr == nil && isRecoverableWorkItemStatus(workItem.Status) {
				dispatchID, dispatchErr := c.ensureDispatchForWorkItem(ctx, workItem)
				if dispatchErr == nil && strings.TrimSpace(dispatchID) != "" {
					return dispatchID, "redispatch", nil
				}
				if dispatchErr != nil {
					err = dispatchErr
				}
			}
		}
	}

	dispatchID, nudgeErr := c.enqueueAgentNudgeDispatch(ctx, recipient, mailboxNudgePrompt)
	if nudgeErr == nil {
		return dispatchID, "nudge", nil
	}
	if err != nil {
		return "", "", err
	}
	return "", "", nudgeErr
}

func isRecoverableWorkItemStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "closed", "completed":
		return false
	default:
		return true
	}
}

func newestNonZeroTime(items ...time.Time) time.Time {
	var newest time.Time
	for _, item := range items {
		if item.IsZero() {
			continue
		}
		if newest.IsZero() || item.After(newest) {
			newest = item
		}
	}
	return newest
}
