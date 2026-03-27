package skillsbrowser

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/skillsmp"
	"github.com/duggal1/Sapphire-cli/internal/ui/list"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

const (
	skillStateInstall skillState = iota
	skillStateInstalling
	skillStateInstalled
	skillStateError
)

var installSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type skillState uint8

type SkillItem struct {
	Skill        skillsmp.Skill
	Styles       *styles.Styles
	State        skillState
	ErrorMessage string

	focused      bool
	spinnerFrame int
}

var _ list.Item = (*SkillItem)(nil)
var _ list.Focusable = (*SkillItem)(nil)

func NewSkillItem(t *styles.Styles, skill skillsmp.Skill, state skillState, errText string) *SkillItem {
	return &SkillItem{
		Skill:        skill,
		Styles:       t,
		State:        state,
		ErrorMessage: errText,
	}
}

func (i *SkillItem) Key() string {
	return i.Skill.Key()
}

func (i *SkillItem) SetFocused(focused bool) {
	i.focused = focused
}

func (i *SkillItem) SetState(state skillState, errText string) {
	i.State = state
	i.ErrorMessage = errText
}

func (i *SkillItem) SetSpinnerFrame(frame int) {
	i.spinnerFrame = frame
}

func (i *SkillItem) Render(width int) string {
	if width <= 0 {
		return ""
	}

	lineStyle := i.Styles.Dialog.NormalItem
	if i.focused {
		lineStyle = i.Styles.Dialog.SelectedItem
	}

	badge := i.badge()
	titleLine := i.renderTitleLine(width, badge)
	bodyLine := i.renderBodyLine(width)
	return lineStyle.Width(width).Render(titleLine + "\n" + bodyLine)
}

func (i *SkillItem) renderTitleLine(width int, badge string) string {
	fields := []string{i.Skill.DisplayName()}
	if category := strings.TrimSpace(i.Skill.Category); category != "" {
		fields = append(fields, category)
	}
	if size := humanSize(i.Skill.SizeBytes); size != "" {
		fields = append(fields, size)
	}
	if i.Skill.IsNested {
		fields = append(fields, "nested")
	}
	left := strings.Join(fields, "  ")
	badgeWidth := ansi.StringWidth(badge)
	left = ansi.Truncate(left, max(0, width-badgeWidth-1), "…")
	gap := strings.Repeat(" ", max(0, width-ansi.StringWidth(left)-badgeWidth))
	return left + gap + badge
}

func (i *SkillItem) renderBodyLine(width int) string {
	text := strings.TrimSpace(i.Skill.RelativePath)
	if text == "" {
		text = strings.TrimSpace(i.Skill.MarkdownPath)
	}
	if text == "" {
		text = strings.TrimSpace(i.Skill.SkillID)
	}
	if i.State == skillStateError && strings.TrimSpace(i.ErrorMessage) != "" {
		text = "ERROR: " + strings.TrimSpace(i.ErrorMessage)
	}
	if text == "" {
		text = "No path available."
	}
	return ansi.Truncate(text, width, "…")
}

func (i *SkillItem) badge() string {
	style := i.Styles.TagBase.Background(i.Styles.BgSubtle).Foreground(i.Styles.Tertiary)
	label := "INSTALL"

	switch i.State {
	case skillStateInstalling:
		frame := installSpinnerFrames[i.spinnerFrame%len(installSpinnerFrames)]
		style = i.Styles.TagBase.Background(i.Styles.Yellow).Foreground(i.Styles.BgBase)
		label = frame + " INSTALLING..."
	case skillStateInstalled:
		style = i.Styles.TagBase.Background(i.Styles.GreenDark).Foreground(i.Styles.BgBase)
		label = "INSTALLED"
	case skillStateError:
		style = i.Styles.TagBase.Background(i.Styles.RedDark).Foreground(i.Styles.White)
		label = "ERROR"
	}

	return style.Render(label)
}

func humanSize(sizeBytes int) string {
	if sizeBytes <= 0 {
		return ""
	}
	if sizeBytes < 1024 {
		return fmt.Sprintf("%d B", sizeBytes)
	}
	if sizeBytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(sizeBytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(sizeBytes)/(1024*1024))
}
