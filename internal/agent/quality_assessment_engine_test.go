package agent

import (
	"context"
	"testing"

	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

func TestAssessTurnQualityProducesWarningForModerateSignals(t *testing.T) {
	t.Parallel()

	state := agenttools.NewToolUsageState()
	for i := 0; i < 8; i++ {
		state.RecordDeterministicToolCall(agenttools.AgenticViewToolName)
	}
	for i := 0; i < 3; i++ {
		state.RecordDeterministicRead("/repo/internal/runtime.go")
	}

	assessment := assessTurnQuality(context.Background(), nil, state, "sess", "/repo", "design/broad/backend", []string{agenttools.ToolSearchToolName})

	require.Equal(t, qualityLevelWarning, assessment.Level)
	require.GreaterOrEqual(t, assessment.Score, 4)
	require.Len(t, assessment.Signals, 2)
}

func TestAssessTurnQualityProducesCriticalForBlindWritesAndFileChurn(t *testing.T) {
	t.Parallel()

	state := agenttools.NewToolUsageState()
	for i := 0; i < 6; i++ {
		state.RecordDeterministicToolCall(agenttools.AgenticEditToolName)
		state.RecordDeterministicWrite("/repo/internal/runtime.go", true, false)
	}

	assessment := assessTurnQuality(context.Background(), nil, state, "sess", "/repo", "implementation/broad/backend", nil)

	require.Equal(t, qualityLevelCritical, assessment.Level)
	require.GreaterOrEqual(t, assessment.Score, 8)
}

func TestRenderQualitySystemReminderIncludesTargetedGuidance(t *testing.T) {
	t.Parallel()

	reminder := renderQualitySystemReminder(qualityAssessment{
		Level: qualityLevelWarning,
		Score: 5,
		Signals: []qualitySignalAssessment{
			{
				Code:        "file_churn",
				Severity:    2,
				Occurrences: 5,
				Title:       "You've modified `src/main.rs` 5 times without convergence.",
				Guidance: []string{
					"Read the full file to understand the complete state.",
					"Prefer a complete replacement instead of another incremental patch.",
				},
			},
			{
				Code:        "tool_tunnel_vision",
				Severity:    3,
				Occurrences: 8,
				Title:       "You've only used 2 unique tools across 8 calls.",
				Guidance: []string{
					"Switch to a materially different tool path.",
				},
			},
		},
		UnusedTools: []string{"tool_search", "rg_files", "update_plan"},
	})

	require.Contains(t, reminder, "<system_reminder>")
	require.Contains(t, reminder, "File Churn Detected")
	require.Contains(t, reminder, "Tool Tunnel Vision Detected")
	require.Contains(t, reminder, "Available tools you have not used yet: `tool_search`, `rg_files`, `update_plan`")
}

func TestBuildQualityAssessmentPromptDeduplicatesReminder(t *testing.T) {
	t.Parallel()

	state := agenttools.NewToolUsageState()
	for i := 0; i < 4; i++ {
		state.RecordDeterministicToolCall(agenttools.AgenticEditToolName)
		state.RecordDeterministicWrite("/repo/internal/runtime.go", false, false)
	}
	for i := 0; i < 5; i++ {
		state.RecordDeterministicToolCall(agenttools.AgenticViewToolName)
	}

	available := []string{agenttools.ToolSearchToolName, agenttools.UpdatePlanToolName}

	first := buildQualityAssessmentPrompt(context.Background(), nil, state, "sess", "/repo", "implementation/broad/backend", available)
	second := buildQualityAssessmentPrompt(context.Background(), nil, state, "sess", "/repo", "implementation/broad/backend", available)

	require.NotEmpty(t, first)
	require.Empty(t, second)
}
