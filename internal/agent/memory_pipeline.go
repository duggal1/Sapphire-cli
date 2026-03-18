package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	memoryPipelineTimeout      = 30 * time.Second
	memoryExtractionMaxRetries = 2
	memoryFolderName           = ".sapphire-memory"
	memoryMDFile               = "MEMORY.md"
	memorySummaryFile          = "memory_summary.md"
	rolloutSummariesDir        = "rollout_summaries"
)

// memoryPipeline implements Codex's Phase 1 + Phase 2 memory system.
// Phase 1: Extract structured memory from a single rollout.
// Phase 2: Consolidate rollout memories into durable MEMORY.md and memory_summary.md.
type memoryPipeline struct {
	coordinator *coordinator
	mu          sync.Mutex
	active      map[string]bool // sessionID -> extraction in progress
}

// newMemoryPipeline creates a new memory pipeline.
func newMemoryPipeline(c *coordinator) *memoryPipeline {
	return &memoryPipeline{
		coordinator: c,
		active:      make(map[string]bool),
	}
}

// memoryExtractionResult holds the Phase 1 output.
type memoryExtractionResult struct {
	RolloutSummary string `json:"rollout_summary"`
	RolloutSlug    string `json:"rollout_slug"`
	RawMemory      string `json:"raw_memory"`
}

// ExtractFromRollout performs Phase 1 extraction: single rollout → structured memory.
// This is called asynchronously after sub-agent completion.
func (p *memoryPipeline) ExtractFromRollout(ctx context.Context, sessionID, rolloutText string) (*memoryExtractionResult, error) {
	if rolloutText == "" {
		return nil, nil
	}

	// Guard against concurrent extractions for the same session
	p.mu.Lock()
	if p.active[sessionID] {
		p.mu.Unlock()
		return nil, fmt.Errorf("extraction already in progress for session %s", sessionID)
	}
	p.active[sessionID] = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.active, sessionID)
		p.mu.Unlock()
	}()

	// Build the extraction prompt with rollout content
	extractionPrompt := string(memoryExtractionPrompt) + "\n\n<rollout>\n" + rolloutText + "\n</rollout>"

	// Use the coordinator's small model to run extraction
	result, err := p.runExtractionWithRetry(ctx, sessionID, extractionPrompt)
	if err != nil {
		return nil, fmt.Errorf("memory extraction failed: %w", err)
	}

	// Write rollout summary to file
	if result != nil && result.RolloutSummary != "" {
		p.writeRolloutSummary(sessionID, result)
	}

	return result, nil
}

// runExtractionWithRetry runs the extraction with retry logic.
func (p *memoryPipeline) runExtractionWithRetry(ctx context.Context, sessionID, prompt string) (*memoryExtractionResult, error) {
	var lastErr error
	for attempt := 0; attempt <= memoryExtractionMaxRetries; attempt++ {
		extractCtx, cancel := context.WithTimeout(ctx, memoryPipelineTimeout)
		result, err := p.runExtraction(extractCtx, sessionID, prompt)
		cancel()
		if err == nil {
			return result, nil
		}
		lastErr = err
		slog.Warn("Memory extraction attempt failed",
			"session_id", sessionID,
			"attempt", attempt+1,
			"error", err,
		)
	}
	return nil, lastErr
}

