package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

type RequestUserInputToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*RequestUserInputToolMessageItem)(nil)

func NewRequestUserInputToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &RequestUserInputToolRenderContext{}, canceled)
}

type RequestUserInputToolRenderContext struct{}

func (r *RequestUserInputToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	header := toolHeader(sty, opts.Status, "Plan Question", cappedWidth, opts.Compact, "clarification needed")
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	var args agenttools.RequestUserInputArgs
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &args); err != nil {
		return header
	}

	bodyWidth := max(0, cappedWidth-toolBodyLeftPaddingTotal)
	sections := make([]string, 0, len(args.Questions))
	for i, question := range args.Questions {
		lines := make([]string, 0, len(question.Options)+2)
		title := strings.TrimSpace(question.Question)
		if title == "" {
			title = fmt.Sprintf("Question %d", i+1)
		}
		lines = append(lines, sty.Tool.NameNormal.Render(fmt.Sprintf("%d. %s", i+1, title)))
		for _, option := range question.Options {
			option = strings.TrimSpace(option)
			if option == "" {
				continue
			}
			lines = append(lines, sty.Muted.Render("  - ")+sty.Base.Render(option))
		}
		if question.IsOther {
			lines = append(lines, sty.Subtle.Render("  - Other"))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}
	if len(sections) == 0 {
		return header
	}

	body := sty.Tool.Body.Render(
		lipgloss.NewStyle().
			Width(bodyWidth).
			Render(strings.Join(sections, "\n\n")),
	)
	return joinToolParts(header, body)
}
