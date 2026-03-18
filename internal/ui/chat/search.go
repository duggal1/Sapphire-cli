package chat

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// -----------------------------------------------------------------------------
// Glob Tool
// -----------------------------------------------------------------------------

// GlobToolMessageItem is a message item that represents a glob tool call.
type GlobToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*GlobToolMessageItem)(nil)

// NewGlobToolMessageItem creates a new [GlobToolMessageItem].
func NewGlobToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &GlobToolRenderContext{}, canceled)
}

// GlobToolRenderContext renders glob tool messages.
type GlobToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (g *GlobToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Glob", opts.Anim)
	}

	var params tools.GlobParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	toolParams := []string{params.Pattern}
	if params.Path != "" {
		toolParams = append(toolParams, "path", params.Path)
	}

	header := toolHeader(sty, opts.Status, "Glob", cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() || opts.Result.Content == "" {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	body := renderGlobOutput(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent)
	return joinToolParts(header, body)
}

// -----------------------------------------------------------------------------
// Grep Tool
// -----------------------------------------------------------------------------

// GrepToolMessageItem is a message item that represents a grep tool call.
type GrepToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*GrepToolMessageItem)(nil)

// NewGrepToolMessageItem creates a new [GrepToolMessageItem].
func NewGrepToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &GrepToolRenderContext{}, canceled)
}

// GrepToolRenderContext renders grep tool messages.
type GrepToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (g *GrepToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Grep", opts.Anim)
	}

	var params tools.GrepParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	toolParams := []string{params.Pattern}
	if params.Path != "" {
		toolParams = append(toolParams, "path", params.Path)
	}
	if params.Include != "" {
		toolParams = append(toolParams, "include", params.Include)
	}
	if params.LiteralText {
		toolParams = append(toolParams, "literal", "true")
	}

	header := toolHeader(sty, opts.Status, "Grep", cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.HasEmptyResult() {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	body := renderGrepOutput(sty, params.Pattern, params.LiteralText, opts.Result.Content, bodyWidth, opts.ExpandedContent)
	return joinToolParts(header, body)
}

// -----------------------------------------------------------------------------
// LS Tool
// -----------------------------------------------------------------------------

// LSToolMessageItem is a message item that represents an ls tool call.
type LSToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*LSToolMessageItem)(nil)

// NewLSToolMessageItem creates a new [LSToolMessageItem].
func NewLSToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &LSToolRenderContext{}, canceled)
}

// LSToolRenderContext renders ls tool messages.
type LSToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (l *LSToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "List", opts.Anim)
	}

	var params tools.LSParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	path := params.Path
	if path == "" {
		path = "."
	}
	path = formatRelativePath(path)

	header := toolHeader(sty, opts.Status, "List", cappedWidth, opts.Compact, path)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.HasEmptyResult() {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	body := renderListOutput(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent)
	return joinToolParts(header, body)
}

// -----------------------------------------------------------------------------
// Sourcegraph Tool
// -----------------------------------------------------------------------------

// SourcegraphToolMessageItem is a message item that represents a sourcegraph tool call.
type SourcegraphToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*SourcegraphToolMessageItem)(nil)

// NewSourcegraphToolMessageItem creates a new [SourcegraphToolMessageItem].
func NewSourcegraphToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &SourcegraphToolRenderContext{}, canceled)
}

// SourcegraphToolRenderContext renders sourcegraph tool messages.
type SourcegraphToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (s *SourcegraphToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Sourcegraph", opts.Anim)
	}

	var params tools.SourcegraphParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	toolParams := []string{params.Query}
	if params.Count != 0 {
		toolParams = append(toolParams, "count", formatNonZero(params.Count))
	}
	if params.ContextWindow != 0 {
		toolParams = append(toolParams, "context", formatNonZero(params.ContextWindow))
	}

	header := toolHeader(sty, opts.Status, "Sourcegraph", cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.HasEmptyResult() {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	body := sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
	return joinToolParts(header, body)
}

func renderGlobOutput(sty *styles.Styles, content string, width int, expanded bool) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	maxLines := responseContextHeight
	if expanded {
		maxLines = len(lines)
	}

	var out []string
	for i, line := range lines {
		if i >= maxLines {
			break
		}
		clean := strings.TrimSpace(line)
		isDir := strings.HasSuffix(clean, "/")
		clean = strings.TrimSuffix(clean, "/")
		clean = formatRelativePath(clean)
		if isDir {
			clean += "/"
		}
		clean = ansi.Truncate(clean, width, "…")
		styled := sty.Tool.ListFile.Render(clean)
		if strings.HasSuffix(clean, "/") {
			styled = sty.Tool.ListDirectory.Render(clean)
		}
		out = append(out, sty.Tool.Body.Render(styled))
	}
	if len(lines) > maxLines && !expanded {
		out = append(out, sty.Tool.Body.Render(sty.Tool.ListHint.Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-maxLines))))
	}
	return strings.Join(out, "\n")
}

