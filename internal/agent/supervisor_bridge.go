package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/agent/longhorizon"
	agentsupervisor "github.com/duggal1/Sapphire-cli/internal/agent/supervisor"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

func (c *coordinator) supervisorRuntimeSnapshot(agentID string) (agentsupervisor.AgentRuntimeSnapshot, bool) {
	if c == nil || strings.TrimSpace(agentID) == "" {
		return agentsupervisor.AgentRuntimeSnapshot{}, false
	}
	runner := c.ensureSubAgentRegistry().get(agentID)
	if runner == nil {
		return agentsupervisor.AgentRuntimeSnapshot{}, false
	}
	snap := runner.snapshot()
	return agentsupervisor.AgentRuntimeSnapshot{
		AgentID:              snap.ID,
		SessionID:            runner.sessionID,
		ParentSessionID:      runner.parentSession,
		WorkItemID:           runner.assignment.ID,
		Status:               string(snap.Status),
		DefinitionOfDone:     snap.DefinitionOfDone,
		LastResult:           snap.LastResult,
		LastError:            snap.LastError,
		LastProgress:         snap.LastProgress,
		Branch:               snap.Branch,
		ValidationPassed:     snap.ValidationPassed,
		ValidationErrors:     snap.ValidationErrors,
		ValidationHasChanges: snap.ValidationHasChanges,
		LastHeartbeat:        snap.LastHeartbeat,
		HeartbeatContext:     snap.HeartbeatContext,
	}, true
}

func (c *coordinator) ensureDispatchForWorkItem(ctx context.Context, item orchestrationdb.WorkItem) (string, error) {
	if c == nil || c.orchestrationStore == nil {
		return "", fmt.Errorf("orchestration store is not initialized")
	}
	if strings.TrimSpace(item.ID) == "" {
		return "", fmt.Errorf("work item id is required")
	}
	existing, err := c.orchestrationStore.ListDispatchesByWorkItem(ctx, item.ID, []string{"queued", "leased", "running", "completed"}, 10)
	if err == nil {
		for _, dispatch := range existing {
			if strings.TrimSpace(dispatch.ID) != "" {
				return dispatch.ID, nil
			}
		}
	}
	sessionID := firstNonEmptyString(item.ParentID, item.Assignee)
	if strings.HasPrefix(sessionID, "main:") {
		sessionID = strings.TrimPrefix(sessionID, "main:")
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = inferSessionIDFromWorkItemID(item.ID)
	}
	if strings.TrimSpace(sessionID) == "" {
		return "", fmt.Errorf("unable to resolve session id for work item %s", item.ID)
	}
	prompt := strings.TrimSpace(item.Description)
	if prompt == "" {
		prompt = item.Title
	}
	return c.enqueueSubAgentDispatch(ctx, sessionID, item.ID, "subagent", spawnAgentOptions{
		WorkItemID:       item.ID,
		Prompt:           prompt,
		Title:            item.Title,
		Worktree:         true,
		WriteManifest:    []string{"."},
		DefinitionOfDone: "",
		ForkContext:      false,
	})
}

func inferSessionIDFromWorkItemID(workItemID string) string {
	workItemID = strings.TrimSpace(workItemID)
	if workItemID == "" {
		return ""
	}
	if strings.HasPrefix(workItemID, "lh:") {
		parts := strings.Split(workItemID, ":")
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	return ""
}

func (c *coordinator) ensureLongHorizonDispatch(ctx context.Context, sessionID, userPrompt string) {
	if c == nil || c.longHorizon == nil || c.orchestrationStore == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	state, err := c.longHorizon.Ensure(ctx, sessionID, userPrompt)
	if err != nil || state == nil {
		return
	}
	planData, err := c.longHorizon.ReadPlan(sessionID)
	if err != nil || len(planData.Milestones) == 0 {
		return
	}
	specText, _ := c.longHorizon.ReadSpec(sessionID)
	rootID := "lh:" + sessionID

	if _, err := c.orchestrationStore.GetWorkItem(ctx, rootID); err == sql.ErrNoRows {
		_ = c.orchestrationStore.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
			ID:          rootID,
			Type:        "epic",
			Title:       "Long-horizon session " + sessionID,
			Description: truncateForContext(specText, 800),
			Status:      "open",
			Assignee:    mainAgentMailboxID(sessionID),
			CreatedAt:   time.Now().UTC(),
		})
	}

	for i, milestone := range planData.Milestones {
		workItemID := fmt.Sprintf("%s:%s", rootID, milestone.ID)
		deps := []string{}
		status := "open"
		if i > 0 {
			deps = append(deps, fmt.Sprintf("%s:%s", rootID, planData.Milestones[i-1].ID))
			status = "blocked"
		}
		if existing, err := c.orchestrationStore.GetWorkItem(ctx, workItemID); err == nil {
			if normalizeLongHorizonWorkStatus(existing.Status) == "closed" {
				continue
			}
			status = existing.Status
		}
		depsJSON, _ := json.Marshal(deps)
		item := orchestrationdb.WorkItem{
			ID:           workItemID,
			Type:         "task",
			Title:        milestone.Name,
			Description:  buildMilestonePrompt(specText, milestone),
			Status:       status,
			Assignee:     mainAgentMailboxID(sessionID),
			ParentID:     sessionID,
			Dependencies: string(depsJSON),
			CreatedAt:    time.Now().UTC(),
		}
		_ = c.orchestrationStore.UpsertWorkItem(ctx, item)
		if len(deps) == 0 && normalizeLongHorizonWorkStatus(status) != "closed" {
			_, _ = c.ensureDispatchForWorkItem(ctx, item)
		}
	}
	_ = state
}

func buildMilestonePrompt(specText string, milestone longhorizon.Milestone) string {
	specText = truncateForContext(strings.TrimSpace(specText), 900)
	builder := &strings.Builder{}
	builder.WriteString("Long-horizon milestone execution.\n\n")
	builder.WriteString("Milestone: " + strings.TrimSpace(milestone.Name) + "\n")
	if cond := strings.TrimSpace(milestone.Condition); cond != "" {
		builder.WriteString("Completion condition: " + cond + "\n")
	}
	if specText != "" {
		builder.WriteString("\nFrozen spec context:\n")
		builder.WriteString(specText)
	}
	builder.WriteString("\n\nImplement only the current milestone. Validate before reporting completion.")
	return builder.String()
}

func normalizeLongHorizonWorkStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
		return "closed"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}
