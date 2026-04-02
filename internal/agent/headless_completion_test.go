package agent

import (
	"context"
	"testing"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/stretchr/testify/require"
)

func TestBuildHeadlessCompletionBudgetForBroadDesign(t *testing.T) {
	t.Setenv("SAPPHIRE_NON_INTERACTIVE", "1")

	budget := buildHeadlessCompletionBudget(planmode.DefaultSessionMode, SessionAgentCall{
		LearnedToolPolicy: tools.LearnedToolPolicy{TaskFamily: "design/broad/backend"},
	})

	require.True(t, budget.Enabled)
	require.Equal(t, headlessAnalysisSoftBudget, budget.SoftLimit)
	require.Equal(t, headlessAnalysisHardBudget, budget.HardLimit)
}

func TestCanForceHeadlessFinalizationRequiresWritesForImplementation(t *testing.T) {
	t.Parallel()

	state := tools.NewToolUsageState()
	state.MarkStructuredEvidence("tool_search")
	state.MarkReadEvidence("/repo/internal/agent/singularity_learning.go")

	assistant := &message.Message{
		Parts: []message.ContentPart{
			message.TextContent{Text: stringsOfLength(260)},
		},
	}

	require.False(t, canForceHeadlessFinalization("implementation/broad/backend", assistant, state))

	state.RecordDeterministicWrite("/repo/internal/agent/singularity_learning.go", false, false)
	require.True(t, canForceHeadlessFinalization("implementation/broad/backend", assistant, state))
}

func TestCanForceHeadlessExecutionKickRequiresEvidenceAndPlanWithoutWrites(t *testing.T) {
	t.Parallel()

	state := tools.NewToolUsageState()
	state.MarkStructuredEvidence("tool_search")
	state.MarkReadEvidence("/repo/internal/agent/singularity_learning.go")
	state.MarkPlanPublished()
	state.Increment(tools.RunHarnessToolName)
	state.Increment(tools.ToolSearchToolName)
	state.Increment(tools.AgenticViewToolName)

	require.True(t, canForceHeadlessExecutionKick("implementation/broad/backend", state))

	state.RecordDeterministicWrite("/repo/internal/agent/singularity_learning.go", false, false)
	require.False(t, canForceHeadlessExecutionKick("implementation/broad/backend", state))
}

func TestBuildHeadlessCompletionRecoveryCallForFinalize(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		Prompt: "Design the champion/challenger lane",
		LearnedToolPolicy: tools.LearnedToolPolicy{
			TaskFamily: "design/broad/backend",
		},
	}
	assistant := &message.Message{
		Parts: []message.ContentPart{
			message.TextContent{Text: "Grounded draft answer"},
		},
	}
	followUp, ok := buildHeadlessCompletionRecoveryCall(planmode.DefaultSessionMode, call, &headlessCompletionBudgetError{
		Action:     headlessCompletionActionFinalize,
		TaskFamily: "design/broad/backend",
		Elapsed:    50 * time.Second,
	}, assistant)

	require.True(t, ok)
	require.Equal(t, 1, followUp.HeadlessCompletionTry)
	require.Contains(t, followUp.Prompt, "Do not restart discovery.")
	require.Contains(t, followUp.Prompt, "Deliver the final answer now")
}

func TestBuildHeadlessCompletionRecoveryCallForExecute(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		Prompt: "Implement the new benchmark command",
		LearnedToolPolicy: tools.LearnedToolPolicy{
			TaskFamily: "implementation/broad/backend",
		},
	}
	followUp, ok := buildHeadlessCompletionRecoveryCall(planmode.DefaultSessionMode, call, &headlessCompletionBudgetError{
		Action:     headlessCompletionActionExecute,
		TaskFamily: "implementation/broad/backend",
		Elapsed:    35 * time.Second,
	}, nil)

	require.True(t, ok)
	require.Equal(t, 1, followUp.HeadlessCompletionTry)
	require.Contains(t, followUp.Prompt, "have not crossed into execution")
	require.Contains(t, followUp.Prompt, "Move directly into the first minimal concrete implementation step")
}

