// Package chat provides UI components and message items for the chat interface.
package chat

// ViewToolMessageItem handles file view summaries and image rendering.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/fsext"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
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

	toolTitle := "View"
	if opts.ToolCall.Name == tools.SingleViewToolName {
		toolTitle = "Single View"
	} else if opts.ToolCall.Name == tools.AgenticViewToolName {
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
			toolParams = append(toolParams, fmt.Sprintf("reading %d code files in parallel", len(filePaths)))
		} else if len(filePaths) == 1 {
			toolParams = append(toolParams, fsext.PrettyPath(filePaths[0]))
		}
	} else if len(filePaths) > 0 {
		toolParams = append(toolParams, fsext.PrettyPath(filePaths[0]))
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

	var meta tools.ViewResponseMetadata
	if opts.HasResult() && opts.Result.Metadata != "" {
		_ = json.Unmarshal([]byte(opts.Result.Metadata), &meta)
	}
	lineCounts := viewLineCounts(&meta)
	fileInfos := buildViewFileInfos(filePaths, lineCounts)

	body := renderViewSummary(sty, opts.ToolCall.Name, fileInfos, cappedWidth-toolBodyLeftPaddingTotal)
	if body == "" {
		return header
	}
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

func renderViewSummary(sty *styles.Styles, toolName string, infos []viewFileInfo, width int) string {
	if len(infos) == 0 {
		return ""
	}
	if toolName != tools.AgenticViewToolName && len(infos) == 1 {
		return renderSingleViewTree(sty, infos[0], width)
	}
	return renderViewTree(sty, infos, width)
}

type viewFileInfo struct {
	path  string
	lines int
}

func buildViewFileInfos(paths []string, lineCounts map[string]int) []viewFileInfo {
	seen := map[string]struct{}{}
	out := make([]viewFileInfo, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		rel := normalizeUIPath(path)
		if rel == "" {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		out = append(out, viewFileInfo{path: rel, lines: lineCounts[path]})
	}
	return out
}

func viewLineCounts(meta *tools.ViewResponseMetadata) map[string]int {
	out := map[string]int{}
	if meta == nil {
		return out
	}
	if meta.FilePath != "" && meta.Content != "" {
		out[meta.FilePath] = strings.Count(meta.Content, "\n") + 1
	}
	for _, f := range meta.Files {
		if f.FilePath == "" || f.Content == "" {
			continue
		}
		out[f.FilePath] = strings.Count(f.Content, "\n") + 1
	}
	return out
}

func renderSingleViewTree(sty *styles.Styles, info viewFileInfo, width int) string {
	fileStyle := sty.Files.Path.Copy().Foreground(lipgloss.Color("#F6DDF5"))
	root := &fileTreeNode{children: map[string]*fileTreeNode{}}
	addFileNode(root, info.path, info.lines)
	var lines []string
	renderTreeLinesWithStyle(&lines, root, "", width, sty, fileStyle)
	return strings.Join(lines, "\n")
}

type fileTreeNode struct {
	name     string
	lines    int
	children map[string]*fileTreeNode
	isFile   bool
}

func renderViewTree(sty *styles.Styles, infos []viewFileInfo, width int) string {
	root := &fileTreeNode{children: map[string]*fileTreeNode{}}
	for _, info := range infos {
		addFileNode(root, info.path, info.lines)
	}
	var lines []string
	renderTreeLines(&lines, root, "", width, sty)
	return strings.Join(lines, "\n")
}

func addFileNode(root *fileTreeNode, rel string, lineCount int) {
	parts := strings.Split(rel, "/")
	curr := root
	for i, part := range parts {
		if part == "" {
			continue
		}
		child, ok := curr.children[part]
		if !ok {
			child = &fileTreeNode{name: part, children: map[string]*fileTreeNode{}}
			curr.children[part] = child
		}
		curr = child
		if i == len(parts)-1 {
			curr.isFile = true
			curr.lines = lineCount
		}
	}
}

func renderTreeLines(out *[]string, node *fileTreeNode, indent string, width int, sty *styles.Styles) {
	fileStyle := sty.Files.Path.Copy().Foreground(lipgloss.Color("#F6DDF5"))
	renderTreeLinesWithStyle(out, node, indent, width, sty, fileStyle)
}

func renderTreeLinesWithStyle(out *[]string, node *fileTreeNode, indent string, width int, sty *styles.Styles, fileStyle lipgloss.Style) {
	if node == nil || len(node.children) == 0 {
		return
	}
	var dirs []string
	var files []string
	for name, child := range node.children {
		if child.isFile {
			files = append(files, name)
		} else {
			dirs = append(dirs, name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	children := append(dirs, files...)
	for i, name := range children {
		child := node.children[name]
		isLast := i == len(children)-1
		branch := "├─ "
		nextIndent := indent + "│  "
		if isLast {
			branch = "└─ "
			nextIndent = indent + "   "
		}
		label := child.name
		if !child.isFile {
			label += "/"
		}
		lineWidth := width - ansi.StringWidth(indent+branch)
		if lineWidth < 0 {
			lineWidth = 0
		}
		var rendered string
		if child.isFile {
			nameText := label
			countText := ""
			if child.lines > 0 {
				countText = fmt.Sprintf(" (%d lines)", child.lines)
			}
			if countText != "" && lineWidth > ansi.StringWidth(countText) {
				nameText = ansi.Truncate(nameText, lineWidth-ansi.StringWidth(countText), "…")
				rendered = fileStyle.Render(nameText) + sty.Tool.ListMeta.Render(countText)
			} else {
				rendered = fileStyle.Render(ansi.Truncate(nameText+countText, lineWidth, "…"))
			}
		} else {
			rendered = sty.Tool.ListDirectory.Render(ansi.Truncate(label, lineWidth, "…"))
		}
		*out = append(*out, indent+branch+rendered)
		renderTreeLinesWithStyle(out, child, nextIndent, width, sty, fileStyle)
	}
}

func normalizeUIPath(path string) string {
	clean := filepath.Clean(path)
	wd, err := os.Getwd()
	if err == nil {
		if rel, err := filepath.Rel(wd, clean); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	clean = filepath.ToSlash(clean)
	if strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "~") {
		parts := strings.Split(strings.TrimPrefix(clean, "/"), "/")
		if len(parts) >= 2 {
			return strings.Join(parts[len(parts)-2:], "/")
		}
	}
	return clean
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
	if opts.IsPending() {
		title := "Edit"
		if opts.ToolCall.Name == tools.SingleEditToolName {
			title = "Single Edit"
		}
		return pendingTool(sty, title, opts.Anim)
	}

	var params tools.EditParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, width)
	}

	file := fsext.PrettyPath(params.FilePath)
	title := "Edit"
	if opts.ToolCall.Name == tools.SingleEditToolName {
		title = "Single Edit"
	}
	header := toolHeader(sty, opts.Status, title, width, opts.Compact, file)
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

	// Render diff.
	body := toolOutputDiffContent(sty, file, meta.OldContent, meta.NewContent, width, opts.ExpandedContent)
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
		toolParams = append(toolParams, fmt.Sprintf("editing %d code files in parallel", len(params.FileEdits)))
		file = fmt.Sprintf("%d files", len(params.FileEdits)) // for diff content if needed
	} else {
		file = params.FilePath
		if len(params.FileEdits) == 1 {
			file = params.FileEdits[0].FilePath
		}
		toolParams = append(toolParams, fsext.PrettyPath(file))

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
		for _, f := range meta.Files {
			prettyPath := fsext.PrettyPath(f.FilePath)
			diffBody := toolOutputMultiEditDiffContent(sty, prettyPath, f, width, opts.ExpandedContent)
			bodies = append(bodies, diffBody)
		}
	} else if meta.OldContent != "" || meta.NewContent != "" {
		// Legacy single file handled here
		prettyPath := fsext.PrettyPath(file)
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
