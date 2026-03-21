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

const JinaAPIKeyInputID = "jina_api_key_input"

type JinaAPIKeyInput struct {
	com           *common.Common
	continueIndex bool
	width         int
	keyMap        struct {
		Submit key.Binding
		Close  key.Binding
	}
	input textinput.Model
	help  help.Model
}

var _ Dialog = (*JinaAPIKeyInput)(nil)

func NewJinaAPIKeyInput(com *common.Common, continueIndex bool) *JinaAPIKeyInput {
	t := com.Styles
	m := &JinaAPIKeyInput{
		com:           com,
		continueIndex: continueIndex,
		width:         66,
	}

	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize() - 2
	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Enter your Jina API key..."
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

func (m *JinaAPIKeyInput) ID() string {
	return JinaAPIKeyInputID
}

func (m *JinaAPIKeyInput) HandleMsg(msg tea.Msg) Action {
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
			return ActionSaveJinaAPIKey{
				APIKey:        value,
				ContinueIndex: m.continueIndex,
			}
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

func (m *JinaAPIKeyInput) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	dialogStyle := t.Dialog.View.Width(m.width)
	helpStyle := t.Dialog.HelpView.Width(m.width - dialogStyle.GetHorizontalFrameSize())

	content := strings.Join([]string{
		m.headerView(),
		t.Dialog.InputPrompt.Render(m.inputView()),
		t.Dialog.SecondaryText.Render("Codebase indexing uses Jina embeddings."),
		t.Dialog.SecondaryText.Render("The key will be written to your global configuration:"),
		t.Dialog.SecondaryText.Render(config.GlobalConfigData()),
		"",
		helpStyle.Render(m.help.View(m)),
	}, "\n")

	view := dialogStyle.Render(content)
	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *JinaAPIKeyInput) headerView() string {
	t := m.com.Styles
	title := t.Dialog.TitleText.Render("Enter your ") +
		t.Dialog.TitleAccent.Render("Jina API key") +
		t.Dialog.TitleText.Render(".")
	headerOffset := t.Dialog.Title.GetHorizontalFrameSize() + t.Dialog.View.GetHorizontalFrameSize()
	return common.DialogTitle(t, t.Dialog.Title.Render(title), m.width-headerOffset, m.com.Styles.Primary, m.com.Styles.Secondary)
}

func (m *JinaAPIKeyInput) inputView() string {
	m.input.Prompt = "> "
	m.input.Focus()
	return m.input.View()
}

func (m *JinaAPIKeyInput) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

func (m *JinaAPIKeyInput) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.keyMap.Submit, m.keyMap.Close}}
}

func (m *JinaAPIKeyInput) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Submit, m.keyMap.Close}
}
