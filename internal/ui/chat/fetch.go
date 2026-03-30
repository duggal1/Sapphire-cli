// Package chat provides UI components and message items for the chat interface.
package chat

// FetchToolMessageItem manages the UI for web content retrieval.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/shimmer"
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
	nameStyle := searchHeaderTagStyle(sty)
	iconStyle := sty.Base.Foreground(sty.Primary)
	if opts.IsPending() {
		return renderPendingSearchHeader(sty, "Search", "Searching the web", nameStyle, iconStyle)
	}

	var params tools.WebSearchParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	header := toolHeaderWithNameStyle(sty, opts.Status, "Search", cappedWidth, nameStyle, iconStyle, webSearchHeaderLabel(params))
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.HasEmptyResult() {
		return header
	}

	if opts.Result != nil && opts.Result.IsError {
		return joinToolParts(header, toolErrorContent(sty, opts.Result, cappedWidth))
	}

	results, parsed, noResults := parseWebSearchResultsContent(opts.Result.Content)
	if parsed {
		body := renderWebSearchResultsTree(sty, cappedWidth, results, noResults, opts.ExpandedContent)
		return joinToolParts(header, body)
	}

	grouped, ok := parseParallelWebSearchResultsContent(opts.Result.Content)
	if ok {
		body := renderParallelWebSearchResultsTree(sty, cappedWidth, grouped, opts.ExpandedContent)
		return joinToolParts(header, body)
	}

	body := toolOutputMarkdownContent(sty, opts.Result.Content, cappedWidth, opts.ExpandedContent)
	return joinToolParts(header, body)
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
	nameStyle := searchHeaderTagStyle(sty)
	iconStyle := sty.Base.Foreground(sty.Primary)
	if opts.IsPending() {
		return renderPendingSearchHeader(sty, "Google Grounding", "Searching with Google grounding", nameStyle, iconStyle)
	}

	var params tools.GoogleSearchParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	header := toolHeaderWithNameStyle(sty, opts.Status, "Google Grounding", cappedWidth, nameStyle, iconStyle, googleSearchHeaderLabel(params))
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.HasEmptyResult() {
		return header
	}

	if opts.Result != nil && opts.Result.IsError {
		return joinToolParts(header, toolErrorContent(sty, opts.Result, cappedWidth))
	}

	results, parsed, noResults := parseWebSearchResultsContent(opts.Result.Content)
	if parsed {
		body := renderWebSearchResultsTree(sty, cappedWidth, results, noResults, opts.ExpandedContent)
		return joinToolParts(header, body)
	}

	grouped, ok := parseParallelWebSearchResultsContent(opts.Result.Content)
	if ok {
		body := renderParallelWebSearchResultsTree(sty, cappedWidth, grouped, opts.ExpandedContent)
		return joinToolParts(header, body)
	}

	grounded, ok := parseGoogleGroundingContent(opts.Result.Content)
	if ok {
		body := renderGoogleGroundingTree(sty, cappedWidth, grounded, opts.ExpandedContent)
		return joinToolParts(header, body)
	}

	body := toolOutputMarkdownContent(sty, opts.Result.Content, cappedWidth, opts.ExpandedContent)
	return joinToolParts(header, body)
}

func webSearchHeaderLabel(params tools.WebSearchParams) string {
	query := strings.TrimSpace(params.Query)
	switch {
	case query != "":
		return query
	case len(params.Queries) > 1:
		return fmt.Sprintf("%s (+%d more)", strings.TrimSpace(params.Queries[0]), len(params.Queries)-1)
	case len(params.Queries) == 1:
		return strings.TrimSpace(params.Queries[0])
	default:
		return "Google Search"
	}
}

func googleSearchHeaderLabel(params tools.GoogleSearchParams) string {
	switch {
	case strings.TrimSpace(params.Query) != "":
		return strings.TrimSpace(params.Query)
	case strings.TrimSpace(params.URL) != "":
		return strings.TrimSpace(params.URL)
	case len(params.URLs) > 0:
		return strings.TrimSpace(params.URLs[0])
	default:
		return "Google Search"
	}
}

var webSearchResultLineRE = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)

type webSearchResult struct {
	Index   int
	Title   string
	URL     string
	Summary string
}

type parallelWebSearchQuery struct {
	Query     string
	Results   []webSearchResult
	NoResults bool
	Raw       string
}

