package chat

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/attachments"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

// UserMessageItem represents a user message in the chat UI.
type UserMessageItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	attachments *attachments.Renderer
	message     *message.Message
	sty         *styles.Styles
}

// NewUserMessageItem creates a new UserMessageItem.
func NewUserMessageItem(sty *styles.Styles, message *message.Message, attachments *attachments.Renderer) MessageItem {
	return &UserMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		attachments:              attachments,
		message:                  message,
		sty:                      sty,
	}
}

// RawRender implements [MessageItem].
func (m *UserMessageItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)

	content, height, ok := m.getCachedRender(cappedWidth)
	// cache hit
	if ok {
		return m.renderHighlighted(content, cappedWidth, height)
	}

	renderer := common.MarkdownRenderer(m.sty, cappedWidth)

	msgContent := strings.TrimSpace(m.message.Content().Text)
	result, err := renderer.Render(msgContent)
	if err != nil {
		content = msgContent
	} else {
		content = strings.TrimSuffix(result, "\n")
	}

	if len(m.message.BinaryContent()) > 0 {
		attachmentsStr := m.renderAttachments(cappedWidth)
		if content == "" {
			content = attachmentsStr
		} else {
			content = strings.Join([]string{content, "", attachmentsStr}, "\n")
		}
	}

	height = lipgloss.Height(content)
	m.setCachedRender(content, cappedWidth, height)
	return m.renderHighlighted(content, cappedWidth, height)
}

// Render implements MessageItem.
func (m *UserMessageItem) Render(width int) string {
	var prefix string
	if m.focused {
		prefix = m.sty.Chat.Message.UserFocused.Render(">") + " "
	} else {
		prefix = m.sty.Chat.Message.UserBlurred.Render(">") + " "
	}
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	lines := strings.Split(m.RawRender(width), "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = prefix + line
		} else {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

// ID implements MessageItem.
func (m *UserMessageItem) ID() string {
	return m.message.ID
}

// Message returns the underlying message.
func (m *UserMessageItem) Message() *message.Message {
	return m.message
}

// renderAttachments renders attachments.
func (m *UserMessageItem) renderAttachments(width int) string {
	var attachments []message.Attachment
	var pasteBlocks []message.Attachment

	for _, at := range m.message.BinaryContent() {
		att := message.Attachment{
			FileName: at.Path,
			MimeType: at.MIMEType,
			Content:  at.Data,
		}
		// Check if this is a paste block
		if isPasteBlock(at.Path) {
			pasteBlocks = append(pasteBlocks, att)
		} else {
			attachments = append(attachments, att)
		}
	}

	var parts []string

	// Render regular attachments as badges
	if len(attachments) > 0 {
		parts = append(parts, m.attachments.Render(attachments, false, false, -1, width))
	}

	// Render paste blocks with full content
	for _, pb := range pasteBlocks {
		content := string(pb.Content)
		if content != "" {
			parts = append(parts, m.sty.Tool.Body.Render(content))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// isPasteBlock checks if a filename is a paste block.
func isPasteBlock(filename string) bool {
	return strings.HasPrefix(filename, "paste_") && strings.HasSuffix(filename, ".txt")
}

// HandleKeyEvent implements KeyEventHandler.
func (m *UserMessageItem) HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	if k := key.String(); k == "c" || k == "y" {
		text := m.message.Content().Text
		return true, common.CopyToClipboard(text, "Message copied to clipboard")
	}
	return false, nil
}
