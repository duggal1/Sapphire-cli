package background

import (
	"context"
	"time"
)

type MonitorHooks struct {
	Notify func(context.Context, SubAgent)
}

type Monitor struct {
	registry *Registry
	hooks    MonitorHooks
	interval time.Duration
}

func NewMonitor(registry *Registry, hooks MonitorHooks) *Monitor {
	return &Monitor{registry: registry, hooks: hooks, interval: 5 * time.Second}
}

func (m *Monitor) Start(ctx context.Context) {
	if m == nil {
		return
	}
	m.runMonitorLoop(ctx)
}

func (m *Monitor) runMonitorLoop(ctx context.Context) {
	if m == nil || m.registry == nil {
		return
	}
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			completed := append(m.registry.ListByStatus(StatusCompleted), m.registry.ListByStatus(StatusFailed)...)
			for _, agent := range completed {
				if !m.registry.MarkNotified(agent.ID) {
					continue
				}
				if m.hooks.Notify != nil {
					m.hooks.Notify(ctx, agent)
				}
			}
		}
	}
}
