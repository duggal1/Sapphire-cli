package supervisor

import (
	"context"
	"strings"
	"testing"
	"time"

	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestSupervisorUnblocksDependentWorkItems(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:        "dep-1",
		Type:      "task",
		Title:     "dependency",
		Status:    "closed",
		CreatedAt: time.Now().UTC(),
		ClosedAt:  time.Now().UTC(),
	}))
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:           "task-2",
		Type:         "task",
		Title:        "dependent",
		Status:       "blocked",
		Dependencies: `["dep-1"]`,
		CreatedAt:    time.Now().UTC(),
	}))

	dispatchCalls := 0
	service := NewService(
		store,
		agentstate.NewService(store),
		agentactivity.NewService(store),
		agentmailbox.NewService(store, nil),
		Hooks{
			EnsureDispatchForWorkItem: func(ctx context.Context, item orchestrationdb.WorkItem) (string, error) {
				dispatchCalls++
				return "dispatch-1", nil
			},
		},
	)

	service.unblockWaitingAgents(ctx)

	item, err := store.GetWorkItem(ctx, "task-2")
	require.NoError(t, err)
	require.Equal(t, "open", item.Status)
	require.Equal(t, 1, dispatchCalls)
}

func TestSupervisorDoesNotUnblockBlockedDispatchBarrier(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	now := time.Now().UTC()
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:        "task-1",
		Type:      "task",
		Title:     "stopped work",
		Status:    "blocked",
		CreatedAt: now,
	}))
	_, err = store.EnqueueDispatch(ctx, orchestrationdb.DispatchQueueItem{
		ID:          "dispatch-1",
		SessionID:   "session-1",
		WorkItemID:  "task-1",
		TargetScope: "subagent",
		Status:      "blocked",
		LastError:   "background activity stopped by user",
		AvailableAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	require.NoError(t, err)

	dispatchCalls := 0
	service := NewService(
		store,
		agentstate.NewService(store),
		agentactivity.NewService(store),
		agentmailbox.NewService(store, nil),
		Hooks{
			EnsureDispatchForWorkItem: func(ctx context.Context, item orchestrationdb.WorkItem) (string, error) {
				dispatchCalls++
				return "dispatch-2", nil
			},
		},
	)

	service.unblockWaitingAgents(ctx)

	item, err := store.GetWorkItem(ctx, "task-1")
	require.NoError(t, err)
	require.Equal(t, "blocked", item.Status)
	require.Equal(t, 0, dispatchCalls)
}

func TestSupervisorValidatesCompletionAndReportsToMain(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	runtime := AgentRuntimeSnapshot{
		AgentID:          "agent-1",
		SessionID:        "session-1",
		WorkItemID:       "work-1",
		Status:           "completed",
		LastResult:       "STATUS: done\nSUMMARY: implemented",
		ValidationPassed: true,
		LastHeartbeat:    time.Now().UTC(),
	}
	service := NewService(
		store,
		agentstate.NewService(store),
		agentactivity.NewService(store),
		agentmailbox.NewService(store, nil),
		Hooks{
			GetRuntimeSnapshot: func(agentID string) (AgentRuntimeSnapshot, bool) {
				return runtime, true
			},
			ResolveMainMailboxID: func(sessionID string) string {
				return "main:" + sessionID
			},
		},
	)

	service.TrackAgent(runtime)
	service.NotifyCompletion(runtime)
	service.processCompletions(ctx, service.snapshotTrackers())

	inbox, err := store.ListInbox(ctx, "main:session-1", true, 10)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	require.Equal(t, "SUBAGENT_VALIDATED", inbox[0].Subject)
}

func TestSupervisorReassignsRecoverableAgents(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:        "work-1",
		Type:      "task",
		Title:     "Recover task",
		Status:    "blocked",
		Assignee:  "agent-1",
		CreatedAt: time.Now().UTC(),
	}))

	dispatchCalls := 0
	service := NewService(
		store,
		agentstate.NewService(store),
		agentactivity.NewService(store),
		agentmailbox.NewService(store, nil),
		Hooks{
			EnsureDispatchForWorkItem: func(ctx context.Context, item orchestrationdb.WorkItem) (string, error) {
				dispatchCalls++
				return "dispatch-1", nil
			},
		},
	)

	service.TrackAgent(AgentRuntimeSnapshot{
		AgentID:       "agent-1",
		SessionID:     "session-1",
		WorkItemID:    "work-1",
		Status:        "needs_reassignment",
		LastHeartbeat: time.Now().UTC(),
	})
	service.reassignRecoverableAgents(ctx, service.snapshotTrackers())

	item, err := store.GetWorkItem(ctx, "work-1")
	require.NoError(t, err)
	require.Equal(t, "open", item.Status)
	require.Equal(t, 1, dispatchCalls)
}

