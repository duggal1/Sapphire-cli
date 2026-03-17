package chat

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/planmode"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

type RequestUserInputToolMessageItem struct {
	*baseToolMessageItem
}

func NewRequestUserInputToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &RequestUserInputToolRenderContext{}, canceled)
}

type RequestUserInputToolRenderContext struct{}

func (t *RequestUserInputToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Questions", opts.Anim)
	}

	var params tools.RequestUserInputParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	header := toolHeader(sty, opts.Status, "Questions", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	if !opts.HasResult() || strings.TrimSpace(opts.Result.Content) == "" {
		return header
	}

	var response planmode.Response
	if err := json.Unmarshal([]byte(opts.Result.Content), &response); err != nil {
		return header
	}

	lines := make([]string, 0, len(params.Questions)*2)
	for _, question := range params.Questions {
		lines = append(lines, sty.Base.Bold(true).Render(question.Question))
		answer := response.Answers[question.ID]
		if len(answer.Answers) == 0 {
			lines = append(lines, sty.Subtle.Render("  unanswered"))
			continue
		}
		for _, entry := range answer.Answers {
			if note, ok := strings.CutPrefix(entry, "user_note: "); ok {
				lines = append(lines, sty.Base.Render("  "+note))
				continue
			}
			lines = append(lines, sty.Base.Render("  "+entry))
		}
	}

	return joinToolParts(header, sty.Tool.Body.Render(strings.Join(lines, "\n")))
}
