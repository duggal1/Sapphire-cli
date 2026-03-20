package background

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryTracksStatusAndNotification(t *testing.T) {
	registry := NewRegistry()
	registry.Register(SubAgent{ID: "bg-1", SessionID: "session-1", Status: StatusQueued})

	item, ok := registry.Get("bg-1")
	require.True(t, ok)
	require.Equal(t, StatusQueued, item.Status)

	registry.UpdateStatus("bg-1", StatusRunning)
	item, ok = registry.Get("bg-1")
	require.True(t, ok)
	require.Equal(t, StatusRunning, item.Status)
	require.False(t, item.StartedAt.IsZero())

	registry.SetResult("bg-1", "done")
	registry.UpdateStatus("bg-1", StatusCompleted)
	completed := registry.ListByStatus(StatusCompleted)
	require.Len(t, completed, 1)
	require.Equal(t, "done", completed[0].Result)

	require.True(t, registry.MarkNotified("bg-1"))
	require.False(t, registry.MarkNotified("bg-1"))
}
