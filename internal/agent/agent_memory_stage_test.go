package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	"github.com/duggal1/Sapphire-cli/internal/csync"
	"github.com/duggal1/Sapphire-cli/internal/db"
	pmem "github.com/duggal1/Sapphire-cli/internal/memory"
	"github.com/stretchr/testify/require"
)

type stubPromptMemory struct{}

func (stubPromptMemory) GetProjectConstitution(context.Context, string) (string, error) {
	return "", nil
}

func (stubPromptMemory) UpsertProjectConstitution(context.Context, string, string) error {
	return nil
}

func (stubPromptMemory) GetStructuredSummary(context.Context, string) (*agentmemory.StructuredSummaryData, error) {
	return nil, nil
}

func (stubPromptMemory) CreateStructuredSummary(context.Context, string, agentmemory.StructuredSummaryData) error {
	return nil
}

func (stubPromptMemory) GetCodebaseKnowledge(context.Context, string) ([]db.CodebaseKnowledge, error) {
	return nil, nil
}

func (stubPromptMemory) UpsertCodebaseKnowledge(context.Context, db.UpsertCodebaseKnowledgeParams) error {
	return nil
}

func (stubPromptMemory) ListStructuredSummaries(context.Context, int) ([]db.StructuredSummary, error) {
	return nil, nil
}

func (stubPromptMemory) SearchCodebaseKnowledge(context.Context, string, int) ([]db.CodebaseKnowledge, error) {
	return nil, nil
}

func TestInjectTieredMemoryDefersCodebaseStatusUntilFiftyPercent(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "Stage Test")
	require.NoError(t, err)

	agent := &sessionAgent{
		sessions:                env.sessions,
		workingDir:              csync.NewValue(env.workingDir),
		codebaseIndexStatus:     func(_ context.Context, _, _ string) string { return "## CODEBASE INDEX STATUS\n- durable_graph: ready" },
		memory:                  stubPromptMemory{},
		postCompactionInjection: csync.NewMap[string, bool](),
	}

	sess.PromptTokens = 8_000
	_, err = env.sessions.Save(t.Context(), sess)
	require.NoError(t, err)
	lowHistory := agent.injectTieredMemory(t.Context(), []fantasy.Message{fantasy.NewUserMessage("hi")}, SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "Inspect the repo",
	}, 100_000)
	require.Len(t, lowHistory, 1)

	sess.PromptTokens = 60_000
	_, err = env.sessions.Save(t.Context(), sess)
	require.NoError(t, err)
	highHistory := agent.injectTieredMemory(t.Context(), []fantasy.Message{fantasy.NewUserMessage("hi")}, SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "Inspect the repo",
	}, 100_000)
	require.Len(t, highHistory, 2)
	text, ok := fantasy.AsMessagePart[fantasy.TextPart](highHistory[0].Content[0])
	require.True(t, ok)
	require.Contains(t, text.Text, "## CODEBASE INDEX STATUS")
}

func TestDetermineContextLoadStageBucketsByTenPercent(t *testing.T) {
	t.Parallel()

	require.Equal(t, pmem.ContextLoadStageCold, determineContextLoadStage(0, 100_000, false))
	require.Equal(t, pmem.ContextLoadStage10, determineContextLoadStage(10_000, 100_000, false))
	require.Equal(t, pmem.ContextLoadStage20, determineContextLoadStage(20_000, 100_000, false))
	require.Equal(t, pmem.ContextLoadStage40, determineContextLoadStage(49_999, 100_000, false))
	require.Equal(t, pmem.ContextLoadStage50, determineContextLoadStage(50_000, 100_000, false))
	require.Equal(t, pmem.ContextLoadStage50, determineContextLoadStage(1, 100_000, true))
}
