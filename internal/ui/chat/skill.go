package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
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

// SkillToolRenderContext renders load_skill and list_skills tool messages.
type SkillToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (t *SkillToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)

	displayMode := "Skill"
	headerText := "Reading Skill"
	msg := "Activating specialized context"

	switch opts.ToolCall.Name {
	case "list_skills":
		displayMode = "Skills Directory"
		headerText = "Reading Skills Directory"
		msg = "Listing available specialized engineering skills"
	case "search_skills":
		displayMode = "Skill Search"
		headerText = "Searching Skills"
		var params struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)
		msg = "Searching specialized skills"
		if query := strings.TrimSpace(params.Query); query != "" {
			msg = fmt.Sprintf("Searching specialized skills for: %s", query)
		}
	default:
		var params struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)
		skillName := strings.TrimSpace(params.Name)
		if skillName != "" {
			msg = fmt.Sprintf("Activating specialized context: %s Instructions", genericPrettyName(skillName))
		}
	}

	header := toolHeader(sty, opts.Status, "Skill", cappedWidth, opts.Compact, headerText)
	if opts.Compact {
		return header
	}

	// Use our new SkillTag for a premium look
	tag := sty.Tool.SkillTag.Render(displayMode)
	tagWidth := lipgloss.Width(tag)

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

	// Don't display the raw skill file content in the terminal UI.
	// The model reads it internally, but users should not see the raw markdown.
	// if opts.HasResult() && opts.Result.Content != "" {
	// 	// If expanded, show the first few lines of instructions
	// 	resultBody := toolOutputMarkdownContent(sty, opts.Result.Content, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
	// 	return joinToolParts(body, resultBody)
	// }

	return body
}
