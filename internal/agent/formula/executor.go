package formula

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

var ErrApprovalDenied = errors.New("plan approval denied")

type ProgressStatus string

const (
	ProgressPending    ProgressStatus = "pending"
	ProgressInProgress ProgressStatus = "in_progress"
	ProgressCompleted  ProgressStatus = "completed"
)

type ProgressEntry struct {
	Step   string
	Status ProgressStatus
}

type LLMRequest struct {
	SessionID       string
	Step            Step
	Prompt          string
	ReasoningEffort string
	ToolNames       []string
	UseDefaultTools bool
}

type LLMResult struct {
	Output string
}

type LLMClient interface {
	ExecuteStep(context.Context, LLMRequest) (LLMResult, error)
}

type ExplorationResult struct {
	AgentID string
	Status  string
	Result  string
	Error   string
}

type StepResult struct {
	StepID      string
	Output      string
	StartedAt   time.Time
	CompletedAt time.Time
}

type ExecutionState struct {
	SessionID           string
	Variables           map[string]string
	Completed           map[string]bool
	Results             map[string]StepResult
	ExplorationAgentIDs []string
	ExplorationResults  []ExplorationResult
	PlanPath            string
	Approved            bool
}

type Executor struct {
	Formula            *Formula
	ToolRegistry       *tools.Registry
	LLM                LLMClient
	WorkingDir         string
	UpdateProgress     func(context.Context, []ProgressEntry) error
	Approve            func(context.Context, *ExecutionState) (bool, error)
	WaitForExploration func(context.Context, []string) ([]ExplorationResult, error)
}

