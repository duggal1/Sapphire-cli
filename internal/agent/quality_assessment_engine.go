package agent

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/filetracker"
)

type qualityLevel string

const (
	qualityLevelHealthy  qualityLevel = "healthy"
	qualityLevelWarning  qualityLevel = "warning"
	qualityLevelCritical qualityLevel = "critical"
)

type qualitySignalAssessment struct {
	Code        string
	Severity    int
	Occurrences int
	Title       string
	Guidance    []string
}

type qualityAssessment struct {
	Level       qualityLevel
	Score       int
	TaskFamily  string
	Signals     []qualitySignalAssessment
	UnusedTools []string
}

func assessTurnQuality(ctx context.Context, tracker filetracker.Service, usage *agenttools.ToolUsageState, sessionID, workingDir, taskFamily string, availableTools []string) qualityAssessment {
	metrics := agenttools.DeterministicLoopMetrics{}
	if usage != nil {
		metrics = usage.SnapshotDeterministicLoopMetrics()
	}
	drift := collectWorkspaceDrift(ctx, tracker, sessionID, workingDir)
	assessment := qualityAssessment{
		TaskFamily: strings.TrimSpace(taskFamily),
	}

	if signal, ok := assessFileChurnSignal(metrics); ok {
		assessment.add(signal)
	}
	if signal, ok := assessReadAmnesiaSignal(metrics); ok {
		assessment.add(signal)
	}
	if signal, ok := assessBlindWritingSignal(metrics); ok {
		assessment.add(signal)
	}
	if signal, ok := assessToolTunnelVisionSignal(metrics); ok {
		assessment.add(signal)
		assessment.UnusedTools = unusedStructuredTools(metrics.UniqueToolNames, availableTools)
	}
	if signal, ok := assessSolutionCreepSignal(metrics); ok {
		assessment.add(signal)
	}
	if signal, ok := assessContextStalenessSignal(drift); ok {
		assessment.add(signal)
	}

	switch {
	case assessment.Score >= 8:
		assessment.Level = qualityLevelCritical
	case assessment.Score >= 4:
		assessment.Level = qualityLevelWarning
	default:
		assessment.Level = qualityLevelHealthy
	}
	return assessment
}

func (a *qualityAssessment) add(signal qualitySignalAssessment) {
	a.Signals = append(a.Signals, signal)
	a.Score += signal.Severity
}

func assessFileChurnSignal(metrics agenttools.DeterministicLoopMetrics) (qualitySignalAssessment, bool) {
	path, count := highestCount(metrics.WriteCounts)
	switch {
	case count >= 6:
		return qualitySignalAssessment{
			Code:        "file_churn",
			Severity:    3,
			Occurrences: count,
			Title:       fmt.Sprintf("You've modified `%s` %d times without convergence.", path, count),
			Guidance: []string{
				"Read the full file to understand the complete state.",
				"Prefer a complete replacement or a smaller isolated change instead of more incremental patching.",
				"If the same design keeps failing, change the architecture instead of defending it.",
			},
		}, true
	case count >= 4:
		return qualitySignalAssessment{
			Code:        "file_churn",
			Severity:    2,
			Occurrences: count,
			Title:       fmt.Sprintf("You've modified `%s` %d times without convergence.", path, count),
			Guidance: []string{
				"Re-read the whole file before editing again.",
				"Prefer one clean, complete edit over another partial patch.",
			},
		}, true
	default:
		return qualitySignalAssessment{}, false
	}
}

