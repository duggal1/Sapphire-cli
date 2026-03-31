package chat

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

const InterruptNoticeID = "local:interrupt-notice"
const LocalErrorNoticeID = "local:error-notice"

const interruptNoticeText = "Conversation interrupted — tell the agent what to do differently. Did something go wrong? Please hit `/feedback` to report the issue."

type NoticeMessageItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	id      string
	label   string
	text    string
	details string
	sty     *styles.Styles
	isError bool
}

func NewInterruptNoticeMessageItem(sty *styles.Styles) MessageItem {
	return &NoticeMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		id:                       InterruptNoticeID,
		label:                    "■",
		text:                     interruptNoticeText,
		sty:                      sty,
		isError:                  true,
	}
}

func NewErrorNoticeMessageItem(sty *styles.Styles, text, details string) MessageItem {
	return &NoticeMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		id:                       LocalErrorNoticeID,
		label:                    "■",
		text:                     strings.TrimSpace(text),
		details:                  strings.TrimSpace(details),
		sty:                      sty,
		isError:                  true,
	}
}

func (n *NoticeMessageItem) ID() string {
	return n.id
}

func (n *NoticeMessageItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)
	if cached, height, ok := n.getCachedRender(cappedWidth); ok {
		return n.renderHighlighted(cached, cappedWidth, height)
	}

	rendered := n.renderContent(cappedWidth)
	height := lipgloss.Height(rendered)
	n.setCachedRender(rendered, cappedWidth, height)
	return n.renderHighlighted(rendered, cappedWidth, height)
}

func (n *NoticeMessageItem) Render(width int) string {
	rendered := n.RawRender(width)
	lines := strings.Split(rendered, "\n")
	focused := n.sty.Chat.Message.AssistantFocused.Render()
	blurred := n.sty.Chat.Message.AssistantBlurred.Render()
	for i, line := range lines {
		if n.focused {
			lines[i] = focused + line
		} else {
			lines[i] = blurred + line
		}
	}
	return strings.Join(lines, "\n")
}

func (n *NoticeMessageItem) renderContent(width int) string {
	if n.id == InterruptNoticeID {
		return renderInterruptNotice(n.sty, n.text, width)
	}
	return renderErrorNotice(n.sty, n.label, n.text, n.details, width)
}

func renderInterruptNotice(sty *styles.Styles, text string, width int) string {
	contentWidth := max(0, width-4)
	lines := wrapPrefixedText(strings.TrimSpace(text), max(1, contentWidth), "", "")
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		body = append(body, sty.Chat.Message.ErrorTitle.Render(line))
	}
	return prefixRenderedBlock(sty.Chat.Message.ErrorTag.Render("■"), strings.Join(body, "\n"))
}

func renderErrorNotice(sty *styles.Styles, label, text, details string, width int) string {
	contentWidth := max(0, width-4)
	lines := wrapPrefixedText(strings.TrimSpace(text), max(1, contentWidth), "", "")
	body := make([]string, 0, len(lines)+1)
	if len(lines) > 0 {
		body = append(body, sty.Chat.Message.ErrorTitle.Render("Error "+lines[0]))
		for _, line := range lines[1:] {
			body = append(body, sty.Chat.Message.ErrorTitle.Render(line))
		}
	} else {
		body = append(body, sty.Chat.Message.ErrorTitle.Render("Error"))
	}
	if strings.TrimSpace(details) != "" {
		for _, line := range wrapPrefixedText(strings.TrimSpace(details), max(1, contentWidth), "", "") {
			body = append(body, sty.Chat.Message.ErrorDetails.Render(line))
		}
	}
	prefix := sty.Chat.Message.ErrorTag.Render(label)
	return prefixRenderedBlock(prefix, strings.Join(body, "\n"))
}
