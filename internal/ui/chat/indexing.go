package chat

import (
	"fmt"
	"strings"

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
	messageText := strings.TrimSpace(i.progress.Message)
	if errText := strings.TrimSpace(i.progress.Error); errText != "" {
		messageText = errText
	}
	if messageText == "" {
		messageText = "Preparing codebase indexing"
	}
	percentLabel := fmt.Sprintf("%d%%", clampIndexPercent(i.progress.Percent))
	barWidth := max(18, min(42, width-12))
	bar := renderIndexingProgressBar(barWidth, i.progress.Percent)

	detail := i.renderDetail(percentLabel)

	progressLine := lipgloss.JoinHorizontal(
		lipgloss.Left,
		bar,
		"   ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F3E8FF")).Bold(true).Render(percentLabel),
	)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		i.sty.HalfMuted.Render(i.progress.Workspace),
		"",
		i.sty.Muted.Render(messageText),
		"",
		progressLine,
		"",
		i.sty.HalfMuted.Render(detail),
		"",
	)

	return lipgloss.NewStyle().
		PaddingLeft(2).
		PaddingRight(2).
		Render(body)
}

func (i *IndexingMessageItem) renderTitle() string {
	switch {
	case i.progress.Active:
		return shimmer.RenderIndexingText("Indexing codebase...", i.frame)
	case strings.TrimSpace(i.progress.Error) != "":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185")).Bold(true).Render("Codebase indexing failed")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#C084FC")).Bold(true).Render("Codebase indexing complete")
	}
}

func (i *IndexingMessageItem) renderDetail(percentLabel string) string {
	phase := strings.ReplaceAll(strings.TrimSpace(i.progress.Phase), "_", " ")
	switch i.progress.Phase {
	case "discovering":
		return fmt.Sprintf("%s · %d/%d files · %s", percentLabel, i.progress.FilesProcessed, max(1, i.progress.FilesDiscovered), phase)
	case "parsing":
		return fmt.Sprintf("%s · %d/%d files · extracting graph facts", percentLabel, i.progress.FilesProcessed, max(1, i.progress.FilesDiscovered))
	case "persisting":
		return fmt.Sprintf("%s · %d files indexed · writing durable graph", percentLabel, max(i.progress.FilesIndexed, i.progress.FilesProcessed))
	case "preparing":
		return fmt.Sprintf("%s · %d chunks prepared · %s", percentLabel, max(0, i.progress.ChunksTotal), phase)
	case "embedding":
		return fmt.Sprintf("%s · %d/%d chunks · %s", percentLabel, i.progress.ChunksEmbedded, max(1, i.progress.ChunksTotal), phase)
	case "upserting":
		return fmt.Sprintf("%s · writing vectors · %s", percentLabel, phase)
	default:
		if phase == "" {
			return percentLabel
		}
		return fmt.Sprintf("%s · %s", percentLabel, phase)
	}
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
	fill := lipgloss.NewStyle().Foreground(lipgloss.Color("#C084FC")).Bold(true).Render(strings.Repeat("█", filled))
	rest := lipgloss.NewStyle().Foreground(lipgloss.Color("#4C1D95")).Render(strings.Repeat("░", width-filled))
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
