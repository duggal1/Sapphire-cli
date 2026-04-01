package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

// HarnessToolMessageItem renders run_harness with a concise plan summary.
type HarnessToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*HarnessToolMessageItem)(nil)

// NewHarnessToolMessageItem creates a new run_harness message item.
func NewHarnessToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &HarnessToolRenderContext{}, canceled)
}

// HarnessToolRenderContext renders run_harness without exposing raw contract JSON.
type HarnessToolRenderContext struct{}

// RenderTool implements the ToolRenderer interface.
func (h *HarnessToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	title := "Planned"
	if opts.Status != ToolStatusSuccess {
		title = "Plan"
	}
	header := toolHeader(sty, opts.Status, title, cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	if opts.IsPending() {
		return pendingTool(sty, "Planing..")
	}
	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}
	if !opts.HasResult() || strings.TrimSpace(resultContent(opts)) == "" {
		return header
	}

	var params agent.RunHarnessParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var contract agent.HarnessExecutionContract
	if err := json.Unmarshal([]byte(resultContent(opts)), &contract); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid plan output"}, cappedWidth)
	}

	body := renderHarnessBody(sty, params, contract, cappedWidth-toolBodyLeftPaddingTotal)
	if body == "" {
		return header
	}
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

func renderHarnessBody(sty *styles.Styles, params agent.RunHarnessParams, contract agent.HarnessExecutionContract, width int) string {
	sections := make([]string, 0, 8)
	task := oneLine(firstNonEmptyOrchestrationValue(params.Task, contract.Reason))
	if task != "" {
		sections = append(sections, renderHarnessSection(sty, "Task", task, width))
	}

	sections = append(sections, renderHarnessSection(sty, "Route", presentHarnessRoute(contract), width))

	if goal := presentHarnessGoal(contract.GoalType); goal != "" {
		sections = append(sections, renderHarnessSection(sty, "Goal", goal, width))
	}

	if mode := presentHarnessMode(contract.Mode); mode != "" {
		sections = append(sections, renderHarnessSection(sty, "Mode", mode, width))
	}

	if team := presentHarnessTeam(contract.Agents); team != "" {
		sections = append(sections, renderHarnessSection(sty, "Team", team, width))
	}

	if skills := presentHarnessSkills(contract.RequiredSkills); skills != "" {
		sections = append(sections, renderHarnessSection(sty, "Skills", skills, width))
	}

	if flow := presentHarnessFlow(contract.Phases); flow != "" {
		sections = append(sections, renderHarnessSection(sty, "Flow", flow, width))
	}

	if verify := presentHarnessVerify(contract.VerificationPlan); verify != "" {
		sections = append(sections, renderHarnessSection(sty, "Verify", verify, width))
	}

	if next := presentHarnessNextAction(contract.NextAction); next != "" {
		sections = append(sections, renderHarnessSection(sty, "Next", next, width))
	}

	return strings.Join(sections, "\n\n")
}

func presentHarnessRoute(contract agent.HarnessExecutionContract) string {
	if !contract.Required {
		return "Single agent"
	}
	switch strings.TrimSpace(contract.ExecutionMode) {
	case "agent_team":
		return "Agent team"
	case "planning_only":
		return "Planning only"
	default:
		return "Structured plan"
	}
}

func presentHarnessGoal(goal string) string {
	return genericPrettyName(strings.TrimSpace(goal))
}

func presentHarnessMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "plan_only":
		return "Plan only"
	case "", "execute":
		return ""
	default:
		return genericPrettyName(mode)
	}
}

func presentHarnessTeam(agents []agent.HarnessAgentRole) string {
	if len(agents) == 0 {
		return ""
	}
	parts := make([]string, 0, len(agents))
	for _, entry := range agents {
		name := genericPrettyName(entry.Name)
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, ", ")
}

func presentHarnessSkills(skills []string) string {
	if len(skills) == 0 {
		return ""
	}
	parts := make([]string, 0, len(skills))
	for _, skill := range skills {
		skill = strings.TrimSpace(skill)
		if skill != "" {
			parts = append(parts, skill)
		}
	}
	return strings.Join(parts, ", ")
}

func presentHarnessFlow(phases []string) string {
	if len(phases) == 0 {
		return ""
	}
	parts := make([]string, 0, len(phases))
	for _, phase := range phases {
		phase = strings.TrimSpace(phase)
		if phase == "" {
			continue
		}
		parts = append(parts, strings.ToLower(genericPrettyName(phase)))
	}
	return strings.Join(parts, " -> ")
}

func presentHarnessVerify(steps []string) string {
	if len(steps) == 0 {
		return ""
	}
	parts := make([]string, 0, min(2, len(steps)))
	for _, step := range steps {
		step = oneLine(step)
		if step != "" {
			parts = append(parts, step)
		}
		if len(parts) == 2 {
			break
		}
	}
	return strings.Join(parts, "; ")
}

func presentHarnessNextAction(action string) string {
	switch strings.TrimSpace(action) {
	case "load_skills_then_execute":
		return "Load skills, then execute"
	case "load_skills_then_plan":
		return "Load skills, then plan"
	case "load_skills_then_spawn_agents":
		return "Load skills, then start agents"
	case "spawn_agents":
		return "Start agents"
	case "plan":
		return "Draft the plan"
	case "execute":
		return "Execute"
	default:
		return genericPrettyName(action)
	}
}

func renderHarnessSection(sty *styles.Styles, label, value string, width int) string {
	label = strings.TrimSpace(label)
	value = strings.TrimSpace(value)
	if label == "" || value == "" {
		return ""
	}

	lines := wrapPrefixedText(value, max(1, width), "", "")
	for i := range lines {
		lines[i] = sty.Base.Render(lines[i])
	}

	return fmt.Sprintf("%s\n%s", sty.Subtle.Render(label), strings.Join(lines, "\n"))
}