func renderListOutput(sty *styles.Styles, content string, width int, expanded bool) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	var out []string
	var listBlock []string
	flushListBlock := func() {
		if len(listBlock) == 0 {
			return
		}
		treeNodes := buildListTree(listBlock)
		renderNodes := make([]*TreeNode, 0, len(treeNodes))
		for _, node := range treeNodes {
			renderNodes = append(renderNodes, listTreeToRenderNode(sty, node, true))
		}
		for _, line := range renderTreeLines(renderNodes, "", width) {
			out = append(out, sty.Tool.Body.Render(line))
		}
		listBlock = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- "):
			listBlock = append(listBlock, line)
		default:
			flushListBlock()
			if trimmed == "" {
				out = append(out, "")
				continue
			}
			if strings.HasPrefix(trimmed, "There are more than") || strings.HasPrefix(trimmed, "The directory tree is shown") {
				out = append(out, sty.Tool.Body.Render(sty.Tool.ListHint.Render(ansi.Truncate(trimmed, max(0, width), "…"))))
				continue
			}
			out = append(out, sty.Tool.Body.Render(sty.Tool.ListMeta.Render(ansi.Truncate(trimmed, max(0, width), "…"))))
		}
	}
	flushListBlock()

	if !expanded && len(out) > responseContextHeight {
		hidden := len(out) - responseContextHeight
		out = out[:responseContextHeight]
		out = append(out, sty.Tool.Body.Render(sty.Tool.ListHint.Render(fmt.Sprintf(assistantMessageTruncateFormat, hidden))))
	}
	return strings.Join(out, "\n")
}

type listNode struct {
	name     string
	children []*listNode
}

func buildListTree(lines []string) []*listNode {
	root := &listNode{}
	stack := []*listNode{root}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		indentWidth := len(line) - len(strings.TrimLeft(line, " "))
		level := indentWidth / 2
		if level < 0 {
			level = 0
		}
		name := strings.TrimPrefix(trimmed, "- ")

		for len(stack) > level+1 {
			stack = stack[:len(stack)-1]
		}
		if len(stack) < level+1 {
			level = len(stack) - 1
		}

		node := &listNode{name: name}
		parent := stack[len(stack)-1]
		parent.children = append(parent.children, node)
		stack = append(stack, node)
	}

	return root.children
}

func listTreeToRenderNode(sty *styles.Styles, node *listNode, isRoot bool) *TreeNode {
	name := formatRelativePath(node.name)
	style := sty.Tool.ListFile
	if strings.HasSuffix(name, "/") {
		style = sty.Tool.ListDirectory
	}
	if isRoot {
		style = sty.Tool.ListRoot
	}

	label := style.Render(name)
	children := make([]*TreeNode, 0, len(node.children))
	for _, child := range node.children {
		children = append(children, listTreeToRenderNode(sty, child, false))
	}
	return &TreeNode{
		Label:    label,
		Children: children,
	}
}

var grepLineRE = regexp.MustCompile(`^\s*Line (\d+)(?:, Char (\d+))?: (.*)$`)

func renderGrepOutput(sty *styles.Styles, pattern string, literal bool, content string, width int, expanded bool) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	maxLines := responseContextHeight * 2
	if expanded {
		maxLines = len(lines)
	}

	highlighter := compileGrepHighlighter(pattern, literal)
	var out []string
	for i, line := range lines {
		if i >= maxLines {
			break
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out = append(out, "")
		case strings.HasPrefix(trimmed, "Found "):
			out = append(out, sty.Tool.Body.Render(sty.Tool.ListMeta.Render(ansi.Truncate(trimmed, width, "…"))))
		case strings.HasPrefix(trimmed, "(Results are truncated"):
			out = append(out, sty.Tool.Body.Render(sty.Tool.ListHint.Render(ansi.Truncate(trimmed, width, "…"))))
		case strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "Line "):
			name := strings.TrimSuffix(trimmed, ":")
			name = formatRelativePath(filepath.ToSlash(name))
			out = append(out, "")
			out = append(out, sty.Tool.Body.Render(sty.Tool.GrepFile.Render(ansi.Truncate(name, width, "…"))))
		default:
			if matches := grepLineRE.FindStringSubmatch(trimmed); len(matches) == 4 {
				lineInfo := "Line " + matches[1]
				if matches[2] != "" {
					lineInfo += ", Char " + matches[2]
				}
				rendered := sty.Tool.GrepLine.Render(lineInfo) + " " + highlightGrepText(sty, highlighter, matches[3], width-len(lineInfo)-1)
				out = append(out, sty.Tool.Body.Render(rendered))
			} else {
				out = append(out, sty.Tool.Body.Render(sty.Tool.GrepContext.Render(ansi.Truncate(trimmed, width, "…"))))
			}
		}
	}
	if len(lines) > maxLines && !expanded {
		out = append(out, sty.Tool.Body.Render(sty.Tool.ListHint.Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-maxLines))))
	}
	return strings.Join(out, "\n")
}

func compileGrepHighlighter(pattern string, literal bool) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	if literal {
		return regexp.MustCompile(regexp.QuoteMeta(pattern))
	}
	rx, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return rx
}

func highlightGrepText(sty *styles.Styles, rx *regexp.Regexp, text string, width int) string {
	text = ansi.Truncate(text, max(0, width), "…")
	if rx == nil {
		return sty.Tool.GrepContext.Render(text)
	}
	idx := rx.FindAllStringIndex(text, -1)
	if len(idx) == 0 {
		return sty.Tool.GrepContext.Render(text)
	}
	var b strings.Builder
	last := 0
	for _, match := range idx {
		if match[0] > last {
			b.WriteString(sty.Tool.GrepContext.Render(text[last:match[0]]))
		}
		b.WriteString(sty.Tool.GrepMatch.Render(text[match[0]:match[1]]))
		last = match[1]
	}
	if last < len(text) {
		b.WriteString(sty.Tool.GrepContext.Render(text[last:]))
	}
	return b.String()
}
