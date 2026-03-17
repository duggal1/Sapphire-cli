package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequestClockDoesNotResetOnPauseResume(t *testing.T) {
	t.Parallel()

	var clock requestClock
	start := time.Unix(100, 0)
	pause := start.Add(3 * time.Second)
	resume := pause.Add(2 * time.Second)
	finish := resume.Add(4 * time.Second)

	clock.Start(start)
	clock.Pause(pause)
	require.Equal(t, 3*time.Second, clock.elapsedRunning)

	clock.Resume(resume)
	clock.Complete(finish)

	require.Equal(t, start, clock.startedAt)
	require.Equal(t, finish, clock.completedAt)
	require.Equal(t, 7*time.Second, clock.elapsedRunning)
}

func TestRequestClockStartAfterCompletionBeginsFreshRun(t *testing.T) {
	t.Parallel()

	var clock requestClock
	firstStart := time.Unix(200, 0)
	firstFinish := firstStart.Add(5 * time.Second)
	secondStart := firstFinish.Add(1 * time.Second)

	clock.Start(firstStart)
	clock.Complete(firstFinish)
	clock.Start(secondStart)

	require.Equal(t, secondStart, clock.startedAt)
	require.True(t, clock.completedAt.IsZero())
	require.Zero(t, clock.elapsedRunning)
	require.True(t, clock.active)
}
