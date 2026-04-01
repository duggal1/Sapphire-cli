package agent

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"
)

func TestTurnStepLimitErrorMatchesSentinel(t *testing.T) {
	t.Parallel()

	err := &turnStepLimitError{limit: 12}
	if !errors.Is(err, ErrStepLimitReached) {
		t.Fatal("expected step limit error to match sentinel")
	}
}

func TestRepeatedToolLoopErrorMatchesSentinel(t *testing.T) {
	t.Parallel()

	err := &repeatedToolLoopError{loop: repeatedToolLoop{
		RepeatCount: 6,
		WindowSize:  10,
		ToolNames:   []string{"read"},
	}}
	if !errors.Is(err, ErrRepeatedToolLoopDetected) {
		t.Fatal("expected repeated loop error to match sentinel")
	}
}

func TestMaxStepsPerTurn(t *testing.T) {
	t.Parallel()

	if got := maxStepsPerTurn(false, false, false); got != defaultMaxStepsPerTurn {
		t.Fatalf("unexpected default max steps: %d", got)
	}
	if got := maxStepsPerTurn(false, true, false); got != postCompactionMaxStepsPerTurn {
		t.Fatalf("unexpected post-compaction max steps: %d", got)
	}
	if got := maxStepsPerTurn(false, false, true); got != structuredTurnMaxStepsPerTurn {
		t.Fatalf("unexpected structured-turn max steps: %d", got)
	}
	if got := maxStepsPerTurn(true, false, false); got != longHorizonMaxStepsPerTurn {
		t.Fatalf("unexpected long-horizon max steps: %d", got)
	}
	if got := maxStepsPerTurn(true, false, true); got != longHorizonMaxStepsPerTurn {
		t.Fatalf("unexpected long-horizon structured max steps: %d", got)
	}
}

func TestNextStreamRetryBackoffCaps(t *testing.T) {
	t.Parallel()

	if got := nextStreamRetryBackoff(0); got != streamRetryBackoff {
		t.Fatalf("unexpected first retry backoff: %s", got)
	}
	if got := nextStreamRetryBackoff(4); got != maxStreamRetryBackoff {
		t.Fatalf("expected capped retry backoff, got %s", got)
	}
}

func TestShouldRetryStreamError(t *testing.T) {
	t.Parallel()

	if !shouldRetryStreamError(context.DeadlineExceeded) {
		t.Fatal("expected deadline exceeded to be retryable")
	}
	if !shouldRetryStreamError(errors.New(`Post "https://openrouter.ai/api/v1/chat/completions": read tcp 127.0.0.1:1->127.0.0.1:2: read: connection reset by peer`)) {
		t.Fatal("expected connection reset to be retryable")
	}

	providerErr := &fantasy.ProviderError{StatusCode: 429, Message: "Too many requests"}
	if !shouldRetryStreamError(providerErr) {
		t.Fatal("expected 429 provider error to be retryable")
	}

	nonRetryable := &fantasy.ProviderError{StatusCode: 400, Message: "bad request"}
	if shouldRetryStreamError(nonRetryable) {
		t.Fatal("did not expect 400 provider error to be retryable")
	}
}
