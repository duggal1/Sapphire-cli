package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestRecordPreparedToolUsageTracksContextEvidence(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, usage)

	recordPreparedToolUsage(ctx, ToolSearchToolName, map[string]any{"query": "auth routing"})
	recordPreparedToolUsage(ctx, AgenticViewToolName, map[string]any{"paths": []any{"internal/agent/coordinator.go"}})

	require.Equal(t, 1, usage.StructuredEvidenceCount())
	require.Equal(t, 1, usage.ReadEvidenceCount())
}

func TestExtractContextEvidenceCapturesVerificationTargets(t *testing.T) {
	t.Parallel()

	evidence := ExtractContextEvidence(ViewToolName, map[string]any{
		"paths": []any{"AGENTS.md", "internal/agent/coordinator.go"},
	})

	require.Contains(t, evidence.Read, "AGENTS.md")
	require.Contains(t, evidence.Read, "internal/agent/coordinator.go")
	require.Contains(t, evidence.Verification, "AGENTS.md")
}

func TestPrepareToolCallBlocksDelegationUntilBroadContextExists(t *testing.T) {
	t.Parallel()

	spawnTool := fantasy.NewAgentTool(
		"spawn_agent",
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:         "design/broad/backend+infra",
		Reason:             "learned route policy for recurring design/broad/backend+infra turns",
		RequireContextRead: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, NewToolUsageState())

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "spawn-without-context",
		Name:  "spawn_agent",
		Input: `{"message":"Compare both designs"}`,
	}, map[string]fantasy.AgentTool{"spawn_agent": spawnTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must gather repository evidence")
}

func TestPrepareToolCallAllowsDelegationAfterBroadContextExists(t *testing.T) {
	t.Parallel()

	spawnTool := fantasy.NewAgentTool(
		"spawn_agent",
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.MarkStructuredEvidence("backend design")
	usage.MarkReadEvidence("internal/agent/coordinator.go")

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:         "design/broad/backend+infra",
		Reason:             "learned route policy for recurring design/broad/backend+infra turns",
		RequireContextRead: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "spawn-with-context",
		Name:  "spawn_agent",
		Input: `{"message":"Compare both designs"}`,
	}, map[string]fantasy.AgentTool{"spawn_agent": spawnTool})
	require.NoError(t, err)
	require.Equal(t, "spawn_agent", prepared.Name)
}

func TestRequireContextReadCompletionBlocksBroadCompletionWithoutEvidence(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, NewToolUsageState())
	err := RequireContextReadCompletion(ctx, LearnedToolPolicy{
		TaskFamily:         "design/broad/backend+infra",
		RequireContextRead: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "without enough repository evidence")
}

func TestRequireExplicitPlanCompletionBlocksBroadCompletionWithoutPlan(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	usage.MarkStructuredEvidence("backend design")
	usage.MarkReadEvidence("internal/agent/coordinator.go")

	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, usage)
	err := RequireExplicitPlanCompletion(ctx, LearnedToolPolicy{
		TaskFamily:          "design/broad/backend+infra",
		RequireExplicitPlan: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "without publishing `update_plan`")
}
