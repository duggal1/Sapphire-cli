package daemon

import (
	"context"
	"testing"

	agentscheduler "github.com/duggal1/Sapphire-cli/internal/agent/scheduler"
	"github.com/stretchr/testify/require"
)

func TestRunCycleDispatchesAndPatrols(t *testing.T) {
	ctx := context.Background()
	dispatchCalls := 0
	reconcileCalls := 0

	dispatcher := agentscheduler.NewDispatcher(agentscheduler.Hooks{
		ProcessQueue: func(context.Context) error {
			dispatchCalls++
			return nil
		},
		Reconcile: func(context.Context) error {
			reconcileCalls++
			return nil
		},
	})

	service := NewService(dispatcher, nil)
	err := service.RunCycle(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, dispatchCalls)
	require.Equal(t, 1, reconcileCalls)
}
