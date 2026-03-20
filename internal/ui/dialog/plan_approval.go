package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

const (
	// PlanApprovalID is the identifier for the plan approval dialog.
	PlanApprovalID              = "plan_approval"
	planApprovalDialogMaxWidth  = 65
	planApprovalDialogMaxHeight = 16
)

// PlanApprovalDialog represents a dialog that appears after a plan is created.
// Reference: Codex CLI PR #9712 "TUI: prompt to implement plan and switch to Execute"
type PlanApprovalDialog struct {
	com         *common.Common
	help        help.Model
	plan        []session.Todo
	explanation string
	selected    int // 0 = Switch to Pair Programming & Implement, 1 = Stay in Plan Mode

	keyMap struct {
		Select   key.Binding
		Previous key.Binding
		Next     key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*PlanApprovalDialog)(nil)

// NewPlanApprovalDialog creates a new plan approval dialog.
func NewPlanApprovalDialog(com *common.Common, plan []session.Todo, explanation string) (*PlanApprovalDialog, error) {
	p := &PlanApprovalDialog{
		com:         com,
		plan:        plan,
		explanation: explanation,
		selected:    0, // Default to "Implement Plan"
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	p.help = help

	p.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	p.keyMap.Previous = key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "previous"),
	)
	p.keyMap.Next = key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next"),
	)
	p.keyMap.Close = CloseKey

	return p, nil
}

// ID implements Dialog.
func (p *PlanApprovalDialog) ID() string {
	return PlanApprovalID
}

// HandleMsg implements [Dialog].
func (p *PlanApprovalDialog) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.Close):
			return ActionRefinePlan{}
		case key.Matches(msg, p.keyMap.Previous):
			if p.selected > 0 {
				p.selected--
			}
		case key.Matches(msg, p.keyMap.Next):
			if p.selected < 1 {
				p.selected++
			}
		case key.Matches(msg, p.keyMap.Select):
			switch p.selected {
			case 0:
				return ActionImplementPlan{}
			case 1:
				return ActionRefinePlan{}
			}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (p *PlanApprovalDialog) Cursor() *tea.Cursor {
	return nil
}

// Draw implements [Dialog].
func (p *PlanApprovalDialog) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := p.com.Styles
	width := max(0, min(planApprovalDialogMaxWidth, area.Dx()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	p.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Plan is ready. Ready to implement?"
	rc.Gap = 1

	// Add explanation if provided
	if p.explanation != "" {
		rc.AddPart(t.Dialog.SecondaryText.Render(p.explanation))
	}

	// Add plan summary
	planView := p.renderPlan(innerWidth)
	rc.AddPart(planView)

	// Add action buttons (Codex PR #9712: prompt to implement plan and switch)
	buttons := p.renderButtons()
	rc.AddPart(buttons)

	rc.Help = p.help.View(p)

	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements [help.KeyMap].
func (p *PlanApprovalDialog) ShortHelp() []key.Binding {
	return []key.Binding{
		p.keyMap.Previous,
		p.keyMap.Next,
		p.keyMap.Select,
		p.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (p *PlanApprovalDialog) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{p.keyMap.Previous, p.keyMap.Next, p.keyMap.Select, p.keyMap.Close},
	}
}

func (p *PlanApprovalDialog) renderPlan(width int) string {
	if len(p.plan) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	for i, todo := range p.plan {
		if i >= 8 {
			// Truncate long plans
			remaining := len(p.plan) - i
			b.WriteString(p.com.Styles.Muted.Render(fmt.Sprintf("  … and %d more steps", remaining)))
			break
		}

		var icon string
		var textStyle lipgloss.Style

		switch todo.Status {
		case session.TodoStatusCompleted:
			icon = p.com.Styles.Tool.TodoCompletedIcon.Render(styles.TodoCompletedIcon + " ")
			textStyle = p.com.Styles.Muted
		case session.TodoStatusInProgress:
			icon = p.com.Styles.Tool.TodoInProgressIcon.Render(styles.TodoInProgressIcon + " ")
			textStyle = p.com.Styles.Base
		default:
			icon = p.com.Styles.Tool.TodoPendingIcon.Render(styles.TodoPendingIcon + " ")
			textStyle = p.com.Styles.Subtle
		}

		text := todo.Content
		if todo.Status == session.TodoStatusInProgress && todo.ActiveForm != "" {
			text = todo.ActiveForm
		}

		line := icon + textStyle.Render(text)
		b.WriteString(line + "\n")
	}

	return b.String()
}

func (p *PlanApprovalDialog) renderButtons() string {
	buttons := []string{
		"Implement This Plan",
		"Suggest Changes",
	}

	var b strings.Builder
	b.WriteString("\n")

	for i, btn := range buttons {
		var btnStyle lipgloss.Style
		if i == p.selected {
			btnStyle = p.com.Styles.ButtonFocus.Bold(true)
		} else {
			btnStyle = p.com.Styles.ButtonBlur
		}

		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(btnStyle.Render(" " + btn + " "))
	}

	return b.String()
}

// ActionImplementPlan is sent when user chooses to implement the plan.
type ActionImplementPlan struct{}
