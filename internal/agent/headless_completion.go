package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
)

const (
	maxHeadlessCompletionRecoveryAttempts = 1

	headlessAnalysisSoftBudget       = 45 * time.Second
	headlessAnalysisHardBudget       = 75 * time.Second
	headlessImplementationSoftBudget = 60 * time.Second
	headlessImplementationHardBudget = 90 * time.Second
	headlessInitializationSoftBudget = 50 * time.Second
	headlessInitializationHardBudget = 80 * time.Second
)

type headlessCompletionAction string

const (
	headlessCompletionActionNone     headlessCompletionAction = ""
	headlessCompletionActionFinalize headlessCompletionAction = "finalize"
	headlessCompletionActionReject   headlessCompletionAction = "reject"
)

type headlessCompletionBudget struct {
	Enabled    bool
	SoftLimit  time.Duration
	HardLimit  time.Duration
	TaskFamily string
}

type headlessCompletionController struct {
	budget        headlessCompletionBudget
	startedAt     time.Time
	softTriggered bool
	hardTriggered bool
}

type headlessCompletionBudgetError struct {
	Action     headlessCompletionAction
	TaskFamily string
	Elapsed    time.Duration
	SoftLimit  time.Duration
	HardLimit  time.Duration
	Reason     string
}

func (e *headlessCompletionBudgetError) Error() string {
	if e == nil {
		return ""
	}
	taskFamily := strings.TrimSpace(e.TaskFamily)
	switch e.Action {
	case headlessCompletionActionFinalize:
		if taskFamily == "" {
			return fmt.Sprintf("headless runtime budget reached after %s with enough evidence; finalize the answer without restarting discovery", e.Elapsed.Round(time.Second))
		}
		return fmt.Sprintf("headless runtime budget reached for %s after %s with enough evidence; finalize the answer without restarting discovery", taskFamily, e.Elapsed.Round(time.Second))
	case headlessCompletionActionReject:
		if taskFamily == "" {
			return fmt.Sprintf("headless runtime budget exceeded after %s without converging to a terminal answer", e.Elapsed.Round(time.Second))
		}
		return fmt.Sprintf("headless runtime budget exceeded for %s after %s without converging to a terminal answer", taskFamily, e.Elapsed.Round(time.Second))
	default:
		return "headless runtime budget exceeded"
	}
}

func (e *headlessCompletionBudgetError) Is(target error) bool {
	return target == ErrHeadlessCompletionBudgetReached
}

func buildHeadlessCompletionBudget(mode planmode.SessionMode, call SessionAgentCall) headlessCompletionBudget {
	if !isNonInteractiveMode() || planmode.NormalizeMode(mode) != planmode.DefaultSessionMode {
		return headlessCompletionBudget{}
	}
	taskFamily := strings.TrimSpace(call.LearnedToolPolicy.TaskFamily)
	switch {
	case taskFamily == "initialize/broad/codebase":
		return headlessCompletionBudget{
			Enabled:    true,
			SoftLimit:  headlessInitializationSoftBudget,
			HardLimit:  headlessInitializationHardBudget,
			TaskFamily: taskFamily,
		}
	case strings.HasPrefix(taskFamily, "implementation/"):
		return headlessCompletionBudget{
			Enabled:    true,
			SoftLimit:  headlessImplementationSoftBudget,
			HardLimit:  headlessImplementationHardBudget,
			TaskFamily: taskFamily,
		}
	case strings.HasPrefix(taskFamily, "design/"),
		strings.HasPrefix(taskFamily, "research/"),
		strings.HasPrefix(taskFamily, "review/"),
		strings.HasPrefix(taskFamily, "migration/"):
		return headlessCompletionBudget{
			Enabled:    true,
			SoftLimit:  headlessAnalysisSoftBudget,
			HardLimit:  headlessAnalysisHardBudget,
			TaskFamily: taskFamily,
		}
	default:
		return headlessCompletionBudget{}
	}
}

func newHeadlessCompletionController(budget headlessCompletionBudget) *headlessCompletionController {
	if !budget.Enabled || budget.HardLimit <= 0 {
		return nil
	}
	return &headlessCompletionController{
		budget:    budget,
		startedAt: time.Now(),
	}
}

func (c *headlessCompletionController) WrapContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c == nil || !c.budget.Enabled || c.budget.HardLimit <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.budget.HardLimit)
}

