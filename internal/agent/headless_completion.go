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
	maxHeadlessCompletionRecoveryAttempts = 2

	headlessAnalysisSoftBudget       = 45 * time.Second
	headlessAnalysisHardBudget       = 75 * time.Second
	headlessImplementationSoftBudget = 60 * time.Second
	headlessImplementationHardBudget = 90 * time.Second
	headlessInitializationSoftBudget = 50 * time.Second
	headlessInitializationHardBudget = 80 * time.Second
)

type headlessCompletionAction string
type headlessCompletionPhase string

const (
	headlessCompletionActionNone      headlessCompletionAction = ""
	headlessCompletionActionStructure headlessCompletionAction = "structure"
	headlessCompletionActionExecute   headlessCompletionAction = "execute"
	headlessCompletionActionFinalize  headlessCompletionAction = "finalize"
	headlessCompletionActionReject    headlessCompletionAction = "reject"

	headlessPhaseRead      headlessCompletionPhase = "read"
	headlessPhaseStructure headlessCompletionPhase = "structure"
	headlessPhaseExecute   headlessCompletionPhase = "execute"
	headlessPhaseSynthesis headlessCompletionPhase = "synthesis"
	headlessPhaseClose     headlessCompletionPhase = "close"
)

type headlessCompletionBudget struct {
	Enabled    bool
	SoftLimit  time.Duration
	HardLimit  time.Duration
	TaskFamily string
}

type headlessCompletionController struct {
	budget          headlessCompletionBudget
	startedAt       time.Time
	executePrompted bool
	softTriggered   bool
	hardTriggered   bool
}

