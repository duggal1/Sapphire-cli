package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemoryPipelineTriggerPostCompletionBatchesBySession(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		calls    int
		sessions []string
		payloads []string
	)
	pipe := &memoryPipeline{
		active:   make(map[string]bool),
		pending:  make(map[string][]string),
		timers:   make(map[string]*time.Timer),
		debounce: time.Hour,
		extractFunc: func(ctx context.Context, sessionID, rolloutText string) (*memoryExtractionResult, error) {
			mu.Lock()
			calls++
			sessions = append(sessions, sessionID)
			payloads = append(payloads, rolloutText)
			mu.Unlock()
			return &memoryExtractionResult{}, nil
		},
	}

	pipe.TriggerPostCompletion("parent-session", "first outcome")
	pipe.TriggerPostCompletion("parent-session", "second outcome")
	pipe.TriggerPostCompletion("parent-session", "third outcome")

	pipe.mu.Lock()
	require.Len(t, pipe.pending["parent-session"], 3)
	if timer := pipe.timers["parent-session"]; timer != nil {
		timer.Stop()
	}
	pipe.mu.Unlock()

	pipe.flushPostCompletion("parent-session")

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, calls)
	require.Equal(t, []string{"parent-session"}, sessions)
	require.Len(t, payloads, 1)
	require.Contains(t, payloads[0], "first outcome")
	require.Contains(t, payloads[0], "second outcome")
	require.Contains(t, payloads[0], "third outcome")
}
