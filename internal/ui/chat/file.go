// Package chat provides UI components and message items for the chat interface.
package chat

// ViewToolMessageItem handles file view summaries and image rendering.

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/fsext"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
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

type lineRange struct {
	start int
	end   int
}

// RenderTool implements the [ToolRenderer] interface.
func (v *ViewToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)

	toolTitle := viewToolTitle(opts.ToolCall.Name)

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

	if opts.IsPending() {
		pendingHeader := pendingTool(sty, toolTitle, opts.Anim)
		body := renderViewSummary(sty, toolTitle, filePaths, params, nil, ToolStatusRunning, cappedWidth-toolBodyLeftPaddingTotal, opts.ToolCall.Name == tools.SingleViewToolName)
		if body == "" {
			return pendingHeader
		}
		return joinToolParts(pendingHeader, sty.Tool.Body.Render(body))
	}

	if !opts.HasResult() {
		body := renderViewSummary(sty, toolTitle, filePaths, params, nil, opts.Status, cappedWidth-toolBodyLeftPaddingTotal, opts.ToolCall.Name == tools.SingleViewToolName)
		if body == "" {
			return header
		}
		return joinToolParts(header, sty.Tool.Body.Render(body))
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

	body := renderViewSummary(sty, toolTitle, filePaths, params, opts.Result, opts.Status, cappedWidth-toolBodyLeftPaddingTotal, opts.ToolCall.Name == tools.SingleViewToolName)
	if body == "" {
		if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
			return joinToolParts(header, earlyState)
		}
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

func viewToolTitle(toolName string) string {
	switch toolName {
	case tools.SingleViewToolName:
		return "View"
	case tools.AgenticViewToolName:
		return "Agentic View"
	default:
		return "View"
	}
}

// renderViewSummary renders structured file context and metadata for view tools.
func renderViewSummary(
	sty *styles.Styles,
	toolTitle string,
	filePaths []string,
	params tools.ViewParams,
	result *message.ToolResult,
	status ToolStatus,
	width int,
	isSingleView bool,
) string {
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

	entries := make([]fileContextEntry, 0, len(normalizedPaths))
	for _, path := range normalizedPaths {
		entry := fileContextEntry{Path: path}
		if r, ok := lineRanges[path]; ok {
			entry.LineStart = r.start
			entry.LineEnd = r.end
		}
		entries = append(entries, entry)
	}

	root := buildViewDetailsRoot(sty, toolTitle, entries, status, result, isSingleView)
	if root == nil {
		return ""
	}
	return strings.Join(renderTreeWithRoot(root, width), "\n")
}

func buildViewDetailsRoot(
	sty *styles.Styles,
	toolTitle string,
	entries []fileContextEntry,
	status ToolStatus,
	result *message.ToolResult,
	isSingleView bool,
) *TreeNode {
	if len(entries) == 0 {
		return nil
	}

	if isSingleView && len(entries) > 1 {
		entries = entries[:1]
	}

	children := make([]*TreeNode, 0, 6)
	if scope := viewScope(entries); scope != "" {
		children = append(children, &TreeNode{Label: subAgentKVLabel("Scope", scope)})
	}

	if isSingleView {
		children = append(children, &TreeNode{Label: subAgentKVLabel("File", renderViewFileLabel(entries[0]))})
	} else {
		children = append(children, &TreeNode{Label: subAgentKVLabel("Files", fmt.Sprintf("%d", len(entries)))})
		if filesTree := buildFileContextRoot(sty, "Files Read", entries); filesTree != nil {
			children = append(children, filesTree)
		}
	}

	children = append(children, &TreeNode{Label: subAgentKVLabel("Status", viewStatusLabel(status, result))})

	purpose := "inspect file context"
	if !isSingleView && len(entries) > 1 {
		purpose = "inspect multi-file context"
	}
	children = append(children, &TreeNode{Label: subAgentKVLabel("Purpose", purpose)})

	if errLine := viewErrorLine(status, result); errLine != "" {
		children = append(children, &TreeNode{Label: subAgentKVLabel("Error", errLine)})
	}

	return &TreeNode{
		Label:    sty.Tool.ListRoot.Render(toolTitle),
		Children: children,
	}
}

func renderViewFileLabel(entry fileContextEntry) string {
	label := filepath.Base(entry.Path)
	if entry.LineStart > 0 && entry.LineEnd >= entry.LineStart {
		label += fmt.Sprintf(" L%d-L%d", entry.LineStart, entry.LineEnd)
	}
	return label
}

func viewScope(entries []fileContextEntry) string {
	if len(entries) == 0 {
		return ""
	}

	if len(entries) == 1 {
		dir := filepath.ToSlash(filepath.Dir(entries[0].Path))
		if dir == "." || dir == "" {
			return filepath.Base(entries[0].Path)
		}
		return dir
	}

	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		dir := filepath.ToSlash(filepath.Dir(entry.Path))
		if dir == "." || dir == "" {
			dir = filepath.Base(entry.Path)
		}
		dirs = append(dirs, dir)
	}

	common := commonSlashPathPrefix(dirs)
	if common != "" && common != "." {
		return common
	}

	seen := make(map[string]struct{})
	unique := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		unique = append(unique, dir)
	}
	if len(unique) == 1 {
		return unique[0]
	}
	if len(unique) == 2 {
		return unique[0] + " + " + unique[1]
	}
	return unique[0] + fmt.Sprintf(" + %d more", len(unique)-1)
}

func commonSlashPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	parts := strings.Split(strings.Trim(paths[0], "/"), "/")
	prefixLen := len(parts)

	for _, path := range paths[1:] {
		current := strings.Split(strings.Trim(path, "/"), "/")
		i := 0
		for i < prefixLen && i < len(current) && parts[i] == current[i] {
			i++
		}
		prefixLen = i
		if prefixLen == 0 {
			return ""
		}
	}

	return strings.Join(parts[:prefixLen], "/")
}

func viewStatusLabel(status ToolStatus, result *message.ToolResult) string {
	switch status {
	case ToolStatusError:
		return "error"
	case ToolStatusCanceled:
		return "canceled"
	case ToolStatusAwaitingPermission:
		return "awaiting permission"
	case ToolStatusRunning:
		return "reading"
	}
	if result == nil {
		return "reading"
	}
	return "read"
}

func viewPurpose(toolTitle string, fileCount int) string {
	switch {
	case toolTitle == "Single View":
		return "inspect file context"
	case fileCount > 1:
		return "inspect multi-file context"
	default:
		return "inspect file context"
	}
}

func viewErrorLine(status ToolStatus, result *message.ToolResult) string {
	if status != ToolStatusError || result == nil {
		return ""
	}
	text := oneLine(result.Content)
	return strings.TrimSpace(text)
}

func resolveLineStart(path string, normalizedPaths []string, params tools.ViewParams) int {
	if len(normalizedPaths) == 1 && params.Offset > 0 && normalizedPaths[0] == path {
		return params.Offset + 1
	}
	return 1
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
