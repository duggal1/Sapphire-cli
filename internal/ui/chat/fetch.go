// Package chat provides UI components and message items for the chat interface.
package chat

// FetchToolMessageItem manages the UI for web content retrieval.

import (
	"encoding/json"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

// -----------------------------------------------------------------------------
// Fetch Tool
// -----------------------------------------------------------------------------

// FetchToolMessageItem is a message item that represents a fetch tool call.
type FetchToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*FetchToolMessageItem)(nil)

// NewFetchToolMessageItem creates a new [FetchToolMessageItem].
func NewFetchToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &FetchToolRenderContext{}, canceled)
}

// FetchToolRenderContext renders fetch tool messages.
type FetchToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (f *FetchToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Fetch")
	}

	var params tools.FetchParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	toolParams := []string{params.URL}
	if params.Format != "" {
		toolParams = append(toolParams, "format", params.Format)
	}
	if params.Timeout != 0 {
		toolParams = append(toolParams, "timeout", formatTimeout(params.Timeout))
	}

	header := toolHeader(sty, opts.Status, "Fetch", cappedWidth, opts.Compact, toolParams...)
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
	root := &TreeNode{Label: sty.Tool.ListRoot.Render("Fetch")}
	root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("URL", params.URL)})
	if params.Format != "" {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Format", params.Format)})
	}
	if params.Timeout != 0 {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Timeout", formatTimeout(params.Timeout))})
	}

	statusStr := "running"
	if opts.HasResult() {
		if opts.Status == ToolStatusError {
			statusStr = "error"
		} else {
			statusStr = "success"
		}
	}
	root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Status", statusStr)})

	if !opts.ExpandedContent {
		summary := renderBashOutputSummary(sty, opts.Result.Content, bodyWidth)
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Output", summary)})
		body := strings.Join(renderTreeWithRoot(root, bodyWidth), "\n")
		return joinToolParts(header, sty.Tool.Body.Render(body))
	}

	root.Children = append(root.Children, &TreeNode{Label: sty.Tool.BashOutputLabel.Render("Output")})
	body := strings.Join(renderTreeWithRoot(root, bodyWidth), "\n")
	file := getFileExtensionForFormat(params.Format)
	outBlock := toolOutputCodeContent(sty, file, opts.Result.Content, 0, bodyWidth, opts.ExpandedContent)
	fullBody := body + "\n" + outBlock

	return joinToolParts(header, sty.Tool.Body.Render(fullBody))
}

// getFileExtensionForFormat returns a filename with appropriate extension for syntax highlighting.
func getFileExtensionForFormat(format string) string {
	switch format {
	case "text":
		return "fetch.txt"
	case "html":
		return "fetch.html"
	default:
		return "fetch.md"
	}
}

// -----------------------------------------------------------------------------
// WebFetch Tool
// -----------------------------------------------------------------------------

// WebFetchToolMessageItem is a message item that represents a web_fetch tool call.
type WebFetchToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*WebFetchToolMessageItem)(nil)

// NewWebFetchToolMessageItem creates a new [WebFetchToolMessageItem].
func NewWebFetchToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &WebFetchToolRenderContext{}, canceled)
}

// WebFetchToolRenderContext renders web_fetch tool messages.
type WebFetchToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (w *WebFetchToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Fetch")
	}

	var params tools.WebFetchParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	toolParams := []string{params.URL}
	header := toolHeader(sty, opts.Status, "Fetch", cappedWidth, opts.Compact, toolParams...)
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
	root := &TreeNode{Label: sty.Tool.ListRoot.Render("Web Fetch")}
	root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("URL", params.URL)})

	statusStr := "running"
	if opts.HasResult() {
		if opts.Status == ToolStatusError {
			statusStr = "error"
		} else {
			statusStr = "success"
		}
	}
	root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Status", statusStr)})

	if !opts.ExpandedContent {
		summary := renderBashOutputSummary(sty, opts.Result.Content, bodyWidth)
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Output", summary)})
		body := strings.Join(renderTreeWithRoot(root, bodyWidth), "\n")
		return joinToolParts(header, sty.Tool.Body.Render(body))
	}

	root.Children = append(root.Children, &TreeNode{Label: sty.Tool.BashOutputLabel.Render("Output")})
	body := strings.Join(renderTreeWithRoot(root, bodyWidth), "\n")
	outBlock := toolOutputMarkdownContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent)
	fullBody := body + "\n" + outBlock

	return joinToolParts(header, sty.Tool.Body.Render(fullBody))
}

