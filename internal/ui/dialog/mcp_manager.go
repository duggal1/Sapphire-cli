package dialog

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/sapphire/internal/agent/tools/mcp"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/list"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	uv "github.com/charmbracelet/ultraviolet"
)

// MCPManagerID is the identifier for the MCP manager dialog.
const MCPManagerID = "mcp_manager"

const (
	mcpManagerMaxWidth  = 96
	mcpManagerMaxHeight = 26
)

type mcpManagerMode uint8

const (
	mcpManagerModeNormal mcpManagerMode = iota
	mcpManagerModeDeleting
)

// MCPManager represents the MCP manager dialog.
type MCPManager struct {
	com   *common.Common
	list  *list.FilterableList
	input textinput.Model
	help  help.Model
	mode  mcpManagerMode

	keyMap struct {
		Select        key.Binding
		Next          key.Binding
		Previous      key.Binding
		UpDown        key.Binding
		Add           key.Binding
		Edit          key.Binding
		Delete        key.Binding
		ConfirmDelete key.Binding
		CancelDelete  key.Binding
		Toggle        key.Binding
		Tools         key.Binding
		Refresh       key.Binding
		Close         key.Binding
	}
}

var _ Dialog = (*MCPManager)(nil)

// NewMCPManager creates a new MCP manager dialog.
func NewMCPManager(com *common.Common) (*MCPManager, error) {
	m := &MCPManager{
		com:  com,
		mode: mcpManagerModeNormal,
	}

	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	m.list = list.NewFilterableList(mcpManagerItems(com.Styles, com.Config())...)
	m.list.Focus()
	m.list.SetSelected(0)

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Filter MCP servers"
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "tools"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous"),
	)
	m.keyMap.Add = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add"),
	)
	m.keyMap.Edit = key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	)
	m.keyMap.Delete = key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "remove"),
	)
	m.keyMap.ConfirmDelete = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "confirm"),
	)
	m.keyMap.CancelDelete = key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n", "cancel"),
	)
	m.keyMap.Toggle = key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle"),
	)
	m.keyMap.Tools = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "tools"),
	)
	m.keyMap.Refresh = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	)
	m.keyMap.Close = CloseKey

	return m, nil
}

// ID implements Dialog.
func (m *MCPManager) ID() string {
	return MCPManagerID
}

// HandleMsg implements Dialog.
func (m *MCPManager) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch m.mode {
		case mcpManagerModeDeleting:
			switch {
			case key.Matches(msg, m.keyMap.ConfirmDelete):
				name := m.selectedName()
				m.mode = mcpManagerModeNormal
				return ActionDeleteMCPConfig{Name: name}
			case key.Matches(msg, m.keyMap.CancelDelete):
				m.mode = mcpManagerModeNormal
			}
		default:
			switch {
			case key.Matches(msg, m.keyMap.Close):
				return ActionClose{}
			case key.Matches(msg, m.keyMap.Add):
				return ActionOpenMCPConfig{IsNew: true}
			case key.Matches(msg, m.keyMap.Edit):
				name := m.selectedName()
				cfg, ok := m.com.Config().MCP[name]
				if !ok {
					return nil
				}
				return ActionOpenMCPConfig{Name: name, Config: cfg}
			case key.Matches(msg, m.keyMap.Delete):
				m.mode = mcpManagerModeDeleting
			case key.Matches(msg, m.keyMap.Toggle):
				name := m.selectedName()
				return ActionToggleMCPConfig{Name: name}
			case key.Matches(msg, m.keyMap.Refresh):
				name := m.selectedName()
				return ActionRefreshMCPServer{Name: name}
			case key.Matches(msg, m.keyMap.Select, m.keyMap.Tools):
				name := m.selectedName()
				return ActionOpenMCPTools{Name: name}
			case key.Matches(msg, m.keyMap.Previous):
				m.list.Focus()
				if m.list.IsSelectedFirst() {
					m.list.SelectLast()
				} else {
					m.list.SelectPrev()
				}
				m.list.ScrollToSelected()
			case key.Matches(msg, m.keyMap.Next):
				m.list.Focus()
				if m.list.IsSelectedLast() {
					m.list.SelectFirst()
				} else {
					m.list.SelectNext()
				}
				m.list.ScrollToSelected()
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				value := m.input.Value()
				m.list.SetFilter(value)
				m.list.ScrollToTop()
				m.list.SetSelected(0)
				return ActionCmd{Cmd: cmd}
			}
		}
	}
	return nil
}

