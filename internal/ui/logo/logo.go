// Package logo renders a Sapphire wordmark in a stylized way.
package logo

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// letterform represents a letterform. It can be stretched horizontally by
// a given amount via the boolean argument.
type letterform func(bool) string

const diag = `╱`

// Opts are the options for rendering the Sapphire title art.
type Opts struct {
	FieldColor   color.Color // diagonal lines
	TitleColorA  color.Color // left gradient ramp point
	TitleColorB  color.Color // right gradient ramp point
	CharmColor   color.Color // Beta text color
	VersionColor color.Color // Version text color
	Width        int         // width of the rendered logo, used for truncation
}

func Render(s *styles.Styles, version string, compact bool, o Opts) string {
	const charm = ""

	fg := func(c color.Color, s string) string {
		return lipgloss.NewStyle().Foreground(c).Render(s)
	}

	// Title.
	sapphire := heredoc.Doc(`
		█▀ ▄▀█ █▀█ █▀█ █░█ █ █▀█ █▀▀
		▄█ █▀█ █▀▀ █▀▀ █▀█ █ █▀▄ ██▄`)
	sapphire = strings.TrimPrefix(sapphire, "\n")
	sapphire = strings.TrimRight(sapphire, "\n")
	sapphireWidth := lipgloss.Width(sapphire)
	b := new(strings.Builder)
	for r := range strings.SplitSeq(sapphire, "\n") {
		fmt.Fprintln(b, styles.ApplyForegroundGrad(s, r, o.TitleColorA, o.TitleColorB))
	}
	sapphire = b.String()

	// Beta and version.
	metaRowGap := 1
	maxVersionWidth := sapphireWidth - lipgloss.Width(charm) - metaRowGap
	version = ansi.Truncate(version, maxVersionWidth, "…") // truncate version if too long.
	gap := max(0, sapphireWidth-lipgloss.Width(charm)-lipgloss.Width(version))
	metaRow := fg(o.CharmColor, charm) + strings.Repeat(" ", gap) + fg(o.VersionColor, version)

	// Join the meta row and big Sapphire title.
	sapphire = strings.TrimSpace(metaRow + "\n" + sapphire)

	// Narrow version with gradient diagonals.
	if compact {
		field := fg(o.FieldColor, strings.Repeat(diag, sapphireWidth))
		return strings.Join([]string{field, field, sapphire, field, ""}, "\n")
	}

	fieldHeight := lipgloss.Height(sapphire)

	// Left field with gradient.
	const leftWidth = 6
	leftFieldRow := fg(o.FieldColor, strings.Repeat(diag, leftWidth))
	leftField := new(strings.Builder)
	for range fieldHeight {
		fmt.Fprintln(leftField, leftFieldRow)
	}

	// Right field with gradient.
	rightWidth := max(15, o.Width-sapphireWidth-leftWidth-2) // 2 for the gap.
	const stepDownAt = 0
	rightField := new(strings.Builder)
	for i := range fieldHeight {
		width := rightWidth
		if i >= stepDownAt {
			width = rightWidth - (i - stepDownAt)
		}
		fmt.Fprint(rightField, fg(o.FieldColor, strings.Repeat(diag, width)), "\n")
	}

	// Return the wide version.
	const hGap = " "
	logo := lipgloss.JoinHorizontal(lipgloss.Top, leftField.String(), hGap, sapphire, hGap, rightField.String())
	if o.Width > 0 {
		// Truncate the logo to the specified width.
		lines := strings.Split(logo, "\n")
		for i, line := range lines {
			lines[i] = ansi.Truncate(line, o.Width, "")
		}
		logo = strings.Join(lines, "\n")
	}
	return logo
}

// SmallRender renders a smaller version of the Sapphire logo, suitable for
// smaller windows or sidebar usage.
func SmallRender(t *styles.Styles, width int) string {
	title := styles.ApplyBoldForegroundGrad(t, "Sapphire", t.LogoTitleColorA, t.LogoTitleColorB)
	remainingWidth := width - lipgloss.Width(title) - 1 // 1 for the space after "Sapphire"
	if remainingWidth > 0 {
		lines := strings.Repeat("╱", remainingWidth)
		title = fmt.Sprintf("%s %s", title, lipgloss.NewStyle().Foreground(t.LogoFieldColor).Render(lines))
	}
	return title
}
