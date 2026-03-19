package scheduler

import (
	"context"
	"fmt"
	"time"
)

const (
	DefaultDispatchInterval = 3 * time.Minute
	DefaultPatrolInterval   = 3 * time.Minute
)

type Hooks struct {
	ProcessQueue func(context.Context) error
	Reconcile    func(context.Context) error
	CountActive  func() int
	MaxActive    func() int
}

type Dispatcher struct {
	hooks            Hooks
	dispatchInterval time.Duration
	patrolInterval   time.Duration
}

func NewDispatcher(hooks Hooks) *Dispatcher {
	return &Dispatcher{
		hooks:            hooks,
		dispatchInterval: DefaultDispatchInterval,
		patrolInterval:   DefaultPatrolInterval,
	}
}

func (d *Dispatcher) DispatchInterval() time.Duration {
	if d == nil || d.dispatchInterval <= 0 {
		return DefaultDispatchInterval
	}
	return d.dispatchInterval
}

func (d *Dispatcher) PatrolInterval() time.Duration {
	if d == nil || d.patrolInterval <= 0 {
		return DefaultPatrolInterval
	}
	return d.patrolInterval
}

func (d *Dispatcher) ActiveCount() int {
	if d == nil || d.hooks.CountActive == nil {
		return 0
	}
	return d.hooks.CountActive()
}

func (d *Dispatcher) MaxActive() int {
	if d == nil || d.hooks.MaxActive == nil {
		return 0
	}
	return d.hooks.MaxActive()
}

func (d *Dispatcher) AvailableCapacity() int {
	capacity := d.MaxActive() - d.ActiveCount()
	if capacity < 0 {
		return 0
	}
	return capacity
}

func (d *Dispatcher) RunDispatchCycle(ctx context.Context) error {
	if d == nil || d.hooks.ProcessQueue == nil {
		return nil
	}
	return d.hooks.ProcessQueue(ctx)
}

func (d *Dispatcher) RunPatrolCycle(ctx context.Context) error {
	if d == nil || d.hooks.Reconcile == nil {
		return nil
	}
	return d.hooks.Reconcile(ctx)
}

func (d *Dispatcher) Validate() error {
	if d == nil {
		return fmt.Errorf("dispatcher is nil")
	}
	if d.hooks.ProcessQueue == nil {
		return fmt.Errorf("dispatcher process hook is required")
	}
	if d.hooks.Reconcile == nil {
		return fmt.Errorf("dispatcher reconcile hook is required")
	}
	return nil
}
