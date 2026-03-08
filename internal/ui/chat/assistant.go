// Package chat provides UI components and message items for the chat interface.
package chat

// AssistantMessageItem manages the display of AI-generated responses and thinking process.

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/shell"
	"github.com/charmbracelet/sapphire/internal/ui/anim"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// assistantMessageTruncateFormat is the text shown when an assistant message is
// truncated.
const assistantMessageTruncateFormat = "… (%d lines hidden) [click or space to expand]"

// maxCollapsedThinkingHeight defines the maximum height of the thinking
const maxCollapsedThinkingHeight = 10

// AssistantMessageItem represents an assistant message in the chat UI.
//
// This item includes thinking, and the content but does not include the tool calls.
type AssistantMessageItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	message           *message.Message
	sty               *styles.Styles
	anim              *anim.Anim
	skillFrame        int
	thinkingExpanded  bool
	thinkingBoxHeight int // Tracks the rendered thinking box height for click detection.
	lastClick         time.Time
}

// NewAssistantMessageItem creates a new AssistantMessageItem.
func NewAssistantMessageItem(sty *styles.Styles, message *message.Message) MessageItem {
	a := &AssistantMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		message:                  message,
		sty:                      sty,
	}

	a.anim = anim.New(anim.Settings{
		ID:          a.ID(),
		Size:        15,
		GradColors:  []color.Color{sty.Primary, sty.Secondary, sty.Tertiary},
		LabelColor:  sty.FgBase,
		CycleColors: true,
	})
	return a
}

// StartAnimation starts the assistant message animation if it should be spinning.
func (a *AssistantMessageItem) StartAnimation() tea.Cmd {
	if !a.isSpinning() && !a.hasAnimatedContext() {
		return nil
	}
	return a.anim.Start()
}

// Animate progresses the assistant message animation if it should be spinning.
func (a *AssistantMessageItem) Animate(msg anim.StepMsg) tea.Cmd {
	if !a.isSpinning() && !a.hasAnimatedContext() {
		return nil
	}
	if a.hasAnimatedContext() || a.message.IsThinking() {
		a.skillFrame++
		a.clearCache()
	}
	return a.anim.Animate(msg)
}

// ID implements MessageItem.
func (a *AssistantMessageItem) ID() string {
	return a.message.ID
}

// Message returns the underlying message.
func (a *AssistantMessageItem) Message() *message.Message {
	return a.message
}

// RawRender implements [MessageItem].
func (a *AssistantMessageItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)

	var spinner string
	if a.isSpinning() {
		spinner = a.renderSpinning()
	}

	content, height, ok := a.getCachedRender(cappedWidth)
	if !ok {
		content = a.renderMessageContent(cappedWidth)
		height = lipgloss.Height(content)
		// cache the rendered content
		a.setCachedRender(content, cappedWidth, height)
	}

	highlightedContent := a.renderHighlighted(content, cappedWidth, height)
	if spinner != "" {
		if highlightedContent != "" {
			highlightedContent += "\n\n"
		}
		return highlightedContent + spinner
	}

	return highlightedContent
}

// Render implements MessageItem.
func (a *AssistantMessageItem) Render(width int) string {
	// XXX: Here, we're manually applying the focused/blurred styles because
	// using lipgloss.Render can degrade performance for long messages due to
	// it's wrapping logic.
	// We already know that the content is wrapped to the correct width in
	// RawRender, so we can just apply the styles directly to each line.
	focused := a.sty.Chat.Message.AssistantFocused.Render()
	blurred := a.sty.Chat.Message.AssistantBlurred.Render()
	rendered := a.RawRender(width)
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		if a.focused {
			lines[i] = focused + line
		} else {
			lines[i] = blurred + line
		}
	}
	return strings.Join(lines, "\n")
}

// renderMessageContent renders the message content including thinking, main content, and finish reason.
func (a *AssistantMessageItem) renderMessageContent(width int) string {
	var messageParts []string
	if skill := a.renderSkillContext(width); skill != "" {
		messageParts = append(messageParts, skill)
	}

	if bgContext := a.renderBackgroundContext(width); bgContext != "" {
		messageParts = append(messageParts, bgContext)
	}
	thinking := strings.TrimSpace(a.message.ReasoningContent().Thinking)
	content := strings.TrimSpace(a.message.Content().Text)
	// if the massage has reasoning content add that first
	if thinking != "" {
		if len(messageParts) > 0 {
			messageParts = append(messageParts, "")
		}
		messageParts = append(messageParts, a.renderThinking(a.message.ReasoningContent().Thinking, width))
	}

	// then add the main content
	if content != "" {
		// add a spacer between thinking and content
		if thinking != "" || len(messageParts) > 0 {
			messageParts = append(messageParts, "")
		}
		messageParts = append(messageParts, a.renderMarkdown(content, width))
	}

	// finally add any finish reason info
	if a.message.IsFinished() {
		switch a.message.FinishReason() {
		case message.FinishReasonCanceled:
			messageParts = append(messageParts, a.sty.Base.Italic(true).Render("Canceled"))
		case message.FinishReasonError:
			messageParts = append(messageParts, a.renderError(width))
		}
	}

	return strings.Join(messageParts, "\n")
}

