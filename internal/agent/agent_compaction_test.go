package agent

import (
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/stretchr/testify/require"
)

func TestBuildCompactionContinuationCallUsesContinuationPromptForPartialText(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{Prompt: "Fix the failing request"}
	assistant := &message.Message{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "I found the root cause and started applying the fix"},
		},
	}

	continued := buildCompactionContinuationCall(call, assistant, "")
	require.Contains(t, continued.Prompt, "continue from where it stopped")
	require.Contains(t, continued.Prompt, "Fix the failing request")
	require.Contains(t, continued.Prompt, "I found the root cause")
}

func TestBuildCompactionContinuationCallTrimsPartialTail(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{Prompt: "Continue"}
	assistant := &message.Message{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("x", 1500)},
		},
	}

	continued := buildCompactionContinuationCall(call, assistant, "")
	require.LessOrEqual(t, len(continued.Prompt), len(call.Prompt)+1600)
	require.NotContains(t, continued.Prompt, strings.Repeat("x", 1300))
}

func TestBuildCompactionContinuationCallCarriesResumePointID(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{Prompt: "Continue the fix"}
	continued := buildCompactionContinuationCall(call, nil, "resume-123")

	require.Equal(t, "resume-123", continued.ResumePointID)
	require.Contains(t, continued.Prompt, "durable boot packet")
}

func TestBuildTodoReconciliationCallCreatesHiddenFollowUp(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		SessionID: "session-1",
		Prompt:    "Fix the bug",
	}

	followUp := buildTodoReconciliationCall(call)

	require.True(t, followUp.SkipUserMessage)
	require.Equal(t, 1, followUp.TodoReconcileTry)
	require.Contains(t, followUp.Prompt, "reconcile the live todo list")
	require.Contains(t, followUp.Prompt, "every retained item ends completed")
}

func TestBuildStructuredBlockRepairCallCreatesHiddenFollowUp(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		SessionID: "session-1",
		Prompt:    "Plan the architecture changes",
	}

	followUp := buildStructuredBlockRepairCall(planmode.PlanMode, call)

	require.True(t, followUp.SkipUserMessage)
	require.Equal(t, 1, followUp.StructuredTry)
	require.Contains(t, followUp.Prompt, "still in Plan mode")
	require.Contains(t, followUp.Prompt, "<proposed_plan>")
}

func TestBuildCompletionGuardrailRecoveryCallCreatesHiddenEvidenceFollowUp(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		SessionID: "session-1",
		Prompt:    "Architecture task only. Compare two backend designs and recommend the best fit.",
		LearnedToolPolicy: tools.LearnedToolPolicy{
			TaskFamily:          "design/broad/backend",
			RequireContextRead:  true,
			RequireExplicitPlan: true,
		},
	}
	assistant := &message.Message{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "I recommend Approach A because it feels cleaner."},
		},
	}

	followUp, ok := buildCompletionGuardrailRecoveryCall(planmode.DefaultSessionMode, call, &tools.TurnGuardrailError{
		Title:   "More Evidence Required",
		Message: "This broad turn completed without enough repository evidence. Use structured discovery plus at least one real code read before concluding, delegating, or editing.",
	}, assistant)

	require.True(t, ok)
	require.True(t, followUp.SkipUserMessage)
	require.Equal(t, 1, followUp.CompletionGuardrailTry)
	require.Contains(t, followUp.Prompt, "tool_search, rg_files, or rg")
	require.Contains(t, followUp.Prompt, "agentic_view, view, or single_view")
	require.Contains(t, followUp.Prompt, "update_plan")
	require.Contains(t, followUp.Prompt, "Approach A")
}

