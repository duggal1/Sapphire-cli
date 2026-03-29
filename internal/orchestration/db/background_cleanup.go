package orchestrationdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type BackgroundCleanupSummary struct {
	StoppedDispatches int
	BlockedWorkItems  int
	BlockedAgentState int
	DeadLetteredMail  int
}

func (s *Store) StopBackgroundActivity(ctx context.Context, reason string) (BackgroundCleanupSummary, error) {
	if s == nil || s.conn == nil {
		return BackgroundCleanupSummary{}, fmt.Errorf("orchestration store is not initialized")
	}

	reason = stringsTrim(reason)
	if reason == "" {
		reason = "background activity stopped by user"
	}

	now := time.Now().UTC()
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return BackgroundCleanupSummary{}, fmt.Errorf("begin background cleanup transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	var summary BackgroundCleanupSummary
	workItemIDs := make(map[string]struct{})

	dispatchRows, err := tx.QueryContext(
		ctx,
		`SELECT id, session_id, work_item_id, target_scope, status, priority, payload_json, retry_count, last_error,
		        available_at, leased_by, leased_at, assigned_agent_id, submission_id, created_at, updated_at
		   FROM dispatch_queue
		  WHERE status IN ('queued', 'leased', 'running')
		  ORDER BY priority ASC, created_at ASC`,
	)
	if err != nil {
		return BackgroundCleanupSummary{}, fmt.Errorf("query active dispatches for cleanup: %w", err)
	}
	dispatches, err := scanDispatchRows(dispatchRows)
	dispatchRows.Close()
	if err != nil {
		return BackgroundCleanupSummary{}, err
	}
	for _, item := range dispatches {
		if _, execErr := tx.ExecContext(
			ctx,
			`UPDATE dispatch_queue
			    SET status = ?, last_error = ?, available_at = ?, leased_by = '', leased_at = 0,
			        assigned_agent_id = '', submission_id = '', updated_at = ?
			  WHERE id = ?`,
			"blocked",
			reason,
			now.Unix(),
			now.Unix(),
			item.ID,
		); execErr != nil {
			return BackgroundCleanupSummary{}, fmt.Errorf("block dispatch %s: %w", item.ID, execErr)
		}
		summary.StoppedDispatches++
		if workItemID := stringsTrim(item.WorkItemID); workItemID != "" {
			workItemIDs[workItemID] = struct{}{}
		}
	}

	blockedDispatchRows, err := tx.QueryContext(
		ctx,
		`SELECT id, session_id, work_item_id, target_scope, status, priority, payload_json, retry_count, last_error,
		        available_at, leased_by, leased_at, assigned_agent_id, submission_id, created_at, updated_at
		   FROM dispatch_queue
		  WHERE status = 'blocked'
		  ORDER BY priority ASC, created_at ASC`,
	)
	if err != nil {
		return BackgroundCleanupSummary{}, fmt.Errorf("query blocked dispatches for cleanup: %w", err)
	}
	blockedDispatches, err := scanDispatchRows(blockedDispatchRows)
	blockedDispatchRows.Close()
	if err != nil {
		return BackgroundCleanupSummary{}, err
	}
	for _, item := range blockedDispatches {
		if workItemID := stringsTrim(item.WorkItemID); workItemID != "" {
			workItemIDs[workItemID] = struct{}{}
		}
	}

	agentRows, err := tx.QueryContext(
		ctx,
		`SELECT agent_id, role, status, session_id, worktree_path, branch, hook_bead_id, parent_agent_id, last_heartbeat, created_at, updated_at
		   FROM agent_state
		  WHERE role = 'subagent'
		    AND status IN ('queued', 'starting', 'ready', 'waiting_on_mail', 'retrying', 'running', 'degraded', 'stuck', 'blocked')`,
	)
	if err != nil {
		return BackgroundCleanupSummary{}, fmt.Errorf("query active agent states for cleanup: %w", err)
	}
	states, err := scanAgentStateRows(agentRows)
	agentRows.Close()
	if err != nil {
		return BackgroundCleanupSummary{}, err
	}
	for _, state := range states {
		heartbeat := state.LastHeartbeat
		if heartbeat.IsZero() {
			heartbeat = now
		}
		if _, execErr := tx.ExecContext(
			ctx,
			`UPDATE agent_state
			    SET status = ?, last_heartbeat = ?, updated_at = ?
			  WHERE agent_id = ?`,
			"closed",
			heartbeat.Unix(),
			now.Unix(),
			state.AgentID,
		); execErr != nil {
			return BackgroundCleanupSummary{}, fmt.Errorf("block agent state %s: %w", state.AgentID, execErr)
		}
		summary.BlockedAgentState++
		if workItemID := stringsTrim(state.HookBeadID); workItemID != "" {
			workItemIDs[workItemID] = struct{}{}
		}
	}

	mailRows, err := tx.QueryContext(
		ctx,
		`SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		        delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		   FROM agent_mail
		  WHERE delivery_state IN (?, ?)
		  ORDER BY created_at ASC, rowid ASC`,
		MailDeliveryStatePending,
		MailDeliveryStateLeased,
	)
	if err != nil {
		return BackgroundCleanupSummary{}, fmt.Errorf("query actionable mail for cleanup: %w", err)
	}
	mailItems, err := scanMailRows(mailRows)
	mailRows.Close()
	if err != nil {
		return BackgroundCleanupSummary{}, err
	}
	for _, item := range mailItems {
		if _, execErr := tx.ExecContext(
			ctx,
			`UPDATE agent_mail
			    SET delivery_state = ?, lease_owner = '', lease_expires_at = 0, read = 1,
			        read_at = CASE WHEN read_at = 0 THEN ? ELSE read_at END
			  WHERE rowid = ?`,
			MailDeliveryStateDeadLetter,
			now.Unix(),
			item.RowID,
		); execErr != nil {
			return BackgroundCleanupSummary{}, fmt.Errorf("dead-letter mail %s: %w", item.ID, execErr)
		}
		summary.DeadLetteredMail++
	}

	for workItemID := range workItemIDs {
		row := tx.QueryRowContext(
			ctx,
			`SELECT id, type, title, description, status, assignee, parent_id, convoy_id, dependencies, created_at, closed_at
			   FROM work_items
			  WHERE id = ?`,
			workItemID,
		)
		item, getErr := scanWorkItem(row)
		switch {
		case getErr == nil:
		case errors.Is(getErr, sql.ErrNoRows):
			item = WorkItem{
				ID:          workItemID,
				Type:        "task",
				Title:       workItemID,
				Description: reason,
				CreatedAt:   now,
			}
		default:
			return BackgroundCleanupSummary{}, fmt.Errorf("load work item %s for cleanup: %w", workItemID, getErr)
		}

		item.Status = "closed"
		item.Assignee = ""
		item.ClosedAt = now
		if strings.TrimSpace(item.Description) == "" {
			item.Description = reason
		}
		if strings.TrimSpace(item.Type) == "" {
			item.Type = "task"
		}
		if strings.TrimSpace(item.Title) == "" {
			item.Title = workItemID
		}
		if strings.TrimSpace(item.Dependencies) == "" {
			item.Dependencies = "[]"
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}

		if _, execErr := tx.ExecContext(
			ctx,
			`INSERT INTO work_items (id, type, title, description, status, assignee, parent_id, convoy_id, dependencies, created_at, closed_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
				type = excluded.type,
				title = excluded.title,
				description = excluded.description,
				status = excluded.status,
				assignee = excluded.assignee,
				parent_id = excluded.parent_id,
				convoy_id = excluded.convoy_id,
				dependencies = excluded.dependencies,
				closed_at = excluded.closed_at`,
			stringsTrim(item.ID),
			stringsTrim(item.Type),
			stringsTrim(item.Title),
			item.Description,
			stringsTrim(item.Status),
			stringsTrim(item.Assignee),
			stringsTrim(item.ParentID),
			stringsTrim(item.ConvoyID),
			item.Dependencies,
			item.CreatedAt.UTC().Unix(),
			timeToUnix(item.ClosedAt),
		); execErr != nil {
			return BackgroundCleanupSummary{}, fmt.Errorf("block work item %s: %w", item.ID, execErr)
		}
		summary.BlockedWorkItems++
	}

	if err := tx.Commit(); err != nil {
		return BackgroundCleanupSummary{}, fmt.Errorf("commit background cleanup transaction: %w", err)
	}
	tx = nil
	return summary, nil
}
