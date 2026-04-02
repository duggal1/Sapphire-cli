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