// Draw implements Dialog.
func (m *MCPManager) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(mcpManagerMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(mcpManagerMaxHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	inputWidth := max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1)
	m.input.SetWidth(inputWidth)

	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	listHeight := max(0, height-heightOffset)

	leftWidth := max(28, innerWidth/2-1)
	rightWidth := max(0, innerWidth-leftWidth-1)

	m.list.SetSize(leftWidth, listHeight)

	rc := NewRenderContext(t, width)
	rc.Title = "MCP Manager"
	rc.TitleInfo = m.titleInfo(t)

	inputView := t.Dialog.InputPrompt.Render(m.input.View())
	rc.AddPart(inputView)

	listView := t.Dialog.List.Height(listHeight).Width(leftWidth).Render(m.list.Render())
	detailView := t.Dialog.ContentPanel.Width(rightWidth).Height(listHeight).Render(m.renderDetails(rightWidth))
	body := lipgloss.JoinHorizontal(lipgloss.Top, listView, " ", detailView)
	body = lipgloss.NewStyle().Width(innerWidth).Render(body)
	rc.AddPart(body)

	rc.Help = m.help.View(m)

	view := rc.Render()
	cur := InputCursor(m.com.Styles, m.input.Cursor())
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *MCPManager) titleInfo(t *styles.Styles) string {
	count := len(m.com.Config().MCP)
	label := fmt.Sprintf("%d servers", count)
	if count == 1 {
		label = "1 server"
	}
	return t.HalfMuted.Render(label)
}

func (m *MCPManager) renderDetails(width int) string {
	name := m.selectedName()
	if name == "" {
		return m.emptyDetails(width)
	}
	cfg, ok := m.com.Config().MCP[name]
	if !ok {
		return m.emptyDetails(width)
	}

	state, hasState := mcp.GetState(name)
	status := "unknown"
	if cfg.Disabled {
		status = "disabled"
	} else if hasState {
		switch state.State {
		case mcp.StateStarting:
			status = "starting"
		case mcp.StateConnected:
			status = "connected"
		case mcp.StateError:
			status = "error"
		case mcp.StateDisabled:
			status = "disabled"
		}
	}

	lines := []string{
		m.detailLine("Name", name),
		m.detailLine("Status", status),
		m.detailLine("Type", string(cfg.Type)),
	}
	if cfg.URL != "" {
		lines = append(lines, m.detailLine("URL", cfg.URL))
	}
	if cfg.Command != "" {
		lines = append(lines, m.detailLine("Command", cfg.Command))
	}
	if len(cfg.Args) > 0 {
		lines = append(lines, m.detailLine("Args", strings.Join(cfg.Args, ", ")))
	}
	if cfg.Timeout > 0 {
		lines = append(lines, m.detailLine("Timeout", fmt.Sprintf("%ds", cfg.Timeout)))
	}
	if len(cfg.DisabledTools) > 0 {
		lines = append(lines, m.detailLine("Disabled Tools", strings.Join(cfg.DisabledTools, ", ")))
	}
	if len(cfg.Env) > 0 {
		lines = append(lines, m.detailLine("Env", renderKeyValue(cfg.Env)))
	}
	if len(cfg.Headers) > 0 {
		lines = append(lines, m.detailLine("Headers", renderKeyValue(cfg.Headers)))
	}
	if hasState && state.State == mcp.StateConnected {
		lines = append(lines, m.detailLine("Tools", fmt.Sprintf("%d", state.Counts.Tools)))
		lines = append(lines, m.detailLine("Prompts", fmt.Sprintf("%d", state.Counts.Prompts)))
		lines = append(lines, m.detailLine("Resources", fmt.Sprintf("%d", state.Counts.Resources)))
	}
	if hasState && state.State == mcp.StateError && state.Error != nil {
		lines = append(lines, m.detailLine("Error", state.Error.Error()))
	}

	content := strings.Join(lines, "\n")
	return ansi.Truncate(content, width, "…")
}

