package chat

import (
	"encoding/json"
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

type subAgentSpawnResult struct {
	AgentID      string    `json:"agent_id"`
	SubmissionID string    `json:"submission_id,omitempty"`
	Status       string    `json:"status,omitempty"`
	WorkDir      string    `json:"work_dir,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
}

type subAgentStatusEntry struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	SubmissionID string    `json:"submission_id,omitempty"`
	WorkDir      string    `json:"work_dir,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
}

type subAgentWaitResult struct {
	Agents   []subAgentStatusEntry `json:"agents"`
	TimedOut bool                  `json:"timed_out"`
}

type subAgentCollectedResult struct {
	ID           string    `json:"id"`
	SubmissionID string    `json:"submission_id,omitempty"`
	Status       string    `json:"status"`
	Result       string    `json:"result,omitempty"`
	Error        string    `json:"error,omitempty"`
	Progress     string    `json:"progress,omitempty"`
	WorkDir      string    `json:"work_dir,omitempty"`
	Branch       string    `json:"branch,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
}

type subAgentCollectResult struct {
	Agents []subAgentCollectedResult `json:"agents"`
}

type subAgentReport struct {
	Status   string
	Summary  string
	Progress string
	Files    []string
	Commands []string
	Risks    string
	Next     string
	Blockers string
}

// -----------------------------------------------------------------------------
// Resume Agent Tool
// -----------------------------------------------------------------------------

type ResumeAgentToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*ResumeAgentToolMessageItem)(nil)

func NewResumeAgentToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &ResumeAgentToolRenderContext{}, canceled)
}

type ResumeAgentToolRenderContext struct{}

func (r *ResumeAgentToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	return renderSubAgentSimpleTool(sty, width, opts, "Resume Agent")
}

// -----------------------------------------------------------------------------
// Send Input Tool
// -----------------------------------------------------------------------------

type SendInputToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*SendInputToolMessageItem)(nil)

func NewSendInputToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &SendInputToolRenderContext{}, canceled)
}

type SendInputToolRenderContext struct{}

func (s *SendInputToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	return renderSubAgentSimpleTool(sty, width, opts, "Send Input")
}

// -----------------------------------------------------------------------------
// Wait Agents Tool
// -----------------------------------------------------------------------------

type WaitAgentsToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*WaitAgentsToolMessageItem)(nil)

func NewWaitAgentsToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &WaitAgentsToolRenderContext{}, canceled)
}

type WaitAgentsToolRenderContext struct{}

func (w *WaitAgentsToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Wait Agents")
	}

	header := toolHeader(sty, opts.Status, "Wait Agents", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	var payload subAgentWaitResult
	if err := json.Unmarshal([]byte(resultContent(opts)), &payload); err != nil {
		return header
	}

	body := renderSubAgentWaitBody(sty, payload, cappedWidth-toolBodyLeftPaddingTotal)
	if body == "" {
		return header
	}
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

// -----------------------------------------------------------------------------
// Collect Result Tool
// -----------------------------------------------------------------------------

type CollectResultToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*CollectResultToolMessageItem)(nil)

func NewCollectResultToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &CollectResultToolRenderContext{}, canceled)
}

type CollectResultToolRenderContext struct{}

func (c *CollectResultToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Collect Results")
	}

	header := toolHeader(sty, opts.Status, "Collect Results", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	var payload subAgentCollectResult
	if err := json.Unmarshal([]byte(resultContent(opts)), &payload); err != nil {
		return header
	}

	body := renderSubAgentCollectBody(sty, payload, cappedWidth-toolBodyLeftPaddingTotal)
	if body == "" {
		return header
	}
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

// -----------------------------------------------------------------------------
// Close Agent Tool
// -----------------------------------------------------------------------------

type CloseAgentToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*CloseAgentToolMessageItem)(nil)

func NewCloseAgentToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &CloseAgentToolRenderContext{}, canceled)
}

type CloseAgentToolRenderContext struct{}

func (c *CloseAgentToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	return renderSubAgentSimpleTool(sty, width, opts, "Close Agent")
}

// -----------------------------------------------------------------------------
// Rendering Helpers
// -----------------------------------------------------------------------------

func renderSubAgentSimpleTool(sty *styles.Styles, width int, opts *ToolRenderOpts, title string) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, title)
	}

	header := toolHeader(sty, opts.Status, title, cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	body := renderSubAgentSimpleBody(sty, opts, cappedWidth-toolBodyLeftPaddingTotal)
	if body == "" {
		return header
	}
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

func renderSubAgentSimpleBody(sty *styles.Styles, opts *ToolRenderOpts, width int) string {
	params := parseSubAgentSimpleParams(sty, opts.ToolCall.Input)
	payload := parseSubAgentSimpleResult(resultContent(opts))
	if len(params) == 0 && len(payload) == 0 {
		return ""
	}

	sections := make([]*TreeNode, 0, len(params)+len(payload))
	sections = append(sections, params...)
	sections = append(sections, payload...)

	root := &TreeNode{
		Label:    renderSubAgentRootLabel(sty, "Sub-Agent"),
		Children: sections,
	}
	return strings.Join(renderTreeWithRoot(root, width), "\n")
}

func parseSubAgentSimpleParams(sty *styles.Styles, raw string) []*TreeNode {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var payload struct {
		ID           string   `json:"id"`
		Message      string   `json:"message,omitempty"`
		Interrupt    bool     `json:"interrupt,omitempty"`
		TimeoutMS    int64    `json:"timeout_ms,omitempty"`
		IDs          []string `json:"ids,omitempty"`
		Branch       string   `json:"branch,omitempty"`
		WorktreePath string   `json:"worktree_path,omitempty"`
		WorktreeDir  string   `json:"worktree_dir,omitempty"`
		WorkDir      string   `json:"work_dir,omitempty"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}

	var nodes []*TreeNode
	if payload.ID != "" {
		nodes = append(nodes, &TreeNode{Label: renderSubAgentField(sty, "Agent", shortenSubAgentID(payload.ID))})
	}
	if len(payload.IDs) > 0 {
		children := make([]*TreeNode, 0, len(payload.IDs))
		for _, id := range payload.IDs {
			children = append(children, &TreeNode{Label: renderSubAgentAgentLine(sty, subAgentStatusEntry{ID: id})})
		}
		nodes = append(nodes, &TreeNode{Label: renderSubAgentSectionLabel(sty, "Agents"), Children: children})
	}
	if payload.Message != "" {
		nodes = append(nodes, &TreeNode{Label: renderSubAgentField(sty, "Note", oneLine(payload.Message))})
	}
	if payload.Interrupt {
		nodes = append(nodes, &TreeNode{Label: renderSubAgentField(sty, "Priority", "interrupt now")})
	}
	if payload.TimeoutMS > 0 {
		nodes = append(nodes, &TreeNode{Label: renderSubAgentField(sty, "Wait up to", fmt.Sprintf("%dms", payload.TimeoutMS))})
	}
	if worktreeNode := renderSubAgentWorktreeDetails(sty, firstNonEmptyOrchestrationValue(payload.WorktreePath, payload.WorktreeDir, payload.WorkDir), payload.Branch); worktreeNode != nil {
		nodes = append(nodes, worktreeNode)
	}

	return nodes
}

