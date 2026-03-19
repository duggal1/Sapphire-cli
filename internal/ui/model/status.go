package model

import (
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// DefaultStatusTTL is the default time-to-live for status messages.
const DefaultStatusTTL = 5 * time.Second

// Status is the status bar and help model.
type Status struct {
	com      *common.Common
	hideHelp bool
	help     help.Model
	helpKm   help.KeyMap
	msg      util.InfoMsg
}

// NewStatus creates a new status bar and help model.
func NewStatus(com *common.Common, km help.KeyMap) *Status {
	s := new(Status)
	s.com = com
	s.help = help.New()
	s.help.Styles = com.Styles.Help
	s.helpKm = km
	return s
}

// SetInfoMsg sets the status info message.
func (s *Status) SetInfoMsg(msg util.InfoMsg) {
	s.msg = msg
}

// ClearInfoMsg clears the status info message.
func (s *Status) ClearInfoMsg() {
	s.msg = util.InfoMsg{}
}

// SetWidth sets the width of the status bar and help view.
func (s *Status) SetWidth(width int) {
	helpStyle := s.com.Styles.Status.Help
	horizontalPadding := helpStyle.GetPaddingLeft() + helpStyle.GetPaddingRight()
	s.help.SetWidth(width - horizontalPadding)
}

// ShowingAll returns whether the full help view is shown.
func (s *Status) ShowingAll() bool {
	return s.help.ShowAll
}

// ToggleHelp toggles the full help view.
func (s *Status) ToggleHelp() {
	s.help.ShowAll = !s.help.ShowAll
}

// SetHideHelp sets whether the app is on the onboarding flow.
func (s *Status) SetHideHelp(hideHelp bool) {
	s.hideHelp = hideHelp
}

// Draw draws the status bar onto the screen.
func (s *Status) Draw(scr uv.Screen, area uv.Rectangle) {
	if !s.hideHelp {
		helpView := s.com.Styles.Status.Help.Render(s.help.View(s.helpKm))
		uv.NewStyledString(helpView).Draw(scr, area)
	}

	// Render notifications
	if s.msg.IsEmpty() {
		return
	}

	var toastBg color.Color
	var label string
	textColor := s.com.Styles.Toast.TextColor
	switch s.msg.Type {
	case util.InfoTypeError:
		toastBg = s.com.Styles.Toast.ErrorColor
		label = "ERROR"
	case util.InfoTypeWarn:
		toastBg = s.com.Styles.Toast.WarnColor
		label = "WARN"
	case util.InfoTypeUpdate:
		toastBg = s.com.Styles.Toast.InfoColor
		label = "UPDATE"
	case util.InfoTypeInfo:
		toastBg = s.com.Styles.Toast.InfoColor
		label = "INFO"
	case util.InfoTypeSuccess:
		toastBg = s.com.Styles.Toast.SuccessColor
		label = "SUCCESS"
	default:
		toastBg = s.com.Styles.Toast.SuccessColor
		label = "INFO"
	}

	if strings.HasPrefix(s.msg.Msg, "YOLO mode disabled.") {
		toastBg = s.com.Styles.Yellow
		textColor = s.com.Styles.BgBase
		label = "YOLO"
	}
	if textColor == nil {
		textColor = s.com.Styles.White
	}

	badgeStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(toastBg).
		Padding(0, 1).
		Bold(true)
	badge := badgeStyle.Render(" " + label + " ")

	messageWidth := maxInt(0, area.Dx()-lipgloss.Width(badge)-4)
	msg := ansi.Truncate(s.msg.Msg, messageWidth, "…")
	messageStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(toastBg).
		Padding(0, 2)
	message := messageStyle.Render(" " + msg + " ")

	toast := lipgloss.JoinHorizontal(lipgloss.Left, badge, message)
	toastView := lipgloss.Place(area.Dx(), maxInt(1, area.Dy()), lipgloss.Left, lipgloss.Bottom, toast)

	uv.NewStyledString(toastView).Draw(scr, area)
}

// clearInfoMsgCmd returns a command that clears the info message after the
// given TTL.
func clearInfoMsgCmd(ttl time.Duration) tea.Cmd {
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return util.ClearStatusMsg{}
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