func (m *MCPManager) emptyDetails(width int) string {
	msg := "No MCP servers configured"
	if len(m.com.Config().MCP) > 0 {
		msg = "Select an MCP server"
	}
	return ansi.Truncate(msg, width, "…")
}

func (m *MCPManager) detailLine(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return fmt.Sprintf("%s %s", m.com.Styles.Subtle.Render(label+":"), value)
}

func (m *MCPManager) selectedName() string {
	item := m.list.SelectedItem()
	if item == nil {
		return ""
	}
	if mcpItem, ok := item.(*MCPItem); ok {
		return mcpItem.ID()
	}
	return ""
}

func (m *MCPManager) Refresh() {
	selected := m.selectedName()
	m.list.SetItems(mcpManagerItems(m.com.Styles, m.com.Config())...)
	m.list.SetFilter(m.input.Value())
	m.selectByName(selected)
}

func (m *MCPManager) selectByName(name string) {
	if name == "" {
		m.list.SetSelected(0)
		return
	}
	for i, item := range m.list.FilteredItems() {
		if mcpItem, ok := item.(*MCPItem); ok && mcpItem.ID() == name {
			m.list.SetSelected(i)
			m.list.ScrollToSelected()
			return
		}
	}
	m.list.SetSelected(0)
}

func (m *MCPManager) ShortHelp() []key.Binding {
	if m.mode == mcpManagerModeDeleting {
		return []key.Binding{m.keyMap.ConfirmDelete, m.keyMap.CancelDelete}
	}
	return []key.Binding{m.keyMap.Add, m.keyMap.Edit, m.keyMap.Delete, m.keyMap.Tools, m.keyMap.Close}
}

func (m *MCPManager) FullHelp() [][]key.Binding {
	if m.mode == mcpManagerModeDeleting {
		return [][]key.Binding{{m.keyMap.ConfirmDelete, m.keyMap.CancelDelete}}
	}
	return [][]key.Binding{
		{m.keyMap.Add, m.keyMap.Edit, m.keyMap.Delete, m.keyMap.Toggle, m.keyMap.Refresh},
		{m.keyMap.UpDown, m.keyMap.Select, m.keyMap.Close},
	}
}

func mcpManagerItems(t *styles.Styles, cfg *config.Config) []list.FilterableItem {
	items := make([]list.FilterableItem, 0, len(cfg.MCP))
	states := mcp.GetStates()

	for _, entry := range cfg.MCP.Sorted() {
		state, ok := states[entry.Name]
		status := statusLabel(entry.MCP, state, ok)
		items = append(items, &MCPItem{
			name:   entry.Name,
			status: status,
			t:      t,
		})
	}
	return items
}

func statusLabel(cfg config.MCPConfig, state mcp.ClientInfo, hasState bool) string {
	if cfg.Disabled {
		return "disabled"
	}
	if !hasState {
		return "offline"
	}
	switch state.State {
	case mcp.StateStarting:
		return "starting"
	case mcp.StateConnected:
		return "connected"
	case mcp.StateError:
		return "error"
	case mcp.StateDisabled:
		return "disabled"
	default:
		return "offline"
	}
}

func renderKeyValue(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(values))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, values[k]))
	}
	return strings.Join(parts, ", ")
}
