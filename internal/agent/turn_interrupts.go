package agent

import (
	"fmt"
	"strings"
	"time"
)

const (
	defaultMaxStepsPerTurn        = 12
	postCompactionMaxStepsPerTurn = 14
	structuredTurnMaxStepsPerTurn = 16
	broadImplementationMaxSteps   = 18
	broadInitMaxStepsPerTurn      = 22
	longHorizonMaxStepsPerTurn    = 18
	maxStreamRetryBackoff         = 2 * time.Second
)

func maxStepsPerTurn(longHorizonActive, postCompactionPending, structuredTurn, broadImplementationTurn, broadInitializationTurn bool) int {
	limit := defaultMaxStepsPerTurn
	if postCompactionPending && limit < postCompactionMaxStepsPerTurn {
		limit = postCompactionMaxStepsPerTurn
	}
	if structuredTurn && limit < structuredTurnMaxStepsPerTurn {
		limit = structuredTurnMaxStepsPerTurn
	}
	if broadImplementationTurn && limit < broadImplementationMaxSteps {
		limit = broadImplementationMaxSteps
	}
	if broadInitializationTurn && limit < broadInitMaxStepsPerTurn {
		limit = broadInitMaxStepsPerTurn
	}
	if longHorizonActive && limit < longHorizonMaxStepsPerTurn {
		limit = longHorizonMaxStepsPerTurn
	}
	return limit
}

func nextStreamRetryBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := streamRetryBackoff
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= maxStreamRetryBackoff {
			return maxStreamRetryBackoff
		}
	}
	if delay > maxStreamRetryBackoff {
		return maxStreamRetryBackoff
	}
	return delay
}

type turnStepLimitError struct {
	limit int
}

func (e *turnStepLimitError) Error() string {
	if e == nil {
		return ""
	}
	limit := e.limit
	if limit < 1 {
		limit = 1
	}
	return fmt.Sprintf("turn stopped after %d steps to prevent unbounded execution; finish the highest-value next action or change tactics", limit)
}

func (e *turnStepLimitError) Is(target error) bool {
	return target == ErrStepLimitReached
}

type repeatedToolLoopError struct {
	loop repeatedToolLoop
}

func (e *repeatedToolLoopError) Error() string {
	if e == nil {
		return ""
	}
	names := uniqueNonEmptyStrings(append([]string{}, e.loop.ToolNames...))
	toolSummary := "unknown tool"
	if len(names) == 1 {
		toolSummary = names[0]
	} else if len(names) > 1 {
		toolSummary = strings.Join(names, ", ")
	}
	repeatCount := e.loop.RepeatCount
	if repeatCount < 1 {
		repeatCount = 1
	}
	windowSize := e.loop.WindowSize
	if windowSize < repeatCount {
		windowSize = repeatCount
	}
	if e.loop.LoopSource == "reasoning" {
		if e.loop.PatternSize > 1 {
			return fmt.Sprintf("repeated reasoning-path loop detected: a %d-step reasoning suffix pattern repeated %d times within the last %d steps; stop replaying the same analysis and change the diagnosis", e.loop.PatternSize, repeatCount, windowSize)
		}
		if summary := strings.TrimSpace(e.loop.Summary); summary != "" {
			return fmt.Sprintf("repeated reasoning-path loop detected: near-identical analysis repeated %d times within the last %d steps; repeated analysis sample: %q; stop replaying the same diagnosis and change tactics", repeatCount, windowSize, summary)
		}
		return fmt.Sprintf("repeated reasoning-path loop detected: near-identical analysis repeated %d times within the last %d steps; stop replaying the same diagnosis and change tactics", repeatCount, windowSize)
	}
	if e.loop.PatternSize > 1 {
		return fmt.Sprintf("repeated tool-call loop detected: %s repeated as a %d-step suffix pattern %d times within the last %d steps; stop replaying the same sequence and change tactics", toolSummary, e.loop.PatternSize, repeatCount, windowSize)
	}
	return fmt.Sprintf("repeated tool-call loop detected: %s repeated %d times within the last %d steps; stop retrying the same interaction and change tactics", toolSummary, repeatCount, windowSize)
}

func (e *repeatedToolLoopError) Is(target error) bool {
	return target == ErrRepeatedToolLoopDetected
}

type deterministicDoomLoopError struct {
	loop deterministicDoomLoop
}

func (e *deterministicDoomLoopError) Error() string {
	if e == nil {
		return ""
	}
	if len(e.loop.Signals) == 0 {
		return "deterministic doom loop detected: repeated low-value execution without material progress"
	}
	summaries := make([]string, 0, len(e.loop.Signals))
	for _, signal := range e.loop.Signals {
		summary := strings.TrimSpace(signal.Summary)
		if summary == "" {
			continue
		}
		summaries = append(summaries, summary)
	}
	if len(summaries) == 0 {
		return "deterministic doom loop detected: repeated low-value execution without material progress"
	}
	return "deterministic doom loop detected: " + strings.Join(summaries, " ")
}

func (e *deterministicDoomLoopError) Is(target error) bool {
	return target == ErrDeterministicDoomLoop
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
