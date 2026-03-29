package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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

type ContextLoadStage int

const (
	ContextLoadStageCold ContextLoadStage = 0
	ContextLoadStage10   ContextLoadStage = 10
	ContextLoadStage20   ContextLoadStage = 20
	ContextLoadStage30   ContextLoadStage = 30
	ContextLoadStage40   ContextLoadStage = 40
	ContextLoadStage50   ContextLoadStage = 50
)

// System is the top-level entry point for the persistent memory system.
// It coordinates the Store, Pipeline, Extractor, and Tools.
type System struct {
	Store           *Store
	Pipeline        *Pipeline
	Extractor       MemoryExtractor
	Embedder        Embedder
	SessionID       string
	History         *sessionHistoryManager
	MemoryFile      *memoryFileManager
	repoScanSeen    sync.Map
	checkpointDone  bool
	maxRecallTokens int
}

// NewSystem initializes the full memory system for a session.
func NewSystem(ctx context.Context, sessionID string, cfg Config) (*System, error) {
	store, err := NewStore(cfg.DataDir, sessionID, cfg.ProjectRoot)
	if err != nil {
		slog.Warn("memory: failed to create structured store", "error", err)
		return nil, nil
	}

	history, err := newSessionHistoryManager(cfg.DataDir, cfg.ProjectRoot)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("memory: create session history: %w", err)
	}

	fallback := NewFallbackExtractor()
	var extractor MemoryExtractor
	if cfg.APIKey != "" {
		model := cfg.ExtractionModel
		if model == "" {
			model = "gemini-3-flash-preview"
		}
		extractor, err = NewExtractor(cfg.APIKey, model, cfg.ProjectRoot)
		if err != nil {
			slog.Warn("memory: failed to create extractor, continuing with fallback", "error", err)
		}
	} else {
		slog.Debug("memory: no API key, model extraction disabled; local history memory remains enabled")
	}

	var embedder Embedder
	if cfg.APIKey != "" {
		embedModel := cfg.EmbeddingModel
		if embedModel == "" {
			embedModel = DefaultEmbeddingModel
		}
		embedDims := cfg.EmbeddingDims
		if embedDims <= 0 {
			embedDims = DefaultEmbeddingDimensions
		}
		embedder, err = NewGeminiEmbedder(cfg.APIKey, embedModel, embedDims)
		if err != nil {
			slog.Warn("memory: failed to create embedder, semantic retrieval disabled", "error", err)
			embedder = nil
		}
	}

	_ = EnsureMistakeProtocol(cfg.ProjectRoot)

	pipeline := NewPipeline(store, extractor, fallback, embedder, cfg.ProjectRoot)
	pipeline.Start(ctx)

	memoryFile, err := newMemoryFileManager(cfg.DataDir, cfg.ProjectRoot)
	if err != nil {
		pipeline.Stop()
		store.Close()
		history.Close()
		return nil, fmt.Errorf("memory: create memory file manager: %w", err)
	}

	system := &System{
		Store:           store,
		Pipeline:        pipeline,
		Extractor:       extractor,
		Embedder:        embedder,
		SessionID:       sessionID,
		History:         history,
		MemoryFile:      memoryFile,
		maxRecallTokens: cfg.MaxRecallTokens,
	}
	pipeline.SetSaveArchitecturalDecision(system.saveArchitecturalDecisionFromMistake)
	return system, nil
}

// Close stops the pipeline and closes the store.
func (s *System) Close() {
	if s == nil {
		return
	}
	if s.Pipeline != nil {
		s.Pipeline.Stop()
	}
	if s.History != nil {
		_ = s.History.Close()
	}
	if s.Store != nil {
		_ = s.Store.Close()
	}
}

