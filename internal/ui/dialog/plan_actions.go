package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	PlanActionsID              = "plan-actions"
	planActionsDialogMaxWidth  = 60
	planActionsDialogMaxHeight = 13
)

type PlanActions struct {
	com   *common.Common
	help  help.Model
	index int

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		Close    key.Binding
	}
}

func NewPlanActions(com *common.Common) *PlanActions {
	d := &PlanActions{com: com}

	helpModel := help.New()
	helpModel.Styles = com.Styles.DialogHelpStyles()
	d.help = helpModel

	d.keyMap.Select = key.NewBinding(key.WithKeys("enter", "ctrl+y"), key.WithHelp("enter", "confirm"))
	d.keyMap.Next = key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("↓", "next item"))
	d.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑", "previous item"))
	d.keyMap.Close = CloseKey

	return d
}

func (d *PlanActions) ID() string { return PlanActionsID }

func (d *PlanActions) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, d.keyMap.Previous):
			d.index--
			if d.index < 0 {
				d.index = 2
			}
		case key.Matches(msg, d.keyMap.Next):
			d.index = (d.index + 1) % 3
		case key.Matches(msg, d.keyMap.Select):
			switch d.index {
			case 0:
				return ActionImplementProposedPlan{}
			case 1:
				return ActionReviseProposedPlan{}
			default:
				return ActionExitPlanMode{}
			}
		}
	}
	return nil
}

func (d *PlanActions) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(planActionsDialogMaxWidth, area.Dx()))
	contentWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	rc := NewRenderContext(t, width)
	rc.Gap = 1
	rc.AddPart(lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(t.Dialog.Title.Render("Implement this plan?")))
	rc.AddPart("")
	rc.AddPart(d.renderOption(contentWidth, 0, "Implement this plan"))
	rc.AddPart("")
	rc.AddPart(d.renderOption(contentWidth, 1, "Suggest changes"))
	rc.AddPart("")
	rc.AddPart(d.renderOption(contentWidth, 2, "Exit Plan mode"))
	rc.Help = d.help.View(d)

	DrawCenter(scr, area, rc.Render())
	return nil
}

func (d *PlanActions) renderOption(width int, index int, title string) string {
	t := d.com.Styles
	style := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Padding(0, 2).
		Foreground(t.FgMuted)
	if d.index == index {
		style = lipgloss.NewStyle().
			Width(width).
			Align(lipgloss.Center).
			Padding(0, 2).
			Bold(true).
			Foreground(t.Primary)
	}
	return style.Render(title)
}

func (d *PlanActions) ShortHelp() []key.Binding {
	return []key.Binding{d.keyMap.Previous, d.keyMap.Select, d.keyMap.Close}
}

func (d *PlanActions) FullHelp() [][]key.Binding {
	return [][]key.Binding{{d.keyMap.Select, d.keyMap.Next, d.keyMap.Previous, d.keyMap.Close}}
}
