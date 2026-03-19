package agent

import (
	"strings"
	"testing"

	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/stretchr/testify/require"
)

func TestParseStructuredSummaryDataUsesSessionTodosAsSourceOfTruth(t *testing.T) {
	t.Parallel()

	data, err := parseStructuredSummaryData(`{"decisions":[],"file_changes":[],"failure_modes":[],"dependency_graph":[],"todo_states":[{"content":"stale","status":"pending","dependencies":[]}]}`, []session.Todo{
		{Content: "Fix Gemini signatures", Status: session.TodoStatusInProgress},
		{Content: "Persist handoff state", Status: session.TodoStatusPending},
	})
	require.NoError(t, err)
	require.Len(t, data.TodoStates, 2)
	require.Equal(t, "Fix Gemini signatures", data.TodoStates[0].Content)
	require.Equal(t, string(session.TodoStatusInProgress), data.TodoStates[0].Status)
	require.Equal(t, "Persist handoff state", data.TodoStates[1].Content)
}

func TestBuildSessionContinuityInjectionIncludesStructuredState(t *testing.T) {
	t.Parallel()

	injection := buildSessionContinuityInjection(&agentmemory.StructuredSummaryData{
		Decisions: []agentmemory.Decision{
			{
				File:      "/Users/harshitduggal/Desktop/sapphire-cli/internal/llm/provider/gemini/google.go",
				Decision:  "Preserve first-step Gemini thought signatures exactly.",
				Rationale: "Gemini validates first function calls in the current turn.",
			},
		},
		FileChanges: []agentmemory.FileChange{
			{
				File:           "/Users/harshitduggal/Desktop/sapphire-cli/internal/message/content.go",
				SemanticChange: "Stopped mutating every reasoning block during streaming updates.",
			},
		},
		FailureModes: []agentmemory.FailureMode{
			{
				Issue:      "Gemini returned corrupted thought signature errors.",
				Resolution: "Keep signatures on the original function-call part and preserve tool-call order.",
			},
		},
		TodoStates: []agentmemory.TodoState{
			{Content: "Fix Gemini signatures", Status: "in_progress"},
		},
	}, nil)

	require.Contains(t, injection, "## SESSION CONTINUITY")
	require.Contains(t, injection, "Fix Gemini signatures")
	require.Contains(t, injection, "Preserve first-step Gemini thought signatures exactly.")
	require.Contains(t, injection, "internal/message/content.go")
	require.Contains(t, injection, "Gemini returned corrupted thought signature errors.")
}

func TestBuildSessionContinuityInjectionFallsBackToSessionTodos(t *testing.T) {
	t.Parallel()

	injection := buildSessionContinuityInjection(nil, []session.Todo{
		{Content: "Carry session context across model switches", Status: session.TodoStatusPending},
	})

	require.True(t, strings.Contains(injection, "Carry session context across model switches"))
	require.True(t, strings.Contains(injection, "[pending]"))
}
