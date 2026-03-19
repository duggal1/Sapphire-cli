package dialog

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/ui/list"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

type MCPItem struct {
	name    string
	status  string
	t       *styles.Styles
	focused bool
}

var _ list.FilterableItem = (*MCPItem)(nil)
var _ list.Focusable = (*MCPItem)(nil)

func (m *MCPItem) ID() string {
	return m.name
}

func (m *MCPItem) Filter() string {
	return m.name
}

func (m *MCPItem) SetFocused(focused bool) {
	m.focused = focused
}

func (m *MCPItem) Render(width int) string {
	style := m.t.Dialog.NormalItem
	if m.focused {
		style = m.t.Dialog.SelectedItem
	}

	info := strings.TrimSpace(m.status)
	if info != "" {
		info = fmt.Sprintf(" %s", info)
	}
	line := renderListLine(m.name, info, width)
	return style.Width(width).Render(line)
}

func renderListLine(title, info string, width int) string {
	infoWidth := ansi.StringWidth(info)
	titleWidth := max(0, width-infoWidth)
	title = ansi.Truncate(title, titleWidth, "…")
	gap := strings.Repeat(" ", max(0, width-ansi.StringWidth(title)-infoWidth))
	return lipgloss.JoinHorizontal(lipgloss.Left, title, gap, info)
}