func (a *AssistantMessageItem) renderSkillContext(width int) string {
	skillContext := a.message.SkillContext()
	if skillContext == nil || len(skillContext.Skills) == 0 {
		return ""
	}

	skillNameStr := formatSkillNames(skillContext.Skills)
	if !strings.HasSuffix(strings.ToLower(skillNameStr), "skill") && !strings.HasSuffix(strings.ToLower(skillNameStr), "skills") {
		skillNameStr += " skill"
	}

	hasContent := strings.TrimSpace(a.message.Content().Text) != ""
	isFinished := a.message.IsFinished()

	if hasContent || isFinished {
		// Static completed state: skill has been read
		label := "✓ Read " + skillNameStr
		return a.sty.Muted.Render(label)
	}

	// Animated shimmer: skill is currently being read
	dotsCount := (a.skillFrame / 4) % 4
	dots := strings.Repeat(".", dotsCount)
	label := "Reading " + skillNameStr + dots

	label = styles.ApplyBoldForegroundGradShifted(
		a.sty,
		label,
		a.skillFrame,
		a.sty.Primary,
		a.sty.Secondary,
		a.sty.Tertiary,
		a.sty.Secondary,
	)

	return label
}

func (a *AssistantMessageItem) renderBackgroundContext(width int) string {
	bgManager := shell.GetBackgroundShellManager()
	count := len(bgManager.List())
	if count == 0 {
		return ""
	}

	dotsCount := (a.skillFrame / 3) % 4
	dots := strings.Repeat(".", dotsCount)
	label := fmt.Sprintf("Running %d terminal(s) in background%s", count, dots)

	label = styles.ApplyBoldForegroundGradShifted(
		a.sty,
		label,
		a.skillFrame,
		a.sty.Tertiary,
		a.sty.Secondary,
		a.sty.Primary,
		a.sty.Tertiary, // different cycle color order than skill context
	)

	return label
}

// renderThinking renders the thinking/reasoning content with footer.
func (a *AssistantMessageItem) renderThinking(thinking string, width int) string {
	// Dynamic padding based on width - ultra minimal for clean typography
	hPadding := 2
	vPadding := 1
	if width < 60 {
		hPadding = 1
		vPadding = 0
	}

	renderer := common.PlainMarkdownRenderer(a.sty, width-hPadding*2)
	rendered, err := renderer.Render(thinking)
	if err != nil {
		rendered = thinking
	}
	rendered = strings.TrimSpace(rendered)

	lines := strings.Split(rendered, "\n")
	totalLines := len(lines)

	isTruncated := totalLines > maxCollapsedThinkingHeight
	if !a.thinkingExpanded && isTruncated {
		lines = lines[totalLines-maxCollapsedThinkingHeight:]
		hint := a.sty.Chat.Message.ThinkingTruncationHint.Render(
			fmt.Sprintf(assistantMessageTruncateFormat, totalLines-maxCollapsedThinkingHeight),
		)
		lines = append([]string{hint, ""}, lines...)
	}

	thinkingStyle := a.sty.Chat.Message.ThinkingBox.
		Width(width).
		Padding(vPadding, hPadding)

	result := thinkingStyle.Render(strings.Join(lines, "\n"))
	a.thinkingBoxHeight = lipgloss.Height(result)

	var footer string
	// if thinking is done add the thought for footer
	if !a.message.IsThinking() || len(a.message.ToolCalls()) > 0 {
		duration := a.message.ThinkingDuration()
		if duration.String() != "0s" {
			footer = a.sty.Chat.Message.ThinkingFooterTitle.Render("Thought for ") +
				a.sty.Chat.Message.ThinkingFooterDuration.Render(duration.String())
		}
	}

	if footer != "" {
		result += "\n\n" + footer
	}

	return result
}

