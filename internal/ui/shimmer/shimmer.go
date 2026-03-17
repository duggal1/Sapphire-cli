package shimmer

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// ── process start ────────────────────────────────────────────────────────────

var (
	processStart     time.Time
	processStartOnce sync.Once
)

func elapsedSinceStart() time.Duration {
	processStartOnce.Do(func() {
		processStart = time.Now()
	})
	return time.Since(processStart)
}

// ── true color detection ─────────────────────────────────────────────────────
// v2 breaking change: Lip Gloss is now pure — it no longer queries the
// terminal itself. Bubble Tea owns I/O. Color downsampling is automatic
// via the colorprofile package. We detect the profile here and use it
// to decide whether to use full RGB or fall back to modifier-only styling.

func profile() colorprofile.Profile {
	return colorprofile.Detect(nil, nil)
}

func hasTrueColor() bool {
	return profile() == colorprofile.TrueColor
}

// ── blend ────────────────────────────────────────────────────────────────────
// t = 1.0 → all a, t = 0.0 → all b.

func blend(a, b [3]uint8, t float32) (uint8, uint8, uint8) {
	r := uint8(float32(a[0])*t + float32(b[0])*(1.0-t))
	g := uint8(float32(a[1])*t + float32(b[1])*(1.0-t))
	bl := uint8(float32(a[2])*t + float32(b[2])*(1.0-t))
	return r, g, bl
}

// ── color for level ──────────────────────────────────────────────────────────
// Fallback for terminals without true color support.
// v2 note: AdaptiveColor was removed — no longer needed here since
// colorprofile handles downsampling automatically.

func colorForLevel(intensity float32) lipgloss.Style {
	switch {
	case intensity < 0.2:
		return lipgloss.NewStyle().Faint(true)
	case intensity < 0.6:
		return lipgloss.NewStyle()
	default:
		return lipgloss.NewStyle().Bold(true)
	}
}

// ── ShimmerSpans ─────────────────────────────────────────────────────────────
// Returns one lipgloss-rendered string per character.
// Wire your own defaultFg / defaultBg from your terminal palette.

func ShimmerSpans(text string) []string {
	chars := []rune(text)
	if len(chars) == 0 {
		return nil
	}

	padding := 10
	period := len(chars) + padding*2
	sweepSeconds := 1.35
	elapsed := elapsedSinceStart().Seconds()
	posF := (math.Mod(elapsed, sweepSeconds) / sweepSeconds) * float64(period)
	pos := int(posF)

	trueColor := hasTrueColor()
	bandHalfWidth := 5.0

	// Match the Codex time-based shimmer model, but invert the palette so the
	// resting text is bright and the moving sweep is a softer gray.
	baseColor := [3]uint8{245, 239, 255}
	highlightColor := [3]uint8{183, 177, 196}

	spans := make([]string, 0, len(chars))

	for i, ch := range chars {
		iPos := i + padding
		dist := math.Abs(float64(iPos - pos))

		var t float32
		if dist <= bandHalfWidth {
			x := math.Pi * (dist / bandHalfWidth)
			t = float32(0.5 * (1.0 + math.Cos(x)))
		} else {
			t = 0.0
		}

		var style lipgloss.Style
		if trueColor {
			highlight := t
			if highlight > 1.0 {
				highlight = 1.0
			}
			r, g, b := blend(highlightColor, baseColor, highlight*0.9)
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))).
				Bold(true)
		} else {
			style = colorForLevel(t)
		}

		spans = append(spans, style.Render(string(ch)))
	}

	return spans
}

// ── ShimmerText ───────────────────────────────────────────────────────────────
// Convenience wrapper. Call inside your Bubble Tea View() method.
//
//	func (m Model) View() tea.View {
//	    return tea.NewView(shimmer.ShimmerText("Sapphire is thinking..."))
//	}
//
// v2 breaking change: View() now returns tea.View, not string.
// Wrap ShimmerText output in tea.NewView() at the call site.

func ShimmerText(text string) string {
	return strings.Join(ShimmerSpans(text), "")
}

func ShimmerDot() string {
	spans := ShimmerSpans("•")
	if len(spans) == 0 {
		return "•"
	}
	return spans[0]
}
