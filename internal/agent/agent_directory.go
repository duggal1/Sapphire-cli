package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/fantasy"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const AgentDirectoryToolName = "agent_directory"

type AgentDirectoryParams struct {
	Limit int `json:"limit,omitempty" description:"Maximum number of active agents to include. Defaults to 12."`
}

type agentDirectoryAlias struct {
	Alias          string `json:"alias"`
	Kind           string `json:"kind"`
	TargetAgentID  string `json:"target_agent_id,omitempty"`
	TargetWorkItem string `json:"target_work_item_id,omitempty"`
}

type agentDirectoryAgent struct {
	AgentID      string `json:"agent_id"`
	Status       string `json:"status"`
	WorkItemID   string `json:"work_item_id,omitempty"`
	Branch       string `json:"branch,omitempty"`
	Worktree     string `json:"worktree,omitempty"`
	RouteAlias   string `json:"route_alias,omitempty"`
	HeartbeatAge string `json:"heartbeat_age,omitempty"`
}

type agentDirectoryWorkItem struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	Assignee     string   `json:"assignee,omitempty"`
	RouteAlias   string   `json:"route_alias,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

type agentDirectoryDependency struct {
	FromWorkItemID string `json:"from_work_item_id"`
	DependsOnID    string `json:"depends_on_id"`
}

type agentDirectorySnapshot struct {
	SessionID        string                     `json:"session_id"`
	ParentSessionID  string                     `json:"parent_session_id"`
	CurrentAgentID   string                     `json:"current_agent_id"`
	Agents           []agentDirectoryAgent      `json:"agents"`
	WorkItems        []agentDirectoryWorkItem   `json:"work_items"`
	DependencyEdges  []agentDirectoryDependency `json:"dependency_edges"`
	RouteableAliases []agentDirectoryAlias      `json:"routeable_aliases"`
}

func (c *coordinator) agentDirectoryTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		AgentDirectoryToolName,
		"Read the current agent/work-item directory for this orchestration session, including routeable aliases and dependency edges.",
		func(ctx context.Context, params AgentDirectoryParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if c.orchestrationStore == nil {
				return fantasy.NewTextErrorResponse("orchestration store not initialized"), nil
			}
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			payload := c.buildAgentDirectorySnapshot(ctx, sessionID, params.Limit)
			return fantasy.NewTextResponse(marshalPrettyJSON(payload)), nil
		},
	), nil
}

func (c *coordinator) buildAgentDirectorySnapshot(ctx context.Context, sessionID string, limit int) agentDirectorySnapshot {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return agentDirectorySnapshot{}
	}
	if limit <= 0 || limit > 50 {
		limit = 12
	}

	currentAgentID := c.mailboxIdentityForSession(sessionID)
	parentSessionID := c.directoryParentSessionID(sessionID)
	parentMailboxID := mainAgentMailboxID(parentSessionID)

	states := c.listAgentStateSnapshots(ctx, parentSessionID, parentMailboxID, limit)
	if currentState, ok := c.directoryAgentState(ctx, currentAgentID); ok {
		states = appendAgentStateIfMissing(states, currentState)
	}
	if currentAgentID != parentMailboxID {
		if mainState, ok := c.directoryAgentState(ctx, parentMailboxID); ok {
			states = appendAgentStateIfMissing(states, mainState)
		}
	}

	agentEntries := make([]agentDirectoryAgent, 0, len(states))
	workItemIDs := make([]string, 0, len(states))
	for _, state := range states {
		agentEntries = append(agentEntries, agentDirectoryAgent{
			AgentID:      state.AgentID,
			Status:       state.Status,
			WorkItemID:   strings.TrimSpace(state.HookBeadID),
			Branch:       strings.TrimSpace(state.Branch),
			Worktree:     filepath.Base(strings.TrimSpace(state.WorktreePath)),
			RouteAlias:   "agent:" + strings.TrimSpace(state.AgentID),
			HeartbeatAge: formatHeartbeatAge(state.LastHeartbeat),
		})
		if workItemID := strings.TrimSpace(state.HookBeadID); workItemID != "" {
			workItemIDs = append(workItemIDs, workItemID)
		}
	}

	workItems := c.loadAgentDirectoryWorkItems(ctx, uniqueStrings(workItemIDs))
	workItemEntries := make([]agentDirectoryWorkItem, 0, len(workItems))
	dependencyEdges := make([]agentDirectoryDependency, 0, len(workItems))
	for _, item := range workItems {
		deps := parseWorkItemDependencies(item.Dependencies)
		workItemEntries = append(workItemEntries, agentDirectoryWorkItem{
			ID:           item.ID,
			Title:        item.Title,
			Status:       item.Status,
			Assignee:     strings.TrimSpace(item.Assignee),
			RouteAlias:   "work:" + item.ID,
			Dependencies: deps,
		})
		for _, depID := range deps {
			dependencyEdges = append(dependencyEdges, agentDirectoryDependency{
				FromWorkItemID: item.ID,
				DependsOnID:    depID,
			})
		}
	}

	aliases := []agentDirectoryAlias{
		{Alias: "self", Kind: "agent", TargetAgentID: currentAgentID},
		{Alias: "main", Kind: "agent", TargetAgentID: parentMailboxID},
		{Alias: "parent", Kind: "agent", TargetAgentID: parentMailboxID},
	}
	for _, agentEntry := range agentEntries {
		aliases = append(aliases, agentDirectoryAlias{
			Alias:         "agent:" + agentEntry.AgentID,
			Kind:          "agent",
			TargetAgentID: agentEntry.AgentID,
		})
	}
	for _, item := range workItemEntries {
		aliases = append(aliases, agentDirectoryAlias{
			Alias:          item.RouteAlias,
			Kind:           "work_item",
			TargetAgentID:  item.Assignee,
			TargetWorkItem: item.ID,
		})
	}

	return agentDirectorySnapshot{
		SessionID:        sessionID,
		ParentSessionID:  parentSessionID,
		CurrentAgentID:   currentAgentID,
		Agents:           agentEntries,
		WorkItems:        workItemEntries,
		DependencyEdges:  dependencyEdges,
		RouteableAliases: aliases,
	}
}

func (c *coordinator) buildAgentDirectoryContext(ctx context.Context, sessionID string, limit int) string {
	snapshot := c.buildAgentDirectorySnapshot(ctx, sessionID, limit)
	if len(snapshot.Agents) == 0 && len(snapshot.WorkItems) == 0 {
		return ""
	}
	lines := []string{
		fmt.Sprintf("- Current agent: %s", snapshot.CurrentAgentID),
		fmt.Sprintf("- Main mailbox: %s", mainAgentMailboxID(snapshot.ParentSessionID)),
	}
	for _, agentEntry := range snapshot.Agents {
		line := fmt.Sprintf("- Agent %s | %s", agentEntry.AgentID, agentEntry.Status)
		if agentEntry.WorkItemID != "" {
			line += " | work: " + agentEntry.WorkItemID
		}
		if agentEntry.RouteAlias != "" {
			line += " | route: " + agentEntry.RouteAlias
		}
		if agentEntry.Branch != "" {
			line += " | branch: " + agentEntry.Branch
		}
		if agentEntry.Worktree != "" && agentEntry.Worktree != "." {
			line += " | worktree: " + agentEntry.Worktree
		}
		if agentEntry.HeartbeatAge != "" {
			line += " | heartbeat " + agentEntry.HeartbeatAge + " ago"
		}
		lines = append(lines, line)
	}
	for _, item := range snapshot.WorkItems {
		line := fmt.Sprintf("- Work %s [%s]", item.ID, item.Status)
		if item.Title != "" {
			line += " | " + item.Title
		}
		if item.Assignee != "" {
			line += " | assignee: " + item.Assignee
		}
		if item.RouteAlias != "" {
			line += " | route: " + item.RouteAlias
		}
		if len(item.Dependencies) > 0 {
			line += " | deps: " + strings.Join(item.Dependencies, ", ")
		}
		lines = append(lines, line)
	}
	return "### Peer Directory\n" + strings.Join(lines, "\n")
}

func (c *coordinator) directoryParentSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	if runner := c.runnerBySessionID(sessionID); runner != nil {
		runner.mu.Lock()
		parentSessionID := strings.TrimSpace(runner.parentSession)
		runner.mu.Unlock()
		if parentSessionID != "" {
			return parentSessionID
		}
	}
	return sessionID
}

func (c *coordinator) directoryAgentState(ctx context.Context, agentID string) (orchestrationdb.AgentState, bool) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return orchestrationdb.AgentState{}, false
	}
	if c.stateService != nil {
		if state, err := c.stateService.Status(ctx, agentID); err == nil {
			return state, true
		}
	}
	if c.orchestrationStore != nil {
		if state, err := c.orchestrationStore.GetAgentState(ctx, agentID); err == nil {
			return state, true
		}
	}
	return orchestrationdb.AgentState{}, false
}

func (c *coordinator) loadAgentDirectoryWorkItems(ctx context.Context, ids []string) []orchestrationdb.WorkItem {
	if c == nil || c.orchestrationStore == nil || len(ids) == 0 {
		return nil
	}
	items := make([]orchestrationdb.WorkItem, 0, len(ids))
	for _, id := range ids {
		item, err := c.orchestrationStore.GetWorkItem(ctx, id)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func appendAgentStateIfMissing(states []orchestrationdb.AgentState, state orchestrationdb.AgentState) []orchestrationdb.AgentState {
	agentID := strings.TrimSpace(state.AgentID)
	if agentID == "" {
		return states
	}
	for _, existing := range states {
		if strings.TrimSpace(existing.AgentID) == agentID {
			return states
		}
	}
	return append(states, state)
}

func parseWorkItemDependencies(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var deps []string
	if err := json.Unmarshal([]byte(raw), &deps); err != nil {
		return nil
	}
	return uniqueStrings(deps)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatHeartbeatAge(lastHeartbeat time.Time) string {
	if lastHeartbeat.IsZero() {
		return ""
	}
	return time.Since(lastHeartbeat).Truncate(time.Second).String()
}
