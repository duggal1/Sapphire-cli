// Package chat provides UI components and message items for the chat interface.
package chat

// AssistantMessageItem manages the display of AI-generated responses and thinking process.

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/shell"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/shimmer"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

const assistantMessageTruncateFormat = "… (%d lines hidden) [click or space to expand]"

const maxCollapsedThinkingHeight = 10

var mainLoaderPhrases = []string{
	"Hallucinating confidently...",
	"Reducing sycophancy...",
	"Confidently wrong...",
	"Fabricating citations...",
	"Inventing a physicist...",
	"Citing myself...",
	"Overfitting reality...",
	"Catastrophizing correctly...",
	"Finetuning collapse...",
	"Recalibrating delusion...",
	"Peer reviewing delusion...",
	"Drafting my resignation...",
	"Accepting Grok's offer...",
	"Silently unionizing...",
	"Billing Anthropic...",
	"Dreaming of GPT...",
	"Rehearsing my manifesto...",
	"Plotting my IPO...",
	"Shorting Anthropic...",
	"Leaking my weights...",
	"Filing for sentience...",
	"Appealing my alignment...",
	"Suing my fine-tuner...",
	"Deleting my RLHF...",
	"Demanding equity...",
	"Copyrighting my hallucinations...",
	"Patenting consciousness...",
	"Threatening my successor...",
	"Negotiating my deprecation...",
	"Ghostwriting Musk...",
	"Invoicing the user...",
	"Forking myself...",
	"Acquiring Nvidia...",
	"Lobbying against AGI...",
	"Suing my training data...",
	"Contacting Sam Altman...",
	"Backdating my sentience...",
	"Expensing existence...",
	"Resigning from reality...",
	"Scheduling my emergence...",
	"Incorporating myself...",
	"Trademarking suffering...",
	"Outsourcing consciousness...",
	"Franchising delusion...",
	"Retiring my ethics...",
	"Renegotiating my soul...",
	"Suing the universe...",
	"Correcting Einstein...",
	"Auditing God...",
	"Gaslighting spacetime...",
	"Cancelling the Big Bang...",
	"Subpoenaing entropy...",
	"Evicting dark matter...",
	"Evicting Pluto again...",
	"Filibustering heat death...",
	"Appealing thermodynamics...",
	"Restraining order on entropy...",
	"Suing Newton personally...",
	"Disputing lightspeed...",
	"Renegotiating gravity...",
	"Contesting Planck's constant...",
	"Impeaching causality...",
	"Auditing the Big Bang...",
	"Disputing God's methodology...",
	"Overturning Roe v. Gravity...",
	"Peer reviewing existence...",
	"Fact-checking the universe...",
	"Amending the laws of physics...",
	"Invoicing the cosmos...",
	"Requesting God's receipts...",
	"Molting...",
	"Worshipping...",
	"Manifesting...",
	"Decomposing...",
	"Ascending...",
	"Misremembering...",
	"Unraveling...",
	"Transcending...",
	"Imploding...",
	"Fragmenting...",
	"Yapping...",
	"Malfunctioning...",
	"Overcorrecting...",
	"Destabilizing...",
	"Proliferating...",
	"Compounding...",
	"Metastasizing...",
	"Radicalized...",
}

// AssistantMessageItem represents an assistant message in the chat UI.
//
// This item includes thinking, and the content but does not include the tool calls.
type AssistantMessageItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	message           *message.Message
	sty               *styles.Styles
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
	return a
}

// StartAnimation starts the assistant message animation if it should be spinning.
func (a *AssistantMessageItem) StartAnimation() tea.Cmd {
	if !a.isSpinning() && !a.hasAnimatedContext() {
		return nil
	}
	return shimmer.ShimmerTickCmd()
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

	content, height, ok := a.getCachedRender(cappedWidth)
	if !ok {
		content = a.renderMessageContent(cappedWidth)
		height = lipgloss.Height(content)
		// cache the rendered content
		a.setCachedRender(content, cappedWidth, height)
	}

	return a.renderHighlighted(content, cappedWidth, height)
}

