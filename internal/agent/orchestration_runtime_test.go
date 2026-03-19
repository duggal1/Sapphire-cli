package agent

import (
	"context"
	"testing"
	"time"

	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	"github.com/duggal1/Sapphire-cli/internal/db"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

type orchestrationMemoryStub struct{}

func (orchestrationMemoryStub) GetProjectConstitution(context.Context, string) (string, error) {
	return "", nil
}
func (orchestrationMemoryStub) UpsertProjectConstitution(context.Context, string, string) error {
	return nil
}
func (orchestrationMemoryStub) GetStructuredSummary(context.Context, string) (*agentmemory.StructuredSummaryData, error) {
	return &agentmemory.StructuredSummaryData{
		Decisions:   []agentmemory.Decision{{Decision: "Use mailbox handoffs", File: "internal/agent/orchestration_runtime.go", Rationale: "shared coordination state"}},
		FileChanges: []agentmemory.FileChange{{File: "internal/agent/subagent_manager.go", SemanticChange: "loads orchestration memory each turn"}},
	}, nil
}
func (orchestrationMemoryStub) CreateStructuredSummary(context.Context, string, agentmemory.StructuredSummaryData) error {
	return nil
}
func (orchestrationMemoryStub) GetCodebaseKnowledge(context.Context, string) ([]db.CodebaseKnowledge, error) {
	return nil, nil
}
func (orchestrationMemoryStub) UpsertCodebaseKnowledge(context.Context, db.UpsertCodebaseKnowledgeParams) error {
	return nil
}
func (orchestrationMemoryStub) ListStructuredSummaries(context.Context, int) ([]db.StructuredSummary, error) {
	return nil, nil
}
func (orchestrationMemoryStub) SearchCodebaseKnowledge(context.Context, string, int) ([]db.CodebaseKnowledge, error) {
	return nil, nil
}

func TestBuildSubAgentPersistentMemoryContextIncludesStateWorkAndSummary(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	coord := &coordinator{
		orchestrationStore: store,
		stateService:       agentstate.NewService(store),
		activityService:    agentactivity.NewService(store),
		memory:             orchestrationMemoryStub{},
		mailbox:            nil,
	}
	runner := &subAgentRunner{
		id:            "agent-auth",
		sessionID:     "sub-session",
		parentSession: "parent-session",
		status:        subAgentStatusRunning,
		workDir:       "/tmp/.sapphire/worktrees/agent-auth",
		assignment: subAgentAssignment{
			ID:               "work-auth",
			Title:            "Auth flow",
			Task:             "Finish auth orchestration",
			TaskKey:          "auth-flow",
			DefinitionOfDone: "tests pass",
			TestCommand:      "go test ./internal/agent",
			CreatedAt:        time.Now().UTC().Add(-5 * time.Minute),
		},
	}
	require.NoError(t, store.UpsertAgentState(ctx, orchestrationdb.AgentState{
		AgentID:       "agent-auth",
		Role:          "subagent",
		Status:        "running",
		SessionID:     "sub-session",
		WorktreePath:  runner.workDir,
		Branch:        "feature/agent-auth",
		HookBeadID:    "work-auth",
		ParentAgentID: "main:parent-session",
		LastHeartbeat: time.Now().UTC(),
		CreatedAt:     time.Now().UTC().Add(-5 * time.Minute),
		UpdatedAt:     time.Now().UTC(),
	}))
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:          "work-auth",
		Type:        "task",
		Title:       "Auth flow",
		Description: "Finish auth orchestration",
		Status:      "in_progress",
		Assignee:    "agent-auth",
		CreatedAt:   time.Now().UTC().Add(-5 * time.Minute),
	}))
	require.NoError(t, store.RecordActivity(ctx, orchestrationdb.AgentActivity{
		AgentID:     "agent-auth",
		EventType:   "running",
		DetailsJSON: `{"phase":"implementation"}`,
		CreatedAt:   time.Now().UTC(),
	}))
	_, err = store.SaveCheckpoint(ctx, orchestrationdb.SessionCheckpoint{
		SessionID:      "sub-session",
		AgentID:        "agent-auth",
		WorkItemID:     "work-auth",
		SummaryJSON:    `{"phase":"subagent_turn_completed","status":"completed","result":"auth path updated"}`,
		AuditTail:      "milestone auth complete",
		MailCursor:     1,
		ActivityCursor: 2,
		CreatedAt:      time.Now().UTC(),
	})
	require.NoError(t, err)

	ctxBlock := coord.buildSubAgentPersistentMemoryContext(ctx, runner)
	require.Contains(t, ctxBlock, "PERSISTENT MEMORY")
	require.Contains(t, ctxBlock, "SESSION CONTINUITY")
	require.Contains(t, ctxBlock, "Auth flow")
	require.Contains(t, ctxBlock, "agent-auth")
	require.Contains(t, ctxBlock, "running")
	require.Contains(t, ctxBlock, "Latest Checkpoint")
	require.Contains(t, ctxBlock, "auth path updated")
}
