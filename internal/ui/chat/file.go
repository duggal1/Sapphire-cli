// Package chat provides UI components and message items for the chat interface.
package chat

// ViewToolMessageItem handles file view summaries and image rendering.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/fsext"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

// -----------------------------------------------------------------------------
// View Tool
// -----------------------------------------------------------------------------

// ViewToolMessageItem is a message item that represents a view tool call.
type ViewToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*ViewToolMessageItem)(nil)

// NewViewToolMessageItem creates a new [ViewToolMessageItem].
func NewViewToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &ViewToolRenderContext{}, canceled)
}

// ViewToolRenderContext renders view tool messages.
type ViewToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (v *ViewToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)

	// Use "View" for single_view, "Agentic View" for agentic_view
	toolTitle := "View"
	if opts.ToolCall.Name == tools.AgenticViewToolName {
		toolTitle = "Agentic View"
	}

	if opts.IsPending() {
		return pendingTool(sty, toolTitle, opts.Anim)
	}

	var params tools.ViewParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	filePaths := params.FilePaths
	if params.FilePath != "" {
		filePaths = append(filePaths, params.FilePath)
	}

	var toolParams []string
	if opts.ToolCall.Name == tools.AgenticViewToolName {
		if len(filePaths) > 1 {
			toolParams = append(toolParams, fmt.Sprintf("reading %d files", len(filePaths)))
		} else if len(filePaths) == 1 {
			toolParams = append(toolParams, formatFilePath(filePaths[0]))
		}
	} else if len(filePaths) > 0 {
		toolParams = append(toolParams, formatFilePath(filePaths[0]))
	}

	if params.Limit != 0 {
		toolParams = append(toolParams, "limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.Offset != 0 {
		toolParams = append(toolParams, "offset", fmt.Sprintf("%d", params.Offset))
	}

	header := toolHeader(sty, opts.Status, toolTitle, cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() {
		return header
	}

	// Handle image content.
	if opts.Result.Data != "" && strings.HasPrefix(opts.Result.MIMEType, "image/") {
		body := toolOutputImageContent(sty, opts.Result.Data, opts.Result.MIMEType)
		return joinToolParts(header, body)
	}

	// Try to get content from metadata first (contains actual file content).
	if strings.Contains(opts.Result.Metadata, "resource_type") {
		var meta tools.ViewResponseMetadata
		if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil {
			if meta.ResourceType == tools.ViewResourceSkill {
				body := toolOutputSkillContent(sty, meta.ResourceName, meta.ResourceDescription)
				return joinToolParts(header, body)
			}
			if len(meta.Files) > 0 {
				var skills []string
				for _, file := range meta.Files {
					if file.ResourceType == tools.ViewResourceSkill {
						skills = append(skills, toolOutputSkillContent(sty, file.ResourceName, file.ResourceDescription))
					}
				}
				if len(skills) > 0 {
					return joinToolParts(header, strings.Join(skills, "\n"))
				}
			}
		}
	}

	body := renderViewSummary(sty, filePaths, params, opts.Result, cappedWidth-toolBodyLeftPaddingTotal)
	if body == "" {
		return header
	}
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

// formatFilePath returns a concise file path display:
// - Uses basename when unambiguous
// - Uses shortest suffix that disambiguates when needed
// - Avoids full paths unless required
func formatFilePath(path string) string {
	return formatRelativePath(path)
}

// countLines counts the number of lines in content
func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

// renderViewSummary renders a summary of viewed files with line ranges using tree glyphs.
func renderViewSummary(sty *styles.Styles, filePaths []string, params tools.ViewParams, result *message.ToolResult, width int) string {
	normalizedPaths := make([]string, 0, len(filePaths))
	for _, p := range filePaths {
		if p == "" {
			continue
		}
		normalizedPaths = append(normalizedPaths, formatRelativePath(p))
	}
	if len(normalizedPaths) == 0 {
		return ""
	}

	lineRanges := make(map[string]lineRange)
	var meta tools.ViewResponseMetadata
	if result != nil && result.Metadata != "" {
		if err := json.Unmarshal([]byte(result.Metadata), &meta); err == nil {
			for _, file := range meta.Files {
				path := formatRelativePath(file.FilePath)
				if path == "" {
					continue
				}
				lines := countLines(file.Content)
				if lines > 0 {
					start := resolveLineStart(path, normalizedPaths, params)
					lineRanges[path] = lineRange{start: start, end: start + lines - 1}
				}
			}
			if meta.FilePath != "" && meta.Content != "" {
				path := formatRelativePath(meta.FilePath)
				if path != "" {
					lines := countLines(meta.Content)
					if lines > 0 {
						start := resolveLineStart(path, normalizedPaths, params)
						lineRanges[path] = lineRange{start: start, end: start + lines - 1}
					}
				}
			}
		}
	}

	// Fallback for single-file reads when metadata is missing.
	if len(lineRanges) == 0 && len(normalizedPaths) == 1 && params.Limit > 0 {
		start := params.Offset + 1
		lineRanges[normalizedPaths[0]] = lineRange{start: start, end: start + params.Limit - 1}
	}

	tree := buildFileTree(normalizedPaths, lineRanges)
	renderNodes := make([]*TreeNode, 0, len(tree))
	for _, node := range tree {
		renderNodes = append(renderNodes, fileTreeToRenderNode(sty, node, lineRanges))
	}

	lines := renderTreeLines(renderNodes, "", width)
	return strings.Join(lines, "\n")
}

// treeNode represents a node in the file tree
type treeNode struct {
	name      string
	path      string
	lineStart int
	lineEnd   int
	children  []*treeNode
	isFile    bool
}

type lineRange struct {
	start int
	end   int
}

// buildFileTree constructs a tree from a list of file paths
func buildFileTree(paths []string, lineRanges map[string]lineRange) []*treeNode {
	root := &treeNode{name: "", children: []*treeNode{}}

	for _, path := range paths {
		if path == "" {
			continue
		}
		parts := strings.Split(path, "/")
		current := root
		currentPath := ""

		for i, part := range parts {
			if currentPath != "" {
				currentPath += "/"
			}
			currentPath += part

			// Check if this is a file (last part or has extension)
			isFile := i == len(parts)-1

			// Find or create child
			var child *treeNode
			for _, c := range current.children {
				if c.name == part {
					child = c
					break
				}
			}
			if child == nil {
				lineStart := 0
				lineEnd := 0
				if isFile {
					if r, ok := lineRanges[currentPath]; ok {
						lineStart = r.start
						lineEnd = r.end
					}
				}
				child = &treeNode{
					name:      part,
					path:      currentPath,
					lineStart: lineStart,
					lineEnd:   lineEnd,
					isFile:    isFile,
					children:  []*treeNode{},
				}
				current.children = append(current.children, child)
			}
			current = child
		}
	}

	return root.children
}

func resolveLineStart(path string, normalizedPaths []string, params tools.ViewParams) int {
	if len(normalizedPaths) == 1 && params.Offset > 0 && normalizedPaths[0] == path {
		return params.Offset + 1
	}
	return 1
}

func fileTreeToRenderNode(sty *styles.Styles, node *treeNode, ranges map[string]lineRange) *TreeNode {
	label := node.name
	if node.isFile {
		name := sty.Base.Foreground(sty.FgBase).Bold(true).Render(node.name)
		label = name
		if r, ok := ranges[node.path]; ok && r.start > 0 && r.end >= r.start {
			lineInfo := sty.Base.Foreground(sty.Tertiary).Render(fmt.Sprintf(" L%d-L%d", r.start, r.end))
			label += lineInfo
		}
	} else {
		label = sty.Base.Foreground(sty.FgBase).Render(node.name)
	}

	children := make([]*TreeNode, 0, len(node.children))
	for _, child := range node.children {
		children = append(children, fileTreeToRenderNode(sty, child, ranges))
	}

	return &TreeNode{
		Label:    label,
		Children: children,
	}
}

func truncateAgenticViewLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	return strings.TrimRight(line, "\n")
}

// -----------------------------------------------------------------------------
// Write Tool
// -----------------------------------------------------------------------------

// WriteToolMessageItem is a message item that represents a write tool call.
type WriteToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*WriteToolMessageItem)(nil)