// runExtraction calls the model with the extraction prompt and parses JSON output.
func (p *memoryPipeline) runExtraction(ctx context.Context, sessionID, prompt string) (*memoryExtractionResult, error) {
	if p.coordinator == nil {
		return nil, fmt.Errorf("coordinator not available")
	}

	smallModel := p.coordinator.currentAgent.Model()

	agent := p.coordinator.currentAgent
	if agent == nil {
		return nil, fmt.Errorf("no agent available for memory extraction")
	}

	// Run the extraction prompt through the agent
	result, err := agent.Run(ctx, SessionAgentCall{
		SessionID:       sessionID,
		Prompt:          prompt,
		SkipUserMessage: true,
		MaxOutputTokens: int64(smallModel.CatwalkCfg.DefaultMaxTokens),
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("empty result from extraction")
	}

	// Parse the JSON response
	responseText := result.Response.Content.Text()
	var extracted memoryExtractionResult
	if err := json.Unmarshal([]byte(responseText), &extracted); err != nil {
		// Try to extract JSON from the response if it contains surrounding text
		jsonStart := strings.Index(responseText, "{")
		jsonEnd := strings.LastIndex(responseText, "}")
		if jsonStart >= 0 && jsonEnd > jsonStart {
			if err := json.Unmarshal([]byte(responseText[jsonStart:jsonEnd+1]), &extracted); err != nil {
				return nil, fmt.Errorf("failed to parse extraction JSON: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no JSON found in extraction response")
		}
	}

	return &extracted, nil
}

// writeRolloutSummary writes the rollout summary to the memory folder.
func (p *memoryPipeline) writeRolloutSummary(sessionID string, result *memoryExtractionResult) {
	if p.coordinator == nil || p.coordinator.cfg == nil {
		return
	}

	memoryRoot := p.memoryRoot()
	if memoryRoot == "" {
		return
	}

	summariesDir := filepath.Join(memoryRoot, rolloutSummariesDir)
	if err := os.MkdirAll(summariesDir, 0o755); err != nil {
		slog.Warn("Failed to create rollout summaries dir", "error", err)
		return
	}

	slug := result.RolloutSlug
	if slug == "" {
		slug = sanitizeWorktreeSlug(sessionID)
	}
	if slug == "" {
		slug = "unknown"
	}

	summaryPath := filepath.Join(summariesDir, slug+".md")
	if err := os.WriteFile(summaryPath, []byte(result.RolloutSummary), 0o644); err != nil {
		slog.Warn("Failed to write rollout summary", "path", summaryPath, "error", err)
	} else {
		slog.Info("Wrote rollout summary", "path", summaryPath)
	}

	// Also append raw memory if present
	if result.RawMemory != "" {
		rawPath := filepath.Join(memoryRoot, "raw_memories.md")
		f, err := os.OpenFile(rawPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			slog.Warn("Failed to open raw memories file", "error", err)
			return
		}
		defer f.Close()
		header := fmt.Sprintf("\n---\n## Session: %s\nTimestamp: %s\n\n", sessionID, time.Now().Format(time.RFC3339))
		if _, err := f.WriteString(header + result.RawMemory + "\n"); err != nil {
			slog.Warn("Failed to write raw memory", "error", err)
		}
	}
}

// ConsolidateMemory performs Phase 2: merge raw memories into MEMORY.md and memory_summary.md.
// This is called periodically or on session close.
func (p *memoryPipeline) ConsolidateMemory(ctx context.Context, sessionID string) error {
	memoryRoot := p.memoryRoot()
	if memoryRoot == "" {
		return fmt.Errorf("memory root not configured")
	}

	// Read existing raw memories
	rawPath := filepath.Join(memoryRoot, "raw_memories.md")
	rawData, err := os.ReadFile(rawPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to consolidate
		}
		return fmt.Errorf("failed to read raw memories: %w", err)
	}
	if len(strings.TrimSpace(string(rawData))) == 0 {
		return nil
	}

	// Read existing MEMORY.md if present
	existingMemory := ""
	memoryPath := filepath.Join(memoryRoot, memoryMDFile)
	if data, err := os.ReadFile(memoryPath); err == nil {
		existingMemory = string(data)
	}

	// Read existing memory_summary.md if present
	existingSummary := ""
	summaryPath := filepath.Join(memoryRoot, memorySummaryFile)
	if data, err := os.ReadFile(summaryPath); err == nil {
		existingSummary = string(data)
	}

	// Build the consolidation prompt
	mode := "INIT"
	if existingMemory != "" || existingSummary != "" {
		mode = "INCREMENTAL UPDATE"
	}

	consolidationContent := string(memoryConsolidationPrompt)
	consolidationContent = strings.ReplaceAll(consolidationContent, "{{ memory_root }}", memoryRoot)

	prompt := fmt.Sprintf(
		"%s\n\nMode: %s\n\n<existing_memory>\n%s\n</existing_memory>\n\n<existing_summary>\n%s\n</existing_summary>\n\n<raw_memories>\n%s\n</raw_memories>\n\nProduce updated MEMORY.md and memory_summary.md content. Output as JSON with keys: memory_md, memory_summary_md.",
		consolidationContent, mode, existingMemory, existingSummary, string(rawData),
	)

	consolidateCtx, cancel := context.WithTimeout(ctx, 2*memoryPipelineTimeout)
	defer cancel()

	agent := p.coordinator.currentAgent
	if agent == nil {
		return fmt.Errorf("no agent available for consolidation")
	}

	result, err := agent.Run(consolidateCtx, SessionAgentCall{
		SessionID:       sessionID,
		Prompt:          prompt,
		SkipUserMessage: true,
		MaxOutputTokens: 8192,
	})
	if err != nil {
		return fmt.Errorf("consolidation run failed: %w", err)
	}
	if result == nil {
		return fmt.Errorf("empty consolidation response")
	}

	// Parse consolidation output
	responseText := result.Response.Content.Text()
	var consolidated struct {
		MemoryMD        string `json:"memory_md"`
		MemorySummaryMD string `json:"memory_summary_md"`
	}
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		if err := json.Unmarshal([]byte(responseText[jsonStart:jsonEnd+1]), &consolidated); err != nil {
			return fmt.Errorf("failed to parse consolidation output: %w", err)
		}
	} else {
		return fmt.Errorf("no JSON in consolidation response")
	}

	// Write MEMORY.md
	if consolidated.MemoryMD != "" {
		if err := os.WriteFile(memoryPath, []byte(consolidated.MemoryMD), 0o644); err != nil {
			return fmt.Errorf("failed to write MEMORY.md: %w", err)
		}
		slog.Info("Updated MEMORY.md", "path", memoryPath)
	}

	// Write memory_summary.md
	if consolidated.MemorySummaryMD != "" {
		if err := os.WriteFile(summaryPath, []byte(consolidated.MemorySummaryMD), 0o644); err != nil {
			return fmt.Errorf("failed to write memory_summary.md: %w", err)
		}
		slog.Info("Updated memory_summary.md", "path", summaryPath)
	}

	return nil
}

