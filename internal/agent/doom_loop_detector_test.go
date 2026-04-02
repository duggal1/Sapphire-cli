package agent

import (
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/stretchr/testify/require"
)

func TestEvaluateDeterministicDoomLoopBreaksOnSevereSignals(t *testing.T) {
	t.Parallel()

	report := evaluateDeterministicDoomLoop(tools.DeterministicLoopMetrics{
		TotalCalls:      4,
		UniqueToolNames: []string{tools.AgenticEditToolName, tools.AgenticViewToolName},
		WriteCounts: map[string]int{
			"/repo/internal/runtime.go": 4,
		},
	}, workspaceDriftSummary{}, "implementation/broad/backend")

	require.True(t, shouldBreakDeterministicDoomLoop(report))
	require.Equal(t, "file_churn", report.Signals[0].Code)
	require.Equal(t, 4, report.ConsecutiveCalls)
}

func TestEvaluateDeterministicDoomLoopBreaksOnMultipleModerateSignals(t *testing.T) {
	t.Parallel()

	report := evaluateDeterministicDoomLoop(tools.DeterministicLoopMetrics{
		TotalCalls:      5,
		UniqueToolNames: []string{tools.AgenticViewToolName, tools.ToolSearchToolName},
		ReadCounts: map[string]int{
			"/repo/internal/runtime.go": 3,
		},
	}, workspaceDriftSummary{}, "design/broad/backend")

	require.True(t, shouldBreakDeterministicDoomLoop(report))
	require.Len(t, report.Signals, 2)
	require.Equal(t, "read_amnesia", report.Signals[0].Code)
	require.Equal(t, "tool_tunnel_vision", report.Signals[1].Code)
}

func TestBuildDoomLoopRecoveryCallUsesReminderTemplate(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		SessionID: "session-1",
		Prompt:    "Compare two backend designs and recommend the best fit.",
	}
	assistant := &message.Message{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "I still recommend the same design because it feels right."},
		},
	}
	err := &deterministicDoomLoopError{
		loop: deterministicDoomLoop{
			ConsecutiveCalls: 5,
			Signals: []doomLoopSignal{
				{Code: "tool_tunnel_vision", Summary: "Tool tunnel vision: only 2 unique tools were used across 5 calls."},
			},
		},
	}

	followUp, ok := buildDoomLoopRecoveryCall(planmode.DefaultSessionMode, call, err, assistant)

	require.True(t, ok)
	require.True(t, followUp.SkipUserMessage)
	require.Equal(t, 1, followUp.DoomLoopRecoveryTry)
	require.Contains(t, followUp.Prompt, "Loop Break Protocol")
	require.Contains(t, followUp.Prompt, "5 similar calls")
	require.Contains(t, followUp.Prompt, "Detected Deterministic Signals")
	require.Contains(t, followUp.Prompt, "feels right")
}

func TestBuildDoomLoopRecoveryCallHandlesRepeatedToolLoop(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		SessionID: "session-2",
		Prompt:    "Trace the architecture loop and recommend the right execution path.",
	}
	assistant := &message.Message{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "I keep repeating the same read, grep, and patch sequence."},
		},
	}
	err := &repeatedToolLoopError{
		loop: repeatedToolLoop{
			RepeatCount: 3,
			WindowSize:  9,
			ToolNames:   []string{"read", "grep", "patch"},
			PatternSize: 3,
		},
	}

	followUp, ok := buildDoomLoopRecoveryCall(planmode.DefaultSessionMode, call, err, assistant)

	require.True(t, ok)
	require.True(t, followUp.SkipUserMessage)
	require.Equal(t, 1, followUp.DoomLoopRecoveryTry)
	require.Contains(t, followUp.Prompt, "Loop Break Protocol")
	require.Contains(t, followUp.Prompt, "Detected Repeated Interaction Loop")
	require.Contains(t, followUp.Prompt, "3-step tool/result suffix pattern repeated 3 times")
	require.Contains(t, followUp.Prompt, "read, grep, patch")
}

func TestPrepareTurnToolUsageStateResetsDeterministicMetricsDuringDoomRecovery(t *testing.T) {
	t.Parallel()

	state := tools.ResetSharedToolUsageState("session-doom")
	t.Cleanup(func() {
		tools.ClearSharedToolUsageState("session-doom")
	})
	state.MarkStructuredEvidence("internal/runtime.go")
	state.RecordDeterministicToolCall(tools.AgenticViewToolName)
	state.RecordDeterministicRead("/repo/internal/runtime.go")

	reused := prepareTurnToolUsageState(SessionAgentCall{
		SessionID:           "session-doom",
		DoomLoopRecoveryTry: 1,
	})

	require.Same(t, state, reused)
	require.Equal(t, 1, reused.StructuredEvidenceCount())
	require.Equal(t, 0, reused.SnapshotDeterministicLoopMetrics().TotalCalls)
}
