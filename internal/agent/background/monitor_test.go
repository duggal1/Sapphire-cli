package background

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMonitorNotifiesCompletedAgentOnce(t *testing.T) {
	registry := NewRegistry()
	registry.Register(SubAgent{ID: "bg-1", SessionID: "session-1", Status: StatusCompleted})

	var notified atomic.Int32
	monitor := NewMonitor(registry, MonitorHooks{
		Notify: func(ctx context.Context, agent SubAgent) {
			notified.Add(1)
		},
	})
	monitor.interval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitor.Start(ctx)
	}()

	require.Eventually(t, func() bool { return notified.Load() == 1 }, time.Second, 20*time.Millisecond)
	cancel()
	<-done
	require.EqualValues(t, 1, notified.Load())
}
