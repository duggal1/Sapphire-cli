package activity

import (
	"context"
	"testing"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestActivityServiceLogAndFeed(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	service := NewService(store)
	require.NoError(t, service.Log(ctx, "agent-1", EventSpawned, `{"status":"running"}`))
	require.NoError(t, service.Log(ctx, "agent-2", EventMailSent, `{"to":"agent-1"}`))

	recent, err := service.Recent(ctx, "agent-1", 10)
	require.NoError(t, err)
	require.Len(t, recent, 1)

	feed, err := service.Feed(ctx, []string{"agent-1", "agent-2"}, 10)
	require.NoError(t, err)
	require.Len(t, feed, 2)
}
