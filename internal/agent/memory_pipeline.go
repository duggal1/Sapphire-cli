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

	"github.com/duggal1/Sapphire-cli/internal/agent/memory"
	promptpkg "github.com/duggal1/Sapphire-cli/internal/agent/prompt"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/config"
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

	// Build the extraction prompt with rollout content using the input template
	rolloutContents := rolloutText
	// In a full implementation, we would apply truncation here to match Codex's 70% budget.
	
	inputMsg := string(memoryExtractionInputPrompt)
	inputMsg = strings.ReplaceAll(inputMsg, "{{ rollout_path }}", sessionID) // Using sessionID as proxy for path
	inputMsg = strings.ReplaceAll(inputMsg, "{{ rollout_cwd }}", p.coordinator.mainWorkingDir())
	inputMsg = strings.ReplaceAll(inputMsg, "{{ rollout_contents }}", rolloutContents)

	// Use the coordinator's small model to run extraction
	result, err := p.runExtractionWithRetry(ctx, sessionID, inputMsg)
	if err != nil {
		return nil, fmt.Errorf("memory extraction failed: %w", err)
	}

	// Update extraction context with current directory
	cwd, _ := os.Getwd()
	if p.coordinator != nil {
		cwd = p.coordinator.mainWorkingDir()
	}

	// Internal metadata for consolidation
	now := time.Now().Format(time.RFC3339)

	// Enrich raw memory with metadata before writing
	if result != nil && (result.RawMemory != "" || result.RolloutSummary != "") {
		p.writeRolloutSummary(sessionID, result, cwd, now)
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

	agent := p.coordinator.currentAgent
	if agent == nil {
		return nil, fmt.Errorf("no agent available for memory extraction")
	}

	// Literal Phase 1 Extraction: Use coordinator to build a private agent
	extractionAgent, err := p.coordinator.buildAgentWithWorkingDir(ctx, &promptpkg.Prompt{}, config.Agent{}, true, p.coordinator.mainWorkingDir())
	if err != nil {
		return nil, fmt.Errorf("failed to build extraction agent: %w", err)
	}
	
	systemPromptPrefix := ""
	if cfg, ok := p.coordinator.cfg.Providers.Get(p.coordinator.currentAgent.Model().ModelCfg.Provider); ok {
		systemPromptPrefix = cfg.SystemPromptPrefix
	}
	extractionAgent.SetSystemPrompt(systemPromptPrefix + string(memoryExtractionPrompt))

	// Run the extraction against the agent
	resp, err := extractionAgent.Run(ctx, SessionAgentCall{
		SessionID:       fmt.Sprintf("memory-extraction-%s", sessionID),
		Prompt:          prompt,
		SkipUserMessage: true,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("empty response from extraction agent")
	}

	// Parse the JSON response
	responseText := resp.Response.Content.Text()
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

// writeRolloutSummary writes the rollout summary and enriches raw memory with metadata.
func (p *memoryPipeline) writeRolloutSummary(sessionID string, result *memoryExtractionResult, cwd, timestamp string) {
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

	// Literal 1:1 Filename Stem
	slug := result.RolloutSlug
	stem := memory.RolloutSummaryFileStemFromParts(sessionID, time.Now(), &slug)
	filename := stem + ".md"

	summaryPath := filepath.Join(summariesDir, filename)
	
	// Literal Rollout Summary Header
	summaryHeader := memory.FormatRolloutSummaryHeader(sessionID, timestamp, "rollout_summaries/"+filename, cwd, "")
	if err := os.WriteFile(summaryPath, []byte(summaryHeader+result.RolloutSummary), 0o644); err != nil {
		slog.Warn("Failed to write rollout summary", "path", summaryPath, "error", err)
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

		// Literal Raw Memory Entry Header
		meta := memory.FormatRawMemoryEntryHeader(sessionID, timestamp, cwd, "rollout_summaries/"+filename, filename)

		if _, err := f.WriteString(meta + strings.TrimSpace(result.RawMemory) + "\n\n"); err != nil {
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
	
	// Literal Input Selection Rendering
	inputSelection := p.renderPhase2InputSelection(rawData)
	consolidationContent = strings.ReplaceAll(consolidationContent, "{{ phase2_input_selection }}", inputSelection)

	prompt := fmt.Sprintf(
		"%s\n\nMode: %s\n\n<existing_memory>\n%s\n</existing_memory>\n\n<existing_summary>\n%s\n</existing_summary>\n\n<raw_memories>\n%s\n</raw_memories>",
		consolidationContent, mode, existingMemory, existingSummary, string(rawData),
	)
	consolidateCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// 1. Build the Memory Writing Agent using the coordinator
	// We use the same configuration as a standard sub-agent but with our custom prompt
	memoryAgent, err := p.coordinator.buildAgentWithWorkingDir(ctx, &promptpkg.Prompt{}, config.Agent{}, true, memoryRoot)
	if err != nil {
		return fmt.Errorf("failed to build memory agent: %w", err)
	}

	// Set the literal consolidation instructions
	memoryAgent.SetSystemPrompt(prompt)

	// Enforce strict WriteScope for memory folder
	if sa, ok := memoryAgent.(*sessionAgent); ok {
		sa.writeScope = tools.NewWriteScope(memoryRoot, []string{"."})
	}

	// 2. Run the consolidation session
	consolidationSessionID := fmt.Sprintf("memory-consolidation-%s-%d", sessionID, time.Now().Unix())
	userPrompt := "Proceed with memory consolidation as specified in your instructions. Update MEMORY.md and memory_summary.md based on the provided raw memories."
	
	_, err = memoryAgent.Run(consolidateCtx, SessionAgentCall{
		SessionID:       consolidationSessionID,
		Prompt:          userPrompt,
		SkipUserMessage: true,
	})
	if err != nil {
		return fmt.Errorf("memory consolidation agent failed: %w", err)
	}

	return nil
}

// renderPhase2InputSelection implements the literal rendering logic from Codex prompts.rs.
func (p *memoryPipeline) renderPhase2InputSelection(rawData []byte) string {
	// Simple version for now that captures the spirit of the literal logic.
	// In a full implementation, we would parse raw_memories.md to count added/retained.
	added := strings.Count(string(rawData), "## Thread")
	return fmt.Sprintf("- selected inputs this run: %d\n- newly added since the last successful Phase 2 run: %d\n- retained from the last successful Phase 2 run: 0\n- removed from the last successful Phase 2 run: 0\n",
		added, added)
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