// -----------------------------------------------------------------------------
// WebSearch Tool
// -----------------------------------------------------------------------------

// WebSearchToolMessageItem is a message item that represents a web_search tool call.
type WebSearchToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*WebSearchToolMessageItem)(nil)

// NewWebSearchToolMessageItem creates a new [WebSearchToolMessageItem].
func NewWebSearchToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &WebSearchToolRenderContext{}, canceled)
}

// WebSearchToolRenderContext renders web_search tool messages.
type WebSearchToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (w *WebSearchToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Search")
	}

	var params tools.GoogleSearchParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	query := strings.TrimSpace(params.Query)
	if query == "" {
		query = "Google Search"
	}
	toolParams := []string{query}
	header := toolHeader(sty, opts.Status, "Search", cappedWidth, opts.Compact, toolParams...)
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
	root := &TreeNode{Label: sty.Tool.ListRoot.Render("Search")}
	root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Query", query)})

	statusStr := "running"
	if opts.HasResult() {
		if opts.Status == ToolStatusError {
			statusStr = "error"
		} else {
			statusStr = "success"
		}
	}
	root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Status", statusStr)})

	if !opts.ExpandedContent {
		summary := renderBashOutputSummary(sty, opts.Result.Content, bodyWidth)
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Output", summary)})
		body := strings.Join(renderTreeWithRoot(root, bodyWidth), "\n")
		return joinToolParts(header, sty.Tool.Body.Render(body))
	}

	root.Children = append(root.Children, &TreeNode{Label: sty.Tool.BashOutputLabel.Render("Output")})
	body := strings.Join(renderTreeWithRoot(root, bodyWidth), "\n")
	outBlock := toolOutputMarkdownContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent)
	fullBody := body + "\n" + outBlock

	return joinToolParts(header, sty.Tool.Body.Render(fullBody))
}

// -----------------------------------------------------------------------------
// GoogleSearch Tool
// -----------------------------------------------------------------------------

// GoogleSearchToolMessageItem is a message item that represents a native Google grounding search.
type GoogleSearchToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*GoogleSearchToolMessageItem)(nil)

// NewGoogleSearchToolMessageItem creates a new [GoogleSearchToolMessageItem].
func NewGoogleSearchToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &GoogleSearchToolRenderContext{}, canceled)
}

// GoogleSearchToolRenderContext renders google_search grounding messages.
type GoogleSearchToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (g *GoogleSearchToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Google Grounding")
	}

	var params tools.WebSearchParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	toolParams := []string{params.Query}
	header := toolHeader(sty, opts.Status, "Google Grounding", cappedWidth, opts.Compact, toolParams...)
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
	root := &TreeNode{Label: sty.Tool.ListRoot.Render("Google Grounding")}
	root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Query", params.Query)})

	statusStr := "running"
	if opts.HasResult() {
		if opts.Status == ToolStatusError {
			statusStr = "error"
		} else {
			statusStr = "success"
		}
	}
	root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Status", statusStr)})

	if !opts.ExpandedContent {
		summary := renderBashOutputSummary(sty, opts.Result.Content, bodyWidth)
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Output", summary)})
		body := strings.Join(renderTreeWithRoot(root, bodyWidth), "\n")
		return joinToolParts(header, sty.Tool.Body.Render(body))
	}

	root.Children = append(root.Children, &TreeNode{Label: sty.Tool.BashOutputLabel.Render("Output")})
	body := strings.Join(renderTreeWithRoot(root, bodyWidth), "\n")
	outBlock := toolOutputMarkdownContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent)
	fullBody := body + "\n" + outBlock

	return joinToolParts(header, sty.Tool.Body.Render(fullBody))
}
