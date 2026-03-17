// Package shimmer provides Codex-style shimmer animation for loading states.
// Based on codex-rs/tui/src/shimmer.rs architecture.
package shimmer

import (
	"image/color"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// Codex shimmer RGB structure
type codexShimmerRGB struct {
	r uint8
	g uint8
	b uint8
}

// Global state for time-based animation (synchronized to process start like Codex)
var (
	shimmerStartOnce sync.Once
	shimmerStart     time.Time
	shimmerTrueColorOnce sync.Once
	shimmerHasTrueColor bool
)

// elapsedSinceStart returns time since process start (Codex-compatible)
func elapsedSinceStart() time.Duration {
	shimmerStartOnce.Do(func() {
		shimmerStart = time.Now()
	})
	return time.Since(shimmerStart)
}

// supportsTrueColor detects if terminal supports 16M color (Codex-compatible)
func supportsTrueColor() bool {
	shimmerTrueColorOnce.Do(func() {
		shimmerHasTrueColor = colorprofile.Detect(os.Stdout, os.Environ()) == colorprofile.TrueColor
	})
	return shimmerHasTrueColor
}

// defaultForeground returns terminal foreground color
func defaultForeground() (codexShimmerRGB, bool) {
	// Codex uses (128, 128, 128) as default gray
	return codexShimmerRGB{r: 128, g: 128, b: 128}, false
}

// defaultBackground returns terminal background color
func defaultBackground() (codexShimmerRGB, bool) {
	// Codex uses (255, 255, 255) as default highlight
	return codexShimmerRGB{r: 255, g: 255, b: 255}, false
}

// blend blends two colors with alpha (Codex-compatible)
func blend(fg, bg codexShimmerRGB, alpha float64) codexShimmerRGB {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	return codexShimmerRGB{
		r: uint8(float64(fg.r)*alpha + float64(bg.r)*(1.0-alpha)),
		g: uint8(float64(fg.g)*alpha + float64(bg.g)*(1.0-alpha)),
		b: uint8(float64(fg.b)*alpha + float64(bg.b)*(1.0-alpha)),
	}
}

// codexShimmerColor converts to color.Color for lipgloss
func codexShimmerColor(c codexShimmerRGB) color.Color {
	return color.RGBA{R: c.r, G: c.g, B: c.b, A: 0xff}
}

// shimmerFallbackStyle returns style for non-true-color terminals (Codex-compatible)
func shimmerFallbackStyle(intensity float64) lipgloss.Style {
	if intensity < 0.2 {
		return lipgloss.NewStyle().Faint(true)
	}
	if intensity < 0.6 {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Bold(true)
}

// ShimmerTextCodex renders Codex-style shimmer effect for loading text.
// This is a direct translation of codex-rs/tui/src/shimmer.rs shimmer_spans()
func ShimmerTextCodex(_ *lipgloss.Style, text string) string {
	chars := []rune(text)
	if len(chars) == 0 {
		return ""
	}

	// Codex constants - exact replication
	padding := 10
	period := len(chars) + padding*2
	sweepSeconds := 2.0
	elapsed := elapsedSinceStart().Seconds()
	
	// Time-based sweep position (Codex logic)
	posF := math.Mod(elapsed, sweepSeconds) / sweepSeconds * float64(period)
	pos := int(posF)
	
	hasTrueColor := supportsTrueColor()
	bandHalfWidth := 5.0

	// Base and highlight colors (Codex defaults)
	baseColor := codexShimmerRGB{r: 128, g: 128, b: 128}
	if fg, ok := defaultForeground(); ok {
		baseColor = fg
	}
	highlightColor := codexShimmerRGB{r: 255, g: 255, b: 255}
	if bg, ok := defaultBackground(); ok {
		highlightColor = bg
	}

	var out strings.Builder
	for i, ch := range chars {
		iPos := i + padding
		dist := math.Abs(float64(iPos - pos))

		// Cosine wave highlight band (Codex logic)
		var t float64
		if dist <= bandHalfWidth {
			x := math.Pi * (dist / bandHalfWidth)
			t = 0.5 * (1.0 + math.Cos(x))
		}

		style := shimmerFallbackStyle(t)
		if hasTrueColor {
			// True color RGB blend (Codex logic)
			highlight := t
			if highlight < 0 {
				highlight = 0
			}
			if highlight > 1 {
				highlight = 1
			}
			c := blend(highlightColor, baseColor, highlight*0.9)
			style = lipgloss.NewStyle().
				Foreground(codexShimmerColor(c)).
				Bold(true)
		}

		out.WriteString(style.Render(string(ch)))
	}

	return out.String()
}

// ShimmerDot renders the Codex-style dot indicator (•) with shimmer effect
func ShimmerDot() string {
	return ShimmerTextCodex(nil, "•")
}

// ShimmerWithDot renders text with leading dot indicator (Codex style)
// Example: "• Generating..." with shimmer effect on both dot and text
func ShimmerWithDot(text string) string {
	dot := ShimmerDot()
	shimmered := ShimmerTextCodex(nil, text)
	return dot + " " + shimmered
}

// ShimmerModel is a Bubble Tea model for animated shimmer
type ShimmerModel struct {
	text      string
	withDot   bool
	lastFrame time.Time
}

// ShimmerTickMsg triggers shimmer animation update
type ShimmerTickMsg struct{}

// NewShimmerModel creates a new shimmer model
func NewShimmerModel(text string, withDot bool) ShimmerModel {
	return ShimmerModel{
		text:      text,
		withDot:   withDot,
		lastFrame: time.Now(),
	}
}

// Init starts the shimmer animation
func (m ShimmerModel) Init() tea.Cmd {
	return shimmerTick()
}

// Update handles animation ticks
func (m ShimmerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ShimmerTickMsg:
		m.lastFrame = time.Now()
		return m, shimmerTick()
	}
	return m, nil
}

// View renders the current shimmer frame
func (m ShimmerModel) View() tea.View {
	if m.withDot {
		return tea.NewView(ShimmerWithDot(m.text))
	}
	return tea.NewView(ShimmerTextCodex(nil, m.text))
}

// shimmerTick returns a command that triggers animation update at ~60fps
func shimmerTick() tea.Cmd {
	return tea.Tick(time.Second/60, func(t time.Time) tea.Msg {
		return ShimmerTickMsg{}
	})
}

// ShimmerView renders a single frame of shimmer (for non-Tea usage)
func ShimmerView(text string, withDot bool) string {
	if withDot {
		return ShimmerWithDot(text)
	}
	return ShimmerTextCodex(nil, text)
}