type parallelWebSearchResults struct {
	Queries []parallelWebSearchQuery
	Errors  []string
}

type googleGroundingContent struct {
	Answer        string
	SearchQueries []string
	Sources       []webSearchResult
	URLContexts   []string
}

func parseWebSearchResultsContent(content string) ([]webSearchResult, bool, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, false, false
	}

	if strings.HasPrefix(trimmed, "No results found.") {
		return nil, true, true
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 || !strings.HasPrefix(strings.TrimSpace(lines[0]), "Found ") {
		return nil, false, false
	}

	var results []webSearchResult
	var current *webSearchResult

	flush := func() {
		if current == nil {
			return
		}
		results = append(results, *current)
		current = nil
	}

	for _, line := range lines[1:] {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		if match := webSearchResultLineRE.FindStringSubmatch(trimmedLine); match != nil {
			flush()
			index, err := strconv.Atoi(match[1])
			if err != nil {
				continue
			}
			current = &webSearchResult{
				Index: index,
				Title: strings.TrimSpace(match[2]),
			}
			continue
		}

		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(trimmedLine, "URL: "):
			current.URL = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "URL: "))
		case strings.HasPrefix(trimmedLine, "Summary: "):
			current.Summary = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "Summary: "))
		}
	}

	flush()

	if len(results) == 0 {
		return nil, false, false
	}

	return results, true, false
}

func parseParallelWebSearchResultsContent(content string) (parallelWebSearchResults, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || !strings.HasPrefix(trimmed, "Searched ") {
		return parallelWebSearchResults{}, false
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "queries in parallel") {
		return parallelWebSearchResults{}, false
	}

	var parsed parallelWebSearchResults
	var currentQuery string
	var currentLines []string
	readingErrors := false

	flushQuery := func() {
		if strings.TrimSpace(currentQuery) == "" {
			return
		}
		raw := strings.TrimSpace(strings.Join(currentLines, "\n"))
		results, ok, noResults := parseWebSearchResultsContent(raw)
		parsed.Queries = append(parsed.Queries, parallelWebSearchQuery{
			Query:     strings.TrimSpace(currentQuery),
			Results:   results,
			NoResults: noResults,
			Raw:       raw,
		})
		currentQuery = ""
		currentLines = nil
		_ = ok
	}

	for _, line := range lines[1:] {
		trimmedLine := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmedLine, "Query: "):
			flushQuery()
			currentQuery = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "Query: "))
			readingErrors = false
		case trimmedLine == "Errors:":
			flushQuery()
			readingErrors = true
		case readingErrors:
			if trimmedLine != "" {
				parsed.Errors = append(parsed.Errors, strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- ")))
			}
		default:
			if currentQuery != "" {
				currentLines = append(currentLines, line)
			}
		}
	}
	flushQuery()

	if len(parsed.Queries) == 0 && len(parsed.Errors) == 0 {
		return parallelWebSearchResults{}, false
	}
	return parsed, true
}

func parseGoogleGroundingContent(content string) (googleGroundingContent, bool) {
	var parsed googleGroundingContent
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return parsed, false
	}

	lines := strings.Split(trimmed, "\n")
	section := ""
	var current *webSearchResult
	var answerLines []string

	flushSource := func() {
		if current == nil {
			return
		}
		parsed.Sources = append(parsed.Sources, *current)
		current = nil
	}

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		switch {
		case trimmedLine == "Answer:":
			flushSource()
			section = "answer"
		case trimmedLine == "Google search queries:":
			flushSource()
			section = "queries"
		case strings.HasPrefix(trimmedLine, "Grounded web sources"):
			flushSource()
			section = "sources"
		case trimmedLine == "URL context retrieval:":
			flushSource()
			section = "contexts"
		case trimmedLine == "":
			if section == "answer" && len(answerLines) > 0 {
				answerLines = append(answerLines, "")
			}
		default:
			switch section {
			case "answer":
				answerLines = append(answerLines, trimmedLine)
			case "queries":
				if strings.HasPrefix(trimmedLine, "- ") {
					parsed.SearchQueries = append(parsed.SearchQueries, strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- ")))
				}
			case "sources":
				if match := webSearchResultLineRE.FindStringSubmatch(trimmedLine); match != nil {
					flushSource()
					index, err := strconv.Atoi(match[1])
					if err != nil {
						continue
					}
					current = &webSearchResult{
						Index: index,
						Title: strings.TrimSpace(match[2]),
					}
					continue
				}
				if current == nil {
					continue
				}
				if strings.HasPrefix(trimmedLine, "URL: ") {
					current.URL = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "URL: "))
				}
			case "contexts":
				if strings.HasPrefix(trimmedLine, "- ") {
					parsed.URLContexts = append(parsed.URLContexts, strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- ")))
				}
			}
		}
	}
	flushSource()

	parsed.Answer = strings.TrimSpace(strings.Join(answerLines, "\n"))

	return parsed, parsed.Answer != "" || len(parsed.SearchQueries) > 0 || len(parsed.Sources) > 0 || len(parsed.URLContexts) > 0
}

