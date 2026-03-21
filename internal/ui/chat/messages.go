// Package chat provides UI components and message items for the chat interface.
package chat

// MessageItem is the primary interface for all chat message components.

import (
	"fmt"
	"image"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/attachments"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/list"
	"github.com/duggal1/Sapphire-cli/internal/ui/shimmer"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

// MessageLeftPaddingTotal is the total width reserved for message prefixes.
// We also cap the width so text is readable to the maxTextWidth(120).
const MessageLeftPaddingTotal = 2

// maxTextWidth is the maximum width text messages can be
const maxTextWidth = 120

// Identifiable is an interface for items that can provide a unique identifier.
type Identifiable interface {
	ID() string
}

// Animatable is an interface for items that support animation.
type Animatable interface {
	StartAnimation() tea.Cmd
}

// ShimmerTickable is implemented by items that need cache invalidation on
// shimmer timer frames.
type ShimmerTickable interface {
	OnShimmerTick() bool
}

// Expandable is an interface for items that can be expanded or collapsed.
type Expandable interface {
	// ToggleExpanded toggles the expanded state of the item. It returns
	// whether the item is now expanded.
	ToggleExpanded() bool
}

// KeyEventHandler is an interface for items that can handle key events.
type KeyEventHandler interface {
	HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd)
}

// MessageItem represents a [message.Message] item that can be displayed in the
// UI and be part of a [list.List] identifiable by a unique ID.
type MessageItem interface {
	list.Item
	list.RawRenderable
	Identifiable
}

// HighlightableMessageItem is a message item that supports highlighting.
type HighlightableMessageItem interface {
	MessageItem
	list.Highlightable
}

// FocusableMessageItem is a message item that supports focus.
type FocusableMessageItem interface {
	MessageItem
	list.Focusable
}

// SendMsg represents a message to send a chat message.
type SendMsg struct {
	Text        string
	Attachments []message.Attachment
}

type highlightableMessageItem struct {
	startLine   int
	startCol    int
	endLine     int
	endCol      int
	highlighter list.Highlighter
}

var _ list.Highlightable = (*highlightableMessageItem)(nil)

// isHighlighted returns true if the item has a highlight range set.
func (h *highlightableMessageItem) isHighlighted() bool {
	return h.startLine != -1 || h.endLine != -1
}

// renderHighlighted highlights the content if necessary.
func (h *highlightableMessageItem) renderHighlighted(content string, width, height int) string {
	if !h.isHighlighted() {
		return content
	}
	area := image.Rect(0, 0, width, height)
	return list.Highlight(content, area, h.startLine, h.startCol, h.endLine, h.endCol, h.highlighter)
}

// SetHighlight implements list.Highlightable.
func (h *highlightableMessageItem) SetHighlight(startLine int, startCol int, endLine int, endCol int) {
	// Adjust columns for the style's left inset (border + padding) since we
	// highlight the content only.
	offset := MessageLeftPaddingTotal
	h.startLine = startLine
	h.startCol = max(0, startCol-offset)
	h.endLine = endLine
	if endCol >= 0 {
		h.endCol = max(0, endCol-offset)
	} else {
		h.endCol = endCol
	}
}

// Highlight implements list.Highlightable.
func (h *highlightableMessageItem) Highlight() (startLine int, startCol int, endLine int, endCol int) {
	return h.startLine, h.startCol, h.endLine, h.endCol
}

func defaultHighlighter(sty *styles.Styles) *highlightableMessageItem {
	return &highlightableMessageItem{
		startLine:   -1,
		startCol:    -1,
		endLine:     -1,
		endCol:      -1,
		highlighter: list.ToHighlighter(sty.TextSelection),
	}
}

// cachedMessageItem caches rendered message content to avoid re-rendering.
//
// This should be used by any message that can store a cached version of its render. e.x user,assistant... and so on
//
// THOUGHT(kujtim): we should consider if its efficient to store the render for different widths
// the issue with that could be memory usage
type cachedMessageItem struct {
	// rendered is the cached rendered string
	rendered string
	// width and height are the dimensions of the cached render
	width  int
	height int
}

// getCachedRender returns the cached render if it exists for the given width.
func (c *cachedMessageItem) getCachedRender(width int) (string, int, bool) {
	if c.width == width && c.rendered != "" {
		return c.rendered, c.height, true
	}
	return "", 0, false
}

