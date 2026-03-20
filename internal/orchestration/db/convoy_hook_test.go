package orchestrationdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreConvoyAndHookLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	require.NoError(t, store.UpsertWorkItem(ctx, WorkItem{
		ID:        "work-1",
		Type:      "task",
		Title:     "Tracked work",
		Status:    "open",
		CreatedAt: time.Now().UTC(),
	}))

	convoy, err := store.SaveConvoy(ctx, Convoy{
		ID:            "cv-1",
		Name:          "Convoy 1",
		Owner:         "main:session-1",
		MergeStrategy: "direct",
		Status:        "open",
		CreatedAt:     time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, "cv-1", convoy.ID)

	require.NoError(t, store.AddConvoyTracks(ctx, convoy.ID, []string{"work-1"}))
	tracks, err := store.ListConvoyTracks(ctx, convoy.ID)
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	require.Equal(t, "work-1", tracks[0].WorkItemID)

	items, err := store.ListWorkItemsByConvoy(ctx, convoy.ID, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, convoy.ID, items[0].ConvoyID)

	require.NoError(t, store.UpsertAgentHook(ctx, AgentHook{
		AgentID:    "agent-1",
		HookBeadID: "work-1",
		HookedAt:   time.Now().UTC(),
		Status:     "hooked",
	}))
	hook, err := store.GetAgentHook(ctx, "agent-1")
	require.NoError(t, err)
	require.Equal(t, "work-1", hook.HookBeadID)

	hooks, err := store.ListAgentHooks(ctx, []string{"hooked"}, 10)
	require.NoError(t, err)
	require.Len(t, hooks, 1)
	require.Equal(t, "agent-1", hooks[0].AgentID)
}
