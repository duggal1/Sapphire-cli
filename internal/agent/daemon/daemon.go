package daemon

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	agentscheduler "github.com/duggal1/Sapphire-cli/internal/agent/scheduler"
	agentsupervisor "github.com/duggal1/Sapphire-cli/internal/agent/supervisor"
)

type Service struct {
	dispatcher *agentscheduler.Dispatcher
	supervisor *agentsupervisor.Service

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewService(dispatcher *agentscheduler.Dispatcher, supervisor *agentsupervisor.Service) *Service {
	return &Service{
		dispatcher: dispatcher,
		supervisor: supervisor,
	}
}

func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	go s.run(runCtx)
}

func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) RunCycle(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if s.dispatcher != nil {
		if err := s.dispatcher.RunDispatchCycle(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		if err := s.dispatcher.RunPatrolCycle(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	if s.supervisor != nil {
		if err := s.supervisor.RunPatrolCycle(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

func (s *Service) run(ctx context.Context) {
	dispatchInterval := agentscheduler.DefaultDispatchInterval
	patrolInterval := agentscheduler.DefaultPatrolInterval
	if s.dispatcher != nil {
		dispatchInterval = s.dispatcher.DispatchInterval()
		patrolInterval = s.dispatcher.PatrolInterval()
	}
	tickerInterval := dispatchInterval
	if patrolInterval < tickerInterval {
		tickerInterval = patrolInterval
	}
	if tickerInterval <= 0 {
		tickerInterval = 3 * time.Minute
	}
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	var (
		lastDispatch time.Time
		lastPatrol   time.Time
	)
	for {
		now := time.Now().UTC()
		if s.dispatcher != nil && now.Sub(lastDispatch) >= dispatchInterval {
			if err := s.dispatcher.RunDispatchCycle(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("Daemon dispatch cycle failed", "error", err)
			}
			lastDispatch = now
		}
		if s.dispatcher != nil && now.Sub(lastPatrol) >= patrolInterval {
			if err := s.dispatcher.RunPatrolCycle(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("Daemon dispatch patrol failed", "error", err)
			}
		}
		if s.supervisor != nil && now.Sub(lastPatrol) >= patrolInterval {
			if err := s.supervisor.RunPatrolCycle(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("Daemon supervisor patrol failed", "error", err)
			}
			lastPatrol = now
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