// setCachedRender sets the cached render.
func (c *cachedMessageItem) setCachedRender(rendered string, width, height int) {
	c.rendered = rendered
	c.width = width
	c.height = height
}

// clearCache clears the cached render.
func (c *cachedMessageItem) clearCache() {
	c.rendered = ""
	c.width = 0
	c.height = 0
}

// focusableMessageItem is a base struct for message items that can be focused.
type focusableMessageItem struct {
	focused bool
}

// SetFocused implements MessageItem.
func (f *focusableMessageItem) SetFocused(focused bool) {
	f.focused = focused
}

// AssistantInfoID returns a stable ID for assistant info items.
func AssistantInfoID(messageID string) string {
	return fmt.Sprintf("%s:assistant-info", messageID)
}

// AssistantInfoItem renders model info and response time after assistant completes.
type AssistantInfoItem struct {
	*cachedMessageItem

	id                  string
	message             *message.Message
	sty                 *styles.Styles
	cfg                 *config.Config
	lastUserMessageTime time.Time
	requestStartedAt    time.Time
	requestCompletedAt  time.Time
}

// NewAssistantInfoItem creates a new AssistantInfoItem.
func NewAssistantInfoItem(sty *styles.Styles, message *message.Message, cfg *config.Config, lastUserMessageTime time.Time) MessageItem {
	return &AssistantInfoItem{
		cachedMessageItem:   &cachedMessageItem{},
		id:                  AssistantInfoID(message.ID),
		message:             message,
		sty:                 sty,
		cfg:                 cfg,
		lastUserMessageTime: lastUserMessageTime,
	}
}

// StartAnimation implements Animatable.
func (a *AssistantInfoItem) StartAnimation() tea.Cmd {
	if !a.shouldAnimate() {
		return nil
	}
	return shimmer.ShimmerTickCmd()
}

// SetMessage updates the underlying assistant message.
func (a *AssistantInfoItem) SetMessage(msg *message.Message) {
	a.message = msg
	a.clearCache()
}

// SetLastUserMessageTime updates the timer baseline for the assistant footer.
func (a *AssistantInfoItem) SetLastUserMessageTime(t time.Time) {
	a.lastUserMessageTime = t
	a.clearCache()
}

func (a *AssistantInfoItem) SetRequestTiming(start, end time.Time) {
	a.requestStartedAt = start
	a.requestCompletedAt = end
	a.clearCache()
}

func (a *AssistantInfoItem) Height() int {
	if a == nil {
		return 0
	}
	return 2
}

// MessageID returns the backing assistant message ID.
func (a *AssistantInfoItem) MessageID() string {
	if a.message == nil {
		return ""
	}
	return a.message.ID
}

// ID implements MessageItem.
func (a *AssistantInfoItem) ID() string {
	return a.id
}

// RawRender implements MessageItem.
func (a *AssistantInfoItem) RawRender(width int) string {
	innerWidth := max(0, width-MessageLeftPaddingTotal)
	content, _, ok := a.getCachedRender(innerWidth)
	if !ok {
		content = a.renderContent(innerWidth)
		height := lipgloss.Height(content)
		a.setCachedRender(content, innerWidth, height)
	}
	return content
}

// Render implements MessageItem.
func (a *AssistantInfoItem) Render(width int) string {
	return a.RawRender(width)
}

