package chat

import (
	"fmt"
	"math/rand"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/anim"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/shimmer"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"image/color"
)

// assistantMessageTruncateFormat is the text shown when an assistant message is
// truncated.
const assistantMessageTruncateFormat = "… (%d lines hidden) [click or space to expand]"

var mainLoaderPhrases = []string{
	// AI relatable
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

	// AI would never say this
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

	// universe — logical absurdity only
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

	// Claude Code style — single word but unhinged
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
	loaderPhrase      string
	thinkingExpanded  bool
	thinkingBoxHeight int // Tracks the rendered thinking box height for click detection.
}

// NewAssistantMessageItem creates a new AssistantMessageItem.
func NewAssistantMessageItem(sty *styles.Styles, message *message.Message) MessageItem {
	a := &AssistantMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		message:                  message,
		sty:                      sty,
		loaderPhrase:             mainLoaderPhrases[rand.Intn(len(mainLoaderPhrases))],
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
	if !a.isSpinning() {
		return nil
	}
	return tea.Batch(a.anim.Start(), shimmer.ShimmerTickCmd())
}

// Animate progresses the assistant message animation if it should be spinning.
func (a *AssistantMessageItem) Animate(msg anim.StepMsg) tea.Cmd {
	if !a.isSpinning() {
		return nil
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
	thinking := strings.TrimSpace(a.message.ReasoningContent().Thinking)
	content := strings.TrimSpace(a.message.Content().Text)
	// if the massage has reasoning content add that first
	if thinking != "" {
		messageParts = append(messageParts, a.renderThinking(a.message.ReasoningContent().Thinking, width))
	}

	// then add the main content
	if content != "" {
		// add a spacer between thinking and content
		if thinking != "" {
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

// renderThinking renders the thinking/reasoning content with footer.
func (a *AssistantMessageItem) renderThinking(thinking string, width int) string {
	renderer := common.PlainMarkdownRenderer(a.sty, width)
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

	thinkingStyle := a.sty.Chat.Message.ThinkingBox.Width(width)
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
		return shimmer.ShimmerText("Thinking...")
	} else if a.message.IsSummaryMessage {
		return shimmer.ShimmerText("Summarizing...")
	}
	return shimmer.ShimmerText(a.loaderPhrase)
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
	// !hasToolCalls is explicitly removed here to conform to Crush CLI logic
	// The loader will permanently spin until IsFinished triggers, overriding UI blocks.
	return (isThinking || !isFinished) && !hasContent
}

// SetMessage is used to update the underlying message.
func (a *AssistantMessageItem) SetMessage(message *message.Message) tea.Cmd {
	wasSpinning := a.isSpinning()
	a.message = message
	a.clearCache()
	if !wasSpinning && a.isSpinning() {
		return a.StartAnimation()
	}
	return nil
}

// OnShimmerTick implements ShimmerTickable.
func (a *AssistantMessageItem) OnShimmerTick() bool {
	if !a.isSpinning() {
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
