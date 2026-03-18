package shimmer

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/sapphire/internal/ui/anim"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// ── process start ────────────────────────────────────────────────────────────
// 1:1 of: static PROCESS_START: OnceLock<Instant> = OnceLock::new();

var (
	processStart     time.Time
	processStartOnce sync.Once
)

// 1:1 of: fn elapsed_since_start() -> Duration
func elapsedSinceStart() time.Duration {
	processStartOnce.Do(func() {
		processStart = time.Now()
	})
	return time.Since(processStart)
}

// ── true color detection ─────────────────────────────────────────────────────
// 1:1 of: supports_color::on_cached(supports_color::Stream::Stdout)
//
//	.map(|level| level.has_16m)
//	.unwrap_or(false)
func hasTrueColor() bool {
	return colorprofile.Detect(os.Stdout, os.Environ()) == colorprofile.TrueColor
}

// ── terminal palette ─────────────────────────────────────────────────────────
// 1:1 of: crate::terminal_palette::default_fg / default_bg
// Wire these to your own terminal_palette package.

func defaultFg() (uint8, uint8, uint8) {
	return 128, 128, 128 // unwrap_or((128, 128, 128))
}

func defaultBg() (uint8, uint8, uint8) {
	return 255, 255, 255 // unwrap_or((255, 255, 255))
}

// ── blend ────────────────────────────────────────────────────────────────────
// 1:1 of: crate::color::blend
// t = 1.0 → all a, t = 0.0 → all b.

func blend(a, b [3]uint8, t float32) (uint8, uint8, uint8) {
	r := uint8(float32(a[0])*t + float32(b[0])*(1.0-t))
	g := uint8(float32(a[1])*t + float32(b[1])*(1.0-t))
	bl := uint8(float32(a[2])*t + float32(b[2])*(1.0-t))
	return r, g, bl
}

// ── colorForLevel ────────────────────────────────────────────────────────────
// 1:1 of: fn color_for_level(intensity: f32) -> Style

func colorForLevel(intensity float32) lipgloss.Style {
	if intensity < 0.2 {
		return lipgloss.NewStyle().Faint(true) // Modifier::DIM
	} else if intensity < 0.6 {
		return lipgloss.NewStyle() // Style::default()
	} else {
		return lipgloss.NewStyle().Bold(true) // Modifier::BOLD
	}
}

// ── ShimmerSpans ─────────────────────────────────────────────────────────────
// 1:1 of: pub(crate) fn shimmer_spans(text: &str) -> Vec<Span<'static>>
// Prefixes a shimmering "•" dot before the text — see spinner().

func ShimmerSpans(text string) []string {
	chars := []rune(text)
	if len(chars) == 0 {
		return []string{} // return Vec::new()
	}

	// 1:1 of sweep math block
	padding := 10
	period := len(chars) + padding*2
	sweepSeconds := float32(2.0)
	posF := (float32(elapsedSinceStart().Seconds()) / sweepSeconds) *
		float32(period)
	posF = float32(math.Mod(float64(elapsedSinceStart().Seconds()), float64(sweepSeconds)) /
		float64(sweepSeconds) * float64(period))
	pos := int(posF)

	hasTrueColorVal := hasTrueColor()
	bandHalfWidth := float32(5.0)

	// 1:1 of: let base_color = default_fg().unwrap_or((128, 128, 128));
	// 1:1 of: let highlight_color = default_bg().unwrap_or((255, 255, 255));
	r0, g0, b0 := defaultFg()
	baseColor := [3]uint8{r0, g0, b0}
	r1, g1, b1 := defaultBg()
	highlightColor := [3]uint8{r1, g1, b1}

	spans := make([]string, 0, len(chars)) // Vec::with_capacity(chars.len())

	for i, ch := range chars {
		// 1:1 of: let i_pos = i as isize + padding as isize;
		iPos := i + padding
		// 1:1 of: let dist = (i_pos - pos).abs() as f32;
		dist := float32(math.Abs(float64(iPos - pos)))

		// 1:1 of cosine falloff block
		var t float32
		if dist <= bandHalfWidth {
			x := float32(math.Pi) * (dist / bandHalfWidth)
			t = 0.5 * (1.0 + float32(math.Cos(float64(x))))
		} else {
			t = 0.0
		}

		// 1:1 of style branch
		var style lipgloss.Style
		if hasTrueColorVal {
			// 1:1 of: let highlight = t.clamp(0.0, 1.0);
			highlight := t
			if highlight < 0.0 {
				highlight = 0.0
			}
			if highlight > 1.0 {
				highlight = 1.0
			}
			// 1:1 of: let (r, g, b) = blend(highlight_color, base_color, highlight * 0.9);
			r, g, b := blend(highlightColor, baseColor, highlight*0.9)
			// 1:1 of: Style::default().fg(Color::Rgb(r, g, b)).add_modifier(Modifier::BOLD)
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))).
				Bold(true)
		} else {
			style = colorForLevel(t)
		}

		// 1:1 of: spans.push(Span::styled(ch.to_string(), style));
		spans = append(spans, style.Render(string(ch)))
	}

	return spans
}

// ── Spinner ───────────────────────────────────────────────────────────────────
// 1:1 of: pub(crate) fn spinner(start_time: Option<Instant>, animations_enabled: bool) -> Span<'static>
// startTime is a pointer — nil maps to Option::None, non-nil maps to Option::Some(Instant).

func Spinner(startTime *time.Time, animationsEnabled bool) string {
	// 1:1 of: if !animations_enabled { return "•".dim(); }
	if !animationsEnabled {
		return lipgloss.NewStyle().Faint(true).Render("•")
	}

	// 1:1 of: let elapsed = start_time.map(|st| st.elapsed()).unwrap_or_default();
	var elapsed time.Duration
	if startTime != nil {
		elapsed = time.Since(*startTime)
	}

	// 1:1 of: supports_color check
	if hasTrueColor() {
		// 1:1 of: shimmer_spans("•")[0].clone()
		spans := ShimmerSpans("•")
		if len(spans) > 0 {
			return spans[0]
		}
		return "•"
	}

	// 1:1 of: let blink_on = (elapsed.as_millis() / 600).is_multiple_of(2);
	blinkOn := (elapsed.Milliseconds()/600)%2 == 0

	// 1:1 of: if blink_on { "•".into() } else { "◦".dim() }
	if blinkOn {
		return "•"
	}
	return lipgloss.NewStyle().Faint(true).Render("◦")
}

// ── ShimmerText ───────────────────────────────────────────────────────────────
// Convenience wrapper with "•" dot prefix — matches the spinner prefix pattern.
//
//	func (m Model) View() tea.View {
//	    return tea.NewView(shimmer.ShimmerText("Sapphire is thinking..."))
//	}

func ShimmerText(text string) string {
	return strings.Join(ShimmerSpans(text), "")
}

// ── ShimmerWithDotPrefix ──────────────────────────────────────────────────────
// Renders a shimmering "• text" — dot prefix shimmers alongside the text.

func ShimmerWithDotPrefix(text string) string {
	processStartOnce.Do(func() {
		processStart = time.Now()
	})
	dot := Spinner(&processStart, true)
	if strings.TrimSpace(text) == "" {
		return dot
	}
	return dot + " " + strings.Join(ShimmerSpans(text), "")
}

// ShimmerTickCmd returns a command that triggers a shimmer tick.
func ShimmerTickCmd() func() tea.Msg {
	return func() tea.Msg {
		return anim.StepMsg{ID: "default"}
	}
}
