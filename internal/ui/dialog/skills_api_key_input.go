package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
)

const SapphireAPIKeyInputID = "sapphire_api_key_input"

// SapphireAPIKeyInput is the dialog used to collect a Sapphire Extended Skills API key.
type SapphireAPIKeyInput struct {
	com   *common.Common
	width int

	keyMap struct {
		Submit key.Binding
		Close  key.Binding
	}

	input textinput.Model
	help  help.Model
}

var _ Dialog = (*SapphireAPIKeyInput)(nil)

// NewSapphireAPIKeyInput creates a new Sapphire Extended Skills API key dialog.
func NewSapphireAPIKeyInput(com *common.Common) *SapphireAPIKeyInput {
	t := com.Styles
	m := &SapphireAPIKeyInput{
		com:   com,
		width: 66,
	}

	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize() - 2
	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Enter your Sapphire Extended Skills API key..."
	m.input.SetStyles(com.Styles.TextInput)
	m.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))
	m.input.EchoMode = textinput.EchoPassword
	m.input.Focus()

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "save"),
	)
	m.keyMap.Close = CloseKey

	return m
}

// ID implements Dialog.
func (m *SapphireAPIKeyInput) ID() string {
	return SapphireAPIKeyInputID
}

// HandleMsg implements Dialog.
func (m *SapphireAPIKeyInput) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Submit):
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				return nil
			}
			return ActionSaveSapphireAPIKey{APIKey: value}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				return ActionCmd{Cmd: cmd}
			}
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

// Draw implements Dialog.
func (m *SapphireAPIKeyInput) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	dialogStyle := t.Dialog.View.Width(m.width)
	helpStyle := t.Dialog.HelpView.Width(m.width - dialogStyle.GetHorizontalFrameSize())

	content := strings.Join([]string{
		m.headerView(),
		t.Dialog.InputPrompt.Render(m.inputView()),
		t.Dialog.SecondaryText.Render("This will be written in your global configuration:"),
		t.Dialog.SecondaryText.Render(config.GlobalConfigData()),
		"",
		helpStyle.Render(m.help.View(m)),
	}, "\n")

	view := dialogStyle.Render(content)
	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *SapphireAPIKeyInput) headerView() string {
	t := m.com.Styles
	title := t.Dialog.TitleText.Render("Enter your ") +
		t.Dialog.TitleAccent.Render("Sapphire Extended Skills API key") +
		t.Dialog.TitleText.Render(".")
	headerOffset := t.Dialog.Title.GetHorizontalFrameSize() + t.Dialog.View.GetHorizontalFrameSize()
	return common.DialogTitle(t, t.Dialog.Title.Render(title), m.width-headerOffset, m.com.Styles.Primary, m.com.Styles.Secondary)
}

func (m *SapphireAPIKeyInput) inputView() string {
	m.input.Prompt = "> "
	m.input.Focus()
	return m.input.View()
}

func (m *SapphireAPIKeyInput) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

func (m *SapphireAPIKeyInput) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.keyMap.Submit, m.keyMap.Close}}
}

func (m *SapphireAPIKeyInput) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Submit, m.keyMap.Close}
}
