package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

const (
	RecallToolName = "recall_memory"
	SaveToolName   = "save_memory"
)

// RecallParams is the input schema for the recall_memory tool.
type RecallParams struct {
	Query  string `json:"query" description:"What the agent is looking for in plain language"`
	Filter string `json:"filter,omitempty" description:"Filter type: all, negative_constraints, architectural, failures, progress. Defaults to all."`
	Limit  int    `json:"limit,omitempty" description:"Maximum number of records to return. Defaults to 5."`
}

// SaveParams is the input schema for the save_memory tool.
type SaveParams struct {
	EventType string          `json:"event_type" description:"One of: architectural_decision, negative_constraint, failure_mode, task_progress"`
	Content   json.RawMessage `json:"content" description:"Structured JSON content to persist"`
}

// NewRecallTool creates the recall_memory agent tool.
func NewRecallTool(store *Store) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RecallToolName,
		"Query persistent memory that survives context compaction. Returns structured JSON records ranked by relevance. Use this before modifying files, after compaction, before architectural decisions, or when encountering familiar errors.",
		func(ctx context.Context, params RecallParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if store == nil {
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

			var records []MemoryRecord
			var err error

			// If query is non-empty and filter is "all", try FTS first
			if params.Query != "" && filter == "all" {
				records, err = store.SearchFTS(ctx, params.Query, limit)
				if err != nil || len(records) == 0 {
					// Fallback to scored retrieval
					records, err = store.QueryRecords(ctx, filter, limit)
				}
			} else {
				records, err = store.QueryRecords(ctx, filter, limit)
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
func NewSaveTool(store *Store) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SaveToolName,
		"Write a critical fact to persistent memory immediately, bypassing the background pipeline. Use when the agent makes a decision so important it cannot risk the pipeline missing it. Writes synchronously at maximum salience.",
		func(ctx context.Context, params SaveParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if store == nil {
				return fantasy.NewTextResponse("Memory system not initialized."), nil
			}

			eventType := params.EventType
			validTypes := map[string]bool{
				"architectural_decision": true,
				"negative_constraint":    true,
				"failure_mode":           true,
				"task_progress":          true,
			}
			if !validTypes[eventType] {
				return fantasy.NewTextResponse(
					fmt.Sprintf("Invalid event_type %q. Must be one of: architectural_decision, negative_constraint, failure_mode, task_progress", eventType),
				), nil
			}

			contentStr := string(params.Content)
			if !json.Valid(params.Content) {
				contentStr = fmt.Sprintf(`{"value": %q}`, strings.TrimSpace(contentStr))
			}

			rec := MemoryRecord{
				SessionID:               store.sessionID,
				EventType:               eventType,
				Timestamp:               timeNowUnix(),
				TurnIndex:               0, // Explicit saves have no turn index
				ContentJSON:             contentStr,
				IsNegativeConstraint:    eventType == "negative_constraint",
				IsArchitecturalDecision: eventType == "architectural_decision",
				IsFailureMode:           eventType == "failure_mode",
			}

			// Maximum salience for explicit saves
			rec.Salience = 1.0

			if err := store.WriteRecord(ctx, rec); err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf("Failed to save memory: %s", err)), nil
			}

			return fantasy.NewTextResponse(fmt.Sprintf("Memory saved: %s", eventType)), nil
		},
	)
}

func timeNowUnix() int64 {
	return timeNow().Unix()
}
