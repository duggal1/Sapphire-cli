package skillsbrowser

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/skillsmp"
	"github.com/duggal1/Sapphire-cli/internal/ui/list"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

type focusState uint8

const (
	focusList focusState = iota
	focusInput

	initialBrowseLimit = 120
)

type loadResultsMsg struct {
	Seq   uint64
	Query string
	Items []skillsmp.Skill
	Err   error
}

type installResultMsg struct {
	Key  string
	Item skillsmp.Skill
	Err  error
}

// Model renders the Sapphire extended skills browser.
type Model struct {
	styles    *styles.Styles
	client    *skillsmp.Client
	installer *skillsmp.Installer
	dataDir   string
	input     textinput.Model
	list      *list.List

	items    []*SkillItem
	itemsByK map[string]*SkillItem

	query string

	width  int
	height int

	focus focusState

	statusLine  string
	warningLine string
	errorLine   string

	loading      bool
	loadingLabel string
	loadSeq      uint64

	spinner      spinner.Model
	spinnerFrame int
}

func New(t *styles.Styles, apiKey, dataDir string) *Model {
	if t == nil {
		defaultStyles := styles.DefaultStyles(false)
		t = &defaultStyles
	}
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		dataDir = skillsmp.ResolveDataDir("")
	}

	m := &Model{
		styles:    t,
		client:    skillsmp.NewClient(apiKey),
		installer: skillsmp.NewInstaller(dataDir),
		dataDir:   dataDir,
		list:      list.NewList(),
		itemsByK:  make(map[string]*SkillItem),
		focus:     focusInput,
	}

	m.list.RegisterRenderCallback(list.FocusedRenderCallback(m.list))

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.SetStyles(t.TextInput)
	m.input.Placeholder = "Search Extended Skills"
	m.input.Focus()

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Line),
		spinner.WithStyle(t.Dialog.Spinner),
	)

	if strings.TrimSpace(apiKey) == "" {
		m.warningLine = "Sapphire API key is not set. Browsing and installs are disabled."
		m.focus = focusInput
	}

	return m
}

func (m *Model) Init() tea.Cmd {
	if strings.TrimSpace(m.client.APIKey) == "" {
		return nil
	}
	return m.fetchResults("")
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil
	case spinner.TickMsg:
		if !m.loading && !m.anyInstalling() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		m.spinnerFrame++
		m.updateInstallingFrames()
		if cmd == nil {
			return m, m.nextTick()
		}
		return m, tea.Batch(cmd, m.nextTick())
	case loadResultsMsg:
		if msg.Seq != m.loadSeq {
			return m, nil
		}
		m.loading = false
		m.loadingLabel = ""
		if msg.Err != nil {
			m.errorLine = msg.Err.Error()
			return m, m.nextTick()
		}
		m.errorLine = ""
		m.statusLine = ""
		m.applyResults(msg.Items)
		if strings.TrimSpace(msg.Query) == "" || len(m.items) > 0 {
			m.focus = focusList
			m.list.Focus()
			m.input.Blur()
		} else {
			m.focus = focusInput
			m.list.Blur()
			m.input.Focus()
		}
		return m, m.nextTick()
	case installResultMsg:
		item, ok := m.itemsByK[msg.Key]
		if !ok {
			return m, nil
		}
		if msg.Err != nil {
			item.SetState(skillStateError, msg.Err.Error())
			m.errorLine = msg.Err.Error()
			m.statusLine = ""
		} else {
			item.SetState(skillStateInstalled, "")
			m.errorLine = ""
			m.statusLine = fmt.Sprintf("Installed %s", msg.Item.DisplayName())
		}
		m.list.InvalidateItem(m.indexOfItem(item))
		return m, m.nextTick()
	case tea.KeyPressMsg:
		switch m.focus {
		case focusInput:
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				return m, tea.Quit
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				return m, m.fetchResults(m.input.Value())
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				m.query = m.input.Value()
				return m, cmd
			}
		case focusList:
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.focus = focusInput
				m.list.Blur()
				m.input.Focus()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				return m, m.installSelected()
			case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
				m.moveSelection(-1)
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
				m.moveSelection(1)
				return m, nil
			default:
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *Model) View() tea.View {
	if m.width == 0 {
		m.width = 100
	}
	if m.height == 0 {
		m.height = 28
	}
	m.layout()

	header := m.renderHeader()
	search := m.renderSearch()
	body := m.renderBody()
	status := m.renderStatus()

	parts := []string{header, search}
	if m.warningLine != "" {
		parts = append(parts, m.styles.TagBase.Background(m.styles.BgSubtle).Foreground(m.styles.Warning).Render(m.warningLine))
	}
	if m.errorLine != "" {
		parts = append(parts, m.styles.TagBase.Background(m.styles.BgSubtle).Foreground(m.styles.Red).Render(m.errorLine))
	}
	parts = append(parts, body, status)
	view := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, parts...))
	view.AltScreen = true
	view.WindowTitle = "Sapphire Skills"
	return view
}

