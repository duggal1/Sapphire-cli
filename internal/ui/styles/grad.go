package styles

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"
)

// ForegroundGrad returns a slice of strings representing the input string
// rendered with a horizontal gradient foreground from color1 to color2. Each
// string in the returned slice corresponds to a grapheme cluster in the input
// string. If bold is true, the rendered strings will be bolded.
func ForegroundGrad(t *Styles, input string, bold bool, color1, color2 color.Color) []string {
	if input == "" {
		return []string{""}
	}
	if len(input) == 1 {
		style := t.Base.Foreground(color1)
		if bold {
			style.Bold(true)
		}
		return []string{style.Render(input)}
	}
	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}

	ramp := blendColors(len(clusters), color1, color2)
	for i, c := range ramp {
		style := t.Base.Foreground(c)
		if bold {
			style.Bold(true)
		}
		clusters[i] = style.Render(clusters[i])
	}
	return clusters
}

// ApplyForegroundGrad renders a given string with a horizontal gradient
// foreground.
func ApplyForegroundGrad(t *Styles, input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var o strings.Builder
	clusters := ForegroundGrad(t, input, false, color1, color2)
	for _, c := range clusters {
		fmt.Fprint(&o, c)
	}
	return o.String()
}

// ApplyBoldForegroundGrad renders a given string with a horizontal gradient
// foreground.
func ApplyBoldForegroundGrad(t *Styles, input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var o strings.Builder
	clusters := ForegroundGrad(t, input, true, color1, color2)
	for _, c := range clusters {
		fmt.Fprint(&o, c)
	}
	return o.String()
}

// ApplyBoldForegroundGradShifted renders text with a shifted multi-stop
// gradient. The shift can be advanced over time to create a shimmer effect.
func ApplyBoldForegroundGradShifted(t *Styles, input string, shift int, stops ...color.Color) string {
	if input == "" {
		return ""
	}
	if len(stops) < 2 {
		return t.Base.Bold(true).Render(input)
	}

	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}
	if len(clusters) == 0 {
		return ""
	}

	ramp := blendColors(len(clusters)*2, stops...)
	if len(ramp) == 0 {
		return input
	}

	var o strings.Builder
	start := shift % len(ramp)
	if start < 0 {
		start += len(ramp)
	}

	for i, cluster := range clusters {
		style := t.Base.Foreground(ramp[(start+i)%len(ramp)]).Bold(true)
		fmt.Fprint(&o, style.Render(cluster))
	}

	return o.String()
}

// ApplyForegroundGradShifted renders text with a shifted multi-stop gradient.
func ApplyForegroundGradShifted(t *Styles, input string, shift int, stops ...color.Color) string {
	if input == "" {
		return ""
	}
	if len(stops) < 2 {
		return t.Base.Render(input)
	}

	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}
	if len(clusters) == 0 {
		return ""
	}

	ramp := blendColors(len(clusters)*2, stops...)
	if len(ramp) == 0 {
		return input
	}

	var o strings.Builder
	start := shift % len(ramp)
	if start < 0 {
		start += len(ramp)
	}

	for i, cluster := range clusters {
		style := t.Base.Foreground(ramp[(start+i)%len(ramp)])
		fmt.Fprint(&o, style.Render(cluster))
	}

	return o.String()
}

// shimmerRGB represents a color in the shimmer palette.
type shimmerRGB struct {
	r float64
	g float64
	b float64
}

// shimmerState holds the process start time for time-based shimmer animation.
var shimmerStartTime = time.Now()

