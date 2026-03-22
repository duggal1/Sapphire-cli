package chat

import (
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
	_ = sty
	_ = width
	_ = opts
	return ""
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