func (c *headlessCompletionController) MaybeForceFinalize(now time.Time, assistant *message.Message, usage *tools.ToolUsageState) error {
	if c == nil || !c.budget.Enabled || c.softTriggered || c.budget.SoftLimit <= 0 {
		return nil
	}
	if now.Sub(c.startedAt) < c.budget.SoftLimit {
		return nil
	}
	if !canForceHeadlessFinalization(c.budget.TaskFamily, assistant, usage) {
		return nil
	}
	c.softTriggered = true
	return &headlessCompletionBudgetError{
		Action:     headlessCompletionActionFinalize,
		TaskFamily: c.budget.TaskFamily,
		Elapsed:    now.Sub(c.startedAt),
		SoftLimit:  c.budget.SoftLimit,
		HardLimit:  c.budget.HardLimit,
		Reason:     "sufficient evidence already exists",
	}
}

func (c *headlessCompletionController) TranslateStreamError(genCtx context.Context, err error, assistant *message.Message, usage *tools.ToolUsageState) error {
	if err == nil || c == nil || !c.budget.Enabled {
		return err
	}
	var budgetErr *headlessCompletionBudgetError
	if errors.As(err, &budgetErr) {
		return err
	}
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(genCtx.Err(), context.DeadlineExceeded) {
		return err
	}
	if c.hardTriggered {
		return err
	}
	c.hardTriggered = true
	action := headlessCompletionActionReject
	if canForceHeadlessFinalization(c.budget.TaskFamily, assistant, usage) {
		action = headlessCompletionActionFinalize
	}
	return &headlessCompletionBudgetError{
		Action:     action,
		TaskFamily: c.budget.TaskFamily,
		Elapsed:    time.Since(c.startedAt),
		SoftLimit:  c.budget.SoftLimit,
		HardLimit:  c.budget.HardLimit,
		Reason:     "hard runtime budget exceeded",
	}
}

func canForceHeadlessFinalization(taskFamily string, assistant *message.Message, usage *tools.ToolUsageState) bool {
	if assistant == nil || usage == nil {
		return false
	}
	if !tools.HasRequiredContextReadEvidence(usage) {
		return false
	}
	text := strings.TrimSpace(assistant.Content().Text)
	if len(text) < 240 {
		return false
	}
	taskFamily = strings.TrimSpace(taskFamily)
	if strings.HasPrefix(taskFamily, "implementation/") {
		metrics := usage.SnapshotDeterministicLoopMetrics()
		hasWrites := len(metrics.ModifiedFiles) > 0 || len(metrics.CreatedFiles) > 0
		hasVerification := usage.VerificationEvidenceCount() > 0 || len(usage.PendingArtifactVerificationPaths()) > 0
		return hasWrites || hasVerification
	}
	return true
}

func buildHeadlessCompletionRecoveryCall(mode planmode.SessionMode, call SessionAgentCall, err error, partialAssistant *message.Message) (SessionAgentCall, bool) {
	if planmode.NormalizeMode(mode) != planmode.DefaultSessionMode || call.HeadlessCompletionTry >= maxHeadlessCompletionRecoveryAttempts {
		return SessionAgentCall{}, false
	}
	var budgetErr *headlessCompletionBudgetError
	if !errors.As(err, &budgetErr) || budgetErr == nil || budgetErr.Action != headlessCompletionActionFinalize {
		return SessionAgentCall{}, false
	}

	followUp := call
	followUp.SkipUserMessage = true
	followUp.HeadlessCompletionTry++

	base := fmt.Sprintf(`Continue the same turn from the existing repository evidence. Do not restart the task.

Original user request:
%s`, strings.TrimSpace(call.Prompt))

	if partialAssistant != nil {
		partialTail := strings.TrimSpace(partialAssistant.Content().Text)
		if len(partialTail) > 1200 {
			partialTail = partialTail[len(partialTail)-1200:]
		}
		if partialTail != "" {
			base += fmt.Sprintf("\n\nPrevious draft tail:\n%s", partialTail)
		}
	}

	followUp.Prompt = strings.TrimSpace(base + `

Sapphire interrupted the previous headless turn because the runtime budget was already spent and enough repository evidence exists.

Required now:
- Do not restart discovery.
- Do not call more tools unless one final narrow verification read is strictly required to correct a grounded claim.
- Do not broaden the plan.
- Deliver the final answer now using the evidence already gathered.

If the task is still incomplete, say exactly what remains unresolved instead of continuing to explore.`)
	return followUp, true
}

func buildHeadlessCompletionResult(assistant *message.Message) *fantasy.AgentResult {
	if assistant == nil {
		return nil
	}
	text := strings.TrimSpace(assistant.Content().Text)
	return &fantasy.AgentResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: text},
			},
		},
	}
}