func parseSubAgentSimpleResult(raw string) []*TreeNode {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}

	var nodes []*TreeNode
	sty := (*styles.Styles)(nil)
	if id, ok := payload["agent_id"].(string); ok && id != "" {
		nodes = append(nodes, &TreeNode{Label: renderSubAgentField(sty, "Agent", shortenSubAgentID(id))})
	}
	if submissionID, ok := payload["submission_id"].(string); ok && submissionID != "" {
		nodes = append(nodes, &TreeNode{Label: renderSubAgentField(sty, "Run", shortenSubAgentID(submissionID))})
	}
	if status, ok := payload["status"].(string); ok && status != "" {
		nodes = append(nodes, &TreeNode{Label: renderSubAgentField(sty, "State", humanizeSubAgentStatus(status))})
	}
	worktree := ""
	for _, key := range []string{"worktree_path", "worktree_dir", "worktree_directory", "work_dir"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			worktree = value
			break
		}
	}
	branch := ""
	if value, ok := payload["branch"].(string); ok {
		branch = value
	}
	if worktreeNode := renderSubAgentWorktreeDetails(nil, worktree, branch); worktreeNode != nil {
		nodes = append(nodes, worktreeNode)
	}

	return nodes
}

func renderSubAgentSpawnBody(sty *styles.Styles, params *agent.SpawnAgentParams, payload *subAgentSpawnResult, width int) string {
	sections := make([]*TreeNode, 0, 8)

	if params != nil {
		if params.Title != "" {
			sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Task", params.Title)})
		}
		if params.Agent != "" {
			sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Profile", params.Agent)})
		}
		if params.Model != "" {
			sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Model", params.Model)})
		}
		if params.ReasoningEffort != "" {
			sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Reasoning", params.ReasoningEffort)})
		}
		if worktreeNode := renderWorktreeNode(sty, params.Worktree, params.WorktreePath, params.Branch); worktreeNode != nil {
			sections = append(sections, worktreeNode)
		}
		if len(params.WriteManifest) > 0 {
			if writeScope := buildFileContextRoot(sty, "Write Scope", fileContextEntriesFromPaths(params.WriteManifest)); writeScope != nil {
				sections = append(sections, writeScope)
			}
		}
		if params.DefinitionOfDone != "" {
			sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Done when", oneLine(params.DefinitionOfDone))})
		}
	}

	if payload != nil {
		if payload.AgentID != "" || payload.Status != "" {
			sections = append(sections, &TreeNode{Label: renderSubAgentAgentLine(sty, subAgentStatusEntry{
				ID:        payload.AgentID,
				Status:    payload.Status,
				WorkDir:   payload.WorkDir,
				StartedAt: payload.StartedAt,
			})})
		}
		if payload.AgentID != "" {
			sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Agent", shortenSubAgentID(payload.AgentID))})
		}
		if payload.SubmissionID != "" {
			sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Run", shortenSubAgentID(payload.SubmissionID))})
		}
	}

	if len(sections) == 0 {
		return ""
	}

	root := &TreeNode{
		Label:    renderSubAgentRootLabel(sty, "Sub-Agent"),
		Children: sections,
	}
	return strings.Join(renderTreeWithRoot(root, width), "\n")
}

