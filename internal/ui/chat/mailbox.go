package chat

import (
	"encoding/json"
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

type mailboxToolMessage struct {
	ID        string    `json:"id"`
	To        string    `json:"to"`
	From      string    `json:"from"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Priority  int       `json:"priority"`
	ThreadID  string    `json:"thread_id"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

type mailboxPresentationItem struct {
	Kind     string
	Title    string
	Summary  string
	Source   string
	Relative string
	Details  []string
}

type AgentMailInboxToolMessageItem struct {
	*baseToolMessageItem
}

func NewAgentMailInboxToolMessageItem(sty *styles.Styles, toolCall message.ToolCall, result *message.ToolResult, canceled bool) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &AgentMailInboxToolRenderContext{}, canceled)
}

type AgentMailInboxToolRenderContext struct{}

func (r *AgentMailInboxToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Mailbox")
	}

	header := toolHeader(sty, opts.Status, "Mailbox", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}
	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	items, err := parseMailboxToolMessages(resultContent(opts))
	if err != nil {
		raw := toolOutputSmartContent(sty, "mailbox.json", resultContent(opts), cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
		return joinToolParts(header, sty.Tool.Body.Render(raw))
	}

	body := renderMailboxBody(sty, items, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

type AgentMailSendToolMessageItem struct {
	*baseToolMessageItem
}

func NewAgentMailSendToolMessageItem(sty *styles.Styles, toolCall message.ToolCall, result *message.ToolResult, canceled bool) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &AgentMailSendToolRenderContext{}, canceled)
}

type AgentMailSendToolRenderContext struct{}

func (r *AgentMailSendToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Send Mail")
	}

	header := toolHeader(sty, opts.Status, "Send Mail", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}
	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(resultContent(opts)), &payload); err != nil {
		raw := toolOutputSmartContent(sty, "mailbox.json", resultContent(opts), cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
		return joinToolParts(header, sty.Tool.Body.Render(raw))
	}

	to := strings.TrimSpace(stringValue(payload["to"]))
	subject := strings.TrimSpace(stringValue(payload["subject"]))
	summary := "Delivered coordination message."
	if subject != "" {
		summary = fmt.Sprintf("Delivered \"%s\".", subject)
	}
	lines := []string{
		renderSubAgentField(sty, "Recipient", firstNonEmptyMailboxText(humanizeMailboxIdentity(to), "unknown")),
		renderSubAgentField(sty, "Summary", summary),
	}
	if threadID := strings.TrimSpace(stringValue(payload["thread_id"])); threadID != "" {
		lines = append(lines, renderSubAgentField(sty, "Thread", shortenMailboxValue(threadID)))
	}
	return joinToolParts(header, sty.Tool.Body.Render(strings.Join(lines, "\n")))
}

func parseMailboxToolMessages(raw string) ([]mailboxToolMessage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var items []mailboxToolMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func renderMailboxBody(sty *styles.Styles, items []mailboxToolMessage, width int, expanded bool) string {
	if len(items) == 0 {
		return renderSubAgentField(sty, "Status", "No coordination messages")
	}
	children := make([]*TreeNode, 0, len(items))
	for _, item := range items {
		children = append(children, renderMailboxNode(sty, item, width, expanded))
	}
	root := &TreeNode{
		Label:    renderSubAgentRootLabel(sty, "Mailbox"),
		Children: children,
	}
	return strings.Join(renderTreeWithRoot(root, width), "\n")
}

func renderMailboxNode(sty *styles.Styles, item mailboxToolMessage, width int, expanded bool) *TreeNode {
	presented := presentMailboxItem(item)
	children := []*TreeNode{
		{Label: renderSubAgentField(sty, "Summary", presented.Summary)},
	}
	if expanded && len(presented.Details) > 0 {
		children = append(children, &TreeNode{
			Label:    renderSubAgentSectionLabel(sty, "Technical Details"),
			Children: renderMailboxDetailChildren(sty, presented.Details),
		})
	}
	return &TreeNode{
		Label:    renderMailboxTitle(sty, presented),
		Children: children,
	}
}

func renderMailboxTitle(sty *styles.Styles, item mailboxPresentationItem) string {
	label := fmt.Sprintf("%s %s", mailboxKindIcon(item.Kind), item.Title)
	parts := []string{label}
	if item.Source != "" {
		parts = append(parts, item.Source)
	}
	if item.Relative != "" {
		parts = append(parts, item.Relative)
	}
	joined := strings.Join(parts, " · ")
	if sty == nil {
		return joined
	}
	return sty.Base.Foreground(mailboxKindColor(sty, item.Kind)).Bold(true).Render(joined)
}

func renderMailboxDetailChildren(sty *styles.Styles, details []string) []*TreeNode {
	children := make([]*TreeNode, 0, len(details))
	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail == "" {
			continue
		}
		children = append(children, &TreeNode{Label: renderSubAgentFieldValue(sty, detail)})
	}
	return children
}

