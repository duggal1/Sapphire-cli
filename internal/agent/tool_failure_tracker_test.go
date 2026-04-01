package agent

import (
	"errors"
	"strings"
	"testing"
)

func TestToolFailureTrackerRecordsPerToolCounts(t *testing.T) {
	t.Parallel()

	tracker := newToolFailureTracker(3)

	count, attemptsLeft, limit, reached := tracker.Record("Write")
	if count != 1 || attemptsLeft != 2 || limit != 3 || reached {
		t.Fatalf("unexpected first record state: count=%d attemptsLeft=%d limit=%d reached=%t", count, attemptsLeft, limit, reached)
	}

	count, attemptsLeft, limit, reached = tracker.Record("write")
	if count != 2 || attemptsLeft != 1 || limit != 3 || reached {
		t.Fatalf("unexpected second record state: count=%d attemptsLeft=%d limit=%d reached=%t", count, attemptsLeft, limit, reached)
	}

	count, attemptsLeft, limit, reached = tracker.Record("ls")
	if count != 1 || attemptsLeft != 2 || limit != 3 || reached {
		t.Fatalf("unexpected independent tool state: count=%d attemptsLeft=%d limit=%d reached=%t", count, attemptsLeft, limit, reached)
	}
}

func TestToolFailureTrackerHitsLimit(t *testing.T) {
	t.Parallel()

	tracker := newToolFailureTracker(3)
	for i := 0; i < 3; i++ {
		count, attemptsLeft, limit, reached := tracker.Record("bash")
		if i < 2 && reached {
			t.Fatalf("did not expect limit before third failure: count=%d attemptsLeft=%d limit=%d reached=%t", count, attemptsLeft, limit, reached)
		}
		if i == 2 && !reached {
			t.Fatalf("expected limit on third failure: count=%d attemptsLeft=%d limit=%d reached=%t", count, attemptsLeft, limit, reached)
		}
	}
}

func TestBuildToolFailureRetryMessageIncludesBudget(t *testing.T) {
	t.Parallel()

	msg := buildToolFailureRetryMessage("edit", 2, 1, 3)
	for _, want := range []string{
		"<retry>",
		"tool: edit",
		"failures_this_turn: 2",
		"allowed_max_failures: 3",
		"attempts_left: 1",
		"guidance:",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %q to contain %q", msg, want)
		}
	}
}

func TestToolFailureLimitErrorMatchesSentinel(t *testing.T) {
	t.Parallel()

	err := &toolFailureLimitError{toolName: "write", failures: 3, limit: 3}
	if !errors.Is(err, ErrToolFailureLimitReached) {
		t.Fatal("expected toolFailureLimitError to match the sentinel error")
	}
	if !strings.Contains(err.Error(), "write") {
		t.Fatalf("expected tool name in error, got %q", err.Error())
	}
}
