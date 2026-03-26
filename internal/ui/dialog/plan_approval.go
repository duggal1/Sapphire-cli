package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
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
	planContent string
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
func NewPlanApprovalDialog(com *common.Common, planContent, explanation string) (*PlanApprovalDialog, error) {
	p := &PlanApprovalDialog{
		com:         com,
		planContent: strings.TrimSpace(planContent),
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
				return ActionImplementPlan{Plan: p.planContent}
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
	if strings.TrimSpace(p.planContent) == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	lines := strings.Split(p.planContent, "\n")
	visible := 0
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if visible >= 8 {
			b.WriteString(p.com.Styles.Muted.Render("  …"))
			break
		}
		switch {
		case strings.HasPrefix(line, "#"):
			b.WriteString(p.com.Styles.Base.Bold(true).Render("  "+strings.TrimLeft(line, "# ")) + "\n")
		case strings.HasPrefix(line, "- "), strings.HasPrefix(line, "* "):
			b.WriteString(p.com.Styles.Tool.TodoPendingIcon.Render(styles.TodoPendingIcon+" ") + p.com.Styles.Base.Render(strings.TrimSpace(line[2:])) + "\n")
		case len(line) > 2 && line[1] == '.' && line[0] >= '0' && line[0] <= '9':
			b.WriteString(p.com.Styles.Tool.TodoPendingIcon.Render(styles.TodoPendingIcon+" ") + p.com.Styles.Base.Render(strings.TrimSpace(line[2:])) + "\n")
		default:
			b.WriteString("  " + p.com.Styles.Subtle.Render(line) + "\n")
		}
		visible++
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
type ActionImplementPlan struct {
	Plan string
}
