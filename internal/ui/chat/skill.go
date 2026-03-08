package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/agent"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

// SkillToolMessageItem is a message item that represents a load_skill tool call.
type SkillToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*SkillToolMessageItem)(nil)

// NewSkillToolMessageItem creates a new [SkillToolMessageItem].
func NewSkillToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &SkillToolRenderContext{}, canceled)
}

// SkillToolRenderContext renders load_skill tool messages.
type SkillToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (t *SkillToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)

	var params agent.LoadSkillParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	skillName := params.Name
	if skillName == "" {
		skillName = "Engineering"
	} else {
		skillName = strings.Title(skillName)
	}

	headerText := fmt.Sprintf("Reading %s Skill", skillName)

	header := toolHeader(sty, opts.Status, "Skill", cappedWidth, opts.Compact, headerText)
	if opts.Compact {
		return header
	}

	// Use our new SkillTag for a premium look
	tag := sty.Tool.SkillTag.Render("SKILL")
	tagWidth := lipgloss.Width(tag)

	// Display the intent in a clean way
	msg := fmt.Sprintf("Activating specialized context: %s Instructions", skillName)
	remainingWidth := cappedWidth - tagWidth - 5
	if remainingWidth < 10 {
		remainingWidth = 10
	}
	content := sty.Base.Width(remainingWidth).Render(msg)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		sty.PanelPadded.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				lipgloss.JoinHorizontal(
					lipgloss.Left,
					tag,
					" ",
					content,
				),
			),
		),
	)

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(body, earlyState)
	}

	if opts.HasResult() && opts.Result.Content != "" {
		// If expanded, show the first few lines of instructions
		resultBody := toolOutputMarkdownContent(sty, opts.Result.Content, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
		return joinToolParts(body, resultBody)
	}

	return body
}