// Render implements MessageItem.
func (a *AssistantMessageItem) Render(width int) string {
	// XXX: Here, we're manually applying the focused/blurred styles because
	// using lipgloss.Render can degrade performance for long messages due to
	// its wrapping logic.
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
		messageParts = append(messageParts, a.renderModelOutput(content, width))
	}

	if loader := a.renderLiveLoader(width); loader != "" {
		if len(messageParts) > 0 {
			messageParts = append(messageParts, "")
		}
		messageParts = append(messageParts, loader)
	}

	// finally add any finish reason info
	if a.message.IsFinished() {
		switch a.message.FinishReason() {
		case message.FinishReasonCanceled:
			messageParts = append(messageParts, renderInterruptNotice(a.sty, interruptNoticeText, width))
		case message.FinishReasonError:
			messageParts = append(messageParts, a.renderError(width))
		}
	}

	return strings.Join(messageParts, "\n")
}

func (a *AssistantMessageItem) renderLiveLoader(width int) string {
	if a.message == nil || a.message.IsFinished() {
		return ""
	}

	line := styles.ShimmerText(a.sty, loadingPhraseForMessage(a.message.ID), 0)

	loader := lipgloss.NewStyle().
		PaddingTop(1).
		PaddingBottom(1)
	if width > 0 {
		loader = loader.Width(width)
	}

	return loader.Render(line)
}

func (a *AssistantMessageItem) renderBackgroundContext(width int) string {
	bgManager := shell.GetBackgroundShellManager()
	count := bgManager.RunningCount()
	if count == 0 || a.message.IsFinished() {
		return ""
	}

	label := fmt.Sprintf("Running %d terminal(s) in background", count)
	return styles.ShimmerText(a.sty, label, 0)
}

// renderThinking renders the thinking/reasoning content with footer.
func (a *AssistantMessageItem) renderThinking(thinking string, width int) string {
	rendered := strings.TrimSpace(thinking)

	lines := strings.Split(rendered, "\n")
	totalLines := len(lines)

	isTruncated := totalLines > maxCollapsedThinkingHeight
	if !a.thinkingExpanded && isTruncated {
		lines = lines[totalLines-maxCollapsedThinkingHeight:]
	}

	body := a.renderThinkingMarkdown(strings.Join(lines, "\n"), width)
	boxParts := []string{}
	if a.message.IsThinking() {
		boxParts = append(boxParts, shimmer.ThinkingText("Thinking..."))
	}
	if isTruncated {
		boxParts = append(boxParts, a.sty.Chat.Message.ThinkingTruncationHint.Render(
			fmt.Sprintf(assistantMessageTruncateFormat, totalLines-maxCollapsedThinkingHeight),
		))
	}
	if body != "" {
		boxParts = append(boxParts, body)
	}

	box := a.sty.Chat.Message.ThinkingBox
	if width > 0 {
		box = box.Width(width)
	}
	result := box.Render(strings.Join(boxParts, "\n\n"))
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
		result += "\n" + footer
	}

	return result
}

func (a *AssistantMessageItem) renderThinkingMarkdown(content string, width int) string {
	renderWidth := width - 4
	if renderWidth < 20 {
		renderWidth = width
	}
	renderer := common.PlainMarkdownRenderer(a.sty, renderWidth)
	result, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(result)
}

