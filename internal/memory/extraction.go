package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

// ExtractionResult represents the structured output from the extraction model.
type ExtractionResult struct {
	ArchitecturalDecisions []ArchitecturalDecision `json:"architectural_decisions"`
	FilesModified          []FileModified          `json:"files_modified"`
	FailuresEncountered    []FailureEncountered    `json:"failures_encountered"`
	NegativeConstraints    []NegativeConstraint    `json:"negative_constraints"`
	TaskProgress           TaskProgress            `json:"task_progress"`
	CodebaseDiscoveries    []CodebaseDiscovery     `json:"codebase_discoveries"`
}

// MemoryExtractor defines the extraction interface for persistent memory.
type MemoryExtractor interface {
	Extract(ctx context.Context, rawSource string) (*ExtractionResult, error)
	Name() string
}

// ArchitecturalDecision records a design decision and its rationale.
type ArchitecturalDecision struct {
	Decision      string   `json:"decision"`
	Rationale     string   `json:"rationale"`
	FilesAffected []string `json:"files_affected"`
}

// FileModified records a file change with semantic meaning.
type FileModified struct {
	File           string `json:"file"`
	ChangeSummary  string `json:"change_summary"`
	SemanticChange string `json:"semantic_change"`
}

// FailureEncountered records a failure, its root cause, and resolution.
type FailureEncountered struct {
	WhatFailed     string `json:"what_failed"`
	RootCause      string `json:"root_cause"`
	Resolution     string `json:"resolution"`
	TaskDomain     string `json:"task_domain,omitempty"`
	RootCauseClass string `json:"root_cause_class,omitempty"`
	DeepAnalysis   string `json:"deep_analysis,omitempty"`
	WhyThisClass   string `json:"why_this_class,omitempty"`
	Severity       string `json:"severity,omitempty"`
	PreventionRule string `json:"prevention_rule,omitempty"`
	IsNonTrivial   bool   `json:"is_non_trivial,omitempty"`
	IsIgnorable    bool   `json:"is_ignorable,omitempty"`
}

// NegativeConstraint records something that must NOT be done.
type NegativeConstraint struct {
	Constraint string `json:"constraint"`
	Reason     string `json:"reason"`
}

// TaskProgress tracks the current state of the task.
type TaskProgress struct {
	CompletedSteps []string `json:"completed_steps"`
	CurrentStep    string   `json:"current_step"`
	NextSteps      []string `json:"next_steps"`
	Blockers       []string `json:"blockers"`
}

// CodebaseDiscovery records a discovery about the codebase.
type CodebaseDiscovery struct {
	Discovery  string `json:"discovery"`
	Location   string `json:"location"`
	Importance string `json:"importance"`
}

const extractionPrompt = `You are a memory extraction system. Extract ALL of the following that are 
present in the input. Output ONLY valid JSON. No preamble. No explanation. 
No markdown fences. If a field has nothing to extract, return an empty array.
Never invent information. Only extract what is explicitly present.

{
  "architectural_decisions": [
    {"decision": "...", "rationale": "...", "files_affected": [...]}
  ],
  "files_modified": [
    {"file": "...", "change_summary": "...", "semantic_change": "..."}
  ],
  "failures_encountered": [
    {
      "what_failed": "...",
      "root_cause": "...",
      "resolution": "...",
      "task_domain": "...",
      "root_cause_class": "HALLUCINATION|CONTEXT_GAP|COMPLEXITY_OVERLOAD|WRONG_ASSUMPTION|ORCHESTRATION_FAILURE|TOOL_MISUSE",
      "deep_analysis": "...",
      "why_this_class": "...",
      "severity": "LOW|MEDIUM|HIGH|CRITICAL",
      "prevention_rule": "...",
      "is_non_trivial": true,
      "is_ignorable": false
    }
  ],
  "negative_constraints": [
    {"constraint": "...", "reason": "..."}
  ],
  "task_progress": {
    "completed_steps": [...],
    "current_step": "...",
    "next_steps": [...],
    "blockers": [...]
  },
  "codebase_discoveries": [
    {"discovery": "...", "location": "...", "importance": "high|medium|low"}
  ]
}`

