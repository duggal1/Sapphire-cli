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
	if opts.IsPending() {
		return ""
	}

	if opts.Status == ToolStatusError {
		return ""
	}

	var args tools.UpdatePlanArgs
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &args); err != nil {
		return ""
	}

	if err := tools.ValidatePlanItems(args.Plan); err != nil {
		return ""
	}

	lines := []string{sty.Muted.Render("• ") + sty.Base.Bold(true).Render("To-Do")}

	indented := make([]string, 0, len(args.Plan)+1)
	if expl := strings.TrimSpace(deref(args.Explanation)); expl != "" {
		indented = append(indented, wrapPrefixedText(sty.Base.Faint(true).Italic(true).Render(expl), cappedWidth-4, "", "")...)
	}
	for _, item := range args.Plan {
		indented = append(indented, renderPlanStepLines(sty, item, cappedWidth)...)
	}

	for i, line := range indented {
		prefix := "    "
		if i == 0 {
			prefix = sty.Muted.Render("  └ ")
		}
		lines = append(lines, prefix+line)
	}

	return strings.Join(lines, "\n")
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
		render = func(s string) string { return sty.Base.Foreground(sty.Info).Bold(true).Render(s) }
	default:
		prefix = "□ "
		render = func(s string) string { return sty.Base.Faint(true).Render(s) }
	}

	lines := wrapPrefixedText(strings.TrimSpace(item.Step), max(1, width-4), prefix, "  ")
	for i := range lines {
		lines[i] = render(lines[i])
	}
	return lines
}