func renderWebSearchResultsTree(sty *styles.Styles, width int, results []webSearchResult, noResults, expanded bool) string {
	bodyWidth := max(0, width-toolBodyLeftPaddingTotal)

	if noResults {
		root := &TreeNode{
			Label: searchRootLabel(sty, "Results"),
			Children: []*TreeNode{
				{Label: searchMutedText(sty, "No results found. Try rephrasing your search.")},
			},
		}
		return truncateSearchTree(sty, bodyWidth, strings.Join(renderTreeWithRoot(root, bodyWidth), "\n"), expanded)
	}

	if len(results) == 0 {
		return ""
	}

	root := &TreeNode{Label: searchRootLabel(sty, fmt.Sprintf("Results · %d", len(results)))}
	for _, result := range results {
		root.Children = append(root.Children, renderSearchResultNode(sty, bodyWidth, result))
	}

	return truncateSearchTree(sty, bodyWidth, strings.Join(renderTreeWithRoot(root, bodyWidth), "\n"), expanded)
}

func renderParallelWebSearchResultsTree(sty *styles.Styles, width int, parsed parallelWebSearchResults, expanded bool) string {
	bodyWidth := max(0, width-toolBodyLeftPaddingTotal)
	root := &TreeNode{Label: searchRootLabel(sty, fmt.Sprintf("Results · %d queries", len(parsed.Queries)))}

	for _, query := range parsed.Queries {
		queryNode := &TreeNode{Label: searchSectionLabel(sty, truncateSearchText(query.Query, max(0, bodyWidth-6)))}
		switch {
		case query.NoResults:
			queryNode.Children = append(queryNode.Children, &TreeNode{Label: searchMutedText(sty, "No results found. Try rephrasing your search.")})
		case len(query.Results) > 0:
			for _, result := range query.Results {
				queryNode.Children = append(queryNode.Children, renderSearchResultNode(sty, bodyWidth-4, result))
			}
		case strings.TrimSpace(query.Raw) != "":
			queryNode.Children = append(queryNode.Children, &TreeNode{Label: searchMutedText(sty, truncateSearchText(oneLine(query.Raw), max(0, bodyWidth-10)))})
		}
		root.Children = append(root.Children, queryNode)
	}

	if len(parsed.Errors) > 0 {
		errorNode := &TreeNode{Label: searchSectionLabel(sty, fmt.Sprintf("Errors · %d", len(parsed.Errors)))}
		for _, err := range parsed.Errors {
			for _, line := range wrapPrefixedText(err, max(1, bodyWidth-8), "", "") {
				errorNode.Children = append(errorNode.Children, &TreeNode{Label: sty.Tool.ErrorMessage.Render(line)})
			}
		}
		root.Children = append(root.Children, errorNode)
	}

	return truncateSearchTree(sty, bodyWidth, strings.Join(renderTreeWithRoot(root, bodyWidth), "\n"), expanded)
}