// NewWriteToolMessageItem creates a new [WriteToolMessageItem].
func NewWriteToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &WriteToolRenderContext{}, canceled)
}

// WriteToolRenderContext renders write tool messages.
type WriteToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (w *WriteToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Write", opts.Anim)
	}

	var params tools.WriteParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	file := fsext.PrettyPath(params.FilePath)
	header := toolHeader(sty, opts.Status, "Write", cappedWidth, opts.Compact, file)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if params.Content == "" {
		return header
	}

	// Render code content with syntax highlighting.
	body := toolOutputCodeContent(sty, params.FilePath, params.Content, 0, cappedWidth, opts.ExpandedContent)
	return joinToolParts(header, body)
}

// -----------------------------------------------------------------------------
// Edit Tool
// -----------------------------------------------------------------------------

// EditToolMessageItem is a message item that represents an edit tool call.
type EditToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*EditToolMessageItem)(nil)

// NewEditToolMessageItem creates a new [EditToolMessageItem].
func NewEditToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &EditToolRenderContext{}, canceled)
}

// EditToolRenderContext renders edit tool messages.
type EditToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (e *EditToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	// Edit tool uses full width for diffs.
	// Use "Edit" for single_edit, "Agentic Edit" for agentic_edit
	if opts.IsPending() {
		title := "Edit"
		if opts.ToolCall.Name == tools.AgenticEditToolName {
			title = "Agentic Edit"
		}
		return pendingTool(sty, title, opts.Anim)
	}

	var params tools.EditParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, width)
	}

	file := formatFilePath(params.FilePath)
	title := "Edit"
	if opts.ToolCall.Name == tools.AgenticEditToolName {
		title = "Agentic Edit"
	}

	// Add line count info if available
	var toolParams []string
	toolParams = append(toolParams, file)

	header := toolHeader(sty, opts.Status, title, width, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, width); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() {
		return header
	}

	// Get diff content from metadata.
	var meta tools.EditResponseMetadata
	if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err != nil {
		bodyWidth := width - toolBodyLeftPaddingTotal
		body := sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
		return joinToolParts(header, body)
	}

	// Calculate lines edited
	linesEdited := meta.Additions + meta.Removals

	// Render diff with line count info
	body := toolOutputDiffContent(sty, file, meta.OldContent, meta.NewContent, width, opts.ExpandedContent)

	// Add line count summary
	if linesEdited > 0 {
		lineInfo := fmt.Sprintf(" · %d lines changed", linesEdited)
		body = strings.Replace(body, "\n", lineInfo+"\n", 1)
	}

	return joinToolParts(header, body)
}

