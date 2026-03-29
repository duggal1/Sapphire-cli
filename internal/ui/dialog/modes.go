package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/list"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/sahilm/fuzzy"
)

const (
	// ModesID is the identifier for the modes dialog.
	ModesID              = "modes"
	modesDialogMaxWidth  = 72
	modesDialogMaxHeight = 20
)

// Modes represents a dialog for selecting collaboration mode.
type Modes struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	currentMode planmode.SessionMode

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

// ModeOption represents a mode selection list item.
type ModeOption struct {
	mode        planmode.SessionMode
	title       string
	description string
	icon        string
	isCurrent   bool
	t           *styles.Styles
	m           fuzzy.Match
	cache       map[int]string
	focused     bool
}

var (
	_ Dialog   = (*Modes)(nil)
	_ ListItem = (*ModeOption)(nil)
)

// NewModes creates a new mode selection dialog.
func NewModes(com *common.Common, currentMode planmode.SessionMode) (*Modes, error) {
	m := &Modes{
		com:         com,
		currentMode: currentMode,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	m.help = help

	m.list = list.NewFilterableList()
	m.list.Focus()

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Type to filter"
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	m.keyMap.Close = CloseKey

	if err := m.setModeItems(); err != nil {
		return nil, err
	}

	return m, nil
}

// ID implements Dialog.
func (m *Modes) ID() string {
	return ModesID
}

// HandleMsg implements [Dialog].
func (m *Modes) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Previous):
			m.list.Focus()
			if m.list.IsSelectedFirst() {
				m.list.SelectLast()
				m.list.ScrollToBottom()
				break
			}
			m.list.SelectPrev()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Next):
			m.list.Focus()
			if m.list.IsSelectedLast() {
				m.list.SelectFirst()
				m.list.ScrollToTop()
				break
			}
			m.list.SelectNext()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Select):
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			modeOption, ok := selectedItem.(*ModeOption)
			if !ok {
				break
			}
			return ActionSelectMode{Mode: modeOption.mode}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			value := m.input.Value()
			m.list.SetFilter(value)
			m.list.ScrollToTop()
			m.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (m *Modes) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

// Draw implements [Dialog].
func (m *Modes) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(modesDialogMaxWidth, area.Dx()))
	height := max(0, min(modesDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	m.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	m.list.SetSize(innerWidth, height-heightOffset)
	m.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = m.dialogTitle()
	rc.Gap = 1
	inputView := t.Dialog.InputPrompt.Render(m.input.View())
	rc.AddPart(inputView)

	visibleCount := len(m.list.FilteredItems())
	if m.list.Height() >= visibleCount {
		m.list.ScrollToTop()
	} else {
		m.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	rc.AddPart(listView)
	rc.Help = m.help.View(m)

	view := rc.Render()

	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (m *Modes) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.UpDown,
		m.keyMap.Select,
		m.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (m *Modes) FullHelp() [][]key.Binding {
	help := [][]key.Binding{}
	slice := []key.Binding{
		m.keyMap.Select,
		m.keyMap.Next,
		m.keyMap.Previous,
		m.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		help = append(help, slice[i:end])
	}
	return help
}

func (m *Modes) setModeItems() error {
	selectableModes := planmode.SelectableModes()
	items := make([]list.FilterableItem, 0, len(selectableModes))
	selectedIndex := 0

	for i, mode := range selectableModes {
		items = append(items, &ModeOption{
			mode:        mode.Mode,
			title:       mode.Title,
			description: mode.Description,
			isCurrent:   m.currentMode == mode.Mode,
			t:           m.com.Styles,
		})
		if m.currentMode == mode.Mode {
			selectedIndex = i
		}
	}

	m.list.SetItems(items...)
	m.list.SetSelected(selectedIndex)
	m.list.ScrollToSelected()

	return nil
}

func (m *Modes) dialogTitle() string {
	return "Mode"
}

// Filter returns the filter value for the mode option.
func (m *ModeOption) Filter() string {
	return m.title
}

// ID implements ListItem.
func (m *ModeOption) ID() string {
	return string(m.mode)
}

// FilterValue implements ListItem.
func (m *ModeOption) FilterValue() string {
	return m.title
}

// SetMatch implements ListItem.
func (m *ModeOption) SetMatch(match fuzzy.Match) {
	m.m = match
}

// Match implements ListItem.
func (m *ModeOption) Match() fuzzy.Match {
	return m.m
}

// SetFocused implements ListItem.
func (m *ModeOption) SetFocused(focused bool) {
	if m.focused != focused {
		m.cache = nil
	}
	m.focused = focused
}

// Focused implements ListItem.
func (m *ModeOption) Focused() bool {
	return m.focused
}

// SetCache implements ListItem.
func (m *ModeOption) SetCache(cache map[int]string) {
	m.cache = cache
}

// Cache implements ListItem.
func (m *ModeOption) Cache() map[int]string {
	return m.cache
}

// Render implements ListItem.
func (m *ModeOption) Render(width int) string {
	if m.cache == nil {
		m.cache = make(map[int]string)
	}
	if v, ok := m.cache[width]; ok {
		return v
	}

	t := m.t
	accent := m.mode.AccentColor()
	if accent == "" {
		accent = "#A855F7"
	}
	var icon string
	if m.isCurrent {
		icon = t.Base.Foreground(lipgloss.Color(accent)).Bold(true).Render("● ")
	} else {
		icon = t.Muted.Render("○ ")
	}

	titleStyle := t.Dialog.NormalItem
	if m.focused {
		titleStyle = t.Dialog.SelectedItem.Background(lipgloss.Color(accent))
	}

	title := icon + titleStyle.Render(m.title)
	desc := strings.TrimSpace(m.description)
	if desc == "" {
		m.cache[width] = title
		return title
	}

	available := max(0, width-len(m.title)-6)
	rendered := title + t.Muted.Render(" - "+truncatePlain(desc, available))
	m.cache[width] = rendered
	return rendered
}

func truncatePlain(text string, width int) string {
	text = strings.TrimSpace(text)
	if width <= 0 || len(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	return text[:width-1] + "…"
}

// ActionSelectMode is sent when a mode is selected.
type ActionSelectMode struct {
	Mode planmode.SessionMode
}
