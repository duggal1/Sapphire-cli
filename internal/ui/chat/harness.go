package chat

import (
	"encoding/json"
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
	header := toolHeader(sty, opts.Status, "Plan", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	if opts.IsPending() {
		return pendingTool(sty, "Planning")
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
	root := &TreeNode{Label: sty.Tool.ListRoot.Render("Execution")}

	task := oneLine(firstNonEmptyOrchestrationValue(params.Task, contract.Reason))
	if task != "" {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Task", task)})
	}

	root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Route", presentHarnessRoute(contract))})

	if goal := presentHarnessGoal(contract.GoalType); goal != "" {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Goal", goal)})
	}

	if mode := presentHarnessMode(contract.Mode); mode != "" {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Mode", mode)})
	}

	if team := presentHarnessTeam(sty, contract.Agents); team != nil {
		root.Children = append(root.Children, team)
	}

	if skills := presentHarnessSkills(sty, contract.RequiredSkills); skills != nil {
		root.Children = append(root.Children, skills)
	}

	if flow := presentHarnessFlow(contract.Phases); flow != "" {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Flow", flow)})
	}

	if verify := presentHarnessVerify(sty, contract.VerificationPlan); verify != nil {
		root.Children = append(root.Children, verify)
	}

	if next := presentHarnessNextAction(contract.NextAction); next != "" {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Next", next)})
	}

	return strings.Join(renderTreeWithRoot(root, width), "\n")
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

func presentHarnessTeam(sty *styles.Styles, agents []agent.HarnessAgentRole) *TreeNode {
	if len(agents) == 0 {
		return nil
	}
	root := &TreeNode{Label: "Team"}
	for _, entry := range agents {
		name := genericPrettyName(entry.Name)
		role := strings.TrimSpace(entry.Role)
		if role == "" {
			root.Children = append(root.Children, &TreeNode{Label: sty.Tool.ListFile.Render(name)})
			continue
		}
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel(name, oneLine(role))})
	}
	return root
}

func presentHarnessSkills(sty *styles.Styles, skills []string) *TreeNode {
	if len(skills) == 0 {
		return nil
	}
	root := &TreeNode{Label: "Skills"}
	for _, skill := range skills {
		skill = strings.TrimSpace(skill)
		if skill == "" {
			continue
		}
		root.Children = append(root.Children, &TreeNode{Label: sty.Tool.ListFile.Render(skill)})
	}
	if len(root.Children) == 0 {
		return nil
	}
	return root
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

func presentHarnessVerify(sty *styles.Styles, steps []string) *TreeNode {
	if len(steps) == 0 {
		return nil
	}
	root := &TreeNode{Label: "Verify"}
	for _, step := range steps {
		step = oneLine(step)
		if step == "" {
			continue
		}
		root.Children = append(root.Children, &TreeNode{Label: sty.Tool.ListFile.Render(step)})
	}
	if len(root.Children) == 0 {
		return nil
	}
	return root
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