func TestBuildHeadlessCompletionRecoveryCallForStructuredDiscovery(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		Prompt: "Design the champion/challenger lane",
		LearnedToolPolicy: tools.LearnedToolPolicy{
			TaskFamily: "design/broad/backend",
		},
	}
	followUp, ok := buildHeadlessCompletionRecoveryCall(planmode.DefaultSessionMode, call, &headlessCompletionBudgetError{
		Action:     headlessCompletionActionStructure,
		TaskFamily: "design/broad/backend",
		Phase:      headlessPhaseStructure,
		Elapsed:    35 * time.Second,
	}, nil)

	require.True(t, ok)
	require.Equal(t, "structure", followUp.HeadlessPhaseAtInterrupt)
	require.Contains(t, followUp.Prompt, "Perform exactly one targeted structured discovery step")
	require.Contains(t, followUp.Prompt, "Do not load skills")
}

func TestCanForceHeadlessFinalizationAllowsAnalysisClosureWithEvidence(t *testing.T) {
	t.Parallel()

	state := tools.NewToolUsageState()
	state.MarkStructuredEvidence("tool_search")
	state.MarkReadEvidence("/repo/internal/agent/singularity_learning.go")
	state.Increment(tools.RunHarnessToolName)
	state.Increment(tools.ToolSearchToolName)
	state.Increment(tools.AgenticViewToolName)

	assistant := &message.Message{
		Parts: []message.ContentPart{
			message.TextContent{Text: "Grounded benchmark recommendation with repo fit and rollback details."},
		},
	}

	require.True(t, canForceHeadlessFinalization("design/broad/backend", assistant, state))
}

func TestShouldSalvageHeadlessResultRequiresAnalysisRetryAndSubstantialDraft(t *testing.T) {
	t.Parallel()

	shortAssistant := &message.Message{
		Parts: []message.ContentPart{
			message.TextContent{Text: "too short"},
		},
	}
	require.False(t, shouldSalvageHeadlessResult("design/broad/backend", 1, shortAssistant))

	longAssistant := &message.Message{
		Parts: []message.ContentPart{
			message.TextContent{Text: "Option A keeps cmd/api thin while Option B rewrites the boundary directly. Compared against the current package structure, Option A is the better repo fit because it lowers migration cost and blast radius. I validated the recommendation against the current package structure and listed the trade-offs with rollback notes.\n\nA\nB\nC\nD\nE\nF\n"},
		},
	}
	require.True(t, shouldSalvageHeadlessResult("design/broad/backend", 1, longAssistant))
	require.False(t, shouldSalvageHeadlessResult("implementation/broad/backend", 1, longAssistant))
	require.False(t, shouldSalvageHeadlessResult("design/broad/backend", 0, longAssistant))
}

func TestDetectHeadlessCompletionPhaseRequiresStructuredDiscoveryAfterRead(t *testing.T) {
	t.Parallel()

	state := tools.NewToolUsageState()
	state.MarkReadEvidence("/repo/internal/platform/runtime.go")

	require.Equal(t, headlessPhaseStructure, detectHeadlessCompletionPhase("design/broad/backend", nil, state))
}

func TestTranslateStreamErrorConvertsHardTimeoutToReject(t *testing.T) {
	t.Parallel()

	controller := newHeadlessCompletionController(headlessCompletionBudget{
		Enabled:    true,
		SoftLimit:  10 * time.Second,
		HardLimit:  20 * time.Second,
		TaskFamily: "design/broad/backend",
	})
	controller.startedAt = time.Now().Add(-25 * time.Second)

	state := tools.NewToolUsageState()
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()

	err := controller.TranslateStreamError(ctx, context.DeadlineExceeded, &message.Message{}, state)
	var budgetErr *headlessCompletionBudgetError
	require.ErrorAs(t, err, &budgetErr)
	require.Equal(t, headlessCompletionActionReject, budgetErr.Action)
}

func stringsOfLength(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = 'a'
	}
	return string(buf)
}