// TriggerPostCompletion fires an async memory extraction after sub-agent completion.
func (p *memoryPipeline) TriggerPostCompletion(sessionID, rolloutText string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*memoryPipelineTimeout)
		defer cancel()
		if _, err := p.ExtractFromRollout(ctx, sessionID, rolloutText); err != nil {
			slog.Warn("Post-completion memory extraction failed",
				"session_id", sessionID,
				"error", err,
			)
		}
	}()
}

// memoryRoot returns the memory folder path.
func (p *memoryPipeline) memoryRoot() string {
	if p.coordinator == nil || p.coordinator.cfg == nil {
		return ""
	}
	workDir := p.coordinator.cfg.WorkingDir()
	if workDir == "" {
		return ""
	}
	return filepath.Join(workDir, memoryFolderName)
}

// EnsureMemoryFolder creates the memory folder structure if it doesn't exist.
func (p *memoryPipeline) EnsureMemoryFolder() error {
	memoryRoot := p.memoryRoot()
	if memoryRoot == "" {
		return fmt.Errorf("memory root not configured")
	}

	dirs := []string{
		memoryRoot,
		filepath.Join(memoryRoot, rolloutSummariesDir),
		filepath.Join(memoryRoot, "skills"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create memory dir %s: %w", dir, err)
		}
	}
	return nil
}
