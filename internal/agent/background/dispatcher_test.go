package background

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDispatcherReturnsImmediatelyAndCompletes(t *testing.T) {
	registry := NewRegistry()
	dispatcher := NewDispatcher(registry, Hooks{
		Execute: func(ctx context.Context, spec TaskSpec) (ExecutionResult, error) {
			time.Sleep(50 * time.Millisecond)
			return ExecutionResult{RuntimeAgentID: "agent-1", SubmissionID: "sub-1", Result: "ok"}, nil
		},
		DefaultCtx:    func() context.Context { return context.Background() },
		MaxConcurrent: func() int { return 2 },
	})

	started := time.Now()
	agentID, err := dispatcher.Dispatch(context.Background(), TaskSpec{SessionID: "session-1", Name: "task"})
	require.NoError(t, err)
	require.NotEmpty(t, agentID)
	require.Less(t, time.Since(started), 25*time.Millisecond)

	results, err := dispatcher.WaitForCompletion(context.Background(), []string{agentID})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, StatusCompleted, results[0].Status)
	require.Equal(t, "ok", results[0].Result)
}

func TestDispatcherEnforcesCapacity(t *testing.T) {
	registry := NewRegistry()
	var active atomic.Int32
	var maxSeen atomic.Int32
	release := make(chan struct{})

	dispatcher := NewDispatcher(registry, Hooks{
		Execute: func(ctx context.Context, spec TaskSpec) (ExecutionResult, error) {
			cur := active.Add(1)
			for {
				seen := maxSeen.Load()
				if cur <= seen || maxSeen.CompareAndSwap(seen, cur) {
					break
				}
			}
			<-release
			active.Add(-1)
			return ExecutionResult{RuntimeAgentID: spec.ID, SubmissionID: spec.ID, Result: "ok"}, nil
		},
		DefaultCtx:    func() context.Context { return context.Background() },
		MaxConcurrent: func() int { return 1 },
	})

	first, err := dispatcher.Dispatch(context.Background(), TaskSpec{SessionID: "session-1", Name: "a"})
	require.NoError(t, err)
	second, err := dispatcher.Dispatch(context.Background(), TaskSpec{SessionID: "session-1", Name: "b"})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	require.EqualValues(t, 1, maxSeen.Load())

	close(release)
	_, err = dispatcher.WaitForCompletion(context.Background(), []string{first, second})
	require.NoError(t, err)
}