func assessReadAmnesiaSignal(metrics agenttools.DeterministicLoopMetrics) (qualitySignalAssessment, bool) {
	path, count := highestCount(metrics.ReadCounts)
	switch {
	case count >= 5:
		return qualitySignalAssessment{
			Code:        "read_amnesia",
			Severity:    2,
			Occurrences: count,
			Title:       fmt.Sprintf("You've re-read `%s` %d times.", path, count),
			Guidance: []string{
				"Stop re-reading the same file in isolation and summarize the invariant you need.",
				"Expand to adjacent execution-path files before reading this file again.",
			},
		}, true
	case count >= 3:
		return qualitySignalAssessment{
			Code:        "read_amnesia",
			Severity:    1,
			Occurrences: count,
			Title:       fmt.Sprintf("You've re-read `%s` %d times.", path, count),
			Guidance: []string{
				"Capture the key invariant from this file and move to the next dependency.",
			},
		}, true
	default:
		return qualitySignalAssessment{}, false
	}
}

func assessBlindWritingSignal(metrics agenttools.DeterministicLoopMetrics) (qualitySignalAssessment, bool) {
	path, count := highestCount(metrics.BlindWriteCounts)
	if count <= 0 {
		return qualitySignalAssessment{}, false
	}
	return qualitySignalAssessment{
		Code:        "blind_writing",
		Severity:    3,
		Occurrences: count,
		Title:       fmt.Sprintf("You wrote to `%s` without reading it first.", path),
		Guidance: []string{
			"Read the file before any further write.",
			"Do not rely on stale assumptions about its current contents.",
		},
	}, true
}

func assessToolTunnelVisionSignal(metrics agenttools.DeterministicLoopMetrics) (qualitySignalAssessment, bool) {
	unique := len(metrics.UniqueToolNames)
	switch {
	case metrics.TotalCalls >= 8 && unique <= 2:
		return qualitySignalAssessment{
			Code:        "tool_tunnel_vision",
			Severity:    3,
			Occurrences: metrics.TotalCalls,
			Title:       fmt.Sprintf("You've only used %d unique tools across %d calls.", unique, metrics.TotalCalls),
			Guidance: []string{
				"Switch to a materially different tool path.",
				"Prefer structured discovery, broader reads, or a verification tool before repeating the same move.",
			},
		}, true
	case metrics.TotalCalls >= 5 && unique <= 2:
		return qualitySignalAssessment{
			Code:        "tool_tunnel_vision",
			Severity:    2,
			Occurrences: metrics.TotalCalls,
			Title:       fmt.Sprintf("You've only used %d unique tools across %d calls.", unique, metrics.TotalCalls),
			Guidance: []string{
				"Check whether a different tool would reduce uncertainty faster.",
			},
		}, true
	default:
		return qualitySignalAssessment{}, false
	}
}

func assessSolutionCreepSignal(metrics agenttools.DeterministicLoopMetrics) (qualitySignalAssessment, bool) {
	created := len(metrics.CreatedFiles)
	modified := len(metrics.ModifiedFiles)
	if created < 4 || created <= modified*3 {
		return qualitySignalAssessment{}, false
	}
	severity := 2
	if created >= 8 {
		severity = 3
	}
	return qualitySignalAssessment{
		Code:        "solution_creep",
		Severity:    severity,
		Occurrences: created,
		Title:       fmt.Sprintf("You've created %d files versus %d modified.", created, modified),
		Guidance: []string{
			"Stop adding new files unless the existing execution path genuinely cannot absorb the change.",
			"Prefer modifying the real path over scaffolding parallel structures.",
		},
	}, true
}

func assessContextStalenessSignal(drift workspaceDriftSummary) (qualitySignalAssessment, bool) {
	if !drift.HasDrift() {
		return qualitySignalAssessment{}, false
	}
	changed := append([]string{}, drift.Modified...)
	changed = append(changed, drift.Removed...)
	sort.Strings(changed)
	title := "Previously read workspace files changed externally."
	if len(changed) > 0 {
		title = "Previously read workspace files changed externally: `" + strings.Join(changed, "`, `") + "`."
	}
	return qualitySignalAssessment{
		Code:        "context_staleness",
		Severity:    3,
		Occurrences: len(changed),
		Title:       title,
		Guidance: []string{
			"Re-read the changed files before relying on earlier observations.",
			"Do not continue on cached assumptions.",
		},
	}, true
}