func presentMailboxItem(item mailboxToolMessage) mailboxPresentationItem {
	kind := classifyMailboxKind(item)
	source := mailboxSourceRole(item)
	summary := mailboxSummary(item)
	if summary == "" {
		summary = "Coordination update."
	}
	details := []string{
		fmt.Sprintf("Subject: %s", firstNonEmptyMailboxText(item.Subject, "none")),
		fmt.Sprintf("Source: %s", firstNonEmptyMailboxText(item.From, "unknown")),
	}
	if threadID := strings.TrimSpace(item.ThreadID); threadID != "" {
		details = append(details, fmt.Sprintf("Thread: %s", threadID))
	}
	if body := strings.TrimSpace(item.Body); body != "" {
		details = append(details, "Body:", body)
	}
	title := kind
	return mailboxPresentationItem{
		Kind:     kind,
		Title:    title,
		Summary:  summary,
		Source:   source,
		Relative: relativeMailboxTime(item.CreatedAt),
		Details:  details,
	}
}

func classifyMailboxKind(item mailboxToolMessage) string {
	subject := strings.ToUpper(strings.TrimSpace(item.Subject))
	body := strings.ToUpper(strings.TrimSpace(item.Body))
	switch {
	case strings.Contains(subject, "SUBAGENT_DONE"), strings.Contains(subject, "VALIDATED"):
		return "Completed"
	case strings.Contains(subject, "BLOCKED"):
		return "Blocked"
	case strings.Contains(subject, "NEEDS_FOLLOWUP"):
		return "Needs Review"
	case strings.Contains(subject, "TIMED_OUT"), strings.Contains(subject, "TIMED OUT"), strings.Contains(subject, "TIMEOUT"), strings.Contains(body, "TIMED OUT"):
		return "Timed Out"
	case strings.Contains(subject, "RETRY"):
		return "Retrying"
	case strings.Contains(subject, "CRITICAL"):
		return "Action Required"
	case strings.Contains(subject, "SUPERVISOR"), strings.Contains(subject, "LOOP"):
		return "Supervisor Notice"
	case strings.Contains(body, "DEPENDENCY"):
		return "Waiting on Dependency"
	default:
		return "Update"
	}
}

func mailboxSourceRole(item mailboxToolMessage) string {
	if assignment := extractMailboxField(item.Body, "Assignment"); assignment != "" {
		return assignment
	}
	return humanizeMailboxIdentity(item.From)
}

func mailboxSummary(item mailboxToolMessage) string {
	if summary := extractMailboxField(item.Body, "Summary"); summary != "" {
		return summary
	}
	if blockers := extractMailboxField(item.Body, "Blockers"); blockers != "" && !strings.EqualFold(blockers, "none") {
		return blockers
	}
	if next := extractMailboxField(item.Body, "Next"); next != "" && !strings.EqualFold(next, "none") {
		return next
	}
	lines := strings.Split(strings.TrimSpace(item.Body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "AGENT:") || strings.HasPrefix(upper, "ASSIGNMENT:") || strings.HasPrefix(upper, "SUBMISSION:") || strings.HasPrefix(upper, "STATUS:") || strings.HasPrefix(upper, "TASK:") || strings.HasPrefix(upper, "PROGRESS:") || strings.HasPrefix(upper, "FILES:") || strings.HasPrefix(upper, "COMMANDS:") || strings.HasPrefix(upper, "RISKS:") || strings.HasPrefix(upper, "NEXT:") || strings.HasPrefix(upper, "BLOCKERS:") {
			continue
		}
		return oneLine(line)
	}
	return oneLine(item.Body)
}

func extractMailboxField(body, field string) string {
	prefix := strings.ToUpper(strings.TrimSpace(field)) + ":"
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return ""
}

func humanizeMailboxIdentity(raw string) string {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return ""
	case raw == "supervisor":
		return "Supervisor"
	case strings.HasPrefix(raw, "main:"):
		return "Main Agent"
	case strings.HasPrefix(raw, "agent-"):
		return "Sub-Agent"
	default:
		return strings.ReplaceAll(raw, "_", " ")
	}
}

func relativeMailboxTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	d := time.Since(ts).Round(time.Second)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	default:
		return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
	}
}

func mailboxKindIcon(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "completed":
		return "✓"
	case "blocked", "timed out", "action required":
		return "!"
	case "retrying":
		return "↻"
	case "waiting on dependency":
		return "…"
	default:
		return "•"
	}
}

func mailboxKindColor(sty *styles.Styles, kind string) color.Color {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "completed":
		return sty.Primary
	case "blocked", "timed out", "action required":
		return sty.Error
	case "supervisor notice", "retrying":
		return sty.Warning
	default:
		return sty.Primary
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func firstNonEmptyMailboxText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func shortenMailboxValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 24 {
		return value
	}
	return value[:24] + "…"
}
