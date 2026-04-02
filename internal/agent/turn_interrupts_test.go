package agent

import (
	"context"
	"errors"
	"strings"
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

func TestRepeatedToolLoopErrorDescribesSuffixPattern(t *testing.T) {
	t.Parallel()

	err := &repeatedToolLoopError{loop: repeatedToolLoop{
		RepeatCount: 3,
		WindowSize:  9,
		ToolNames:   []string{"read", "write", "patch"},
		PatternSize: 3,
	}}

	if got := err.Error(); got == "" || !strings.Contains(got, "3-step suffix pattern 3 times") {
		t.Fatalf("unexpected suffix-pattern error: %q", got)
	}
}

func TestRepeatedToolLoopErrorDescribesReasoningLoop(t *testing.T) {
	t.Parallel()

	err := &repeatedToolLoopError{loop: repeatedToolLoop{
		RepeatCount: 4,
		WindowSize:  4,
		LoopSource:  "reasoning",
		Summary:     "the current best option is still the same architecture because the tradeoffs still appear favorable without any new repository evidence.",
	}}

	if got := err.Error(); got == "" || !strings.Contains(got, "repeated reasoning-path loop detected") || !strings.Contains(got, "repeated analysis sample") {
		t.Fatalf("unexpected reasoning-loop error: %q", got)
	}
}

func TestMaxStepsPerTurn(t *testing.T) {
	t.Parallel()

	if got := maxStepsPerTurn(false, false, false, false, false); got != defaultMaxStepsPerTurn {
		t.Fatalf("unexpected default max steps: %d", got)
	}
	if got := maxStepsPerTurn(false, true, false, false, false); got != postCompactionMaxStepsPerTurn {
		t.Fatalf("unexpected post-compaction max steps: %d", got)
	}
	if got := maxStepsPerTurn(false, false, true, false, false); got != structuredTurnMaxStepsPerTurn {
		t.Fatalf("unexpected structured-turn max steps: %d", got)
	}
	if got := maxStepsPerTurn(false, false, false, true, false); got != broadImplementationMaxSteps {
		t.Fatalf("unexpected broad-implementation max steps: %d", got)
	}
	if got := maxStepsPerTurn(false, false, true, false, true); got != broadInitMaxStepsPerTurn {
		t.Fatalf("unexpected broad-init max steps: %d", got)
	}
	if got := maxStepsPerTurn(true, false, false, false, false); got != longHorizonMaxStepsPerTurn {
		t.Fatalf("unexpected long-horizon max steps: %d", got)
	}
	if got := maxStepsPerTurn(true, false, true, true, true); got != broadInitMaxStepsPerTurn {
		t.Fatalf("expected broad-init budget to outrank long-horizon, got %d", got)
	}
	if got := maxStepsPerTurn(true, false, true, true, false); got != longHorizonMaxStepsPerTurn {
		t.Fatalf("unexpected long-horizon broad-implementation max steps: %d", got)
	}
	if got := maxStepsPerTurn(true, false, true, false, false); got != longHorizonMaxStepsPerTurn {
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