func (f FailureEncountered) NormalizedRootCauseClass() MistakeRootCauseClass {
	return NormalizeMistakeRootCauseClass(f.RootCauseClass)
}

func (f FailureEncountered) NormalizedSeverity() MistakeSeverity {
	if severity := NormalizeMistakeSeverity(f.Severity); severity != "" {
		return severity
	}
	switch {
	case f.IsNonTrivial:
		return MistakeSeverityHigh
	default:
		return MistakeSeverityMedium
	}
}

func (f FailureEncountered) ShouldPersistToMistakes() bool {
	if strings.TrimSpace(f.WhatFailed) == "" && strings.TrimSpace(f.RootCause) == "" && strings.TrimSpace(f.Resolution) == "" {
		return false
	}
	if f.IsNonTrivial {
		return true
	}
	if severity := f.NormalizedSeverity(); severity == MistakeSeverityHigh || severity == MistakeSeverityCritical {
		return true
	}
	if class := f.NormalizedRootCauseClass(); class != "" && class != MistakeRootCauseHallucination {
		return true
	}
	combined := strings.ToLower(strings.Join([]string{f.WhatFailed, f.RootCause, f.Resolution}, " "))
	for _, marker := range []string{"build", "regression", "backtrack", "architect", "assumption", "failed test", "panic"} {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}

func (f FailureEncountered) PreventionRuleText() string {
	if rule := strings.TrimSpace(f.PreventionRule); rule != "" {
		return rule
	}
	return strings.TrimSpace(f.Resolution)
}

// Extractor calls the extraction model to produce structured memory from raw context.
type Extractor struct {
	client  *genai.Client
	model   string
	workDir string
}

// NewExtractor creates an Extractor using the Google GenAI SDK.
func NewExtractor(apiKey, model, workDir string) (*Extractor, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("memory: extraction model requires API key")
	}
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("memory: create genai client: %w", err)
	}
	return &Extractor{
		client:  client,
		model:   model,
		workDir: workDir,
	}, nil
}

// Extract calls the model with the raw source text and returns structured memory.
// Returns nil result on model failure (caller should handle raw fallback).
func (e *Extractor) Extract(ctx context.Context, rawSource string) (*ExtractionResult, error) {
	thinkingConfig := &genai.ThinkingConfig{
		ThinkingLevel: "low",
	}

	resp, err := e.client.Models.GenerateContent(ctx, e.model, []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				genai.NewPartFromText(rawSource),
			},
		},
	}, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromText(extractionPrompt),
			},
		},
		ThinkingConfig:  thinkingConfig,
		Temperature:     float32Ptr(0.1),
		MaxOutputTokens: 4096,
	})
	if err != nil {
		return nil, fmt.Errorf("memory: extraction call failed: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("memory: empty extraction response")
	}

	var text string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			text += part.Text
		}
	}

	// Strip markdown fences if present
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		if idx := strings.Index(text[3:], "\n"); idx != -1 {
			text = text[3+idx+1:]
		}
		if strings.HasSuffix(text, "```") {
			text = text[:len(text)-3]
		}
		text = strings.TrimSpace(text)
	}

	var result ExtractionResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("memory: parse extraction JSON: %w (raw: %.200s)", err, text)
	}

	// Hallucination guard: validate file paths
	e.validateFilePaths(&result)

	return &result, nil
}

// validateFilePaths strips any file path that doesn't exist on disk.
func (e *Extractor) validateFilePaths(result *ExtractionResult) {
	for i := range result.ArchitecturalDecisions {
		result.ArchitecturalDecisions[i].FilesAffected = filterExistingPaths(
			result.ArchitecturalDecisions[i].FilesAffected, e.workDir,
		)
	}
	for i := range result.FilesModified {
		if !pathExists(result.FilesModified[i].File, e.workDir) {
			slog.Debug("memory: stripping non-existent file path from extraction",
				"path", result.FilesModified[i].File)
			result.FilesModified[i].File = "[removed: path not found]"
		}
	}
	for i := range result.CodebaseDiscoveries {
		if result.CodebaseDiscoveries[i].Location != "" &&
			!pathExists(result.CodebaseDiscoveries[i].Location, e.workDir) {
			result.CodebaseDiscoveries[i].Location = ""
		}
	}
}