// PushToolResult pushes a tool result event to the extraction pipeline.
func (s *System) PushToolResult(sessionID string, turnIndex int, toolName, rawInput, rawOutput string) {
	if s == nil {
		return
	}
	if s.Pipeline != nil {
		// Combine input and output for extraction context
		raw := fmt.Sprintf("Tool: %s\nInput: %s\nOutput: %.2000s", toolName, rawInput, rawOutput)
		s.Pipeline.Push(ExtractionEvent{
			SessionID: sessionID,
			TurnIndex: turnIndex,
			EventType: toolName,
			RawSource: raw,
		})
	}
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
	return s.BuildContextInjectionForSession(ctx, s.SessionID, maxContextTokens)
}

// BuildContextInjectionForSession assembles the memory block for one session.
func (s *System) BuildContextInjectionForSession(ctx context.Context, sessionID string, maxContextTokens int) string {
	return s.BuildContextInjectionForSessionAtStage(ctx, sessionID, maxContextTokens, ContextLoadStage50)
}

// BuildContextInjectionForSessionAtStage assembles a progressively larger memory block
// as the active model context fills up.
func (s *System) BuildContextInjectionForSessionAtStage(ctx context.Context, sessionID string, maxContextTokens int, stage ContextLoadStage) string {
	if s == nil || s.Store == nil {
		return ""
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(s.SessionID)
	}
	if sessionID == "" {
		return ""
	}
	if stage < ContextLoadStage10 {
		return ""
	}

	budget := s.resolveMemoryBudgetForStage(maxContextTokens, stage)
	if budget <= 0 {
		return ""
	}
	remaining := budget

	var sb strings.Builder

	// 1. Project Constitution — core + recent decisions
	if s.MemoryFile != nil && remaining > 0 {
		if memoryFileContent, err := s.MemoryFile.Read(); err == nil && memoryFileContent != "" {
			memoryFileContent = trimMemoryContentForStage(memoryFileContent, stage)
			block, used := fitBlockToBudget("persistent_memory_map", memoryFileContent, remaining)
			if used > 0 {
				sb.WriteString(block)
				remaining -= used
			}
		}
	}

	// 2. Project Constitution — core + recent decisions
	if stage >= ContextLoadStage20 && remaining > 0 {
		core, err := s.Store.GetConstitution(ctx)
		if err != nil {
			core = ""
		}
		if len(core) > 1024 {
			core = core[:1024]
		}

		recentDecisions, err := s.Store.QueryRecordsBySession(ctx, sessionID, MemoryFilterArchitectural, 20)
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
	}

	// 3. Proven strategy patterns — validated reusable tactics
	if stage >= ContextLoadStage20 && remaining > 0 {
		strategies, err := s.Store.QueryRecordsBySession(ctx, sessionID, MemoryFilterStrategies, 8)
		if err == nil && len(strategies) > 0 {
			block, used := buildRecordBlock("persistent_memory_strategies",
				"## Proven Strategies (validated reusable tactics)\n",
				strategies,
				remaining,
			)
			if used > 0 {
				sb.WriteString(block)
				remaining -= used
			}
		}
	}

	// 4. Negative Constraints — high priority
	if stage >= ContextLoadStage30 && remaining > 0 {
		constraints, err := s.Store.GetNegativeConstraintsBySession(ctx, sessionID)
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

	// 5. Improvement evals — focused probes learned from failures
	if stage >= ContextLoadStage30 && remaining > 0 {
		evals, err := s.Store.QueryRecordsBySession(ctx, sessionID, MemoryFilterEvals, 8)
		if err == nil && len(evals) > 0 {
			block, used := buildRecordBlock("persistent_memory_evals",
				"## Validated Improvement Probes (rerun these before trusting the lesson)\n",
				evals,
				remaining,
			)
			if used > 0 {
				sb.WriteString(block)
				remaining -= used
			}
		}
	}

	// 6. Top-K Relevant Records by retrieval score
	if stage >= ContextLoadStage40 && remaining > 0 {
		records, err := s.Store.QueryRecordsBySession(ctx, sessionID, MemoryFilterAll, 15)
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

	// 7. Latest Compaction Checkpoint
	if stage >= ContextLoadStage50 && remaining > 0 {
		checkpoint, err := s.Store.GetLatestCheckpointBySession(ctx, sessionID)
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

func (s *System) resolveMemoryBudgetForStage(maxContextTokens int, stage ContextLoadStage) int {
	fullBudget := s.resolveMemoryBudget(maxContextTokens)
	switch {
	case stage < ContextLoadStage10:
		return 0
	case stage < ContextLoadStage20:
		return min(max(fullBudget/3, 180), 600)
	case stage < ContextLoadStage30:
		return min(max(fullBudget/2, 320), 900)
	case stage < ContextLoadStage40:
		return min(max((fullBudget*2)/3, 500), 1400)
	case stage < ContextLoadStage50:
		return min(max((fullBudget*4)/5, 700), 2000)
	default:
		return fullBudget
	}
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
	return s.Store.SearchHybridBySession(ctx, s.SessionID, query, limit, s.Embedder)
}

// SearchHybridForSession performs hybrid retrieval scoped to one session.
func (s *System) SearchHybridForSession(ctx context.Context, sessionID, query string, limit int) ([]MemoryRecord, error) {
	if s == nil || s.Store == nil {
		return nil, nil
	}
	return s.Store.SearchHybridBySession(ctx, sessionID, query, limit, s.Embedder)
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
	recordCount, _ := s.Store.CountRecordsBySession(ctx, s.SessionID)
	topSalience, _ := s.Store.TopSalienceBySession(ctx, s.SessionID, 5)
	checkpointAge, checkpointUnix, _ := s.Store.LatestCheckpointAgeSecondsBySession(ctx, s.SessionID)
	deadLetterCount, _ := s.Store.DeadLetterCountBySession(ctx, s.SessionID)

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

func (s *System) RecordUserTurn(ctx context.Context, sessionID, prompt string) {
	if s == nil || s.History == nil {
		return
	}
	_ = s.History.RecordUserPrompt(ctx, sessionID, prompt)
}

func (s *System) RecordAssistantTurn(ctx context.Context, sessionID, content string) {
	if s == nil || s.History == nil || strings.TrimSpace(content) == "" {
		return
	}
	_ = s.History.RecordAssistantResponse(ctx, sessionID, content)
}

func (s *System) RecordToolCall(ctx context.Context, sessionID, toolName, input string) {
	if s == nil || s.History == nil || strings.TrimSpace(toolName) == "" {
		return
	}
	_ = s.History.RecordToolCall(ctx, sessionID, toolName, input)
}

func (s *System) RecordToolResult(ctx context.Context, sessionID, toolName, output string, isError bool) {
	if s == nil || s.History == nil || strings.TrimSpace(toolName) == "" || strings.TrimSpace(output) == "" {
		return
	}
	_ = s.History.RecordToolResult(ctx, sessionID, toolName, output, isError)
	if s.MemoryFile != nil && s.shouldRefreshAfterRepoScan(sessionID, toolName) {
		go func(memoryFile *memoryFileManager, history *sessionHistoryManager, store *Store, sessionID string) {
			refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = memoryFile.MaybeRefresh(refreshCtx, sessionID, history, store, true)
		}(s.MemoryFile, s.History, s.Store, sessionID)
	}
}

func (s *System) RecordSavedMemory(ctx context.Context, sessionID, label, content string) {
	if s == nil || s.History == nil || strings.TrimSpace(content) == "" {
		return
	}
	_ = s.History.RecordDecision(ctx, sessionID, label, content)
	if s.MemoryFile != nil {
		_ = s.MemoryFile.MaybeRefresh(ctx, sessionID, s.History, s.Store, true)
	}
}

func (s *System) MarkSessionComplete(ctx context.Context, sessionID string) {
	if s == nil {
		return
	}
	if s.History != nil {
		_ = s.History.MarkSessionComplete(ctx, sessionID)
	}
	if s.MemoryFile != nil && s.History != nil {
		_ = s.MemoryFile.MaybeRefresh(ctx, sessionID, s.History, s.Store, true)
	}
}

func (s *System) RefreshMemory(ctx context.Context, sessionID string, force bool) error {
	if s == nil || s.MemoryFile == nil || s.History == nil {
		return nil
	}
	return s.MemoryFile.MaybeRefresh(ctx, sessionID, s.History, s.Store, force)
}

func (s *System) writeSavedRecord(ctx context.Context, sessionID string, rec MemoryRecord) error {
	if s == nil {
		return nil
	}
	if err := s.WriteRecord(ctx, rec); err != nil {
		return err
	}
	if rec.IsArchitecturalDecision {
		if err := s.persistArchitecturalDecisionToConstitution(ctx, rec.ContentJSON); err != nil {
			return err
		}
	}
	s.RecordSavedMemory(ctx, sessionID, rec.EventType, rec.ContentJSON)
	return nil
}

func (s *System) persistArchitecturalDecisionToConstitution(ctx context.Context, contentJSON string) error {
	if s == nil || s.Store == nil {
		return nil
	}
	decision := extractArchitecturalDecisionText(contentJSON)
	if decision == "" {
		return nil
	}
	existing, err := s.Store.GetConstitution(ctx)
	if err != nil {
		return err
	}
	merged := appendConstitutionDecision(existing, decision)
	if merged == existing {
		return nil
	}
	return s.Store.UpsertConstitution(ctx, merged)
}

func extractArchitecturalDecisionText(contentJSON string) string {
	contentJSON = strings.TrimSpace(contentJSON)
	if contentJSON == "" {
		return ""
	}
	contentJSON = normalizeSavedMemoryContent(json.RawMessage(contentJSON))
	var payload any
	if err := json.Unmarshal([]byte(contentJSON), &payload); err != nil {
		return contentJSON
	}
	return extractArchitecturalDecisionValue(payload)
}

func extractArchitecturalDecisionValue(payload any) string {
	switch value := payload.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		for _, key := range []string{"decision", "prevention_rule", "rule", "value"} {
			if text := extractArchitecturalDecisionValue(value[key]); text != "" {
				return text
			}
		}
	case []any:
		buf := make([]byte, 0, len(value))
		for _, item := range value {
			number, ok := item.(float64)
			if !ok || number < 0 || number > 255 {
				return ""
			}
			buf = append(buf, byte(number))
		}
		return extractArchitecturalDecisionText(string(buf))
	}
	return ""
}

func normalizeSavedMemoryContent(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return `{"value":""}`
	}
	var decoded []byte
	if err := json.Unmarshal(trimmed, &decoded); err == nil && len(decoded) > 0 {
		trimmed = bytes.TrimSpace(decoded)
	}
	if json.Valid(trimmed) {
		return string(trimmed)
	}
	return fmt.Sprintf(`{"value": %q}`, strings.TrimSpace(string(trimmed)))
}

func (s *System) saveArchitecturalDecisionFromMistake(ctx context.Context, sessionID string, decision ArchitecturalDecision) error {
	if s == nil {
		return nil
	}
	payload, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	return s.writeSavedRecord(ctx, sessionID, MemoryRecord{
		SessionID:               strings.TrimSpace(sessionID),
		EventType:               "architectural_decision",
		Timestamp:               timeNowUnix(),
		TurnIndex:               0,
		Salience:                1.0,
		ContentJSON:             string(payload),
		IsArchitecturalDecision: true,
	})
}

func (s *System) shouldRefreshAfterRepoScan(sessionID, toolName string) bool {
	sessionID = strings.TrimSpace(sessionID)
	toolName = strings.TrimSpace(toolName)
	if sessionID == "" || toolName == "" {
		return false
	}
	switch toolName {
	case "ls", "glob", "grep", "view", "single_view", "agentic_view", "index_codebase":
	default:
		return false
	}
	if _, loaded := s.repoScanSeen.LoadOrStore(sessionID, true); loaded {
		return false
	}
	return true
}