func TestBuildCompletionGuardrailRecoveryCallCreatesRepoGroundingFollowUp(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		SessionID: "session-1",
		Prompt:    "Research the repository broadly and recommend the best runtime design.",
		LearnedToolPolicy: tools.LearnedToolPolicy{
			TaskFamily: "research/broad/backend",
		},
	}

	followUp, ok := buildCompletionGuardrailRecoveryCall(planmode.DefaultSessionMode, call, &tools.TurnGuardrailError{
		Title:   "Repo Grounding Failed",
		Message: "This turn made repository-grounding claims that do not exist in the codebase: internal/platform/runtime.go -> NewRuntimeConfig. Re-read the cited files and ground the answer in real code before finishing.",
	}, nil)

	require.True(t, ok)
	require.Contains(t, followUp.Prompt, "do not exist in the codebase")
	require.Contains(t, followUp.Prompt, "Remove or correct")
}

func TestBuildCompletionGuardrailRecoveryCallSkipsInitializationFamilies(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		SessionID: "session-1",
		Prompt:    "Initialize the repository thoroughly.",
		LearnedToolPolicy: tools.LearnedToolPolicy{
			TaskFamily: "initialize/broad/codebase",
		},
	}

	_, ok := buildCompletionGuardrailRecoveryCall(planmode.DefaultSessionMode, call, &tools.TurnGuardrailError{
		Title:   "More Evidence Required",
		Message: "This broad turn completed without enough repository evidence.",
	}, nil)

	require.False(t, ok)
}

func TestBuildCompletionGuardrailRecoveryCallHonorsRetryBudget(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		SessionID:              "session-1",
		Prompt:                 "Architecture task only. Compare two backend designs.",
		CompletionGuardrailTry: maxCompletionGuardrailRecoveryAttempts,
		LearnedToolPolicy: tools.LearnedToolPolicy{
			TaskFamily: "design/broad/backend",
		},
	}

	_, ok := buildCompletionGuardrailRecoveryCall(planmode.DefaultSessionMode, call, &tools.TurnGuardrailError{
		Title:   "More Evidence Required",
		Message: "This broad turn completed without enough repository evidence.",
	}, nil)

	require.False(t, ok)
}

func TestPrepareTurnToolUsageStateReusesSharedStateDuringCompletionRecovery(t *testing.T) {
	t.Parallel()

	state := tools.ResetSharedToolUsageState("session-recovery")
	t.Cleanup(func() {
		tools.ClearSharedToolUsageState("session-recovery")
	})
	state.MarkStructuredEvidence("cmd/api/main.go")

	reused := prepareTurnToolUsageState(SessionAgentCall{
		SessionID:              "session-recovery",
		CompletionGuardrailTry: 1,
	})

	require.Same(t, state, reused)
	require.Equal(t, 1, reused.StructuredEvidenceCount())
}

func TestCompleteSingleTrailingInProgressTodo(t *testing.T) {
	t.Parallel()

	todos := []session.Todo{
		{Content: "Inspect runtime", Status: session.TodoStatusCompleted},
		{Content: "Wire reconciliation", Status: session.TodoStatusInProgress, ActiveForm: "Wiring reconciliation"},
	}

	updated, changed := completeSingleTrailingInProgressTodo(todos)

	require.True(t, changed)
	require.Equal(t, session.TodoStatusCompleted, updated[1].Status)
	require.Empty(t, updated[1].ActiveForm)
}

func TestCompleteAllIncompleteTodos(t *testing.T) {
	t.Parallel()

	todos := []session.Todo{
		{Content: "Inspect runtime", Status: session.TodoStatusCompleted},
		{Content: "Wire reconciliation", Status: session.TodoStatusInProgress, ActiveForm: "Wiring reconciliation"},
		{Content: "Verify tests", Status: session.TodoStatusPending},
	}

	updated, changed := completeAllIncompleteTodos(todos)

	require.True(t, changed)
	require.Equal(t, session.TodoStatusCompleted, updated[1].Status)
	require.Equal(t, session.TodoStatusCompleted, updated[2].Status)
	require.Empty(t, updated[1].ActiveForm)
}
