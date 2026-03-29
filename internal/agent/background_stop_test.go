package agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	agentbackground "github.com/duggal1/Sapphire-cli/internal/agent/background"
	"github.com/duggal1/Sapphire-cli/internal/config"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestStopBackgroundActivityClosesRunnersAndBlocksDispatches(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg, err := config.Init(root, filepath.Join(root, ".sapphire"), false)
	require.NoError(t, err)

	store, err := orchestrationdb.Open(ctx, cfg.Options.DataDirectory)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	coord := &coordinator{
		cfg:                    cfg,
		orchestrationStore:     store,
		subAgentRegistry:       newSubAgentRegistry(),
		backgroundRegistry:     agentbackground.NewRegistry(),
		orchestrationSvcCancel: func() {},
	}

	cancelled := make(chan struct{}, 1)
	indexCancelled := make(chan struct{}, 1)
	coord.codeIndexCancel = func() {
		select {
		case indexCancelled <- struct{}{}:
		default:
		}
	}
	now := time.Now().UTC()
	runner := &subAgentRunner{
		id:            "agent-1",
		sessionID:     "session-1",
		parentSession: "session-parent",
		status:        subAgentStatusRunning,
		cancel: func() {
			select {
			case cancelled <- struct{}{}:
			default:
			}
		},
		inputCh: make(chan subAgentInput),
		submissions: map[string]*subAgentSubmission{
			"submission-1": {
				ID:        "submission-1",
				Status:    subAgentStatusRunning,
				StartedAt: now,
			},
		},
		lastSubmission: "submission-1",
		assignment: subAgentAssignment{
			ID:        "work-1",
			Title:     "Work One",
			Task:      "Investigate lag",
			CreatedAt: now.Add(-time.Minute),
		},
	}
	coord.subAgentRegistry.upsert(runner.id, runner)

	require.NoError(t, store.UpsertAgentState(ctx, orchestrationdb.AgentState{
		AgentID:       "agent-1",
		Role:          "subagent",
		Status:        "running",
		SessionID:     "session-1",
		HookBeadID:    "work-1",
		ParentAgentID: mainAgentMailboxID("session-parent"),
		LastHeartbeat: now.Add(-time.Minute),
		CreatedAt:     now.Add(-2 * time.Minute),
		UpdatedAt:     now.Add(-time.Minute),
	}))
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:          "work-1",
		Type:        "task",
		Title:       "Work One",
		Description: "Investigate lag",
		Status:      "in_progress",
		Assignee:    "agent-1",
		CreatedAt:   now.Add(-time.Minute),
	}))
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:          "work-2",
		Type:        "task",
		Title:       "Work Two",
		Description: "Queued work",
		Status:      "open",
		CreatedAt:   now.Add(-time.Minute),
	}))
	_, err = store.EnqueueDispatch(ctx, orchestrationdb.DispatchQueueItem{
		ID:         "dispatch-running",
		SessionID:  "session-1",
		WorkItemID: "work-1",
		Status:     "running",
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now.Add(-time.Minute),
	})
	require.NoError(t, err)
	_, err = store.EnqueueDispatch(ctx, orchestrationdb.DispatchQueueItem{
		ID:         "dispatch-queued",
		SessionID:  "session-1",
		WorkItemID: "work-2",
		Status:     "queued",
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now.Add(-time.Minute),
	})
	require.NoError(t, err)

	coord.backgroundRegistry.Register(agentbackground.SubAgent{
		ID:        "bg-1",
		SessionID: "session-1",
		Status:    agentbackground.StatusRunning,
	})

	summary, err := coord.StopBackgroundActivity(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, summary.ClosedSubAgents)
	require.Equal(t, 1, summary.StoppedBackgroundTasks)
	require.Equal(t, 2, summary.StoppedDispatches)
	require.Equal(t, 2, summary.BlockedWorkItems)
	require.Equal(t, 0, summary.BlockedAgentStates)
	require.Equal(t, 0, summary.DeadLetteredMail)
	require.Equal(t, 1, summary.CancelledCodebaseIndexes)
	require.Nil(t, coord.orchestrationSvcCancel)

	select {
	case <-cancelled:
	default:
		t.Fatal("runner cancel was not called")
	}

	select {
	case <-indexCancelled:
	default:
		t.Fatal("code index cancel was not called")
	}

	_, getErr := coord.getSubAgent("agent-1")
	require.Error(t, getErr)

	workOne, err := store.GetWorkItem(ctx, "work-1")
	require.NoError(t, err)
	require.Equal(t, "closed", workOne.Status)

	workTwo, err := store.GetWorkItem(ctx, "work-2")
	require.NoError(t, err)
	require.Equal(t, "closed", workTwo.Status)

	state, err := store.GetAgentState(ctx, "agent-1")
	require.NoError(t, err)
	require.NotEqual(t, "running", state.Status)

	runningDispatches, err := store.ListDispatchesByWorkItem(ctx, "work-1", []string{"blocked"}, 10)
	require.NoError(t, err)
	require.Len(t, runningDispatches, 1)

	queuedDispatches, err := store.ListDispatchesByWorkItem(ctx, "work-2", []string{"blocked"}, 10)
	require.NoError(t, err)
	require.Len(t, queuedDispatches, 1)

	bgTask, ok := coord.backgroundRegistry.Get("bg-1")
	require.True(t, ok)
	require.Equal(t, agentbackground.StatusFailed, bgTask.Status)
	require.Equal(t, "background activity stopped by user", bgTask.Error)
}