func renderSubAgentWaitBody(sty *styles.Styles, payload subAgentWaitResult, width int) string {
	if len(payload.Agents) == 0 && !payload.TimedOut {
		return ""
	}

	sections := make([]*TreeNode, 0, 2)
	if len(payload.Agents) > 0 {
		children := make([]*TreeNode, 0, len(payload.Agents))
		for _, entry := range payload.Agents {
			children = append(children, &TreeNode{Label: renderSubAgentAgentLine(sty, entry)})
		}
		sections = append(sections, &TreeNode{Label: renderSubAgentSectionLabel(sty, "Agents"), Children: children})
	}
	if payload.TimedOut {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Wait result", "timed out")})
	}

	root := &TreeNode{Label: renderSubAgentRootLabel(sty, "Sub-Agents"), Children: sections}
	return strings.Join(renderTreeWithRoot(root, width), "\n")
}

func renderSubAgentCollectBody(sty *styles.Styles, payload subAgentCollectResult, width int) string {
	if len(payload.Agents) == 0 {
		return ""
	}

	children := make([]*TreeNode, 0, len(payload.Agents))
	for _, entry := range payload.Agents {
		children = append(children, renderCollectedSubAgent(sty, entry))
	}
	root := &TreeNode{Label: renderSubAgentRootLabel(sty, "Sub-Agents"), Children: children}
	return strings.Join(renderTreeWithRoot(root, width), "\n")
}

func renderCollectedSubAgent(sty *styles.Styles, entry subAgentCollectedResult) *TreeNode {
	sections := make([]*TreeNode, 0, 8)

	if entry.ID != "" || entry.Status != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentAgentLine(sty, subAgentStatusEntry{
			ID:           entry.ID,
			Status:       entry.Status,
			SubmissionID: entry.SubmissionID,
			WorkDir:      entry.WorkDir,
			StartedAt:    entry.StartedAt,
		})})
	}
	if entry.Status != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "State", humanizeSubAgentStatus(entry.Status))})
	}
	if entry.SubmissionID != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Run", shortenSubAgentID(entry.SubmissionID))})
	}
	if entry.WorkDir != "" || entry.Branch != "" {
		worktreeChildren := make([]*TreeNode, 0, 2)
		if entry.WorkDir != "" {
			worktreeChildren = append(worktreeChildren, buildFileContextNodes(sty, fileContextEntriesFromPaths([]string{entry.WorkDir}))...)
		}
		if entry.Branch != "" {
			worktreeChildren = append(worktreeChildren, &TreeNode{Label: renderSubAgentField(sty, "Branch", entry.Branch)})
		}
		sections = append(sections, &TreeNode{Label: renderSubAgentSectionLabel(sty, "Worktree"), Children: worktreeChildren})
	}
	if entry.Progress != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Progress", oneLine(entry.Progress))})
	}
	if entry.Error != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Issue", oneLine(entry.Error))})
	}

	report := parseSubAgentReport(entry.Result)
	if report.Summary != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Result", report.Summary)})
	} else if entry.Result != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Result", oneLine(entry.Result))})
	}
	if len(report.Files) > 0 {
		if filesRead := buildFileContextRoot(sty, "Files Read", fileContextEntriesFromPaths(report.Files)); filesRead != nil {
			sections = append(sections, filesRead)
		}
	}
	if len(report.Commands) > 0 {
		sections = append(sections, &TreeNode{Label: renderSubAgentSectionLabel(sty, "Tool Calls"), Children: renderCommandChildren(report.Commands)})
	}
	if report.Risks != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Risks", oneLine(report.Risks))})
	}
	if report.Next != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Next", oneLine(report.Next))})
	}
	if report.Blockers != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Blockers", oneLine(report.Blockers))})
	}

	label := entry.ID
	if label == "" {
		label = "Sub-Agent"
	}
	return &TreeNode{Label: renderSubAgentAgentTitle(sty, label), Children: sections}
}

