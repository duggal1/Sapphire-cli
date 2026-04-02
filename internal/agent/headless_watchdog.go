package agent

import (
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
)

const (
	headlessAnalysisWatchdogGrace       = 15 * time.Second
	headlessImplementationWatchdogGrace = 20 * time.Second
	headlessInitializationWatchdogGrace = 15 * time.Second
)

type headlessWatchdogBudget struct {
	Enabled    bool
	TaskFamily string
	Timeout    time.Duration
}

type headlessWatchdogTimeoutError struct {
	TaskFamily string
	Timeout    time.Duration
}

func (e *headlessWatchdogTimeoutError) Error() string {
	if e == nil {
		return ""
	}
	taskFamily := strings.TrimSpace(e.TaskFamily)
	if taskFamily == "" {
		return fmt.Sprintf("headless watchdog timeout reached after %s without terminalization", e.Timeout.Round(time.Second))
	}
	return fmt.Sprintf("headless watchdog timeout reached for %s after %s without terminalization", taskFamily, e.Timeout.Round(time.Second))
}

func (e *headlessWatchdogTimeoutError) Is(target error) bool {
	return target == ErrHeadlessWatchdogTimeout
}

func buildHeadlessWatchdogBudget(mode planmode.SessionMode, call SessionAgentCall) headlessWatchdogBudget {
	completionBudget := buildHeadlessCompletionBudget(mode, call)
	if !completionBudget.Enabled {
		return headlessWatchdogBudget{}
	}
	return headlessWatchdogBudget{
		Enabled:    true,
		TaskFamily: strings.TrimSpace(completionBudget.TaskFamily),
		Timeout:    completionBudget.HardLimit + headlessWatchdogGraceForTaskFamily(completionBudget.TaskFamily),
	}
}

func headlessWatchdogGraceForTaskFamily(taskFamily string) time.Duration {
	taskFamily = strings.TrimSpace(taskFamily)
	switch {
	case taskFamily == "initialize/broad/codebase":
		return headlessInitializationWatchdogGrace
	case strings.HasPrefix(taskFamily, "implementation/"):
		return headlessImplementationWatchdogGrace
	case strings.HasPrefix(taskFamily, "design/"),
		strings.HasPrefix(taskFamily, "research/"),
		strings.HasPrefix(taskFamily, "review/"),
		strings.HasPrefix(taskFamily, "migration/"):
		return headlessAnalysisWatchdogGrace
	default:
		return 10 * time.Second
	}
}

func inferHeadlessWatchdogPhase(taskFamily, resultText string) string {
	taskFamily = strings.TrimSpace(taskFamily)
	resultText = strings.TrimSpace(resultText)
	switch {
	case resultText == "":
		return string(headlessPhaseRead)
	case shouldWatchdogFinalizeAnalysis(taskFamily, resultText):
		return string(headlessPhaseClose)
	case strings.HasPrefix(taskFamily, "implementation/"):
		return string(headlessPhaseExecute)
	case taskFamily == "initialize/broad/codebase":
		return string(headlessPhaseExecute)
	default:
		return string(headlessPhaseSynthesis)
	}
}

func shouldWatchdogFinalizeAnalysis(taskFamily, resultText string) bool {
	taskFamily = strings.TrimSpace(taskFamily)
	if !strings.HasPrefix(taskFamily, "design/") &&
		!strings.HasPrefix(taskFamily, "research/") &&
		!strings.HasPrefix(taskFamily, "review/") &&
		!strings.HasPrefix(taskFamily, "migration/") {
		return false
	}
	return hasSubstantialAnalysisDraft(resultText)
}

func buildHeadlessWatchdogResult(resultText string) *fantasy.AgentResult {
	resultText = strings.TrimSpace(resultText)
	if resultText == "" {
		return nil
	}
	return &fantasy.AgentResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: resultText},
			},
		},
	}
}
