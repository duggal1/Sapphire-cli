package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func renderBackgroundSubAgentsTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)

	input := agent.BackgroundSubAgentsToolInput{}
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &input)

	payload := agent.BackgroundSubAgentsToolPayload{}
	hasPayload := json.Unmarshal([]byte(resultContent(opts)), &payload) == nil
	if !hasPayload {
		payload = agent.BackgroundSubAgentsToolPayload{
			Status: "launching",
			Title:  input.Title,
			Count:  input.Count,
			Active: input.Count,
		}
	}
	if strings.TrimSpace(payload.Title) == "" {
		payload.Title = firstNonEmptyBackgroundText(input.Title, "Background Sub-Agents")
	}
	if payload.Count == 0 {
		payload.Count = input.Count
	}

	rootLabel := renderBackgroundSubAgentRootLabel(sty, backgroundSubAgentsStatusLabel(payload, opts.IsSpinning))
	root := &TreeNode{
		Label: rootLabel,
		Children: []*TreeNode{
			{Label: renderSubAgentField(sty, "State", humanizeBackgroundStatus(payload.Status, opts.IsSpinning))},
			{Label: renderSubAgentField(sty, "Total", fmt.Sprintf("%d", max(0, payload.Count)))},
			{Label: renderSubAgentField(sty, "Active", fmt.Sprintf("%d", max(0, payload.Active)))},
			{Label: renderSubAgentField(sty, "Completed", fmt.Sprintf("%d", max(0, payload.Completed)))},
		},
	}
	if payload.Failed > 0 {
		root.Children = append(root.Children, &TreeNode{Label: renderSubAgentField(sty, "Failed", fmt.Sprintf("%d", payload.Failed))})
	}

	if len(payload.Agents) > 0 {
		children := make([]*TreeNode, 0, len(payload.Agents))
		for i, entry := range payload.Agents {
			children = append(children, renderBackgroundSubAgentNode(sty, entry, i+1, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent))
		}
		root.Children = append(root.Children, &TreeNode{
			Label:    renderSubAgentSectionLabel(sty, "Sub-Agents"),
			Children: children,
		})
	}

	return strings.Join(renderTreeWithRoot(root, cappedWidth-toolBodyLeftPaddingTotal), "\n")
}

func renderBackgroundSubAgentRootLabel(sty *styles.Styles, label string) string {
	if sty == nil {
		return label
	}
	if strings.TrimSpace(label) == "" {
		label = "Background Sub-Agents"
	}
	if strings.Contains(strings.ToLower(label), "launch") || strings.Contains(strings.ToLower(label), "running") {
		return styles.ShimmerText(sty, label, 0)
	}
	return sty.Base.Foreground(sty.Primary).Bold(true).Render(label)
}

func backgroundSubAgentsStatusLabel(payload agent.BackgroundSubAgentsToolPayload, spinning bool) string {
	switch {
	case spinning && len(payload.Agents) == 0:
		return "Launching Sub-Agents in Background"
	case spinning:
		return "Running Sub-Agents in Background"
	case strings.EqualFold(payload.Status, "failed"):
		return "Background Sub-Agents Finished with Issues"
	default:
		return "Background Sub-Agents"
	}
}

func humanizeBackgroundStatus(status string, spinning bool) string {
	status = strings.TrimSpace(strings.ToLower(status))
	switch {
	case spinning && status == "launching":
		return "Launching"
	case spinning:
		return "Running"
	case status == "completed":
		return "Completed"
	case status == "failed":
		return "Completed with issues"
	default:
		return "Running"
	}
}

func renderBackgroundSubAgentNode(sty *styles.Styles, entry agent.BackgroundSubAgentView, index, width int, expanded bool) *TreeNode {
	title := backgroundSubAgentDisplayTitle(entry, index)
	sections := make([]*TreeNode, 0, 8)
	status := humanizeSubAgentStatus(entry.Status)
	if status == "" {
		status = "Running"
	}
	sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "State", status)})

	if focus := backgroundSubAgentFocus(entry); focus != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Focus", focus)})
	}
	if workdir := strings.TrimSpace(entry.WorkDir); workdir != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Workspace", backgroundSubAgentWorkspaceValue(workdir))})
	}
	if branch := strings.TrimSpace(entry.Branch); branch != "" && isManagedSubAgentWorktree(entry.WorkDir) {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Branch", branch)})
	}

	report := parseSubAgentReport(entry.Result)
	summary := firstNonEmptyBackgroundText(strings.TrimSpace(entry.Summary), strings.TrimSpace(report.Summary))
	if summary != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Summary", summary)})
	}
	if strings.TrimSpace(entry.Error) != "" {
		sections = append(sections, &TreeNode{Label: renderSubAgentField(sty, "Issue", oneLine(entry.Error))})
	}
	if len(report.Files) > 0 {
		if files := buildFileContextRoot(sty, "Files Read", fileContextEntriesFromPaths(report.Files)); files != nil {
			sections = append(sections, files)
		}
	}
	if len(report.Commands) > 0 {
		sections = append(sections, &TreeNode{
			Label:    renderSubAgentSectionLabel(sty, "Commands"),
			Children: renderCommandChildren(report.Commands),
		})
	}

	preview := strings.TrimSpace(firstNonEmptyBackgroundText(entry.Preview, entry.Result))
	if preview != "" {
		previewBody := toolOutputSmartContent(sty, "preview.md", preview, max(24, width-8), expanded)
		sections = append(sections, &TreeNode{
			Label: renderSubAgentSectionLabel(sty, "Preview"),
			Children: []*TreeNode{
				{Label: previewBody},
			},
		})
	}

	return &TreeNode{
		Label:    renderSubAgentAgentTitle(sty, title),
		Children: sections,
	}
}

func backgroundSubAgentDisplayTitle(entry agent.BackgroundSubAgentView, index int) string {
	return fmt.Sprintf("Sub-Agent %d", index)
}

func backgroundSubAgentFocus(entry agent.BackgroundSubAgentView) string {
	if leg := strings.TrimSpace(entry.LegType); leg != "" {
		return humanizeBackgroundLeg(leg)
	}
	return strings.TrimSpace(entry.Name)
}

func humanizeBackgroundLeg(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "-", " "))
	raw = strings.ReplaceAll(raw, "_", " ")
	if raw == "" {
		return ""
	}
	parts := strings.Fields(strings.ToLower(raw))
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func firstNonEmptyBackgroundText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func backgroundSubAgentWorkspaceValue(path string) string {
	if isManagedSubAgentWorktree(path) {
		return formatRelativePath(path)
	}
	return "repo"
}
