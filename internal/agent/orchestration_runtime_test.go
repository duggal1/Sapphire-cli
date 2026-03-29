package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	"github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/duggal1/Sapphire-cli/internal/message"
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
		mailbox:            agentmailbox.NewService(store, nil),
		subAgentRegistry:   newSubAgentRegistry(),
	}
	coord.checkpointService = agentmemory.NewCheckpointService(store, checkpointMessageSourceStub{}, coord.memory, nil)
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
	coord.subAgentRegistry.upsert(runner.id, runner)
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
		SessionID:         "sub-session",
		AgentID:           "agent-auth",
		WorkItemID:        "work-auth",
		MessageCount:      60,
		SummaryJSON:       `{"phase":"subagent_turn_completed","status":"completed","result":"auth path updated","summary":"use mailbox handoffs"}`,
		AuditTail:         "milestone auth complete",
		PendingTasksJSON:  `["verify auth tests"]`,
		FilesModifiedJSON: `["internal/agent/subagent_manager.go"]`,
		MailCursor:        1,
		ActivityCursor:    2,
		CreatedAt:         time.Now().UTC(),
	})
	require.NoError(t, err)
	_, err = coord.mailbox.Send(ctx, "agent-auth", "main:parent-session", "Dependency update", "Auth dependency completed", agentmailbox.SendOptions{})
	require.NoError(t, err)

	ctxBlock := coord.buildSubAgentPersistentMemoryContext(ctx, runner)
	require.Contains(t, ctxBlock, "PERSISTENT MEMORY")
	require.Contains(t, ctxBlock, "SESSION CONTINUITY")
	require.Contains(t, ctxBlock, "Auth flow")
	require.Contains(t, ctxBlock, "agent-auth")
	require.Contains(t, ctxBlock, "running")
	require.Contains(t, ctxBlock, "Latest Checkpoint")
	require.Contains(t, ctxBlock, "auth path updated")
	require.Contains(t, ctxBlock, "verify auth tests")
	require.Contains(t, ctxBlock, "Actionable Mail")
	require.Contains(t, ctxBlock, "Dependency update")
	require.Contains(t, ctxBlock, "Peer Directory")
	require.Contains(t, ctxBlock, "agent:agent-auth")
	require.Contains(t, ctxBlock, "work:work-auth")
}

func TestReportSubAgentOutcomeToParentSendsStructuredMail(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	coord := &coordinator{
		orchestrationStore: store,
		mailbox:            agentmailbox.NewService(store, nil),
		activityService:    agentactivity.NewService(store),
	}
	runner := &subAgentRunner{
		id:            "agent-auth",
		parentSession: "parent-session",
		assignment: subAgentAssignment{
			ID:    "work-auth",
			Title: "Auth flow",
			Task:  "Finish auth orchestration",
		},
	}

	coord.reportSubAgentOutcomeToParent(ctx, runner, "submission-1", subAgentReport{
		Status:   "needs_followup",
		Summary:  "Auth flow is ready for review",
		Progress: "Implemented the orchestration path",
		Files:    []string{"/tmp/auth.go"},
		Commands: []string{"go test ./internal/agent"},
		Next:     "Review and approve merge",
	}, "raw result")

	inbox, err := coord.mailbox.Inbox(ctx, "main:parent-session", true, 10)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	require.Equal(t, "SUBAGENT_NEEDS_FOLLOWUP", inbox[0].Subject)
	require.Contains(t, inbox[0].Body, "Assignment: Auth flow")
	require.Contains(t, inbox[0].Body, "Status: NEEDS_FOLLOWUP")
	require.Contains(t, inbox[0].Body, "Files: /tmp/auth.go")
	require.Equal(t, "work-auth", inbox[0].ThreadID)
}

type checkpointMessageSourceStub struct{}

func (checkpointMessageSourceStub) List(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func TestCurrentCheckpointCursorsUseDurableRowIDs(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	sessionID := "parent-session"
	mainAgentID := mainAgentMailboxID(sessionID)
	coord := &coordinator{
		orchestrationStore: store,
		stateService:       agentstate.NewService(store),
		subAgentRegistry:   newSubAgentRegistry(),
	}

	require.NoError(t, store.UpsertAgentState(ctx, orchestrationdb.AgentState{
		AgentID:       "agent-auth",
		Role:          "subagent",
		Status:        "running",
		SessionID:     "sub-session",
		HookBeadID:    "work-auth",
		ParentAgentID: mainAgentID,
		LastHeartbeat: time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}))
	require.NoError(t, store.RecordActivity(ctx, orchestrationdb.AgentActivity{
		AgentID:     mainAgentID,
		EventType:   "main_running",
		DetailsJSON: `{}`,
		CreatedAt:   time.Now().UTC(),
	}))
	require.NoError(t, store.RecordActivity(ctx, orchestrationdb.AgentActivity{
		AgentID:     "agent-auth",
		EventType:   "sub_running",
		DetailsJSON: `{}`,
		CreatedAt:   time.Now().UTC(),
	}))
	_, err = store.SendMail(ctx, orchestrationdb.AgentMail{
		Address:         "main",
		ToAgent:         mainAgentID,
		ResolvedToAgent: mainAgentID,
		FromAgent:       "agent-auth",
		Subject:         "handoff",
		Body:            "done",
	})
	require.NoError(t, err)

	expectedMailCursor, err := store.LatestMailRowID(ctx, mainAgentID)
	require.NoError(t, err)
	expectedActivityCursor, err := store.LatestActivityRowID(ctx, []string{mainAgentID, "agent-auth"})
	require.NoError(t, err)

	mailCursor, activityCursor := coord.currentCheckpointCursors(ctx, sessionID, mainAgentID)
	require.Equal(t, expectedMailCursor, mailCursor)
	require.Equal(t, expectedActivityCursor, activityCursor)
}

func TestShouldPersistCheckpointHandoffSkipsOrdinaryMainTurns(t *testing.T) {
	coord := &coordinator{}
	require.False(t, coord.shouldPersistCheckpointHandoff("session-1", mainAgentMailboxID("session-1"), "", "hi", "completed"))
	require.False(t, coord.shouldPersistCheckpointHandoff("session-1", mainAgentMailboxID("session-1"), "", "continue", "running"))

	coord.memoryCompiler = &agentmemory.Compiler{}
	require.False(t, coord.shouldPersistCheckpointHandoff("session-1", mainAgentMailboxID("session-1"), "", "hi", "completed"))
	require.True(t, coord.shouldPersistCheckpointHandoff("session-1", "agent-1", "work-1", "finish implementation", "completed"))
	require.True(t, coord.shouldPersistCheckpointHandoff("session-1", mainAgentMailboxID("session-1"), "", strings.Repeat("complex ", 90), "completed"))
}
