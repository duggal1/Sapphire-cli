package chat

import (
	"encoding/json"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
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
		bullet := lipgloss.NewStyle().Faint(true).Render("• ")
		title := lipgloss.NewStyle().Bold(true).Render("Plan update failed")
		return bullet + title
	}

	var args tools.UpdatePlanArgs
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &args)

	var contentLines []string
	wrapWidth := max(1, cappedWidth-6) // Account for box paddings

	if expl := strings.TrimSpace(deref(args.Explanation)); expl != "" {
		noteStyle := lipgloss.NewStyle().Faint(true).Italic(true)
		for _, line := range wrapWithIndent(expl, wrapWidth, "", "") {
			contentLines = append(contentLines, noteStyle.Render(line))
		}
		contentLines = append(contentLines, "") // spacer
	}

	if len(args.Plan) == 0 {
		contentLines = append(contentLines, lipgloss.NewStyle().Faint(true).Italic(true).Render("(no steps provided)"))
	} else {
		for _, item := range args.Plan {
			step := strings.TrimSpace(item.Step)
			if step == "" {
				continue
			}
			box, style := planStepStyle(item.Status)
			for i, line := range wrapWithIndent(step, wrapWidth-2, box, "  ") {
				if i == 0 {
					contentLines = append(contentLines, style.Render(line))
				} else {
					contentLines = append(contentLines, style.Render(line))
				}
			}
		}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(sty.Tool.ContentLine.GetForeground()).
		Padding(0, 1).
		Width(cappedWidth - toolBodyLeftPaddingTotal)

	contentStr := strings.Join(contentLines, "\n")
	boxedContent := boxStyle.Render(contentStr)

	bullet := lipgloss.NewStyle().Faint(true).Render("• ")
	title := lipgloss.NewStyle().Bold(true).Render("Updated Plan")
	headerLine := bullet + title

	return headerLine + "\n" + boxedContent
}

func planStepStyle(status tools.StepStatus) (string, lipgloss.Style) {
	switch status {
	case tools.StepStatusCompleted:
		return "✔ ", lipgloss.NewStyle().Strikethrough(true).Faint(true)
	case tools.StepStatusInProgress:
		return "□ ", lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	default:
		return "□ ", lipgloss.NewStyle().Faint(true)
	}
}

func wrapWithIndent(text string, width int, initialIndent, subsequentIndent string) []string {
	if width <= 0 {
		return []string{initialIndent + text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{initialIndent}
	}

	lines := []string{}
	current := initialIndent
	currentWidth := ansi.StringWidth(initialIndent)
	indentWidth := currentWidth
	for i, word := range words {
		wordWidth := ansi.StringWidth(word)
		sep := 0
		if currentWidth > indentWidth {
			sep = 1
		}
		if currentWidth+sep+wordWidth > width && currentWidth > indentWidth {
			lines = append(lines, current)
			current = subsequentIndent + word
			currentWidth = ansi.StringWidth(subsequentIndent) + wordWidth
			indentWidth = ansi.StringWidth(subsequentIndent)
			continue
		}
		if i == 0 && currentWidth == indentWidth {
			current += word
			currentWidth += wordWidth
		} else {
			if sep == 1 {
				current += " "
				currentWidth++
			}
			current += word
			currentWidth += wordWidth
		}
	}
	lines = append(lines, current)
	return lines
}

func deref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
