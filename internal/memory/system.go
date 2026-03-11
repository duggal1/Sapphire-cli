package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// timeNow is a package-level function for testability.
var timeNow = time.Now

// Config holds memory system configuration.
type Config struct {
	ExtractionModel string // e.g., "gemini-3-flash"
	APIKey          string
	DataDir         string
	ProjectRoot     string
	MaxRecallTokens int // Token budget for context injection
}

// System is the top-level entry point for the persistent memory system.
// It coordinates the Store, Pipeline, Extractor, and Tools.
type System struct {
	Store          *Store
	Pipeline       *Pipeline
	Extractor      *Extractor
	SessionID      string
	checkpointDone bool
}

// NewSystem initializes the full memory system for a session.
// Returns nil system (not error) if API key is missing — memory is optional.
func NewSystem(ctx context.Context, sessionID string, cfg Config) (*System, error) {
	if cfg.APIKey == "" {
		slog.Debug("memory: no API key, persistent memory disabled")
		return nil, nil
	}

	model := cfg.ExtractionModel
	if model == "" {
		model = "gemini-3-flash"
	}

	store, err := NewStore(cfg.DataDir, sessionID, cfg.ProjectRoot)
	if err != nil {
		slog.Warn("memory: failed to create store, continuing without memory", "error", err)
		return nil, nil
	}

	extractor, err := NewExtractor(cfg.APIKey, model, cfg.ProjectRoot)
	if err != nil {
		slog.Warn("memory: failed to create extractor, continuing without memory", "error", err)
		store.Close()
		return nil, nil
	}

	pipeline := NewPipeline(store, extractor)
	pipeline.Start(ctx)

	return &System{
		Store:     store,
		Pipeline:  pipeline,
		Extractor: extractor,
		SessionID: sessionID,
	}, nil
}

// Close stops the pipeline and closes the store.
func (s *System) Close() {
	if s == nil {
		return
	}
	if s.Pipeline != nil {
		s.Pipeline.Stop()
	}
	if s.Store != nil {
		s.Store.Close()
	}
}

// PushToolResult pushes a tool result event to the extraction pipeline.
func (s *System) PushToolResult(sessionID string, turnIndex int, toolName, rawInput, rawOutput string) {
	if s == nil || s.Pipeline == nil {
		return
	}
	// Combine input and output for extraction context
	raw := fmt.Sprintf("Tool: %s\nInput: %s\nOutput: %.2000s", toolName, rawInput, rawOutput)
	s.Pipeline.Push(ExtractionEvent{
		SessionID: sessionID,
		TurnIndex: turnIndex,
		EventType: toolName,
		RawSource: raw,
	})
}

// ShouldRunCheckpoint returns true if a checkpoint hasn't been run yet in the current cycle.
func (s *System) ShouldRunCheckpoint() bool {
	if s == nil {
		return false
	}
	return !s.checkpointDone
}

// MarkCheckpointDone marks that a checkpoint was run.
func (s *System) MarkCheckpointDone() {
	if s != nil {
		s.checkpointDone = true
	}
}

// ResetCheckpointState resets the checkpoint state for a new compaction cycle.
func (s *System) ResetCheckpointState() {
	if s != nil {
		s.checkpointDone = false
	}
}

// RunPreCompactionCheckpoint runs a synchronous extraction pass before compaction.
func (s *System) RunPreCompactionCheckpoint(ctx context.Context, sessionID string, lastTurns string) error {
	if s == nil || s.Pipeline == nil {
		return nil
	}

	// Synchronous extraction
	if err := s.Pipeline.ExtractSync(ctx, sessionID, 0, lastTurns); err != nil {
		slog.Warn("memory: pre-compaction extraction failed", "error", err)
	}

	// Write checkpoint
	checkpointJSON, err := BuildCheckpointJSON(ctx, s.Store, sessionID)
	if err != nil {
		slog.Warn("memory: failed to build checkpoint", "error", err)
		return err
	}

	return s.Store.WriteCheckpoint(ctx, checkpointJSON)
}

// BuildContextInjection assembles the memory block for context injection.
// Returns the injection string in the priority order specified by the spec.
func (s *System) BuildContextInjection(ctx context.Context) string {
	if s == nil || s.Store == nil {
		return ""
	}

	var sb strings.Builder

	// 1. Project Constitution — always first, max 2K tokens
	constitution, err := s.Store.GetConstitution(ctx)
	if err == nil && constitution != "" {
		sb.WriteString("<persistent_memory_constitution>\n")
		if len(constitution) > 2048 {
			constitution = constitution[:2048]
		}
		sb.WriteString(constitution)
		sb.WriteString("\n</persistent_memory_constitution>\n\n")
	}

	// 2. Negative Constraints — always inject all, never decayed
	constraints, err := s.Store.GetNegativeConstraints(ctx)
	if err == nil && len(constraints) > 0 {
		sb.WriteString("<persistent_memory_constraints>\n")
		sb.WriteString("## Active Negative Constraints (NEVER violate these)\n")
		sb.WriteString(MarshalRecordsJSON(constraints))
		sb.WriteString("\n</persistent_memory_constraints>\n\n")
	}

	// 3. Top-K Relevant Records by retrieval score
	records, err := s.Store.QueryRecords(ctx, "all", 15)
	if err == nil && len(records) > 0 {
		sb.WriteString("<persistent_memory_records>\n")
		sb.WriteString("## Recent Memory (ranked by salience)\n")
		sb.WriteString(MarshalRecordsJSON(records))
		sb.WriteString("\n</persistent_memory_records>\n\n")
	}

	// 4. Latest Compaction Checkpoint
	checkpoint, err := s.Store.GetLatestCheckpoint(ctx)
	if err == nil && checkpoint != "" {
		sb.WriteString("<persistent_memory_checkpoint>\n")
		sb.WriteString("## Last Compaction Checkpoint\n")
		if len(checkpoint) > 1500 {
			checkpoint = checkpoint[:1500] + "..."
		}
		sb.WriteString(checkpoint)
		sb.WriteString("\n</persistent_memory_checkpoint>\n\n")
	}

	return sb.String()
}
