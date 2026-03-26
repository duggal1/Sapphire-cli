package agent

import (
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
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