// shimmerTextWithPalette renders text with a time-based shimmer effect.
// This implementation follows the Codex Rust shimmer.rs logic:
// - Time-based sweep (not frame counter) for smooth animation
// - Cosine interpolation for smooth bell-curve shimmer band
// - Band half-width of 5 characters with smooth fade
func shimmerTextWithPalette(t *Styles, input string, shift int, palette []shimmerRGB, durationSec float64) string {
	if input == "" {
		return ""
	}

	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}
	if len(clusters) == 0 {
		return ""
	}

	n := len(clusters)
	
	// Codex-style time-based sweep
	padding := 10
	period := n + padding*2
	sweepSeconds := float32(2.0)
	
	// Use elapsed time for smooth animation (like Codex)
	elapsed := time.Since(shimmerStartTime).Seconds()
	posF := float32(math.Mod(float64(elapsed), float64(sweepSeconds))) / sweepSeconds * float32(period)
	pos := int(posF)
	
	// Band half-width for cosine interpolation (Codex uses 5.0)
	const bandHalfWidth = 5.0
	
	// Base and highlight colors from palette
	// palette[0] = base (dim), palette[len-1] = highlight (bright)
	baseColor := palette[0]
	highlightColor := palette[len(palette)-1]
	
	var o strings.Builder
	for i := range clusters {
		// Distance from shimmer band center (Codex formula)
		iPos := float64(i) + float64(padding)
		dist := math.Abs(iPos - float64(pos))
		
		// Cosine interpolation for smooth bell-curve (Codex formula)
		var interp float64
		if dist <= bandHalfWidth {
			// Cosine fade: 0.5 * (1 + cos(PI * dist / bandHalfWidth))
			x := math.Pi * (dist / bandHalfWidth)
			interp = 0.5 * (1.0 + math.Cos(x))
		} else {
			interp = 0.0
		}
		
		// Blend highlight with base based on cosine value
		highlight := math.Max(0.0, math.Min(1.0, interp*0.9))
		blended := lerpShimmer(baseColor, highlightColor, highlight)
		
		style := t.Base.Foreground(color.RGBA{
			R: uint8(math.Round(blended.r)),
			G: uint8(math.Round(blended.g)),
			B: uint8(math.Round(blended.b)),
			A: 0xff,
		})
		fmt.Fprint(&o, style.Render(clusters[i]))
	}
	return o.String()
}

// lerpShimmer linearly interpolates between two shimmerRGB colors.
func lerpShimmer(a, b shimmerRGB, t float64) shimmerRGB {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return shimmerRGB{
		r: a.r + (b.r-a.r)*t,
		g: a.g + (b.g-a.g)*t,
		b: a.b + (b.b-a.b)*t,
	}
}

// shimmerNeutralPalette: base (dim gray) to highlight (bright white).
var shimmerNeutralPalette = []shimmerRGB{
	{90, 90, 90},    // Dim base gray
	{255, 255, 255}, // Bright white highlight
}

// shimmerWarmPalette: base (warm dim) to highlight (warm bright).
var shimmerWarmPalette = []shimmerRGB{
	{120, 80, 60},   // Dim warm brown
	{255, 220, 180}, // Bright warm white
}

func ShimmerText(t *Styles, input string, shift int) string {
	return ShimmerTextNeutral(t, input, shift)
}

func ShimmerTextNeutral(t *Styles, input string, shift int) string {
	return shimmerTextWithPalette(t, input, shift, shimmerNeutralPalette, 1.2)
}

func ShimmerTextWarm(t *Styles, input string, shift int) string {
	return shimmerTextWithPalette(t, input, shift, shimmerWarmPalette, 1.2)
}

// blendColors returns a slice of colors blended between the given keys.
// Blending is done in RGB space to avoid hue shifts through purple.
func blendColors(size int, stops ...color.Color) []color.Color {
	if len(stops) < 2 {
		return nil
	}

	stopsPrime := make([]colorful.Color, len(stops))
	for i, k := range stops {
		stopsPrime[i], _ = colorful.MakeColor(k)
	}

	numSegments := len(stopsPrime) - 1
	blended := make([]color.Color, 0, size)

	// Calculate how many colors each segment should have.
	segmentSizes := make([]int, numSegments)
	baseSize := size / numSegments
	remainder := size % numSegments

	// Distribute the remainder across segments.
	for i := range numSegments {
		segmentSizes[i] = baseSize
		if i < remainder {
			segmentSizes[i]++
		}
	}

	// Generate colors for each segment using RGB blending to avoid purple shift.
	for i := range numSegments {
		c1 := stopsPrime[i]
		c2 := stopsPrime[i+1]
		segmentSize := segmentSizes[i]

		for j := range segmentSize {
			var t float64
			if segmentSize > 1 {
				t = float64(j) / float64(segmentSize-1)
			}
			// Use BlendRgb instead of BlendHcl to avoid purple hue shift
			c := c1.BlendRgb(c2, t)
			blended = append(blended, c)
		}
	}

	return blended
}
