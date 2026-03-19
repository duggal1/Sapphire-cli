package state

import (
	"context"
	"testing"
	"time"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestStateServiceRegisterAndListStale(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	service := NewService(store)
	require.NoError(t, service.Register(ctx, Snapshot{
		AgentID:       "agent-1",
		Role:          "subagent",
		Status:        string(StatusRunning),
		SessionID:     "session-1",
		HookBeadID:    "work-1",
		ParentAgentID: "main:session-1",
		LastHeartbeat: time.Now().UTC().Add(-3 * time.Minute),
		CreatedAt:     time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt:     time.Now().UTC().Add(-3 * time.Minute),
	}))

	snapshot, err := service.Status(ctx, "agent-1")
	require.NoError(t, err)
	require.Equal(t, "work-1", snapshot.HookBeadID)

	stale, err := service.ListStale(ctx, time.Now().UTC().Add(-2*time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, stale, 1)
}
