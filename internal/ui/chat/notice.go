package chat

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

const InterruptNoticeID = "local:interrupt-notice"

const interruptNoticeText = "Conversation interrupted - tell the model what to do differently. Something went wrong? Hit `/feedback` to report the issue."

type NoticeMessageItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	id      string
	text    string
	sty     *styles.Styles
	isError bool
}

func NewInterruptNoticeMessageItem(sty *styles.Styles) MessageItem {
	return &NoticeMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		id:                       InterruptNoticeID,
		text:                     interruptNoticeText,
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
	return renderInterruptNotice(n.sty, n.text, width)
}

func renderInterruptNotice(sty *styles.Styles, text string, width int) string {
	contentWidth := max(0, width-4)
	lines := wrapPrefixedText(strings.TrimSpace(text), max(1, contentWidth), "", "")
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		body = append(body, sty.Chat.Message.ErrorTitle.Render(line))
	}
	prefix := sty.Chat.Message.ErrorTag.Render("■")
	return prefixRenderedBlock(prefix, strings.Join(body, "\n"))
}
