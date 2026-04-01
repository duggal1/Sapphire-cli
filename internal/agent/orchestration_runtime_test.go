package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
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

func TestBuildSubAgentPersistentMemoryContextFreshLaunchUsesSharedCache(t *testing.T) {
	ctx := context.Background()
	metrics := newSubAgentLaunchMetrics()
	coord := &coordinator{
		memory:                    orchestrationMemoryStub{},
		subAgentLaunchProbe:       metrics,
		subAgentRegistry:          newSubAgentRegistry(),
		subAgentLaunchMemoryCache: make(map[string]subAgentLaunchMemoryCacheEntry),
		subAgentLaunchMemoryWork:  make(map[string]*subAgentLaunchMemoryFlight),
	}

	runnerOne := &subAgentRunner{
		id:            "agent-one",
		sessionID:     "sub-one",
		parentSession: "parent-session",
		workDir:       "/repo",
		submissions:   make(map[string]*subAgentSubmission),
		freshLaunch:   true,
		assignment: subAgentAssignment{
			ID:    "work-one",
			Title: "Shard one",
			Task:  "Inspect the first repo slice",
		},
	}
	runnerTwo := &subAgentRunner{
		id:            "agent-two",
		sessionID:     "sub-two",
		parentSession: "parent-session",
		workDir:       "/repo",
		submissions:   make(map[string]*subAgentSubmission),
		freshLaunch:   true,
		assignment: subAgentAssignment{
			ID:    "work-two",
			Title: "Shard two",
			Task:  "Inspect the second repo slice",
		},
	}

	first := coord.buildSubAgentPersistentMemoryContext(ctx, runnerOne)
	second := coord.buildSubAgentPersistentMemoryContext(ctx, runnerTwo)

	require.Contains(t, first, "PERSISTENT MEMORY")
	require.Contains(t, first, "SESSION CONTINUITY")
	require.Contains(t, second, "PERSISTENT MEMORY")
	require.Contains(t, second, "SESSION CONTINUITY")

	_, counters := metrics.snapshot()
	require.Equal(t, int64(1), counters["subagent_memory.launch_context_cache_miss"])
	require.GreaterOrEqual(t, counters["subagent_memory.launch_context_cache_hit"], int64(1))
	require.Equal(t, int64(2), counters["subagent_memory.launch_lightweight"])
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

func TestSyncRunnerOrchestrationStateDoesNotTreatDomainsAsDependencies(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	coord := &coordinator{
		orchestrationStore: store,
		stateService:       agentstate.NewService(store),
	}
	runner := &subAgentRunner{
		id:        "agent-auth",
		sessionID: "sub-session",
		status:    subAgentStatusRunning,
		assignment: subAgentAssignment{
			ID:        "work-auth",
			Title:     "Auth flow",
			Task:      "Implement auth flow",
			Domains:   []string{"auth", "backend"},
			CreatedAt: time.Now().UTC(),
		},
	}

	coord.syncRunnerOrchestrationState(ctx, runner)

	item, err := store.GetWorkItem(ctx, "work-auth")
	require.NoError(t, err)
	require.Equal(t, "[]", item.Dependencies)
}

func TestShouldPersistCheckpointHandoffSkipsOrdinaryMainTurns(t *testing.T) {
	coord := &coordinator{}
	require.False(t, coord.shouldPersistCheckpointHandoff("session-1", mainAgentMailboxID("session-1"), "", "hi", "completed"))
	require.False(t, coord.shouldPersistCheckpointHandoff("session-1", mainAgentMailboxID("session-1"), "", "continue", "running"))

	coord.memoryCompiler = &agentmemory.Compiler{}
	require.False(t, coord.shouldPersistCheckpointHandoff("session-1", mainAgentMailboxID("session-1"), "", "hi", "completed"))
	require.True(t, coord.shouldPersistCheckpointHandoff("session-1", mainAgentMailboxID("session-1"), "", strings.Repeat("complex ", 90), "completed"))

	coord.subAgentRegistry = newSubAgentRegistry()
	coord.subAgentRegistry.upsert("agent-1", &subAgentRunner{
		id:            "agent-1",
		sessionID:     "sub-session",
		parentSession: "session-1",
		submissions:   make(map[string]*subAgentSubmission),
	})
	require.False(t, coord.shouldPersistCheckpointHandoff("sub-session", "agent-1", "work-1", "finish implementation", "completed"))
}

func TestCoordinatorSubAgentToolObserverUpdatesTelemetry(t *testing.T) {
	coord := &coordinator{subAgentRegistry: newSubAgentRegistry()}
	runner := &subAgentRunner{
		id:          "agent-1",
		sessionID:   "sub-session",
		status:      subAgentStatusRunning,
		submissions: map[string]*subAgentSubmission{"sub-1": {ID: "sub-1", Status: subAgentStatusRunning}},
	}
	runner.lastSubmission = "sub-1"
	coord.subAgentRegistry.upsert(runner.id, runner)

	coord.OnToolInputStart("sub-session", "agentic_view")
	coord.OnToolCall("sub-session", "agentic_view", "")
	snap := runner.snapshot()
	require.Equal(t, "agentic_view", snap.CurrentTool)
	require.Equal(t, 1, snap.ToolCallCount)

	coord.OnToolResult("sub-session", "agentic_view", "", "", false)
	snap = runner.snapshot()
	require.Empty(t, snap.CurrentTool)
	require.Equal(t, "agentic_view", snap.LastTool)
	require.Equal(t, 1, snap.ToolCallCount)
}

func TestCoordinatorOnToolResultReconcilesRuntimeRewriteForSingularity(t *testing.T) {
	t.Parallel()

	manager := newTestSingularityManager(t)
	prompt := "Initialize the codebase broadly and refresh AGENTS.md."
	manager.StartTurn("sub-session", prompt, manager.repoRoot, nil, learnedRoutePolicy{})

	coord := &coordinator{
		singularity:      manager,
		subAgentRegistry: newSubAgentRegistry(),
	}

	coord.OnToolCall("sub-session", agenttools.LSToolName, `{"path":"."}`)
	metadata := agenttools.AnnotateRuntimeExecutionMetadata("", fantasy.ToolCall{
		ID:    "call-1",
		Name:  agenttools.LSToolName,
		Input: `{"path":"."}`,
	}, fantasy.ToolCall{
		ID:    "call-1",
		Name:  agenttools.ToolSearchToolName,
		Input: `{"query":"Initialize the codebase broadly and refresh AGENTS.md."}`,
	})
	coord.OnToolResult("sub-session", agenttools.ToolSearchToolName, "search ok", metadata, false)

	trace := manager.FinishTurn("sub-session", "completed", "structured discovery executed")
	require.NotNil(t, trace)
	require.Equal(t, []string{agenttools.ToolSearchToolName}, trace.OrderedTools)
	require.Equal(t, 1, trace.ToolCalls[agenttools.ToolSearchToolName])
	require.Zero(t, trace.ToolCalls[agenttools.LSToolName])
	require.Equal(t, 1, trace.ToolResults[agenttools.ToolSearchToolName])
}