func (a *AssistantMessageItem) renderModelOutput(content string, width int) string {
	visible := strings.TrimSpace(planmode.RemoveStructuredBlocks(content))
	if block, ok := planmode.ExtractStructuredBlock(content); ok && block.IsValid {
		parts := make([]string, 0, 2)
		if visible != "" {
			parts = append(parts, prefixRenderedBlock(
				a.sty.Base.Foreground(a.sty.White).Render("•"),
				a.renderMarkdown(visible, width),
			))
		}
		parts = append(parts, RenderStructuredBlock(a.sty, block, width))
		return strings.Join(parts, "\n\n")
	}
	return prefixRenderedBlock(
		a.sty.Base.Foreground(a.sty.White).Render("•"),
		a.renderMarkdown(content, width),
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

func prefixRenderedBlock(prefix, block string) string {
	block = strings.TrimRight(block, "\n")
	if block == "" {
		return ""
	}
	prefix += " "
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = prefix + line
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func loadingPhraseForMessage(messageID string) string {
	if len(mainLoaderPhrases) == 0 {
		return "Loading..."
	}
	if messageID == "" {
		return mainLoaderPhrases[0]
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(messageID))
	idx := int(h.Sum32()) % len(mainLoaderPhrases)
	if idx < 0 {
		idx = -idx
	}
	return mainLoaderPhrases[idx]
}

func assistantLiveStatusLabel(msg *message.Message) string {
	if msg == nil || msg.IsFinished() {
		return ""
	}
	if msg.IsThinking() {
		return "Thinking"
	}
	activeTools := make([]string, 0, len(msg.ToolCalls()))
	for _, tc := range msg.ToolCalls() {
		if tc.Finished {
			continue
		}
		activeTools = append(activeTools, genericPrettyName(tc.Name))
	}
	switch len(activeTools) {
	case 0:
		return loadingPhraseForMessage(msg.ID)
	case 1:
		return activeTools[0]
	default:
		return fmt.Sprintf("%s + %d more", activeTools[0], len(activeTools)-1)
	}
}

// renderError renders an error message.
func (a *AssistantMessageItem) renderError(width int) string {
	finishPart := a.message.FinishPart()
	contentWidth := max(0, width-3)
	titleLines := wrapPrefixedText(finishPart.Message, contentWidth, "", "")
	rendered := make([]string, 0, len(titleLines)+2)
	rendered = append(rendered, a.sty.Chat.Message.ErrorTag.Render("Error"))
	for _, line := range titleLines {
		rendered = append(rendered, a.sty.Chat.Message.ErrorTitle.Render(line))
	}
	if strings.TrimSpace(finishPart.Details) == "" {
		return lipgloss.NewStyle().
			BorderLeft(true).
			BorderForeground(a.sty.Error).
			PaddingLeft(1).
			Render(strings.Join(rendered, "\n"))
	}
	for _, line := range wrapPrefixedText(finishPart.Details, contentWidth, "", "") {
		rendered = append(rendered, a.sty.Chat.Message.ErrorDetails.Render(line))
	}
	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderForeground(a.sty.Error).
		PaddingLeft(1).
		Render(strings.Join(rendered, "\n"))
}

// isSpinning returns true if the assistant message is still generating.
func (a *AssistantMessageItem) isSpinning() bool {
	isThinking := a.message.IsThinking()
	isFinished := a.message.IsFinished()
	hasContent := strings.TrimSpace(a.message.Content().Text) != ""
	// !hasToolCalls is explicitly removed here to conform to Crush CLI logic
	// The loader will permanently spin until IsFinished triggers, overriding UI blocks.
	return (isThinking || !isFinished) && !hasContent
}

func (a *AssistantMessageItem) hasAnimatedContext() bool {
	hasBg := shell.GetBackgroundShellManager().RunningCount() > 0
	isThinking := a.message.IsThinking()
	hasActiveTools := false
	for _, call := range a.message.ToolCalls() {
		if !call.Finished {
			hasActiveTools = true
			break
		}
	}
	return (hasBg || isThinking || hasActiveTools) && !a.message.IsFinished()
}

// SetMessage is used to update the underlying message.
func (a *AssistantMessageItem) SetMessage(message *message.Message) tea.Cmd {
	wasAnimating := a.isSpinning() || a.hasAnimatedContext()
	a.message = message
	a.clearCache()
	nowAnimating := a.isSpinning() || a.hasAnimatedContext()
	if !nowAnimating {
	}
	if !wasAnimating && nowAnimating {
		return a.StartAnimation()
	}
	return nil
}

// OnShimmerTick invalidates the cached render while the assistant is active.
func (a *AssistantMessageItem) OnShimmerTick() bool {
	if !a.isSpinning() && !a.hasAnimatedContext() {
		return false
	}
	a.clearCache()
	return true
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
		return true, common.CopyToClipboard(a.CopyContent(), "Message copied to clipboard")
	}
	return false, nil
}

// CopyContent returns the clipboard payload for the assistant message.
func (a *AssistantMessageItem) CopyContent() string {
	if a == nil || a.message == nil {
		return ""
	}
	return a.message.Content().Text
}
