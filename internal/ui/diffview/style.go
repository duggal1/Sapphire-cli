package diffview

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// LineStyle defines the styles for a given line type in the diff view.
type LineStyle struct {
	LineNumber lipgloss.Style
	Symbol     lipgloss.Style
	Code       lipgloss.Style
}

// HunkLineStyle holds the styles used when rendering the unified hunk header line.
type HunkLineStyle struct {
	// Base is applied to the static text surrounding the ranges (spaces, @@ markers).
	Base lipgloss.Style
	// Minus styles the removal range segment (the -X,Y part).
	Minus lipgloss.Style
	// Plus styles the addition range segment (the +X,Y part).
	Plus lipgloss.Style
}

// Style defines the overall style for the diff view, including styles for
// different line types such as divider, missing, equal, insert, and delete
// lines.
type Style struct {
	HunkLine    HunkLineStyle
	DividerLine LineStyle
	MissingLine LineStyle
	EqualLine   LineStyle
	InsertLine  LineStyle
	DeleteLine  LineStyle
}

// DefaultLightStyle provides a default light theme style for the diff view.
func DefaultLightStyle() Style {
	return Style{
		HunkLine: HunkLineStyle{
			Base: lipgloss.NewStyle().
				Foreground(charmtone.Oyster).
				Background(charmtone.Salt),
			Minus: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FB7185")).
				Background(charmtone.Salt),
			Plus: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#22C55E")).
				Background(charmtone.Salt),
		},
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Iron).
				Background(charmtone.Thunder),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Oyster).
				Background(charmtone.Anchovy),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(charmtone.Ash),
			Code: lipgloss.NewStyle().
				Background(charmtone.Ash),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Charcoal).
				Background(charmtone.Ash),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Pepper).
				Background(charmtone.Salt),
		},
		// Insertions: clean green
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#16A34A")).
				Background(lipgloss.Color("#ECFDF3")),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#16A34A")).
				Background(lipgloss.Color("#F0FDF4")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Pepper).
				Background(lipgloss.Color("#F0FDF4")),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Cherry).
				Background(lipgloss.Color("#ffcdd2")),
			Symbol: lipgloss.NewStyle().
				Foreground(charmtone.Cherry).
				Background(lipgloss.Color("#ffebee")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Pepper).
				Background(lipgloss.Color("#ffebee")),
		},
	}
}

// DefaultDarkStyle provides a default dark theme style for the diff view.
func DefaultDarkStyle() Style {
	return Style{
		HunkLine: HunkLineStyle{
			Base: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(charmtone.Ox),
			Minus: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FB7185")).
				Background(charmtone.Ox),
			Plus: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#22C55E")).
				Background(charmtone.Ox),
		},
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.Sapphire),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.Ox),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(charmtone.Charcoal),
			Code: lipgloss.NewStyle().
				Background(charmtone.Charcoal),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Ash).
				Background(charmtone.Charcoal),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(charmtone.Pepper),
		},
		// Insertions: clean green
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ECE6A")).
				Background(lipgloss.Color("#16231B")),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ECE6A")).
				Background(lipgloss.Color("#1D2D22")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color("#1D2D22")),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F7768E")).
				Background(lipgloss.Color("#24161D")),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F7768E")).
				Background(lipgloss.Color("#2C1C24")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color("#2C1C24")),
		},
	}
}
