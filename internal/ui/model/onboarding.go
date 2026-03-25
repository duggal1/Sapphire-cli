package model

import (
	"fmt"
	"image"
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/home"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/util"
)

// markProjectInitialized marks the current project as initialized in the config.
func (m *UI) markProjectInitialized() tea.Msg {
	cfg := m.com.Config()
	if cfg == nil {
		return util.NewErrorMsg(fmt.Errorf("configuration not loaded"))
	}
	err := config.MarkProjectInitialized(cfg)
	if err != nil {
		slog.Error(err.Error())
		return util.NewErrorMsg(err)
	}
	return util.InfoMsg{Type: util.InfoTypeSuccess, Msg: "Project initialized"}
}

// updateInitializeView handles keyboard input for the project initialization prompt.
func (m *UI) updateInitializeView(msg tea.KeyPressMsg) (cmds []tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.Initialize.Enter):
		if m.onboarding.yesInitializeSelected {
			cmds = append(cmds, m.initializeProject())
		} else {
			cmds = append(cmds, m.skipInitializeProject())
		}
	case key.Matches(msg, m.keyMap.Initialize.Switch):
		m.onboarding.yesInitializeSelected = !m.onboarding.yesInitializeSelected
	case key.Matches(msg, m.keyMap.Initialize.Yes):
		cmds = append(cmds, m.initializeProject())
	case key.Matches(msg, m.keyMap.Initialize.No):
		cmds = append(cmds, m.skipInitializeProject())
	}
	return cmds
}

// handleInitializeClick routes mouse clicks on the initialize prompt buttons.
func (m *UI) handleInitializeClick(msg tea.MouseClickMsg) tea.Cmd {
	if m.state != uiInitialize {
		return nil
	}
	if msg.Button != tea.MouseButton1 {
		return nil
	}
	if !image.Pt(msg.X, msg.Y).In(m.layout.main) {
		return nil
	}

	// The prompt is bottom-aligned, so button clicks are expected on the last
	// couple of rows of the main layout.
	if msg.Y < m.layout.main.Max.Y-2 {
		return nil
	}

	yesButton := common.Button(m.com.Styles, common.ButtonOpts{
		Text:     "Yes",
		Selected: true,
	})
	noButton := common.Button(m.com.Styles, common.ButtonOpts{
		Text:     "No",
		Selected: false,
	})

	yesWidth := lipgloss.Width(yesButton)
	noWidth := lipgloss.Width(noButton)

	yesStart := m.layout.main.Min.X
	yesEnd := yesStart + yesWidth
	noStart := yesEnd + 1
	noEnd := noStart + noWidth

	switch {
	case msg.X >= yesStart && msg.X < yesEnd:
		m.onboarding.yesInitializeSelected = true
		return m.initializeProject()
	case msg.X >= noStart && msg.X < noEnd:
		m.onboarding.yesInitializeSelected = false
		return m.skipInitializeProject()
	default:
		return nil
	}
}

// initializeProject starts project initialization and transitions to the landing view.
func (m *UI) initializeProject() tea.Cmd {
	// clear the session
	var cmds []tea.Cmd
	if cmd := m.newSession(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	cfg := m.com.Config()

	initialize := func() tea.Msg {
		initPrompt, err := agent.InitializePrompt(*cfg)
		if err != nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: err.Error()}
		}
		return sendMessageMsg{Content: initPrompt}
	}
	// Mark the project as initialized
	cmds = append(cmds, initialize, m.markProjectInitialized)

	return tea.Sequence(cmds...)
}

// skipInitializeProject skips project initialization and transitions to the landing view.
func (m *UI) skipInitializeProject() tea.Cmd {
	// TODO: initialize the project
	m.setState(uiLanding, uiFocusEditor)
	// mark the project as initialized
	return m.markProjectInitialized
}

// initializeView renders the project initialization prompt with Yes/No buttons.
func (m *UI) initializeView() string {
	cfg := m.com.Config()
	s := m.com.Styles.Initialize
	cwd := home.Short(cfg.WorkingDir())
	initFile := cfg.Options.InitializeAs

	header := s.Header.Render("Would you like to initialize this project?")
	path := s.Accent.PaddingLeft(2).Render(cwd)
	desc := s.Content.Render(fmt.Sprintf("When I initialize your codebase I examine the project and put the result into an %s file which serves as general context.", initFile))
	hint := s.Content.Render("You can also initialize anytime via ") + s.Accent.Render("ctrl+p") + s.Content.Render(".")
	prompt := s.Content.Render("Would you like to initialize now?")

	buttons := common.ButtonGroup(m.com.Styles, []common.ButtonOpts{
		{Text: "Yes", Selected: m.onboarding.yesInitializeSelected},
		{Text: "No", Selected: !m.onboarding.yesInitializeSelected},
	}, " ")

	// max width 60 so the text is compact
	width := min(m.layout.main.Dx(), 60)

	return lipgloss.NewStyle().
		Width(width).
		Height(m.layout.main.Dy()).
		PaddingBottom(1).
		AlignVertical(lipgloss.Bottom).
		Render(strings.Join(
			[]string{
				header,
				path,
				desc,
				hint,
				prompt,
				buttons,
			},
			"\n\n",
		))
}
