package supervisor

import (
	"context"
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