func renderWorktreeNode(sty *styles.Styles, enabled *bool, path, branch string) *TreeNode {
	useWorktree := true
	if enabled != nil {
		useWorktree = *enabled
	}
	if !useWorktree {
		return nil
	}
	return renderSubAgentWorktreeDetails(sty, path, branch)
}

func renderSubAgentWorktreeDetails(sty *styles.Styles, path, branch string) *TreeNode {
	children := make([]*TreeNode, 0, 2)
	if path != "" {
		if sty != nil {
			children = buildFileContextNodes(sty, fileContextEntriesFromPaths([]string{path}))
		} else {
			children = append(children, &TreeNode{Label: formatRelativePath(path)})
		}
	} else {
		children = append(children, &TreeNode{Label: renderSubAgentFieldValue(sty, "auto")})
	}
	if branch != "" {
		children = append(children, &TreeNode{Label: renderSubAgentField(sty, "Branch", branch)})
	}
	return &TreeNode{Label: renderSubAgentSectionLabel(sty, "Worktree"), Children: children}
}

func renderPathChildren(paths []string) []*TreeNode {
	children := make([]*TreeNode, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		children = append(children, &TreeNode{Label: formatRelativePath(path)})
	}
	return children
}

func renderCommandChildren(commands []string) []*TreeNode {
	children := make([]*TreeNode, 0, len(commands))
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		children = append(children, &TreeNode{Label: command})
	}
	return children
}

func renderSubAgentRootLabel(sty *styles.Styles, label string) string {
	if sty == nil {
		return label
	}
	return sty.Base.Foreground(sty.Primary).Bold(true).Render(label)
}

func renderSubAgentSectionLabel(sty *styles.Styles, label string) string {
	if sty == nil {
		return label
	}
	return sty.Base.Foreground(sty.Tertiary).Bold(true).Render(label)
}

func renderSubAgentAgentTitle(sty *styles.Styles, id string) string {
	if sty == nil {
		return shortenSubAgentID(id)
	}
	return sty.Base.Foreground(sty.FgBase).Bold(true).Render(shortenSubAgentID(id))
}

func renderSubAgentField(sty *styles.Styles, key, value string) string {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if sty == nil {
		return subAgentKVLabel(key, value)
	}
	keyText := sty.Base.Foreground(sty.FgHalfMuted).Render(key + ":")
	return keyText + " " + renderSubAgentFieldValueForKey(sty, key, value)
}

func renderSubAgentFieldValue(sty *styles.Styles, value string) string {
	if sty == nil {
		return value
	}
	return sty.Base.Foreground(sty.FgBase).Render(value)
}

func renderSubAgentFieldValueForKey(sty *styles.Styles, key, value string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "agent", "run":
		return sty.Base.Foreground(sty.FgBase).Bold(true).Render(value)
	case "state":
		return sty.Base.Foreground(subAgentStatusColor(value)).Bold(true).Render(strings.ToLower(value))
	case "wait result":
		if strings.EqualFold(strings.TrimSpace(value), "timed out") {
			return sty.Base.Foreground(sty.Warning).Bold(true).Render(strings.ToLower(value))
		}
	}
	return renderSubAgentFieldValue(sty, value)
}