func (m *Model) fetchResults(query string) tea.Cmd {
	query = strings.TrimSpace(query)
	if strings.TrimSpace(m.client.APIKey) == "" {
		m.warningLine = "Sapphire API key is not set. Browsing and installs are disabled."
		return nil
	}

	m.loading = true
	m.loadingLabel = fmt.Sprintf("Loading %d extended skills...", initialBrowseLimit)
	if query != "" {
		m.loadingLabel = "Searching Sapphire extended skills for " + query + "..."
	}
	m.errorLine = ""
	m.statusLine = ""
	m.loadSeq++
	seq := m.loadSeq
	m.query = query
	m.input.SetValue(query)
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		var (
			items []skillsmp.Skill
			err   error
		)
		if query == "" {
			items, err = m.client.List(context.Background(), initialBrowseLimit)
		} else {
			items, err = m.client.Search(context.Background(), query)
		}
		return loadResultsMsg{Seq: seq, Query: query, Items: items, Err: err}
	})
}

func (m *Model) installSelected() tea.Cmd {
	item := m.selectedItem()
	if item == nil {
		return nil
	}
	if item.State == skillStateInstalling {
		return nil
	}
	item.SetState(skillStateInstalling, "")
	m.errorLine = ""
	m.statusLine = fmt.Sprintf("Installing %s", item.Skill.DisplayName())
	m.list.InvalidateItem(m.indexOfItem(item))
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		loaded, err := m.client.LoadSkill(context.Background(), item.Skill.SkillID)
		if err == nil {
			err = m.installer.Install(loaded.Skill, []byte(loaded.Markdown))
		}
		return installResultMsg{Key: item.Key(), Item: item.Skill, Err: err}
	})
}

func (m *Model) layout() {
	searchFrame := m.styles.Dialog.InputPrompt.GetVerticalFrameSize()
	statusFrame := m.styles.Dialog.HelpView.GetVerticalFrameSize()
	headerHeight := 1
	messageLines := 0
	if m.warningLine != "" {
		messageLines++
	}
	if m.errorLine != "" {
		messageLines++
	}

	listHeight := m.height - headerHeight - searchFrame - statusFrame - messageLines - 4
	if listHeight < 4 {
		listHeight = 4
	}
	listWidth := m.width - m.styles.Dialog.ContentPanel.GetHorizontalFrameSize()
	if listWidth < 20 {
		listWidth = 20
	}
	m.list.SetSize(listWidth, listHeight)
	m.input.SetWidth(max(0, m.width-m.styles.Dialog.InputPrompt.GetHorizontalFrameSize()))
}

func (m *Model) renderHeader() string {
	title := m.styles.Base.Bold(true).Foreground(m.styles.Primary).Render("Extended Skills")
	meta := fmt.Sprintf("%d loaded", len(m.items))
	if m.loadingLabel != "" {
		meta = m.spinner.View() + " " + m.loadingLabel
	}
	if m.statusLine != "" {
		meta = m.statusLine
	}
	metaStyle := m.styles.HalfMuted
	if m.loading {
		metaStyle = m.styles.Base.Foreground(m.styles.Yellow)
	}
	if m.errorLine != "" {
		metaStyle = m.styles.Base.Foreground(m.styles.Red)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, title, strings.Repeat(" ", max(1, m.width-lipgloss.Width(title)-lipgloss.Width(metaStyle.Render(meta)))), metaStyle.Render(meta))
}