// -----------------------------------------------------------------------------
// MultiEdit Tool
// -----------------------------------------------------------------------------

// MultiEditToolMessageItem is a message item that represents a multi-edit tool call.
type MultiEditToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*MultiEditToolMessageItem)(nil)

// NewMultiEditToolMessageItem creates a new [MultiEditToolMessageItem].
func NewMultiEditToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &MultiEditToolRenderContext{}, canceled)
}

// MultiEditToolRenderContext renders multi-edit tool messages.
type MultiEditToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (m *MultiEditToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	// MultiEdit tool uses full width for diffs.
	if opts.IsPending() {
		return pendingTool(sty, "Agentic Edit", opts.Anim)
	}

	var params tools.MultiEditParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, width)
	}

	var toolParams []string
	var file string

	if len(params.FileEdits) > 1 {
		toolParams = append(toolParams, fmt.Sprintf("editing %d files", len(params.FileEdits)))
		file = fmt.Sprintf("%d files", len(params.FileEdits)) // for diff content if needed
	} else {
		file = params.FilePath
		if len(params.FileEdits) == 1 {
			file = params.FileEdits[0].FilePath
		}
		toolParams = append(toolParams, formatFilePath(file))

		edits := len(params.Edits)
		if len(params.FileEdits) == 1 {
			edits = len(params.FileEdits[0].Edits)
		}
		if edits > 0 {
			toolParams = append(toolParams, "edits", fmt.Sprintf("%d", edits))
		}
	}

	header := toolHeader(sty, opts.Status, "Agentic Edit", width, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, width); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() {
		return header
	}

	// Get diff content from metadata.
	var meta tools.MultiEditResponseMetadata
	if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err != nil {
		bodyWidth := width - toolBodyLeftPaddingTotal
		body := sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
		return joinToolParts(header, body)
	}

	var bodies []string
	if len(meta.Files) > 0 {
		totalLinesChanged := 0
		for _, f := range meta.Files {
			prettyPath := formatFilePath(f.FilePath)
			linesChanged := f.Additions + f.Removals
			totalLinesChanged += linesChanged
			diffBody := toolOutputMultiEditDiffContent(sty, prettyPath, f, width, opts.ExpandedContent)
			bodies = append(bodies, diffBody)
		}
		// Add aggregate line count for multi-file edits
		if len(meta.Files) > 1 && totalLinesChanged > 0 {
			summary := fmt.Sprintf("\n%s · %d total lines changed",
				sty.HalfMuted.Render("Agentic edit complete:"),
				totalLinesChanged,
			)
			bodies = append(bodies, summary)
		}
	} else if meta.OldContent != "" || meta.NewContent != "" {
		// Legacy single file handled here
		prettyPath := formatFilePath(file)
		// Construct an ad-hoc FileEditMetadata for the legacy helper call
		fileMeta := tools.FileEditMetadata{
			FilePath:     prettyPath,
			OldContent:   meta.OldContent,
			NewContent:   meta.NewContent,
			Additions:    meta.Additions,
			Removals:     meta.Removals,
			EditsApplied: meta.EditsApplied,
			EditsFailed:  meta.EditsFailed,
		}
		bodies = append(bodies, toolOutputMultiEditDiffContent(sty, prettyPath, fileMeta, width, opts.ExpandedContent))
	}

	if len(bodies) == 0 {
		return header
	}

	return joinToolParts(header, strings.Join(bodies, "\n\n"))
}

// -----------------------------------------------------------------------------
// Download Tool
// -----------------------------------------------------------------------------

// DownloadToolMessageItem is a message item that represents a download tool call.
type DownloadToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*DownloadToolMessageItem)(nil)

// NewDownloadToolMessageItem creates a new [DownloadToolMessageItem].
func NewDownloadToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &DownloadToolRenderContext{}, canceled)
}

// DownloadToolRenderContext renders download tool messages.
type DownloadToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (d *DownloadToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Download", opts.Anim)
	}

	var params tools.DownloadParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	toolParams := []string{params.URL}
	if params.FilePath != "" {
		toolParams = append(toolParams, "file_path", fsext.PrettyPath(params.FilePath))
	}
	if params.Timeout != 0 {
		toolParams = append(toolParams, "timeout", formatTimeout(params.Timeout))
	}

	header := toolHeader(sty, opts.Status, "Download", cappedWidth, opts.Compact, toolParams...)
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