func (a *AssistantInfoItem) renderContent(width int) string {
	if a.message == nil {
		return ""
	}

	finish := a.message.FinishPart()

	modelName := strings.TrimSpace(a.message.Model)
	if idx := strings.Index(strings.ToLower(modelName), " via "); idx >= 0 {
		modelName = strings.TrimSpace(modelName[:idx])
	}

	// Handle Thinking Mode {high/low} or {on/off}
	effort := ""
	if finish != nil {
		effort = finish.ThinkingEffort
	}
	if effort == "" && finish != nil && finish.ThoughtsTokens > 0 {
		// If thoughts were produced but no effort level recorded (e.g. Gemini 2.5 Flash)
		if a.cfg != nil {
			if agentCfg, ok := a.cfg.Agents[config.AgentCoder]; ok {
				if modelCfg, ok := a.cfg.Models[agentCfg.Model]; ok {
					effort = modelCfg.ReasoningEffort
				}
			}
		}
		if effort == "" {
			effort = "on"
		}
	}

	if effort != "" {
		lowerEffort := strings.ToLower(effort)
		if !strings.Contains(strings.ToLower(modelName), lowerEffort) {
			modelName = fmt.Sprintf("%s %s", modelName, common.FormatReasoningEffort(lowerEffort))
		}
	}

	modelName = strings.ReplaceAll(modelName, "{", " ")
	modelName = strings.ReplaceAll(modelName, "}", " ")
	modelName = strings.Join(strings.Fields(modelName), " ")

	modelLine := a.sty.Chat.Message.AssistantInfoModel.Render(modelName)

	duration := a.renderDurationFormatted(finish)
	if a.shouldAnimate() {
		return a.renderFooter(width, duration, modelLine)
	}

	promptTokens := int64(0)
	completionTokens := int64(0)
	cachedTokens := int64(0)
	thoughtsTokens := int64(0)
	if finish != nil {
		promptTokens = finish.PromptTokens
		completionTokens = finish.CompletionTokens
		cachedTokens = finish.CachedTokens
		thoughtsTokens = finish.ThoughtsTokens
	}

	parts := []string{duration}
	if promptTokens > 0 {
		parts = append(parts, formatUsageTokens(promptTokens)+" in")
	}
	if completionTokens > 0 {
		parts = append(parts, formatUsageTokens(completionTokens)+" out")
	}
	if cachedTokens > 0 {
		parts = append(parts, formatUsageTokens(cachedTokens)+" cached")
	}
	if thoughtsTokens > 0 {
		parts = append(parts, formatUsageTokens(thoughtsTokens)+" thoughts")
	}

	if finish != nil && finish.AvgLatencyMs > 0 {
		parts = append(parts, fmt.Sprintf("%.0fms", finish.AvgLatencyMs))
	}

	dot := a.sty.Base.Foreground(lipgloss.Color("240")).Render(" · ")
	dataText := a.sty.Chat.Message.AssistantInfoDuration.Render(strings.Join(parts, dot))
	return a.renderFooter(width, dataText, modelLine)
}

func (a *AssistantInfoItem) renderFooter(width int, stats, model string) string {
	if width <= 0 {
		return ""
	}

	border := strings.Repeat("─", width)
	borderLine := a.sty.Base.Foreground(a.sty.Border).Render(border)

	row := a.renderFooterRow(width, stats, model)
	if row == "" {
		return borderLine
	}

	return strings.Join([]string{borderLine, row}, "\n")
}

func (a *AssistantInfoItem) renderFooterRow(width int, left, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+1+rightWidth <= width {
		return left + strings.Repeat(" ", width-leftWidth-rightWidth) + right
	}

	maxLeft := max(0, width-rightWidth-1)
	if maxLeft <= 0 {
		return right
	}
	if maxLeft > 0 && leftWidth > maxLeft {
		left = ansi.Truncate(left, maxLeft, "…")
		leftWidth = lipgloss.Width(left)
	}

	if leftWidth+1+rightWidth <= width {
		return left + strings.Repeat(" ", width-leftWidth-rightWidth) + right
	}

	if rightWidth >= width {
		return right
	}

	return left + " " + right
}

func (a *AssistantInfoItem) renderDurationFormatted(finish *message.Finish) string {
	seconds := a.renderDurationSeconds(finish)
	var timeStr string
	if seconds < 60 {
		timeStr = fmt.Sprintf("%.0fs", seconds)
	} else {
		mins := int(seconds) / 60
		secs := int(seconds) % 60

		if secs == 0 {
			timeStr = fmt.Sprintf("%dm", mins)
		} else {
			timeStr = fmt.Sprintf("%dm %ds", mins, secs)
		}
	}
	// Format with clock icon and square brackets: [⏱ 29s]
	return fmt.Sprintf("[⏱ %s]", timeStr)
}

func (a *AssistantInfoItem) renderDurationSeconds(finish *message.Finish) float64 {
	if !a.requestStartedAt.IsZero() {
		end := a.requestCompletedAt
		if end.IsZero() {
			end = time.Now()
		}
		duration := end.Sub(a.requestStartedAt).Seconds()
		if duration >= 0 {
			return duration
		}
	}

	if finish != nil && finish.EndTimeMs > 0 && finish.StartTimeMs > 0 {
		duration := float64(finish.EndTimeMs-finish.StartTimeMs) / 1000.0
		if duration > 0 {
			return duration
		}
	}

	start := a.lastUserMessageTime
	if start.IsZero() && a.message != nil && a.message.CreatedAt > 0 {
		start = time.Unix(a.message.CreatedAt, 0)
	}
	if start.IsZero() {
		return 0
	}

	end := time.Now()
	if finish != nil && finish.Time > 0 {
		end = time.Unix(finish.Time, 0)
	}

	duration := end.Sub(start).Seconds()
	if duration < 0 {
		return 0
	}
	return duration
}