func renderQualitySystemReminder(assessment qualityAssessment) string {
	if assessment.Level == qualityLevelHealthy || len(assessment.Signals) == 0 {
		return ""
	}
	lines := []string{
		"<system_reminder>",
		fmt.Sprintf("Quality assessment: %s (%d).", strings.Title(string(assessment.Level)), assessment.Score),
	}
	for _, signal := range assessment.Signals {
		lines = append(lines, "")
		lines = append(lines, "### "+signalTitle(signal))
		lines = append(lines, signal.Title)
		for i, guidance := range signal.Guidance {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, guidance))
		}
		if signal.Code == "tool_tunnel_vision" && len(assessment.UnusedTools) > 0 {
			lines = append(lines, "", "Available tools you have not used yet: `"+strings.Join(assessment.UnusedTools, "`, `")+"`")
		}
	}
	if assessment.Level == qualityLevelCritical {
		lines = append(lines, "", "Break the current path now. Change the architecture, the evidence path, or the tool path before continuing.")
	}
	lines = append(lines, "</system_reminder>")
	return strings.Join(lines, "\n")
}

func signalTitle(signal qualitySignalAssessment) string {
	switch signal.Code {
	case "file_churn":
		return "File Churn Detected"
	case "read_amnesia":
		return "Read Amnesia Detected"
	case "blind_writing":
		return "Blind Writing Detected"
	case "tool_tunnel_vision":
		return "Tool Tunnel Vision Detected"
	case "solution_creep":
		return "Solution Creep Detected"
	case "context_staleness":
		return "Context Staleness Detected"
	default:
		return "Quality Signal Detected"
	}
}

func qualityReminderKey(assessment qualityAssessment) string {
	if assessment.Level == qualityLevelHealthy || len(assessment.Signals) == 0 {
		return ""
	}
	parts := []string{string(assessment.Level), fmt.Sprintf("score:%d", assessment.Score)}
	for _, signal := range assessment.Signals {
		parts = append(parts, fmt.Sprintf("%s:%d", signal.Code, signal.Occurrences))
	}
	return strings.Join(parts, "|")
}

func buildQualityAssessmentPrompt(ctx context.Context, tracker filetracker.Service, usage *agenttools.ToolUsageState, sessionID, workingDir, taskFamily string, availableTools []string) string {
	if usage == nil {
		return ""
	}
	assessment := assessTurnQuality(ctx, tracker, usage, sessionID, workingDir, taskFamily, availableTools)
	if assessment.Level == qualityLevelHealthy {
		return ""
	}
	key := qualityReminderKey(assessment)
	if !usage.ShouldEmitQualityReminder(key) {
		return ""
	}
	return renderQualitySystemReminder(assessment)
}

func highestCount(values map[string]int) (string, int) {
	bestKey := ""
	bestCount := 0
	for key, count := range values {
		if count > bestCount || (count == bestCount && bestKey != "" && key < bestKey) {
			bestKey = key
			bestCount = count
		}
	}
	return bestKey, bestCount
}

func unusedStructuredTools(used []string, availableTools []string) []string {
	usedSet := make(map[string]struct{}, len(used))
	for _, name := range used {
		usedSet[strings.TrimSpace(name)] = struct{}{}
	}
	candidates := []string{
		agenttools.RunHarnessToolName,
		agenttools.UpdatePlanToolName,
		agenttools.ToolSearchToolName,
		agenttools.RGFilesToolName,
		agenttools.RGToolName,
		agenttools.AgenticViewToolName,
		agenttools.SingleViewToolName,
		agenttools.DiagnosticsToolName,
		agenttools.BashToolName,
		"spawn_agent",
	}
	available := make(map[string]struct{}, len(availableTools))
	for _, toolName := range availableTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			continue
		}
		available[toolName] = struct{}{}
	}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := available[candidate]; !ok {
			continue
		}
		if _, ok := usedSet[candidate]; ok {
			continue
		}
		out = append(out, candidate)
	}
	return slices.Clip(out)
}
