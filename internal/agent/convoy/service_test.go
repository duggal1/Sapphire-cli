package convoy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestConvoyServiceStageLaunchFeedAndClose(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	dispatched := make([]string, 0, 3)
	service := NewService(store, Hooks{
		EnsureDispatchForWorkItem: func(ctx context.Context, item orchestrationdb.WorkItem) (string, error) {
			dispatched = append(dispatched, item.ID)
			return "dispatch-" + item.ID, nil
		},
	})

	convoy, err := service.CreateConvoy(ctx, "Batch 1", "main:session-1", "direct")
	require.NoError(t, err)

	dep2, err := json.Marshal([]string{"work-1"})
	require.NoError(t, err)
	dep3, err := json.Marshal([]string{"work-2"})
	require.NoError(t, err)

	for _, item := range []orchestrationdb.WorkItem{
		{ID: "work-1", Type: "task", Title: "Wave 1", Status: "open", CreatedAt: time.Now().UTC()},
		{ID: "work-2", Type: "task", Title: "Wave 2", Status: "open", Dependencies: string(dep2), CreatedAt: time.Now().UTC()},
		{ID: "work-3", Type: "task", Title: "Wave 3", Status: "open", Dependencies: string(dep3), CreatedAt: time.Now().UTC()},
	} {
		require.NoError(t, store.UpsertWorkItem(ctx, item))
	}
	require.NoError(t, service.AddWorkItems(ctx, convoy.ID, []string{"work-1", "work-2", "work-3"}))

	staged, warnings, err := service.StageConvoy(ctx, convoy.ID)
	require.NoError(t, err)
	require.Equal(t, StatusStagedReady, staged.Status)
	require.Empty(t, warnings)

	launched, err := service.LaunchConvoy(ctx, convoy.ID)
	require.NoError(t, err)
	require.Equal(t, StatusOpen, launched.Status)
	require.Equal(t, []string{"work-1"}, dispatched)

	work1, err := store.GetWorkItem(ctx, "work-1")
	require.NoError(t, err)
	work1.Status = "closed"
	work1.ClosedAt = time.Now().UTC()
	require.NoError(t, store.UpsertWorkItem(ctx, work1))
	closed, err := service.CheckConvoyCompletion(ctx, convoy.ID)
	require.NoError(t, err)
	require.False(t, closed)
	require.Equal(t, []string{"work-1", "work-2"}, dispatched)

	work2, err := store.GetWorkItem(ctx, "work-2")
	require.NoError(t, err)
	work2.Status = "closed"
	work2.ClosedAt = time.Now().UTC()
	require.NoError(t, store.UpsertWorkItem(ctx, work2))
	closed, err = service.CheckConvoyCompletion(ctx, convoy.ID)
	require.NoError(t, err)
	require.False(t, closed)
	require.Equal(t, []string{"work-1", "work-2", "work-3"}, dispatched)

	work3, err := store.GetWorkItem(ctx, "work-3")
	require.NoError(t, err)
	work3.Status = "closed"
	work3.ClosedAt = time.Now().UTC()
	require.NoError(t, store.UpsertWorkItem(ctx, work3))
	closed, err = service.CheckConvoyCompletion(ctx, convoy.ID)
	require.NoError(t, err)
	require.True(t, closed)

	convoy, err = store.GetConvoy(ctx, convoy.ID)
	require.NoError(t, err)
	require.Equal(t, StatusClosed, convoy.Status)
}

func TestConvoyServiceFindStrandedConvoys(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	service := NewService(store, Hooks{})
	convoy, err := service.CreateConvoy(ctx, "Stranded", "main:session-1", "direct")
	require.NoError(t, err)
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:        "work-stranded",
		Type:      "task",
		Title:     "Ready work",
		Status:    "open",
		ConvoyID:  convoy.ID,
		CreatedAt: time.Now().UTC(),
	}))
	require.NoError(t, service.AddWorkItems(ctx, convoy.ID, []string{"work-stranded"}))

	stranded, err := service.FindStrandedConvoys(ctx, 10)
	require.NoError(t, err)
	require.Len(t, stranded, 1)
	require.Equal(t, convoy.ID, stranded[0].ID)
}