// renderThinkingShimmer renders a beautiful text shimmer for "Thinking...".
func (a *AssistantMessageItem) renderThinkingShimmer(width int) string {
	dotsCount := (a.skillFrame / 3) % 4
	dots := strings.Repeat(".", dotsCount)
	label := "Thinking" + dots

	return styles.ApplyBoldForegroundGradShifted(
		a.sty,
		label,
		a.skillFrame,
		a.sty.Primary,
		a.sty.Secondary,
		a.sty.Tertiary,
		a.sty.Secondary,
	)
}

// renderMarkdown renders content as markdown.
func (a *AssistantMessageItem) renderMarkdown(content string, width int) string {
	renderer := common.MarkdownRenderer(a.sty, width)
	result, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSuffix(result, "\n")
}

func (a *AssistantMessageItem) renderSpinning() string {
	if a.message.IsThinking() {
		return a.renderThinkingShimmer(0) // width not needed for text shimmer
	} else if a.message.IsSummaryMessage {
		a.anim.SetLabel("Summarizing")
	}
	return a.anim.Render()
}

// renderError renders an error message.
func (a *AssistantMessageItem) renderError(width int) string {
	finishPart := a.message.FinishPart()
	errTag := a.sty.Chat.Message.ErrorTag.Render("ERROR")
	truncated := ansi.Truncate(finishPart.Message, width-2-lipgloss.Width(errTag), "...")
	title := fmt.Sprintf("%s %s", errTag, a.sty.Chat.Message.ErrorTitle.Render(truncated))
	details := a.sty.Chat.Message.ErrorDetails.Width(width - 2).Render(finishPart.Details)
	return fmt.Sprintf("%s\n\n%s", title, details)
}

// isSpinning returns true if the assistant message is still generating.
func (a *AssistantMessageItem) isSpinning() bool {
	isThinking := a.message.IsThinking()
	isFinished := a.message.IsFinished()
	hasContent := strings.TrimSpace(a.message.Content().Text) != ""
	hasToolCalls := len(a.message.ToolCalls()) > 0
	return (isThinking || !isFinished) && !hasContent && !hasToolCalls
}

func (a *AssistantMessageItem) hasAnimatedContext() bool {
	hasSkill := a.message.SkillContext() != nil
	hasBg := len(shell.GetBackgroundShellManager().List()) > 0
	isThinking := a.message.IsThinking()
	return (hasSkill || hasBg || isThinking) && !a.message.IsFinished()
}

// SetMessage is used to update the underlying message.
func (a *AssistantMessageItem) SetMessage(message *message.Message) tea.Cmd {
	wasAnimating := a.isSpinning() || a.hasAnimatedContext()
	a.message = message
	a.clearCache()
	if !wasAnimating && (a.isSpinning() || a.hasAnimatedContext()) {
		return a.StartAnimation()
	}
	return nil
}

func formatSkillNames(skills []string) string {
	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		if skill == "SKILLNAME" {
			continue
		}
		// Convert "frontend" -> "Frontend", "back-end" -> "Back End"
		parts := strings.Split(strings.ReplaceAll(skill, "-", " "), " ")
		for i, p := range parts {
			if len(p) > 0 {
				parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
			}
		}
		names = append(names, strings.Join(parts, " "))
	}
	if len(names) == 0 {
		return "Skill"
	}
	return strings.Join(names, " + ")
}

// ToggleExpanded toggles the expanded state of the thinking box.
func (a *AssistantMessageItem) ToggleExpanded() {
	a.thinkingExpanded = !a.thinkingExpanded
	a.clearCache()
}

// HandleMouseClick implements MouseClickable.
func (a *AssistantMessageItem) HandleMouseClick(btn ansi.MouseButton, x, y int) bool {
	if btn != ansi.MouseLeft {
		return false
	}

	now := time.Now()
	if !a.lastClick.IsZero() && now.Sub(a.lastClick) < 500*time.Millisecond {
		// Double click detected
		text := a.message.Content().Text
		if thinking := a.message.ReasoningContent().Thinking; thinking != "" {
			text = thinking + "\n\n" + text
		}
		// Copying via a command would be better, but HandleMouseClick returns bool.
		// Wait, how do I trigger a Cmd from HandleMouseClick?
		// Usually these interfaces are used in Update and they might return a Msg or Cmd.
		// Let's see how AssistantMessageItem is used.
	}
	a.lastClick = now

	// check if the click is within the thinking box
	if a.thinkingBoxHeight > 0 && y < a.thinkingBoxHeight {
		a.ToggleExpanded()
		return true
	}
	return false
}

// HandleKeyEvent implements KeyEventHandler.
func (a *AssistantMessageItem) HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	if k := key.String(); k == "c" || k == "y" {
		text := a.message.Content().Text
		return true, common.CopyToClipboard(text, "Message copied to clipboard")
	}
	return false, nil
}