func filterExistingPaths(paths []string, workDir string) []string {
	var valid []string
	for _, p := range paths {
		if pathExists(p, workDir) {
			valid = append(valid, p)
		}
	}
	return valid
}

func pathExists(p, workDir string) bool {
	if p == "" {
		return false
	}
	// Try absolute first, then relative to workDir
	if _, err := os.Stat(p); err == nil {
		return true
	}
	joined := fmt.Sprintf("%s/%s", workDir, p)
	if _, err := os.Stat(joined); err == nil {
		return true
	}
	return false
}

// ResultToRecords converts an ExtractionResult into individual MemoryRecords.
func ResultToRecords(sessionID string, turnIndex int, result *ExtractionResult, rawSource string) []MemoryRecord {
	now := time.Now().Unix()
	var records []MemoryRecord

	for _, ad := range result.ArchitecturalDecisions {
		content, _ := json.Marshal(ad)
		records = append(records, MemoryRecord{
			SessionID:               sessionID,
			EventType:               "architectural_decision",
			Timestamp:               now,
			TurnIndex:               turnIndex,
			Salience:                0.95,
			ContentJSON:             string(content),
			RawSource:               truncate(rawSource, 500),
			IsArchitecturalDecision: true,
		})
	}

	for _, fm := range result.FilesModified {
		content, _ := json.Marshal(fm)
		records = append(records, MemoryRecord{
			SessionID:   sessionID,
			EventType:   "file_modified",
			Timestamp:   now,
			TurnIndex:   turnIndex,
			Salience:    0.7,
			ContentJSON: string(content),
			RawSource:   truncate(rawSource, 300),
		})
	}

	for _, f := range result.FailuresEncountered {
		content, _ := json.Marshal(f)
		records = append(records, MemoryRecord{
			SessionID:     sessionID,
			EventType:     "failure_mode",
			Timestamp:     now,
			TurnIndex:     turnIndex,
			Salience:      0.9,
			ContentJSON:   string(content),
			RawSource:     truncate(rawSource, 500),
			IsFailureMode: true,
		})
	}

	for _, nc := range result.NegativeConstraints {
		content, _ := json.Marshal(nc)
		records = append(records, MemoryRecord{
			SessionID:            sessionID,
			EventType:            "negative_constraint",
			Timestamp:            now,
			TurnIndex:            turnIndex,
			Salience:             1.0,
			ContentJSON:          string(content),
			RawSource:            truncate(rawSource, 300),
			IsNegativeConstraint: true,
		})
	}

	if result.TaskProgress.CurrentStep != "" || len(result.TaskProgress.CompletedSteps) > 0 {
		content, _ := json.Marshal(result.TaskProgress)
		records = append(records, MemoryRecord{
			SessionID:   sessionID,
			EventType:   "task_progress",
			Timestamp:   now,
			TurnIndex:   turnIndex,
			Salience:    0.6,
			ContentJSON: string(content),
			RawSource:   truncate(rawSource, 300),
		})
	}

	for _, cd := range result.CodebaseDiscoveries {
		content, _ := json.Marshal(cd)
		salience := 0.5
		if cd.Importance == "high" {
			salience = 0.8
		} else if cd.Importance == "medium" {
			salience = 0.65
		}
		records = append(records, MemoryRecord{
			SessionID:   sessionID,
			EventType:   "codebase_discovery",
			Timestamp:   now,
			TurnIndex:   turnIndex,
			Salience:    salience,
			ContentJSON: string(content),
			RawSource:   truncate(rawSource, 200),
		})
	}

	return records
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

func float32Ptr(v float32) *float32 { return &v }

// Name returns the extractor identifier.
func (e *Extractor) Name() string { return "gemini" }
