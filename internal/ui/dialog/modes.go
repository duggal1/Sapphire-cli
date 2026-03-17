package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/list"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	ModesID              = "modes"
	modesDialogMaxWidth  = 52
	modesDialogMaxHeight = 12
)

type Modes struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

type ModeItem struct {
	mode      config.AgentMode
	title     string
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

var (
	_ Dialog   = (*Modes)(nil)
	_ ListItem = (*ModeItem)(nil)
)

func NewModes(com *common.Common) (*Modes, error) {
	m := &Modes{com: com}

	helpModel := help.New()
	helpModel.Styles = com.Styles.DialogHelpStyles()
	m.help = helpModel

	m.list = list.NewFilterableList()
	m.list.Focus()

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Type to filter"
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.keyMap.Select = key.NewBinding(key.WithKeys("enter", "ctrl+y"), key.WithHelp("enter", "confirm"))
	m.keyMap.Next = key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("↓", "next item"))
	m.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑", "previous item"))
	m.keyMap.UpDown = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "choose"))
	m.keyMap.Close = CloseKey

	m.setModeItems()
	return m, nil
}

func (m *Modes) ID() string { return ModesID }

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
			selected := m.list.SelectedItem()
			if selected == nil {
				break
			}
			item, ok := selected.(*ModeItem)
			if !ok {
				break
			}
			return ActionSelectAgentMode{Mode: item.mode}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.list.SetFilter(m.input.Value())
			m.list.ScrollToTop()
			m.list.SetSelected(0)
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

func (m *Modes) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

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
	rc.Title = "Modes"
	rc.AddPart(t.Dialog.InputPrompt.Render(m.input.View()))

	visibleCount := len(m.list.FilteredItems())
	if m.list.Height() >= visibleCount {
		m.list.ScrollToTop()
	} else {
		m.list.ScrollToSelected()
	}

	rc.AddPart(t.Dialog.List.Height(m.list.Height()).Render(m.list.Render()))
	rc.Help = m.help.View(m)
	view := rc.Render()

	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *Modes) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.UpDown, m.keyMap.Select, m.keyMap.Close}
}

func (m *Modes) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.keyMap.Select, m.keyMap.Next, m.keyMap.Previous, m.keyMap.Close}}
}

func (m *Modes) setModeItems() {
	current := m.com.Config().AgentMode()
	items := make([]list.FilterableItem, 0, 5)
	for _, mode := range []config.AgentMode{
		config.AgentModePlan,
		config.AgentModeDebug,
		config.AgentModeSecurity,
		config.AgentModeArchitect,
		config.AgentModeReview,
	} {
		items = append(items, &ModeItem{
			mode:      mode,
			title:     mode.Label(),
			isCurrent: current == mode,
			t:         m.com.Styles,
		})
	}
	m.list.SetItems(items...)
	m.list.SetSelected(0)
}

func (m *Modes) dialogTitle() string {
	return "Modes"
}

func (i *ModeItem) Filter() string { return i.title }
func (i *ModeItem) ID() string     { return string(i.mode) }

func (i *ModeItem) SetFocused(focused bool) {
	if i.focused != focused {
		i.cache = nil
	}
	i.focused = focused
}

func (i *ModeItem) SetMatch(m fuzzy.Match) {
	i.cache = nil
	i.m = m
}

func (i *ModeItem) Render(width int) string {
	label := i.title
	if i.isCurrent {
		label += " · Current"
	}
	stylesDef := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Base,
		InfoTextFocused: i.t.Base,
	}
	return renderItem(stylesDef, label, "", i.focused, width, i.cache, &i.m)
}
