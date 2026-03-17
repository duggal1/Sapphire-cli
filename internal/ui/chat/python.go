package chat

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

// PythonToolMessageItem represents the UI state for a python code execution.
type PythonToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*PythonToolMessageItem)(nil)

// NewPythonToolMessageItem creates a new [PythonToolMessageItem].
func NewPythonToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &PythonToolRenderContext{}, canceled)
}

// PythonToolRenderContext renders python tool messages.
type PythonToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (p *PythonToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Python Environment", opts.Anim)
	}

	header := toolHeader(sty, opts.Status, "Python Environment", cappedWidth, opts.Compact, "Gemini", "mode", "code_execution")
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	params := tools.PythonToolParams{}
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	metadata := tools.PythonToolResponseMetadata{}
	if opts.Result != nil && opts.Result.Metadata != "" {
		_ = json.Unmarshal([]byte(opts.Result.Metadata), &metadata)
	}

	var sections []string
	if input := strings.TrimSpace(params.Code); input != "" {
		taskHeader := " " + sty.Tool.ResourceName.Render("Task")
		taskBody := toolOutputSmartContent(sty, "task.txt", input, bodyWidth, opts.ExpandedContent)
		sections = append(sections, strings.Join([]string{taskHeader, taskBody}, "\n"))
	}

	code := strings.TrimSpace(metadata.ExecutedCode)
	if code != "" {
		codeHeader := " " + sty.Tool.ResourceName.Render("Executed Python")
		codeBody := toolOutputCodeContent(sty, "script.py", code, 0, bodyWidth, opts.ExpandedContent)
		sections = append(sections, strings.Join([]string{codeHeader, codeBody}, "\n"))
	}

	if opts.HasResult() && opts.Result.Content != "" && opts.Result.Content != "No output" {
		outputHeader := " " + sty.Tool.ResourceName.Render("Execution Result")
		var outputBody string
		if looksLikeMarkdown(opts.Result.Content) {
			outputBody = toolOutputMarkdownContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent)
		} else {
			outputBody = sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
		}
		sections = append(sections, strings.Join([]string{outputHeader, outputBody}, "\n"))
	}

	if len(sections) == 0 {
		fallbackHeader := " " + sty.Tool.ResourceName.Render("Execution Result")
		fallbackBody := sty.Tool.Body.Render(toolOutputPlainContent(sty, "No Python execution details available.", bodyWidth, opts.ExpandedContent))
		sections = append(sections, strings.Join([]string{fallbackHeader, fallbackBody}, "\n"))
	}

	content := strings.Join(sections, "\n")
	return joinToolParts(header, content)
}