func (m *Model) renderSearch() string {
	input := m.input.View()
	prompt := "Search"
	if m.focus == focusInput {
		prompt = "Search"
	}
	return m.styles.Dialog.InputPrompt.Render(prompt + " " + input)
}

func (m *Model) renderBody() string {
	if len(m.items) == 0 {
		msg := "No extended skills loaded."
		if m.loading {
			msg = "Loading extended skills..."
		}
		return m.styles.Dialog.ContentPanel.Width(m.width).Render(msg)
	}
	return m.styles.Dialog.ContentPanel.Width(m.width).Render(m.list.Render())
}

func (m *Model) renderStatus() string {
	parts := []string{}
	switch m.focus {
	case focusInput:
		parts = append(parts, "enter search", "esc quit")
	case focusList:
		parts = append(parts, "enter install", "esc search")
	}
	parts = append(parts, "↑↓/j/k move")
	if strings.TrimSpace(m.client.APIKey) == "" {
		parts = append(parts, "API key missing")
	}
	parts = append(parts, fmt.Sprintf("%d extended skills", len(m.items)))
	line := strings.Join(parts, "  ")
	return m.styles.Dialog.HelpView.Width(m.width).Render(ansi.Truncate(line, m.width, "…"))
}

func (m *Model) applyResults(skills []skillsmp.Skill) {
	m.items = m.items[:0]
	nextItems := make([]*SkillItem, 0, len(skills))
	nextIndex := make(map[string]*SkillItem, len(skills))
	for _, skill := range skills {
		state := skillStateInstall
		if m.isInstalled(skill) {
			state = skillStateInstalled
		}
		item := NewSkillItem(m.styles, skill, state, "")
		if existing, ok := m.itemsByK[item.Key()]; ok {
			item.State = existing.State
			item.ErrorMessage = existing.ErrorMessage
			item.spinnerFrame = existing.spinnerFrame
		}
		nextItems = append(nextItems, item)
		nextIndex[item.Key()] = item
	}
	m.items = nextItems
	m.itemsByK = nextIndex
	renderItems := make([]list.Item, 0, len(nextItems))
	for _, item := range nextItems {
		renderItems = append(renderItems, item)
	}
	m.list.SetItems(renderItems...)
	m.list.SetSelected(0)
	m.list.Focus()
	if len(nextItems) == 0 {
		m.focus = focusInput
		m.input.Focus()
		m.list.Blur()
	}
}

func (m *Model) moveSelection(delta int) {
	if m.list.Len() == 0 {
		return
	}
	if delta < 0 {
		if m.list.IsSelectedFirst() {
			m.list.SelectLast()
		} else {
			m.list.SelectPrev()
		}
	} else {
		if m.list.IsSelectedLast() {
			m.list.SelectFirst()
		} else {
			m.list.SelectNext()
		}
	}
	m.list.ScrollToSelected()
}

func (m *Model) selectedItem() *SkillItem {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}
	skillItem, _ := item.(*SkillItem)
	return skillItem
}

func (m *Model) indexOfItem(target *SkillItem) int {
	if target == nil {
		return -1
	}
	for idx, item := range m.items {
		if item == target {
			return idx
		}
	}
	return -1
}

func (m *Model) anyInstalling() bool {
	for _, item := range m.items {
		if item.State == skillStateInstalling {
			return true
		}
	}
	return false
}

func (m *Model) updateInstallingFrames() {
	for idx, item := range m.items {
		if item.State != skillStateInstalling {
			continue
		}
		item.SetSpinnerFrame(m.spinnerFrame)
		m.list.InvalidateItem(idx)
	}
}

func (m *Model) nextTick() tea.Cmd {
	if m.loading || m.anyInstalling() {
		return m.spinner.Tick
	}
	return nil
}

func (m *Model) isInstalled(skill skillsmp.Skill) bool {
	checks := []string{
		skill.LocalSkillPath(m.dataDir),
		skill.LocalPluginManifestPath(m.dataDir),
		skill.LocalPluginSkillPath(m.dataDir),
	}
	for _, path := range checks {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}
