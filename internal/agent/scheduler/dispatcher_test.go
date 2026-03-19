package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDispatcherAvailableCapacityAndCycles(t *testing.T) {
	ctx := context.Background()
	dispatchCalls := 0
	patrolCalls := 0

	dispatcher := NewDispatcher(Hooks{
		ProcessQueue: func(context.Context) error {
			dispatchCalls++
			return nil
		},
		Reconcile: func(context.Context) error {
			patrolCalls++
			return nil
		},
		CountActive: func() int { return 3 },
		MaxActive:   func() int { return 8 },
	})

	require.NoError(t, dispatcher.Validate())
	require.Equal(t, 5, dispatcher.AvailableCapacity())
	require.NoError(t, dispatcher.RunDispatchCycle(ctx))
	require.NoError(t, dispatcher.RunPatrolCycle(ctx))
	require.Equal(t, 1, dispatchCalls)
	require.Equal(t, 1, patrolCalls)
}