func renderGoogleGroundingTree(sty *styles.Styles, width int, parsed googleGroundingContent, expanded bool) string {
	bodyWidth := max(0, width-toolBodyLeftPaddingTotal)
	root := &TreeNode{Label: searchRootLabel(sty, "Grounding")}

	if parsed.Answer != "" {
		answerNode := &TreeNode{Label: searchSectionLabel(sty, "Answer")}
		answerWidth := max(1, bodyWidth-10)
		for _, line := range wrapPrefixedText(parsed.Answer, answerWidth, "", "") {
			answerNode.Children = append(answerNode.Children, &TreeNode{Label: searchMutedText(sty, line)})
		}
		root.Children = append(root.Children, answerNode)
	}

	if len(parsed.SearchQueries) > 0 {
		queryNode := &TreeNode{Label: searchSectionLabel(sty, fmt.Sprintf("Queries · %d", len(parsed.SearchQueries)))}
		for _, query := range parsed.SearchQueries {
			queryNode.Children = append(queryNode.Children, &TreeNode{Label: searchMutedText(sty, truncateSearchText(query, max(0, bodyWidth-10)))})
		}
		root.Children = append(root.Children, queryNode)
	}

	if len(parsed.Sources) > 0 {
		sourceNode := &TreeNode{Label: searchSectionLabel(sty, fmt.Sprintf("Sources · %d", len(parsed.Sources)))}
		for _, result := range parsed.Sources {
			sourceNode.Children = append(sourceNode.Children, renderSearchResultNode(sty, bodyWidth-4, result))
		}
		root.Children = append(root.Children, sourceNode)
	}

	if len(parsed.URLContexts) > 0 {
		contextNode := &TreeNode{Label: searchSectionLabel(sty, fmt.Sprintf("URL Context · %d", len(parsed.URLContexts)))}
		for _, entry := range parsed.URLContexts {
			contextNode.Children = append(contextNode.Children, &TreeNode{Label: searchMutedText(sty, truncateSearchText(entry, max(0, bodyWidth-10)))})
		}
		root.Children = append(root.Children, contextNode)
	}

	return truncateSearchTree(sty, bodyWidth, strings.Join(renderTreeWithRoot(root, bodyWidth), "\n"), expanded)
}

func renderSearchResultNode(sty *styles.Styles, width int, result webSearchResult) *TreeNode {
	title := result.Title
	if title == "" {
		title = result.URL
	}
	if result.Index > 0 {
		title = fmt.Sprintf("%d. %s", result.Index, title)
	}

	resultTitleWidth := max(0, width-6)
	resultDetailWidth := max(0, width-10)
	resultNode := &TreeNode{Label: searchSectionLabel(sty, truncateSearchText(title, resultTitleWidth))}

	if result.URL != "" {
		resultNode.Children = append(resultNode.Children, &TreeNode{Label: searchMetaLine(sty, "URL", truncateSearchText(result.URL, resultDetailWidth))})
	}
	if result.Summary != "" {
		resultNode.Children = append(resultNode.Children, &TreeNode{Label: searchMetaLine(sty, "Summary", truncateSearchText(result.Summary, resultDetailWidth))})
	}

	return resultNode
}

func truncateSearchTree(sty *styles.Styles, width int, rendered string, expanded bool) string {
	lines := strings.Split(rendered, "\n")
	maxLines := responseContextHeight
	if expanded {
		maxLines = len(lines)
	}

	if len(lines) > maxLines {
		lines = append(
			lines[:maxLines],
			sty.Tool.ContentTruncation.Width(width).Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-maxLines)),
		)
	}

	return sty.Tool.Body.Render(strings.Join(lines, "\n"))
}

func truncateSearchText(text string, width int) string {
	text = strings.TrimSpace(text)
	if width > 0 && lipgloss.Width(text) > width {
		return ansi.Truncate(text, width, "…")
	}
	return text
}

func searchRootLabel(sty *styles.Styles, text string) string {
	return sty.Base.Bold(true).Padding(0, 1).Background(sty.Primary).Foreground(sty.White).Render(text)
}

func searchSectionLabel(sty *styles.Styles, text string) string {
	return sty.Base.Foreground(sty.FgBase).Bold(true).Render(text)
}

func searchMutedText(sty *styles.Styles, text string) string {
	return sty.Base.Foreground(sty.FgMuted).Render(text)
}

func searchMetaLine(sty *styles.Styles, key, value string) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		sty.Base.Foreground(sty.FgHalfMuted).Bold(true).Render(key),
		" ",
		sty.Base.Foreground(sty.FgMuted).Render(value),
	)
}

func searchHeaderTagStyle(sty *styles.Styles) lipgloss.Style {
	return sty.Base.Bold(true).Padding(0, 1).Background(sty.Primary).Foreground(sty.White)
}

func renderPendingSearchHeader(sty *styles.Styles, name, label string, nameStyle, iconStyle lipgloss.Style) string {
	header := pendingToolWithNameStyle(sty, name, nameStyle, iconStyle)
	loader := shimmer.ShimmerWithDot(label)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		sty.Base.Foreground(sty.FgMuted).Render(loader),
	)
}