func (e *Executor) Execute(ctx context.Context, variables map[string]string) (*ExecutionState, error) {
	if e == nil || e.Formula == nil {
		return nil, fmt.Errorf("formula executor is not initialized")
	}
	order, err := e.Formula.TopologicalSort()
	if err != nil {
		return nil, err
	}

	state := &ExecutionState{
		SessionID: getVar(variables, "session_id"),
		Variables: cloneVariables(variables, e.Formula),
		Completed: make(map[string]bool, len(order)),
		Results:   make(map[string]StepResult, len(order)),
	}

	if err := e.pushProgress(ctx, order, state, ""); err != nil {
		return nil, err
	}

	for _, step := range order {
		for _, need := range step.Needs {
			if !state.Completed[need] {
				return nil, fmt.Errorf("step %q blocked by incomplete dependency %q", step.ID, need)
			}
		}

		if err := e.pushProgress(ctx, order, state, step.ID); err != nil {
			return nil, err
		}

		output, err := e.executeStep(ctx, step, state)
		if err != nil {
			return state, err
		}
		if ok, err := e.checkAcceptance(step, state, output); err != nil {
			return state, err
		} else if !ok {
			return state, fmt.Errorf("step %q failed acceptance: %s", step.ID, step.Acceptance)
		}

		state.Completed[step.ID] = true
		state.Results[step.ID] = StepResult{
			StepID:      step.ID,
			Output:      output,
			StartedAt:   time.Now().UTC(),
			CompletedAt: time.Now().UTC(),
		}
		state.Variables[step.ID+"_output"] = output
		if err := e.pushProgress(ctx, order, state, ""); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func (e *Executor) executeStep(ctx context.Context, step Step, state *ExecutionState) (string, error) {
	rendered, err := renderTemplate(step.Description, state.Variables)
	if err != nil {
		return "", fmt.Errorf("render step %q: %w", step.ID, err)
	}

	switch step.ID {
	case "wait-exploration":
		if e.WaitForExploration == nil {
			return "", fmt.Errorf("wait-exploration requested but no waiter configured")
		}
		results, err := e.WaitForExploration(ctx, state.ExplorationAgentIDs)
		if err != nil {
			return "", err
		}
		state.ExplorationResults = results
		return summarizeExploration(results), nil
	case "gate-approval":
		if e.Approve == nil {
			return "", fmt.Errorf("approval gate requested but no approval callback configured")
		}
		approved, err := e.Approve(ctx, state)
		if err != nil {
			return "", err
		}
		if !approved {
			return "", ErrApprovalDenied
		}
		state.Approved = true
		return "approved", nil
	}

	if e.LLM == nil {
		return "", fmt.Errorf("llm client is not configured")
	}

	req := LLMRequest{
		SessionID:       state.SessionID,
		Step:            step,
		Prompt:          e.stepPrompt(step, rendered, state),
		ReasoningEffort: stepReasoningEffort(step.ID),
		ToolNames:       e.stepToolNames(step.ID),
		UseDefaultTools: strings.HasPrefix(step.ID, "execute-phase-"),
	}
	result, err := e.LLM.ExecuteStep(ctx, req)
	if err != nil {
		return "", err
	}

	output := strings.TrimSpace(result.Output)
	switch step.ID {
	case "launch-exploration":
		state.ExplorationAgentIDs = parseTaggedList(output, "exploration_agents")
	case "plan":
		planMarkdown := extractPlanMarkdown(output)
		planPath, err := e.writePlanFile(state, planMarkdown)
		if err != nil {
			return "", err
		}
		state.PlanPath = planPath
		state.Variables["plan_path"] = toFormulaPath(e.WorkingDir, planPath)
		output = planMarkdown
	}
	return output, nil
}

func (e *Executor) checkAcceptance(step Step, state *ExecutionState, output string) (bool, error) {
	switch step.ID {
	case "launch-exploration":
		return len(state.ExplorationAgentIDs) > 0, nil
	case "wait-exploration":
		return len(state.ExplorationResults) == len(state.ExplorationAgentIDs), nil
	case "plan":
		if state.PlanPath == "" {
			return false, nil
		}
		_, err := os.Stat(state.PlanPath)
		return err == nil, err
	case "gate-approval":
		return state.Approved, nil
	default:
		return strings.TrimSpace(output) != "", nil
	}
}

func (e *Executor) stepPrompt(step Step, rendered string, state *ExecutionState) string {
	var builder strings.Builder
	builder.WriteString("Execute the current workflow step.\n")
	builder.WriteString("Step: " + step.ID + " - " + step.Title + "\n\n")
	builder.WriteString(rendered)
	builder.WriteString("\n\n")
	if len(state.ExplorationResults) > 0 {
		builder.WriteString("Exploration findings:\n")
		builder.WriteString(summarizeExploration(state.ExplorationResults))
		builder.WriteString("\n\n")
	}
	switch step.ID {
	case "launch-exploration":
		builder.WriteString("Use launch_exploration_agent up to 5 times. End with <exploration_agents>id1,id2</exploration_agents>.\n")
	case "plan":
		builder.WriteString("Write the final plan inside <proposed_plan>...</proposed_plan>.\n")
	}
	return builder.String()
}

func (e *Executor) stepToolNames(stepID string) []string {
	if e.ToolRegistry == nil {
		return nil
	}
	if strings.HasPrefix(stepID, "execute-phase-") {
		return nil
	}
	switch stepID {
	case "wait-exploration", "gate-approval":
		return nil
	default:
		return e.ToolRegistry.Names()
	}
}

func stepReasoningEffort(stepID string) string {
	switch stepID {
	case "understand", "analyze", "launch-exploration", "wait-exploration", "design", "plan", "gate-approval":
		return "high"
	default:
		return ""
	}
}

func (e *Executor) pushProgress(ctx context.Context, order []Step, state *ExecutionState, activeStep string) error {
	if e == nil || e.UpdateProgress == nil {
		return nil
	}
	entries := make([]ProgressEntry, 0, len(order))
	for _, step := range order {
		status := ProgressPending
		switch {
		case state.Completed[step.ID]:
			status = ProgressCompleted
		case step.ID == activeStep:
			status = ProgressInProgress
		}
		entries = append(entries, ProgressEntry{
			Step:   step.Title,
			Status: status,
		})
	}
	return e.UpdateProgress(ctx, entries)
}

func (e *Executor) writePlanFile(state *ExecutionState, planMarkdown string) (string, error) {
	slug := strings.TrimSpace(state.Variables["task_slug"])
	if slug == "" {
		return "", fmt.Errorf("task_slug is required to write plan file")
	}
	planDir := filepath.Join(e.WorkingDir, ".plans", slug)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return "", fmt.Errorf("create plan directory: %w", err)
	}
	planPath := filepath.Join(planDir, "04-plan.md")
	if err := os.WriteFile(planPath, []byte(strings.TrimSpace(planMarkdown)+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write plan file: %w", err)
	}
	return planPath, nil
}

func cloneVariables(variables map[string]string, formula *Formula) map[string]string {
	cloned := make(map[string]string, len(variables)+len(formula.Vars))
	for key, value := range variables {
		cloned[key] = value
	}
	for key, variable := range formula.Vars {
		if _, exists := cloned[key]; !exists && variable.Default != "" {
			cloned[key] = variable.Default
		}
	}
	return cloned
}

func renderTemplate(raw string, variables map[string]string) (string, error) {
	tmpl, err := template.New("formula-step").Option("missingkey=zero").Parse(raw)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	if err := tmpl.Execute(&builder, variables); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func parseTaggedList(raw, tag string) []string {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	start := strings.Index(raw, openTag)
	end := strings.Index(raw, closeTag)
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	body := raw[start+len(openTag) : end]
	parts := strings.Split(body, ",")
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			ids = append(ids, part)
		}
	}
	return ids
}

func extractPlanMarkdown(raw string) string {
	openTag := "<proposed_plan>"
	closeTag := "</proposed_plan>"
	start := strings.Index(raw, openTag)
	end := strings.Index(raw, closeTag)
	if start == -1 || end == -1 || end <= start {
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(raw[start+len(openTag) : end])
}

func summarizeExploration(results []ExplorationResult) string {
	if len(results) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, result := range results {
		builder.WriteString("- ")
		builder.WriteString(result.AgentID)
		builder.WriteString(" [")
		builder.WriteString(result.Status)
		builder.WriteString("]")
		if strings.TrimSpace(result.Result) != "" {
			builder.WriteString(": ")
			builder.WriteString(strings.TrimSpace(result.Result))
		}
		if strings.TrimSpace(result.Error) != "" {
			builder.WriteString(" (")
			builder.WriteString(strings.TrimSpace(result.Error))
			builder.WriteString(")")
		}
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func toFormulaPath(root, path string) string {
	if root == "" {
		return path
	}
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func getVar(variables map[string]string, key string) string {
	if variables == nil {
		return ""
	}
	return variables[key]
}