type headlessCompletionBudgetError struct {
	Action     headlessCompletionAction
	TaskFamily string
	Phase      headlessCompletionPhase
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
	case headlessCompletionActionStructure:
		if taskFamily == "" {
			return fmt.Sprintf("headless runtime phase budget reached after %s with grounded reads but no structured discovery; stop broad reading and perform one targeted structured discovery pass", e.Elapsed.Round(time.Second))
		}
		return fmt.Sprintf("headless runtime phase budget reached for %s after %s with grounded reads but no structured discovery; stop broad reading and perform one targeted structured discovery pass", taskFamily, e.Elapsed.Round(time.Second))
	case headlessCompletionActionExecute:
		if taskFamily == "" {
			return fmt.Sprintf("headless runtime phase budget reached after %s with enough evidence; stop exploring and move directly into execution or explicit blockage reporting", e.Elapsed.Round(time.Second))
		}
		return fmt.Sprintf("headless runtime phase budget reached for %s after %s with enough evidence; stop exploring and move directly into execution or explicit blockage reporting", taskFamily, e.Elapsed.Round(time.Second))
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
	if c == nil || !c.budget.Enabled || c.budget.SoftLimit <= 0 {
		return nil
	}
	elapsed := now.Sub(c.startedAt)
	phase := detectHeadlessCompletionPhase(c.budget.TaskFamily, assistant, usage)
	if !c.executePrompted && elapsed >= c.budget.SoftLimit/2 && phase == headlessPhaseStructure && canForceStructuredDiscoveryKick(c.budget.TaskFamily, usage) {
		c.executePrompted = true
		return &headlessCompletionBudgetError{
			Action:     headlessCompletionActionStructure,
			TaskFamily: c.budget.TaskFamily,
			Phase:      phase,
			Elapsed:    elapsed,
			SoftLimit:  c.budget.SoftLimit,
			HardLimit:  c.budget.HardLimit,
			Reason:     "structured discovery is overdue",
		}
	}
	if !c.executePrompted && elapsed >= c.budget.SoftLimit/2 && phase == headlessPhaseExecute && canForceHeadlessExecutionKick(c.budget.TaskFamily, usage) {
		c.executePrompted = true
		return &headlessCompletionBudgetError{
			Action:     headlessCompletionActionExecute,
			TaskFamily: c.budget.TaskFamily,
			Phase:      phase,
			Elapsed:    elapsed,
			SoftLimit:  c.budget.SoftLimit,
			HardLimit:  c.budget.HardLimit,
			Reason:     "execution phase is overdue",
		}
	}
	if c.softTriggered || elapsed < c.budget.SoftLimit {
		return nil
	}
	if !canForceHeadlessFinalization(c.budget.TaskFamily, assistant, usage) {
		return nil
	}
	c.softTriggered = true
	return &headlessCompletionBudgetError{
		Action:     headlessCompletionActionFinalize,
		TaskFamily: c.budget.TaskFamily,
		Phase:      phase,
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
	phase := detectHeadlessCompletionPhase(c.budget.TaskFamily, assistant, usage)
	if canForceHeadlessFinalization(c.budget.TaskFamily, assistant, usage) {
		action = headlessCompletionActionFinalize
	}
	return &headlessCompletionBudgetError{
		Action:     action,
		TaskFamily: c.budget.TaskFamily,
		Phase:      phase,
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
	taskFamily = strings.TrimSpace(taskFamily)
	if strings.HasPrefix(taskFamily, "implementation/") {
		metrics := usage.SnapshotDeterministicLoopMetrics()
		hasWrites := len(metrics.ModifiedFiles) > 0 || len(metrics.CreatedFiles) > 0
		hasVerification := usage.VerificationEvidenceCount() > 0 || len(usage.PendingArtifactVerificationPaths()) > 0
		if len(text) < 160 {
			return false
		}
		return hasWrites || hasVerification
	}
	if hasSubstantialAnalysisDraft(text) {
		return true
	}
	return usage.Total(
		tools.RunHarnessToolName,
		tools.ToolSearchToolName,
		tools.RGFilesToolName,
		tools.AgenticViewToolName,
		tools.ViewToolName,
		tools.SingleViewToolName,
		tools.UpdatePlanToolName,
	) >= 3
}

func canForceHeadlessExecutionKick(taskFamily string, usage *tools.ToolUsageState) bool {
	if usage == nil {
		return false
	}
	taskFamily = strings.TrimSpace(taskFamily)
	if !strings.HasPrefix(taskFamily, "implementation/") {
		return false
	}
	if !tools.HasRequiredContextReadEvidence(usage) || !usage.HasPublishedPlan() {
		return false
	}
	metrics := usage.SnapshotDeterministicLoopMetrics()
	hasWrites := len(metrics.ModifiedFiles) > 0 || len(metrics.CreatedFiles) > 0
	hasVerification := usage.VerificationEvidenceCount() > 0 || len(usage.PendingArtifactVerificationPaths()) > 0
	if hasWrites || hasVerification {
		return false
	}
	return usage.Total(
		tools.RunHarnessToolName,
		tools.ToolSearchToolName,
		tools.RGFilesToolName,
		tools.AgenticViewToolName,
		tools.ViewToolName,
		tools.SingleViewToolName,
		tools.UpdatePlanToolName,
	) >= 3
}

func canForceStructuredDiscoveryKick(taskFamily string, usage *tools.ToolUsageState) bool {
	if usage == nil {
		return false
	}
	taskFamily = strings.TrimSpace(taskFamily)
	if !strings.HasPrefix(taskFamily, "design/") &&
		!strings.HasPrefix(taskFamily, "research/") &&
		!strings.HasPrefix(taskFamily, "review/") &&
		!strings.HasPrefix(taskFamily, "migration/") {
		return false
	}
	return usage.ReadEvidenceCount() > 0 && usage.StructuredEvidenceCount() == 0
}

func detectHeadlessCompletionPhase(taskFamily string, assistant *message.Message, usage *tools.ToolUsageState) headlessCompletionPhase {
	taskFamily = strings.TrimSpace(taskFamily)
	if usage == nil || usage.ReadEvidenceCount() == 0 {
		return headlessPhaseRead
	}
	if strings.HasPrefix(taskFamily, "implementation/") {
		if !usage.HasPublishedPlan() {
			return headlessPhaseStructure
		}
		metrics := usage.SnapshotDeterministicLoopMetrics()
		hasWrites := len(metrics.ModifiedFiles) > 0 || len(metrics.CreatedFiles) > 0
		hasVerification := usage.VerificationEvidenceCount() > 0 || len(usage.PendingArtifactVerificationPaths()) > 0
		if !hasWrites && !hasVerification {
			return headlessPhaseExecute
		}
		if assistant == nil || !hasSubstantialAnalysisDraft(strings.TrimSpace(assistant.Content().Text)) {
			return headlessPhaseSynthesis
		}
		return headlessPhaseClose
	}
	if usage.StructuredEvidenceCount() == 0 {
		return headlessPhaseStructure
	}
	if assistant == nil || !hasSubstantialAnalysisDraft(strings.TrimSpace(assistant.Content().Text)) {
		return headlessPhaseSynthesis
	}
	return headlessPhaseClose
}

func hasSubstantialAnalysisDraft(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 320 {
		return false
	}
	lower := strings.ToLower(text)
	signals := 0
	for _, needle := range []string{
		"option a", "option b", "trade-off", "tradeoff", "repo fit", "migration cost",
		"blast radius", "because", "recommend", "validate", "rollback", "risk",
	} {
		if strings.Contains(lower, needle) {
			signals++
		}
	}
	if signals >= 2 {
		return true
	}
	return strings.Count(text, "\n") >= 4
}

func buildHeadlessCompletionRecoveryCall(mode planmode.SessionMode, call SessionAgentCall, err error, partialAssistant *message.Message) (SessionAgentCall, bool) {
	if planmode.NormalizeMode(mode) != planmode.DefaultSessionMode || call.HeadlessCompletionTry >= maxHeadlessCompletionRecoveryAttempts {
		return SessionAgentCall{}, false
	}
	var budgetErr *headlessCompletionBudgetError
	if !errors.As(err, &budgetErr) || budgetErr == nil {
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

	switch budgetErr.Action {
	case headlessCompletionActionStructure:
		followUp.Prompt = strings.TrimSpace(base + `

Sapphire interrupted the previous headless turn because you already have grounded reads but still have not done structured discovery.

Required now:
- Do not restart the task.
- Do not load skills.
- Do not do another broad read-first pass.
- Perform exactly one targeted structured discovery step to map the concrete files, symbols, or boundaries that matter.
- After that, move directly into the architecture answer using that structured evidence.`)
	case headlessCompletionActionExecute:
		followUp.Prompt = strings.TrimSpace(base + `

Sapphire interrupted the previous headless turn because you spent too much time analyzing and have not crossed into execution.

Required now:
- Do not restart discovery.
- Do not widen the plan.
- Move directly into the first minimal concrete implementation step using the files already inspected.
- If one final narrow file read is strictly required before editing, do exactly one and then execute.
- If you are still blocked, state the exact blocker instead of continuing broad exploration.`)
	case headlessCompletionActionFinalize:
		followUp.HeadlessClosureMode = headlessClosureModeForcedFinalize
		followUp.Prompt = strings.TrimSpace(base + `

Sapphire interrupted the previous headless turn because the runtime budget was already spent and enough repository evidence exists.

Required now:
- Do not restart discovery.
- Do not call more tools unless one final narrow verification read is strictly required to correct a grounded claim.
- Do not broaden the plan.
- Deliver the final answer now using the evidence already gathered.

If the task is still incomplete, say exactly what remains unresolved instead of continuing to explore.`)
	default:
		return SessionAgentCall{}, false
	}
	followUp.HeadlessPhaseAtInterrupt = string(budgetErr.Phase)
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

func shouldSalvageHeadlessResult(taskFamily string, attempt int, assistant *message.Message) bool {
	if attempt <= 0 || assistant == nil {
		return false
	}
	taskFamily = strings.TrimSpace(taskFamily)
	if !strings.HasPrefix(taskFamily, "design/") &&
		!strings.HasPrefix(taskFamily, "research/") &&
		!strings.HasPrefix(taskFamily, "review/") &&
		!strings.HasPrefix(taskFamily, "migration/") {
		return false
	}
	text := strings.TrimSpace(assistant.Content().Text)
	if !hasSubstantialAnalysisDraft(text) {
		return false
	}
	return len(text) >= 320
}
