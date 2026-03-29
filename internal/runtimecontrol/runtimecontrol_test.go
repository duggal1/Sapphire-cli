package runtimecontrol

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRuntimeControlRoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC()

	require.NoError(t, WriteRuntimeStatus(dataDir, RuntimeStatus{
		PID:        1234,
		StartedAt:  now.Add(-time.Second),
		UpdatedAt:  now,
		WorkingDir: "/tmp/project",
	}))

	status, err := ReadRuntimeStatus(dataDir)
	require.NoError(t, err)
	require.Equal(t, 1234, status.PID)
	require.True(t, IsLive(status, now.Add(2*time.Second)))
	require.False(t, IsLive(status, now.Add(10*time.Second)))

	require.NoError(t, WriteRequest(dataDir, Request{
		ID:          "req-1",
		Action:      ActionStopBackground,
		RequestedAt: now,
	}))
	req, err := ReadRequest(dataDir)
	require.NoError(t, err)
	require.Equal(t, "req-1", req.ID)

	require.NoError(t, WriteResponse(dataDir, Response{
		ID:        "req-1",
		Action:    ActionStopBackground,
		Status:    "ok",
		Summary:   map[string]int{"closed_sub_agents": 2},
		HandledAt: now,
	}))
	resp, err := ReadResponse(dataDir)
	require.NoError(t, err)
	require.Equal(t, 2, resp.Summary["closed_sub_agents"])

	require.NoError(t, RemoveRequest(dataDir))
	_, err = ReadRequest(dataDir)
	require.True(t, errors.Is(err, os.ErrNotExist))

	require.NoError(t, RemoveRuntimeStatus(dataDir))
	_, err = ReadRuntimeStatus(dataDir)
	require.True(t, errors.Is(err, os.ErrNotExist))
}