func renderSubAgentAgentLine(sty *styles.Styles, entry subAgentStatusEntry) string {
	id := shortenSubAgentID(entry.ID)
	if id == "" {
		id = "agent"
	}
	worktree := subAgentWorktreeSummary(entry.WorkDir)
	status := normalizeSubAgentStatus(entry.Status)
	icon := subAgentStatusIcon(status)
	if sty == nil {
		parts := []string{icon, id, worktree}
		if status != "" {
			parts = append(parts, status)
		}
		if timer := formatSubAgentElapsedAt(entry.StartedAt, time.Now()); timer != "" {
			parts = append(parts, "["+timer+"]")
		}
		return strings.Join(parts, "  ")
	}

	iconText := sty.Base.Foreground(subAgentStatusColor(status)).Render(icon)
	idText := sty.Base.Foreground(sty.FgBase).Bold(true).Render(id)
	worktreeText := sty.Base.Foreground(sty.FgMuted).Render(worktree)
	statusText := sty.Base.Foreground(subAgentStatusColor(status)).Bold(true).Render(status)

	parts := []string{iconText, idText, worktreeText}
	if status != "" {
		parts = append(parts, statusText)
	}
	if timer := formatSubAgentElapsedAt(entry.StartedAt, time.Now()); timer != "" {
		parts = append(parts, sty.Base.Foreground(sty.FgHalfMuted).Render("["+timer+"]"))
	}
	return strings.Join(parts, "  ")
}

func shortenSubAgentID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "agent-")
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func subAgentWorktreeSummary(path string) string {
	if strings.TrimSpace(path) == "" {
		return "worktree/auto"
	}
	rel := formatRelativePath(path)
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	name := rel
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		name = parts[len(parts)-1]
	}
	return "worktree/" + name
}

func normalizeSubAgentStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func humanizeSubAgentStatus(status string) string {
	status = normalizeSubAgentStatus(status)
	switch status {
	case "":
		return ""
	case "queued":
		return "Queued"
	case "running":
		return "Running"
	case "stuck":
		return "Stuck"
	case "completed":
		return "Completed"
	case "error":
		return "Error"
	case "closed":
		return "Closed"
	default:
		return strings.ToUpper(status[:1]) + status[1:]
	}
}

func subAgentStatusIcon(status string) string {
	switch normalizeSubAgentStatus(status) {
	case "completed":
		return "✓"
	case "stuck", "error":
		return "✕"
	case "closed":
		return "○"
	default:
		return "●"
	}
}

func subAgentStatusColor(status string) color.Color {
	switch normalizeSubAgentStatus(status) {
	case "completed", "running":
		return lipgloss.Color("#9FE3C1")
	case "stuck", "error":
		return lipgloss.Color("#E48AA7")
	case "closed":
		return lipgloss.Color("#B8AFBF")
	case "queued":
		return lipgloss.Color("#B9A3E8")
	default:
		return lipgloss.Color("#B9A3E8")
	}
}

func formatSubAgentElapsedAt(start, now time.Time) string {
	if start.IsZero() || now.Before(start) {
		return ""
	}
	d := now.Sub(start).Round(time.Second)
	if d < time.Second {
		return "0s"
	}
	hours := int(d / time.Hour)
	minutes := int((d % time.Hour) / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	switch {
	case hours > 0:
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func subAgentKVLabel(key, value string) string {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return value
	}
	if value == "" {
		return key
	}
	return fmt.Sprintf("%s: %s", key, value)
}

func oneLine(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	return strings.TrimSpace(text)
}

func resultContent(opts *ToolRenderOpts) string {
	if opts.Result == nil {
		return ""
	}
	return strings.TrimSpace(opts.Result.Content)
}

func parseSubAgentReport(content string) subAgentReport {
	report := subAgentReport{}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "STATUS:"):
			report.Status = strings.TrimSpace(strings.TrimPrefix(trimmed, "STATUS:"))
		case strings.HasPrefix(trimmed, "SUMMARY:"):
			report.Summary = strings.TrimSpace(strings.TrimPrefix(trimmed, "SUMMARY:"))
		case strings.HasPrefix(trimmed, "PROGRESS:"):
			report.Progress = strings.TrimSpace(strings.TrimPrefix(trimmed, "PROGRESS:"))
		case strings.HasPrefix(trimmed, "FILES:"):
			report.Files = splitCommaList(strings.TrimPrefix(trimmed, "FILES:"))
		case strings.HasPrefix(trimmed, "COMMANDS:"):
			report.Commands = splitCommaList(strings.TrimPrefix(trimmed, "COMMANDS:"))
		case strings.HasPrefix(trimmed, "RISKS:"):
			report.Risks = strings.TrimSpace(strings.TrimPrefix(trimmed, "RISKS:"))
		case strings.HasPrefix(trimmed, "NEXT:"):
			report.Next = strings.TrimSpace(strings.TrimPrefix(trimmed, "NEXT:"))
		case strings.HasPrefix(trimmed, "BLOCKERS:"):
			report.Blockers = strings.TrimSpace(strings.TrimPrefix(trimmed, "BLOCKERS:"))
		}
	}
	return report
}

func splitCommaList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "none") {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.EqualFold(part, "none") {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
