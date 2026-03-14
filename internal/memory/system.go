package memory

import (
	"context"
	"encoding/json"
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
	EmbeddingModel  string
	EmbeddingDims   int
	DataDir         string
	ProjectRoot     string
	MaxRecallTokens int // Token budget for context injection
}

// System is the top-level entry point for the persistent memory system.
// It coordinates the Store, Pipeline, Extractor, and Tools.
type System struct {
	Store           *Store
	Pipeline        *Pipeline
	Extractor       MemoryExtractor
	Embedder        Embedder
	SessionID       string
	checkpointDone  bool
	maxRecallTokens int
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
		model = "gemini-3-flash-preview"
	}

	store, err := NewStore(cfg.DataDir, sessionID, cfg.ProjectRoot)
	if err != nil {
		slog.Warn("memory: failed to create store, continuing without memory", "error", err)
		return nil, nil
	}

	extractor, err := NewExtractor(cfg.APIKey, model, cfg.ProjectRoot)
	if err != nil {
		slog.Warn("memory: failed to create extractor, continuing with fallback", "error", err)
	}

	fallback := NewFallbackExtractor()

	embedModel := cfg.EmbeddingModel
	if embedModel == "" {
		embedModel = DefaultEmbeddingModel
	}
	embedDims := cfg.EmbeddingDims
	if embedDims <= 0 {
		embedDims = DefaultEmbeddingDimensions
	}
	embedder, err := NewGeminiEmbedder(cfg.APIKey, embedModel, embedDims)
	if err != nil {
		slog.Warn("memory: failed to create embedder, semantic retrieval disabled", "error", err)
		embedder = nil
	}

	pipeline := NewPipeline(store, extractor, fallback, embedder)
	pipeline.Start(ctx)

	return &System{
		Store:           store,
		Pipeline:        pipeline,
		Extractor:       extractor,
		Embedder:        embedder,
		SessionID:       sessionID,
		maxRecallTokens: cfg.MaxRecallTokens,
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
func (s *System) BuildContextInjection(ctx context.Context, maxContextTokens int) string {
	if s == nil || s.Store == nil {
		return ""
	}

	budget := s.resolveMemoryBudget(maxContextTokens)
	if budget <= 0 {
		return ""
	}
	remaining := budget

	var sb strings.Builder

	// 1. Project Constitution — core + recent decisions
	core, err := s.Store.GetConstitution(ctx)
	if err != nil {
		core = ""
	}
	if len(core) > 1024 {
		core = core[:1024]
	}

	recentDecisions, err := s.Store.QueryRecords(ctx, "architectural", 20)
	if err != nil {
		recentDecisions = nil
	}
	recentText := formatArchitecturalDecisions(recentDecisions)

	if core != "" || recentText != "" {
		var content strings.Builder
		if core != "" {
			content.WriteString("## Core Constitution (Immutable)\n")
			content.WriteString(core)
			content.WriteString("\n")
		}
		if recentText != "" {
			content.WriteString("\n## Recent Decisions (Rolling)\n")
			content.WriteString(recentText)
		}
		block, used := fitBlockToBudget("persistent_memory_constitution", content.String(), remaining)
		if used > 0 {
			sb.WriteString(block)
			remaining -= used
		}
	}

	// 2. Negative Constraints — high priority
	if remaining > 0 {
		constraints, err := s.Store.GetNegativeConstraints(ctx)
		if err == nil && len(constraints) > 0 {
			block, used := buildRecordBlock("persistent_memory_constraints",
				"## Active Negative Constraints (NEVER violate these)\n",
				constraints,
				remaining,
			)
			if used > 0 {
				sb.WriteString(block)
				remaining -= used
			}
		}
	}

	// 3. Top-K Relevant Records by retrieval score
	if remaining > 0 {
		records, err := s.Store.QueryRecords(ctx, "all", 15)
		if err == nil && len(records) > 0 {
			block, used := buildRecordBlock("persistent_memory_records",
				"## Recent Memory (ranked by salience)\n",
				records,
				remaining,
			)
			if used > 0 {
				sb.WriteString(block)
				remaining -= used
			}
		}
	}

	// 4. Latest Compaction Checkpoint
	if remaining > 0 {
		checkpoint, err := s.Store.GetLatestCheckpoint(ctx)
		if err == nil && checkpoint != "" {
			if len(checkpoint) > 1500 {
				checkpoint = checkpoint[:1500] + "..."
			}
			content := "## Last Compaction Checkpoint\n" + checkpoint
			block, used := fitBlockToBudget("persistent_memory_checkpoint", content, remaining)
			if used > 0 {
				sb.WriteString(block)
				remaining -= used
			}
		}
	}

	return sb.String()
}

func (s *System) resolveMemoryBudget(maxContextTokens int) int {
	const ratio = 0.15
	budget := 0
	if maxContextTokens > 0 {
		budget = int(float64(maxContextTokens) * ratio)
	}
	if s.maxRecallTokens > 0 && (budget == 0 || budget > s.maxRecallTokens) {
		budget = s.maxRecallTokens
	}
	if budget == 0 {
		budget = 3000
	}
	return budget
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

func trimToTokenBudget(text string, maxTokens int) string {
	if maxTokens <= 0 || text == "" {
		return ""
	}
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	if maxChars <= 1 {
		return text[:maxChars]
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return text[:maxChars-3] + "..."
}

func fitBlockToBudget(tag, content string, remainingTokens int) (string, int) {
	if remainingTokens <= 0 || content == "" {
		return "", 0
	}
	open := "<" + tag + ">\n"
	close := "\n</" + tag + ">\n\n"
	overheadTokens := estimateTokens(open + close)
	if remainingTokens <= overheadTokens {
		return "", 0
	}

	trimmed := trimToTokenBudget(content, remainingTokens-overheadTokens)
	block := open + trimmed + close
	used := estimateTokens(block)
	if used > remainingTokens {
		return "", 0
	}
	return block, used
}

func buildRecordBlock(tag, header string, records []MemoryRecord, remainingTokens int) (string, int) {
	if remainingTokens <= 0 || len(records) == 0 {
		return "", 0
	}
	open := "<" + tag + ">\n" + header
	close := "\n</" + tag + ">\n\n"
	overheadTokens := estimateTokens(open + close)
	if remainingTokens <= overheadTokens {
		return "", 0
	}

	maxContentTokens := remainingTokens - overheadTokens
	var selected []MemoryRecord
	var jsonBlock string
	for _, rec := range records {
		selected = append(selected, rec)
		candidate := MarshalRecordsJSON(selected)
		if estimateTokens(candidate) > maxContentTokens {
			selected = selected[:len(selected)-1]
			break
		}
		jsonBlock = candidate
	}
	if len(selected) == 0 || jsonBlock == "" {
		return "", 0
	}
	block := open + jsonBlock + close
	used := estimateTokens(block)
	if used > remainingTokens {
		return "", 0
	}
	return block, used
}

func formatArchitecturalDecisions(records []MemoryRecord) string {
	if len(records) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, rec := range records {
		var ad ArchitecturalDecision
		if err := json.Unmarshal([]byte(rec.ContentJSON), &ad); err != nil || ad.Decision == "" {
			sb.WriteString("- ")
			sb.WriteString(truncate(rec.ContentJSON, 180))
			sb.WriteString("\n")
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(ad.Decision)
		if ad.Rationale != "" {
			sb.WriteString(": ")
			sb.WriteString(ad.Rationale)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// BackpressureNotice returns a system message when queue pressure is high.
func (s *System) BackpressureNotice() string {
	if s == nil || s.Pipeline == nil {
		return ""
	}
	pressure := s.Pipeline.Pressure()
	if !pressure.High {
		return ""
	}
	return fmt.Sprintf(
		"## MEMORY BACKPRESSURE\nThe persistent memory queue is %.0f%% full (%d/%d). "+
			"Slow down tool calls, batch where possible, and avoid noisy tool chatter until pressure drops below 80%%.",
		pressure.Ratio*100,
		pressure.Len,
		pressure.Cap,
	)
}

// SearchHybrid performs hybrid FTS + semantic retrieval.
func (s *System) SearchHybrid(ctx context.Context, query string, limit int) ([]MemoryRecord, error) {
	if s == nil || s.Store == nil {
		return nil, nil
	}
	return s.Store.SearchHybrid(ctx, query, limit, s.Embedder)
}

// WriteRecord writes a record and stores its embedding if enabled.
func (s *System) WriteRecord(ctx context.Context, rec MemoryRecord) error {
	if s == nil || s.Store == nil {
		return nil
	}
	recordID, err := s.Store.WriteRecord(ctx, rec)
	if err != nil {
		return err
	}
	if s.Embedder != nil && recordID != 0 {
		text := recordEmbeddingText(rec)
		vectors, err := s.Embedder.EmbedDocuments(ctx, []string{text})
		if err == nil && len(vectors) == 1 {
			if err := s.Store.UpsertEmbedding(ctx, recordID, vectors[0], s.Embedder.Dimensions()); err != nil {
				slog.Debug("memory: failed to write embedding", "error", err)
			}
		}
	}
	return nil
}

// HealthSnapshot returns a structured health report for persistent memory.
func (s *System) HealthSnapshot(ctx context.Context) (map[string]any, error) {
	if s == nil || s.Store == nil {
		return map[string]any{"enabled": false}, nil
	}

	var stats PipelineStatsSnapshot
	if s.Pipeline != nil {
		stats = s.Pipeline.StatsSnapshot()
	}
	recordCount, _ := s.Store.CountRecords(ctx)
	topSalience, _ := s.Store.TopSalience(ctx, 5)
	checkpointAge, checkpointUnix, _ := s.Store.LatestCheckpointAgeSeconds(ctx)
	deadLetterCount, _ := s.Store.DeadLetterCount(ctx)

	report := map[string]any{
		"enabled":           true,
		"timestamp_unix":    time.Now().Unix(),
		"extraction":        stats,
		"record_count":      recordCount,
		"top_salience":      topSalience,
		"checkpoint_age_s":  checkpointAge,
		"checkpoint_unix":   checkpointUnix,
		"dead_letter_count": deadLetterCount,
	}
	return report, nil
}
