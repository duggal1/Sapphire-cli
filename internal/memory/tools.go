package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

const (
	ViewToolName    = "view_memory"
	RefreshToolName = "refresh_memory"
	RecallToolName  = "recall_memory"
	SaveToolName    = "save_memory"
	HealthToolName  = "memory_health"
)

// RecallParams is the input schema for the recall_memory tool.
type RecallParams struct {
	Query  string `json:"query" description:"What the agent is looking for in plain language"`
	Filter string `json:"filter,omitempty" description:"Filter type: all, negative_constraints, architectural, failures, progress, evals, strategies. Defaults to all."`
	Limit  int    `json:"limit,omitempty" description:"Maximum number of records to return. Defaults to 5."`
}

// SaveParams is the input schema for the save_memory tool.
type SaveParams struct {
	EventType string          `json:"event_type" description:"One of: architectural_decision, negative_constraint, failure_mode, task_progress, improvement_eval, strategy_pattern"`
	Content   json.RawMessage `json:"content" description:"Structured JSON content to persist"`
}

type RefreshParams struct {
	SessionID string `json:"session_id,omitempty" description:"Optional target session ID. Defaults to the current session."`
	Reason    string `json:"reason,omitempty" description:"Short reason for the refresh request. Used only for audit context."`
}

// NewRecallTool creates the recall_memory agent tool.
func NewViewMemoryTool(system *System, resolveSessionID func(context.Context) string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ViewToolName,
		"Fetch durable session memory from the per-session history journal. Use it when the conversation is long, after compaction, when resuming earlier work, when the user refers to a prior decision, or when you need the exact earlier tool/result trail. Do not use it for the immediately visible local context.",
		func(ctx context.Context, params ViewMemoryParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if system == nil || system.History == nil {
				return fantasy.NewTextResponse("Memory system not initialized."), nil
			}
			sessionID := ""
			if resolveSessionID != nil {
				sessionID = resolveSessionID(ctx)
			}
			result, err := system.History.View(ctx, sessionID, params)
			if err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf("Memory query failed: %s", err)), nil
			}
			payload, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf("Failed to encode memory response: %s", err)), nil
			}
			return fantasy.NewTextResponse(string(payload)), nil
		},
	)
}

// NewRefreshTool creates the refresh_memory agent tool.
func NewRefreshTool(system *System, resolveSessionID func(context.Context) string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RefreshToolName,
		"Force regeneration of memory.md from the live codebase map and current session state. Use after major codebase reads, major architecture changes, or when memory appears stale. Do not use repeatedly without new information.",
		func(ctx context.Context, params RefreshParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if system == nil || system.MemoryFile == nil || system.History == nil {
				return fantasy.NewTextResponse("Memory system not initialized."), nil
			}
			sessionID := strings.TrimSpace(params.SessionID)
			if sessionID == "" && resolveSessionID != nil {
				sessionID = strings.TrimSpace(resolveSessionID(ctx))
			}
			if sessionID == "" {
				sessionID = strings.TrimSpace(system.SessionID)
			}
			if sessionID == "" {
				return fantasy.NewTextResponse("Memory refresh failed: no active session."), nil
			}
			if err := system.RefreshMemory(ctx, sessionID, true); err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf("Memory refresh failed: %s", err)), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Memory refreshed for session %s.", sessionID)), nil
		},
	)
}