func (a *AssistantInfoItem) renderLiveStatusLine(width int) string {
	label := a.renderLiveStatusLabel()
	if label == "" {
		return ""
	}
	line := styles.ShimmerText(a.sty, label, 0)
	if width > 0 && lipgloss.Width(line) > width {
		line = ansi.Truncate(line, width, "…")
	}
	return line
}

func (a *AssistantInfoItem) renderLiveStatusLabel() string {
	return assistantLiveStatusLabel(a.message)
}

func (a *AssistantInfoItem) shouldAnimate() bool {
	if !a.requestStartedAt.IsZero() && a.requestCompletedAt.IsZero() {
		return true
	}
	return a.message != nil && !a.message.IsFinished()
}

// OnShimmerTick invalidates the footer while request timing is still changing.
func (a *AssistantInfoItem) OnShimmerTick() bool {
	if !a.shouldAnimate() {
		return false
	}
	a.clearCache()
	return true
}

func formatUsageTokens(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000.0)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}

// cappedMessageWidth returns the maximum width for message content for readability.
func cappedMessageWidth(availableWidth int) int {
	return min(availableWidth-MessageLeftPaddingTotal, maxTextWidth)
}

func ExtractMessageItems(sty *styles.Styles, msg *message.Message, toolResults map[string]message.ToolResult) []MessageItem {
	switch msg.Role {
	case message.User:
		r := attachments.NewRenderer(
			sty.Attachments.Normal,
			sty.Attachments.Deleting,
			sty.Attachments.Image,
			sty.Attachments.Text,
			sty.Attachments.PasteBlock,
			sty.Attachments.PasteSelected,
			sty.Attachments.PastePalette,
			sty.Attachments.PasteSelectedPalette,
		)
		return []MessageItem{NewUserMessageItem(sty, msg, r)}
	case message.Assistant:
		var items []MessageItem
		assistantItem := NewAssistantMessageItem(sty, msg)
		renderAssistant := ShouldRenderAssistantMessage(msg)
		assistantAfterTools := shouldRenderAssistantAfterTools(msg)
		if renderAssistant && !assistantAfterTools {
			items = append(items, assistantItem)
		}
		for _, tc := range msg.ToolCalls() {
			var result *message.ToolResult
			if tr, ok := toolResults[tc.ID]; ok {
				result = &tr
			}
			items = append(items, NewToolMessageItem(
				sty,
				msg.ID,
				tc,
				result,
				msg.FinishReason() == message.FinishReasonCanceled,
			))
		}
		if renderAssistant && assistantAfterTools {
			items = append(items, assistantItem)
		}
		return items
	}
	return []MessageItem{}
}

// ShouldRenderAssistantMessage determines if an assistant message should be rendered
//
// In some cases the assistant message only has tools so we do not want to render an
// empty message.
func ShouldRenderAssistantMessage(msg *message.Message) bool {
	content := strings.TrimSpace(msg.Content().Text)
	thinking := strings.TrimSpace(msg.ReasoningContent().Thinking)
	isError := msg.FinishReason() == message.FinishReasonError
	isCancelled := msg.FinishReason() == message.FinishReasonCanceled
	hasToolCalls := len(msg.ToolCalls()) > 0
	return !hasToolCalls || content != "" || thinking != "" || msg.IsThinking() || !msg.IsFinished() || isError || isCancelled
}

func shouldRenderAssistantAfterTools(msg *message.Message) bool {
	return false
}

// BuildToolResultMap creates a map of tool call IDs to their results from a list of messages.
// Tool result messages (role == message.Tool) contain the results that should be linked
// to tool calls in assistant messages.
func BuildToolResultMap(messages []*message.Message) map[string]message.ToolResult {
	resultMap := make(map[string]message.ToolResult)
	for _, msg := range messages {
		if msg.Role == message.Tool {
			for _, result := range msg.ToolResults() {
				if result.ToolCallID != "" {
					resultMap[result.ToolCallID] = result
				}
			}
		}
	}
	return resultMap
}
