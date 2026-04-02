package agent

import (
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

func TestBuildHeadlessWatchdogBudgetForBroadDesign(t *testing.T) {
	t.Setenv("SAPPHIRE_NON_INTERACTIVE", "1")

	budget := buildHeadlessWatchdogBudget(planmode.DefaultSessionMode, SessionAgentCall{
		LearnedToolPolicy: tools.LearnedToolPolicy{TaskFamily: "design/broad/backend"},
	})

	require.True(t, budget.Enabled)
	require.Equal(t, "design/broad/backend", budget.TaskFamily)
	require.Equal(t, headlessAnalysisHardBudget+headlessAnalysisWatchdogGrace, budget.Timeout)
}

func TestShouldWatchdogFinalizeAnalysis(t *testing.T) {
	t.Parallel()

	text := "Option A keeps cmd/api thin while Option B rewrites the boundary directly. Compared against the current package structure, Option A is the better repo fit because it lowers migration cost and blast radius. I validated the recommendation against the current package structure and listed the trade-offs with rollback notes.\n\nA\nB\nC\nD\nE\nF\n"
	require.True(t, shouldWatchdogFinalizeAnalysis("design/broad/backend", text))
	require.False(t, shouldWatchdogFinalizeAnalysis("implementation/broad/backend", text))
}

func TestInferHeadlessWatchdogPhase(t *testing.T) {
	t.Parallel()

	longAnalysis := "Option A keeps cmd/api thin while Option B rewrites the boundary directly. Compared against the current package structure, Option A is the better repo fit because it lowers migration cost and blast radius. I validated the recommendation against the current package structure and listed the trade-offs with rollback notes.\n\nA\nB\nC\nD\nE\nF\n"

	require.Equal(t, string(headlessPhaseRead), inferHeadlessWatchdogPhase("design/broad/backend", ""))
	require.Equal(t, string(headlessPhaseClose), inferHeadlessWatchdogPhase("design/broad/backend", longAnalysis))
	require.Equal(t, string(headlessPhaseExecute), inferHeadlessWatchdogPhase("implementation/broad/backend", "partial implementation draft"))
	require.Equal(t, string(headlessPhaseExecute), inferHeadlessWatchdogPhase("initialize/broad/codebase", "partial init draft"))
}

func TestHeadlessWatchdogTimeoutErrorMatchesSentinel(t *testing.T) {
	t.Parallel()

	err := &headlessWatchdogTimeoutError{TaskFamily: "design/broad/backend"}
	require.ErrorIs(t, err, ErrHeadlessWatchdogTimeout)
}