// NewRecallTool creates the recall_memory agent tool.
func NewRecallTool(system *System, resolveSessionID func(context.Context) string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RecallToolName,
		"Query persistent memory that survives context compaction. Returns structured JSON records ranked by relevance. Use this before modifying files, after compaction, before architectural decisions, or when encountering familiar errors.",
		func(ctx context.Context, params RecallParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if system == nil || system.Store == nil {
				return fantasy.NewTextResponse("Memory system not initialized."), nil
			}

			limit := params.Limit
			if limit <= 0 {
				limit = 5
			}
			if limit > 20 {
				limit = 20
			}

			filter := params.Filter
			if filter == "" {
				filter = "all"
			}
			sessionID := ""
			if resolveSessionID != nil {
				sessionID = strings.TrimSpace(resolveSessionID(ctx))
			}
			if sessionID == "" {
				sessionID = strings.TrimSpace(system.SessionID)
			}
			if sessionID == "" {
				return fantasy.NewTextResponse("Memory query failed: no active session."), nil
			}

			var records []MemoryRecord
			var err error

			// If query is non-empty and filter is "all", use hybrid search
			if params.Query != "" && filter == "all" {
				records, err = system.SearchHybridForSession(ctx, sessionID, params.Query, limit)
				if err != nil || len(records) == 0 {
					records, err = system.Store.QueryRecordsBySession(ctx, sessionID, filter, limit)
				}
			} else {
				records, err = system.Store.QueryRecordsBySession(ctx, sessionID, filter, limit)
			}

			if err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf("Memory query failed: %s", err)), nil
			}

			if len(records) == 0 {
				return fantasy.NewTextResponse("No relevant memory records found."), nil
			}

			return fantasy.NewTextResponse(MarshalRecordsJSON(records)), nil
		},
	)
}

// NewSaveTool creates the save_memory agent tool.
func NewSaveTool(system *System, resolveSessionID func(context.Context) string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SaveToolName,
		"Write a critical fact to persistent memory immediately, bypassing the background pipeline. Use when the agent makes a decision so important it cannot risk the pipeline missing it. Writes synchronously at maximum salience.",
		func(ctx context.Context, params SaveParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if system == nil || system.Store == nil {
				return fantasy.NewTextResponse("Memory system not initialized."), nil
			}

			eventType := params.EventType
			if !isValidSavedMemoryEventType(eventType) {
				return fantasy.NewTextResponse(
					fmt.Sprintf("Invalid event_type %q. Must be one of: architectural_decision, negative_constraint, failure_mode, task_progress, improvement_eval, strategy_pattern", eventType),
				), nil
			}

			contentStr := normalizeSavedMemoryContent(params.Content)

			sessionID := ""
			if resolveSessionID != nil {
				sessionID = strings.TrimSpace(resolveSessionID(ctx))
			}
			if sessionID == "" {
				sessionID = strings.TrimSpace(system.SessionID)
			}
			if sessionID == "" {
				return fantasy.NewTextResponse("Failed to save memory: no active session."), nil
			}

			rec := MemoryRecord{
				SessionID:               sessionID,
				EventType:               eventType,
				Timestamp:               timeNowUnix(),
				TurnIndex:               0, // Explicit saves have no turn index
				ContentJSON:             contentStr,
				IsNegativeConstraint:    eventType == MemoryEventNegativeConstraint,
				IsArchitecturalDecision: eventType == MemoryEventArchitecturalDecision,
				IsFailureMode:           eventType == MemoryEventFailureMode,
			}

			// Maximum salience for explicit saves
			rec.Salience = 1.0

			if err := system.writeSavedRecord(ctx, sessionID, rec); err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf("Failed to save memory: %s", err)), nil
			}

			return fantasy.NewTextResponse(fmt.Sprintf("Memory saved: %s", eventType)), nil
		},
	)
}

// NewHealthTool creates the memory_health agent tool.
func NewHealthTool(system *System) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		HealthToolName,
		"Get a structured health report on persistent memory extraction, queue pressure, and storage stats.",
		func(ctx context.Context, params struct{}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if system == nil {
				return fantasy.NewTextResponse("Memory system not initialized."), nil
			}
			report, err := system.HealthSnapshot(ctx)
			if err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf("Failed to fetch memory health: %s", err)), nil
			}
			payload, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf("Failed to encode memory health: %s", err)), nil
			}
			return fantasy.NewTextResponse(string(payload)), nil
		},
	)
}

func timeNowUnix() int64 {
	return timeNow().Unix()
}
