package agent

import (
	"fmt"
	"strings"
	"sync"
)

const maxToolFailuresPerTurn = 3

type toolFailureTracker struct {
	mu     sync.Mutex
	counts map[string]int
	limit  int
}

func newToolFailureTracker(limit int) *toolFailureTracker {
	if limit < 1 {
		limit = 1
	}
	return &toolFailureTracker{
		counts: map[string]int{},
		limit:  limit,
	}
}

func (t *toolFailureTracker) Record(toolName string) (count, attemptsLeft, limit int, limitReached bool) {
	if t == nil {
		return 0, 0, 0, false
	}
	name := strings.ToLower(strings.TrimSpace(toolName))
	if name == "" {
		name = "unknown"
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.counts[name]++
	count = t.counts[name]
	limit = t.limit
	attemptsLeft = limit - count
	if attemptsLeft < 0 {
		attemptsLeft = 0
	}
	limitReached = count >= limit
	return count, attemptsLeft, limit, limitReached
}

func appendToolFailureRetryMessage(content, toolName string, failures, attemptsLeft, limit int) string {
	content = strings.TrimSpace(content)
	retryMessage := buildToolFailureRetryMessage(toolName, failures, attemptsLeft, limit)
	if content == "" {
		return retryMessage
	}
	return content + "\n\n" + retryMessage
}

func buildToolFailureRetryMessage(toolName string, failures, attemptsLeft, limit int) string {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		toolName = "unknown"
	}
	if failures < 0 {
		failures = 0
	}
	if limit < 1 {
		limit = 1
	}
	if attemptsLeft < 0 {
		attemptsLeft = 0
	}

	guidance := "Do not repeat the same failing call. Fix the inputs or switch tools."
	if attemptsLeft == 0 {
		guidance = "This tool has exhausted its per-turn budget. Stop retrying it and change tactics."
	}

	return strings.TrimSpace(fmt.Sprintf(
		`<retry>
- tool: %s
- failures_this_turn: %d
- allowed_max_failures: %d
- attempts_left: %d
- guidance: %s
</retry>`,
		toolName,
		failures,
		limit,
		attemptsLeft,
		guidance,
	))
}

type toolFailureLimitError struct {
	toolName string
	failures int
	limit    int
}

func (e *toolFailureLimitError) Error() string {
	if e == nil {
		return ""
	}
	toolName := strings.TrimSpace(e.toolName)
	if toolName == "" {
		toolName = "unknown"
	}
	limit := e.limit
	if limit < 1 {
		limit = 1
	}
	return fmt.Sprintf("tool %q failed %d times this turn (limit %d)", toolName, e.failures, limit)
}

func (e *toolFailureLimitError) Is(target error) bool {
	return target == ErrToolFailureLimitReached
}
