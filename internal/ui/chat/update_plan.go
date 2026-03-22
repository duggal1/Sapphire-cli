package chat

import (
	"encoding/json"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

// UpdatePlanToolMessageItem renders update_plan as a Codex-style todo list.
type UpdatePlanToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*UpdatePlanToolMessageItem)(nil)

// NewUpdatePlanToolMessageItem creates a new update_plan message item.
func NewUpdatePlanToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &UpdatePlanToolRenderContext{}, canceled)
}

// UpdatePlanToolRenderContext renders update_plan tool messages.
type UpdatePlanToolRenderContext struct{}

// RenderTool implements the ToolRenderer interface.
func (u *UpdatePlanToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)

	var args tools.UpdatePlanArgs
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &args); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	args = tools.NormalizeUpdatePlanArgs(args)
	header := toolHeader(sty, opts.Status, "To-Do", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if len(args.Plan) == 0 {
		return header
	}

	bodyWidth := max(1, cappedWidth-toolBodyLeftPaddingTotal)
	nodes := make([]*TreeNode, 0, len(args.Plan)+1)
	if explanation := deref(args.Explanation); explanation != "" {
		nodes = append(nodes, &TreeNode{
			Label: renderPlanExplanationLabel(sty, explanation, bodyWidth),
		})
	}
	for _, item := range args.Plan {
		nodes = append(nodes, &TreeNode{
			Label: renderPlanStepLabel(sty, item, bodyWidth),
		})
	}
	if len(nodes) == 0 {
		return header
	}

	return joinToolParts(header, sty.Tool.Body.Render(strings.Join(renderTreeLines(nodes, "", bodyWidth), "\n")))
}

func deref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func renderPlanStepLines(sty *styles.Styles, item tools.PlanItem, width int) []string {
	var prefix string
	var render func(string) string
	switch item.Status {
	case tools.StepStatusCompleted:
		prefix = "✔ "
		render = func(s string) string { return sty.Base.Strikethrough(true).Faint(true).Render(s) }
	case tools.StepStatusInProgress:
		prefix = "□ "
		render = func(s string) string { return sty.Base.Foreground(sty.GreenLight).Bold(true).Render(s) }
	default:
		prefix = "□ "
		render = func(s string) string { return sty.Base.Foreground(sty.FgHalfMuted).Render(s) }
	}

	lines := wrapPrefixedText(strings.TrimSpace(item.Step), max(1, width-4), prefix, "  ")
	for i := range lines {
		lines[i] = render(lines[i])
	}
	return lines
}

func renderPlanStepLabel(sty *styles.Styles, item tools.PlanItem, width int) string {
	return strings.Join(renderPlanStepLines(sty, item, max(1, width-4)), "\n")
}

func renderPlanExplanationLabel(sty *styles.Styles, explanation string, width int) string {
	lines := wrapPrefixedText(strings.TrimSpace(explanation), max(1, width-4), "", "")
	for i := range lines {
		lines[i] = sty.Muted.Render(lines[i])
	}
	return strings.Join(lines, "\n")
}