func TestSupervisorHandleStuckAgentThrottleSendsSingleMail(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	service := NewService(
		store,
		agentstate.NewService(store),
		agentactivity.NewService(store),
		agentmailbox.NewService(store, nil),
		Hooks{},
	)

	tracker := &AgentTracker{
		AgentID:       "agent-1",
		SessionID:     "session-1",
		WorkItemID:    "work-1",
		Status:        "running",
		SpawnedAt:     time.Now().UTC().Add(-30 * time.Minute),
		LastHeartbeat: time.Now().UTC().Add(-20 * time.Minute),
	}
	service.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
		*existing = *tracker
	})

	service.handleStuckAgent(ctx, tracker)
	service.handleStuckAgent(ctx, tracker)

	inbox, err := store.ListInbox(ctx, tracker.AgentID, false, 10)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	require.Equal(t, "SUPERVISOR", inbox[0].Subject)
}

func TestSupervisorDoesNotSendUnreadMailReminderLoop(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	mailbox := agentmailbox.NewService(store, nil)
	_, err = mailbox.Send(ctx, "agent-1", "supervisor", "SUPERVISOR", "Existing unread mail", agentmailbox.SendOptions{SkipNudge: true})
	require.NoError(t, err)

	service := NewService(
		store,
		agentstate.NewService(store),
		agentactivity.NewService(store),
		mailbox,
		Hooks{
			GetRuntimeSnapshot: func(agentID string) (AgentRuntimeSnapshot, bool) {
				return AgentRuntimeSnapshot{
					AgentID:       agentID,
					SessionID:     "session-1",
					WorkItemID:    "work-1",
					Status:        "idle",
					LastHeartbeat: time.Now().UTC(),
				}, true
			},
		},
	)

	tracker := &AgentTracker{
		AgentID:       "agent-1",
		SessionID:     "session-1",
		WorkItemID:    "work-1",
		Status:        "idle",
		SpawnedAt:     time.Now().UTC().Add(-5 * time.Minute),
		LastHeartbeat: time.Now().UTC(),
	}
	service.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
		*existing = *tracker
	})

	service.superviseAgent(ctx, tracker)

	inbox, err := store.ListInbox(ctx, tracker.AgentID, false, 10)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	require.Equal(t, "Existing unread mail", inbox[0].Body)
}

func TestSupervisorReportsHeartbeatContext(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	mainMailboxID := "main-mailbox"
	runtime := AgentRuntimeSnapshot{
		AgentID:          "agent-1",
		SessionID:        "session-1",
		WorkItemID:       "work-1",
		Status:           "running",
		LastHeartbeat:    time.Now().UTC().Add(-20 * time.Minute),
		HeartbeatContext: "Executing tool: single_view",
	}

	service := NewService(
		store,
		agentstate.NewService(store),
		agentactivity.NewService(store),
		agentmailbox.NewService(store, nil),
		Hooks{
			GetRuntimeSnapshot: func(agentID string) (AgentRuntimeSnapshot, bool) {
				return runtime, true
			},
			ResolveMainMailboxID: func(sessionID string) string {
				return mainMailboxID
			},
		},
	)

	tracker := &AgentTracker{
		AgentID:       "agent-1",
		SessionID:     "session-1",
		WorkItemID:    "work-1",
		Status:        "running",
		SpawnedAt:     time.Now().UTC().Add(-30 * time.Minute),
		LastHeartbeat: time.Now().UTC().Add(-20 * time.Minute),
	}
	service.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
		*existing = *tracker
	})

	service.superviseAgent(ctx, tracker)

	inbox, err := store.ListInbox(ctx, mainMailboxID, false, 10)
	require.NoError(t, err)

	found := false
	for _, msg := range inbox {
		if strings.Contains(msg.Body, "Executing tool: single_view") {
			found = true
			break
		}
	}
	require.True(t, found, "Heartbeat context should be included in escalation message")
}
