package chat

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/codeindex"
	"github.com/duggal1/Sapphire-cli/internal/ui/shimmer"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

const IndexingMessageID = "local:index-codebase"

type IndexingMessageItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	sty      *styles.Styles
	progress codeindex.Progress
	frame    int
}

func NewIndexingMessageItem(sty *styles.Styles, progress codeindex.Progress, frame int) *IndexingMessageItem {
	return &IndexingMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		sty:                      sty,
		progress:                 progress,
		frame:                    frame,
	}
}

func (i *IndexingMessageItem) ID() string {
	return IndexingMessageID
}

func (i *IndexingMessageItem) SetProgress(progress codeindex.Progress) {
	i.progress = progress
	i.clearCache()
}

func (i *IndexingMessageItem) SetFrame(frame int) {
	if i.frame == frame {
		return
	}
	i.frame = frame
	i.clearCache()
}

func (i *IndexingMessageItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)
	if cached, height, ok := i.getCachedRender(cappedWidth); ok {
		return i.renderHighlighted(cached, cappedWidth, height)
	}

	rendered := i.renderContent(cappedWidth)
	height := lipgloss.Height(rendered)
	i.setCachedRender(rendered, cappedWidth, height)
	return i.renderHighlighted(rendered, cappedWidth, height)
}

func (i *IndexingMessageItem) Render(width int) string {
	rendered := i.RawRender(width)
	lines := strings.Split(rendered, "\n")
	focused := i.sty.Chat.Message.AssistantFocused.Render()
	blurred := i.sty.Chat.Message.AssistantBlurred.Render()
	for idx, line := range lines {
		if i.focused {
			lines[idx] = focused + line
		} else {
			lines[idx] = blurred + line
		}
	}
	return strings.Join(lines, "\n")
}

func (i *IndexingMessageItem) renderContent(width int) string {
	title := i.renderTitle()
	workspace := i.renderWorkspace()
	status := i.renderStatus()
	percentLabel := fmt.Sprintf("%d%%", clampIndexPercent(i.progress.Percent))
	barWidth := max(18, min(42, width-10))
	bar := renderIndexingProgressBar(barWidth, i.progress.Percent)

	progressLine := lipgloss.JoinHorizontal(
		lipgloss.Left,
		bar,
		"  ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#E7E5E4")).Bold(true).Render(percentLabel),
	)

	lines := []string{title}
	if workspace != "" {
		lines = append(lines, i.sty.HalfMuted.Render(workspace))
	}
	if status != "" {
		lines = append(lines, i.sty.Muted.Render(status))
	}
	if tree := i.renderSemanticAgentTree(width - 4); tree != "" {
		lines = append(lines, tree)
	}
	lines = append(lines, progressLine)
	if detail := i.renderMetrics(); detail != "" {
		lines = append(lines, i.sty.HalfMuted.Render(detail))
	}

	return lipgloss.NewStyle().
		PaddingLeft(2).
		PaddingRight(2).
		Render(strings.Join(lines, "\n"))
}

func (i *IndexingMessageItem) renderTitle() string {
	switch {
	case i.progress.Active:
		if i.progress.Phase == "starting" {
			return shimmer.RenderIndexingText("Preparing codebase index", i.frame)
		}
		return shimmer.RenderIndexingText("Indexing codebase", i.frame)
	case i.progress.Phase == "canceled":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Render("Codebase indexing stopped")
	case strings.TrimSpace(i.progress.Error) != "":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185")).Bold(true).Render("Codebase indexing failed")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E7E5E4")).Bold(true).Render("Codebase indexing complete")
	}
}

func (i *IndexingMessageItem) renderWorkspace() string {
	workspace := strings.TrimSpace(formatRelativePath(i.progress.Workspace))
	if workspace == "" {
		workspace = strings.TrimSpace(i.progress.Workspace)
	}
	return workspace
}

