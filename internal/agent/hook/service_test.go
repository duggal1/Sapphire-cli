package hook

import (
	"context"
	"testing"
	"time"

	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestHookServiceAssignMarkAndClear(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	stateSvc := agentstate.NewService(store)
	require.NoError(t, stateSvc.Register(ctx, orchestrationdb.AgentState{
		AgentID:       "agent-1",
		Role:          "subagent",
		Status:        "running",
		SessionID:     "session-1",
		WorktreePath:  "/tmp/agent-1",
		Branch:        "agent/test",
		LastHeartbeat: time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}))
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:          "work-1",
		Type:        "task",
		Title:       "Implement work",
		Status:      "open",
		Description: "Do the thing",
		CreatedAt:   time.Now().UTC(),
	}))

	service := NewService(store, stateSvc)
	require.NoError(t, service.AssignHook(ctx, "agent-1", "work-1"))

	snapshot, err := service.GetHook(ctx, "agent-1")
	require.NoError(t, err)
	require.Equal(t, "work-1", snapshot.Hook.HookBeadID)
	require.Equal(t, "hooked", snapshot.Hook.Status)
	require.Equal(t, "agent-1", snapshot.WorkItem.Assignee)

	state, err := stateSvc.Status(ctx, "agent-1")
	require.NoError(t, err)
	require.Equal(t, "work-1", state.HookBeadID)

	require.NoError(t, service.MarkInProgress(ctx, "agent-1", "work-1"))
	snapshot, err = service.GetHook(ctx, "agent-1")
	require.NoError(t, err)
	require.Equal(t, "in_progress", snapshot.Hook.Status)

	require.NoError(t, service.ClearHook(ctx, "agent-1", "work-1"))
	snapshot, err = service.GetHook(ctx, "agent-1")
	require.NoError(t, err)
	require.Empty(t, snapshot.Hook.HookBeadID)
	require.Equal(t, "idle", snapshot.Hook.Status)

	state, err = stateSvc.Status(ctx, "agent-1")
	require.NoError(t, err)
	require.Empty(t, state.HookBeadID)
}
