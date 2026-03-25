package mailbox

import (
	"context"
	"time"
)

const (
	nudgeTimeout          = 2 * time.Second
	nudgeFailureThreshold = 3
	nudgeRetryAfter       = 5 * time.Minute
)

func (s *Service) Nudge(ctx context.Context, recipient string) error {
	if s == nil || s.nudge == nil {
		return nil
	}

	// Circuit breaker check
	s.mu.RLock()
	fails := s.nudgeFailures[recipient]
	lastFail := s.nudgeLastFail[recipient]
	s.mu.RUnlock()

	if fails >= nudgeFailureThreshold && time.Since(lastFail) < nudgeRetryAfter {
		return nil // Circuit open
	}

	// Timeout
	ctx, cancel := context.WithTimeout(ctx, nudgeTimeout)
	defer cancel()

	err := s.nudge(ctx, recipient)

	// Update circuit breaker state
	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.nudgeFailures[recipient]++
		s.nudgeLastFail[recipient] = time.Now()
	} else {
		delete(s.nudgeFailures, recipient)
		delete(s.nudgeLastFail, recipient)
	}

	return err
}