func (i *IndexingMessageItem) renderStatus() string {
	messageText := strings.TrimSpace(i.progress.Message)
	if errText := strings.TrimSpace(i.progress.Error); errText != "" {
		return errText
	}
	if messageText != "" {
		return messageText
	}

	phase := strings.ReplaceAll(strings.TrimSpace(i.progress.Phase), "_", " ")
	switch i.progress.Phase {
	case "starting":
		return "Starting durable indexer"
	case "canceled":
		return "Indexing stopped"
	case "discovering":
		return "Discovering files"
	case "parsing":
		return "Extracting structure"
	case "persisting":
		return "Writing graph index"
	case "preparing":
		return "Preparing chunks"
	case "embedding":
		return "Embedding chunks"
	case "upserting":
		return "Writing vectors"
	default:
		return phase
	}
}

func (i *IndexingMessageItem) renderMetrics() string {
	parts := make([]string, 0, 3)

	fileTotal := max(i.progress.FilesDiscovered, i.progress.FilesIndexed)
	if fileTotal > 0 {
		processed := max(i.progress.FilesProcessed, i.progress.FilesIndexed)
		parts = append(parts, fmt.Sprintf("%d/%d files", processed, fileTotal))
	} else if i.progress.FilesProcessed > 0 {
		parts = append(parts, fmt.Sprintf("%d files", i.progress.FilesProcessed))
	}

	if i.progress.ChunksTotal > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d chunks", i.progress.ChunksEmbedded, i.progress.ChunksTotal))
	}

	if elapsed := renderIndexingElapsed(i.progress); elapsed != "" {
		parts = append(parts, elapsed)
	}

	return strings.Join(parts, " · ")
}

func (i *IndexingMessageItem) renderSemanticAgentTree(width int) string {
	if len(i.progress.SemanticAgents) == 0 || width <= 0 {
		return ""
	}
	root := &TreeNode{
		Label:    i.sty.HalfMuted.Render("AI sub-agents"),
		Children: make([]*TreeNode, 0, len(i.progress.SemanticAgents)),
	}
	for _, agent := range i.progress.SemanticAgents {
		header := strings.TrimSpace(agent.Label)
		if header == "" {
			header = "Shard"
		}
		status := strings.TrimSpace(agent.Status)
		if status != "" {
			header += " · " + status
		}
		if agent.FileCount > 0 {
			header += fmt.Sprintf(" · %d files", agent.FileCount)
		}

		detailParts := make([]string, 0, 2)
		if scope := strings.TrimSpace(agent.Scope); scope != "" {
			detailParts = append(detailParts, scope)
		}
		if task := strings.TrimSpace(agent.Task); task != "" {
			detailParts = append(detailParts, task)
		}
		label := header
		if len(detailParts) > 0 {
			label += "\n" + i.sty.HalfMuted.Render(strings.Join(detailParts, " · "))
		}
		root.Children = append(root.Children, &TreeNode{Label: label})
	}
	return strings.Join(renderTreeWithRoot(root, width), "\n")
}

func renderIndexingProgressBar(width int, percent float64) string {
	width = max(width, 10)
	filled := int(percent * float64(width))
	if percent > 0 && filled == 0 {
		filled = 1
	}
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	fill := lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A29E")).Render(strings.Repeat("█", filled))
	rest := lipgloss.NewStyle().Foreground(lipgloss.Color("#44403C")).Render(strings.Repeat("█", width-filled))
	return fill + rest
}

func clampIndexPercent(percent float64) int {
	switch {
	case percent <= 0:
		return 0
	case percent >= 1:
		return 100
	default:
		value := int(percent * 100)
		if value == 0 {
			return 1
		}
		return value
	}
}

func renderIndexingElapsed(progress codeindex.Progress) string {
	if progress.StartedAt.IsZero() {
		return ""
	}
	end := progress.UpdatedAt
	if end.IsZero() {
		end = time.Now()
	}
	if end.Before(progress.StartedAt) {
		return ""
	}
	elapsed := end.Sub(progress.StartedAt).Round(time.Second)
	if elapsed < time.Second {
		return "0s"
	}
	minutes := int(elapsed / time.Minute)
	seconds := int((elapsed % time.Minute) / time.Second)
	if minutes == 0 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
