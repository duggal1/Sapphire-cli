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

	clusters := splitGraphemeClusters(input)
	if len(clusters) == 0 {
		return []string{""}
	}
	if len(clusters) == 1 {
		style := t.Base.Foreground(color1)
		if bold {
			style = style.Bold(true)
		}
		return []string{style.Render(clusters[0])}
	}

	ramp := blendColors(len(clusters), color1, color2)
	for i, c := range ramp {
		style := t.Base.Foreground(c)
		if bold {
			style = style.Bold(true)
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

	clusters := splitGraphemeClusters(input)
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

	clusters := splitGraphemeClusters(input)
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

// shimmerStartTime anchors the shimmer sweep to process lifetime.
var shimmerStartTime = time.Now()

func splitGraphemeClusters(input string) []string {
	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}
	return clusters
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// cosineBell returns a smooth 0..1 intensity for a distance within a half-width.
func cosineBell(dist, halfWidth float64) float64 {
	if halfWidth <= 0 || dist >= halfWidth {
		return 0
	}
	x := math.Pi * (dist / halfWidth)
	return 0.5 * (1.0 + math.Cos(x))
}

func shimmerColor(c shimmerRGB) color.Color {
	return color.RGBA{
		R: uint8(math.Round(clampFloat(c.r, 0, 255))),
		G: uint8(math.Round(clampFloat(c.g, 0, 255))),
		B: uint8(math.Round(clampFloat(c.b, 0, 255))),
		A: 0xff,
	}
}

// lerpShimmer linearly interpolates between two shimmerRGB colors.
func lerpShimmer(a, b shimmerRGB, t float64) shimmerRGB {
	t = clamp01(t)
	return shimmerRGB{
		r: a.r + (b.r-a.r)*t,
		g: a.g + (b.g-a.g)*t,
		b: a.b + (b.b-a.b)*t,
	}
}

// shimmerTextWithPalette renders luxurious off-white text with a cool metallic
// shimmer band that has grey shoulders and a brighter core.
//
// palette[0] = base text
// palette[1] = shimmer shoulder
// palette[2] = shimmer core
func shimmerTextWithPalette(t *Styles, input string, shift int, palette []shimmerRGB, durationSec float64) string {
	if input == "" {
		return ""
	}

	clusters := splitGraphemeClusters(input)
	if len(clusters) == 0 {
		return ""
	}

	if len(palette) < 3 {
		return t.Base.Bold(true).Render(input)
	}

	n := len(clusters)

	// Extra runway so the shimmer enters and exits cleanly instead of clipping.
	padding := maxInt(8, int(math.Ceil(float64(n)*0.55)))
	period := float64(n + padding*2)

	if durationSec <= 0 {
		durationSec = 1.85
	}

	// Primary motion is time-based for smoothness.
	// shift is used only as a subtle phase offset so external ticks can still
	// influence the sweep without making motion jumpy.
	elapsed := time.Since(shimmerStartTime).Seconds()
	basePos := math.Mod((elapsed/durationSec)*period, period)
	phaseOffset := math.Mod(float64(shift)*0.025, period)
	pos := math.Mod(basePos+phaseOffset, period)
	if pos < 0 {
		pos += period
	}

	// Tighter and cleaner than the original wide wash.
	// This makes the shimmer actually read on short labels.
	bandHalfWidth := clampFloat(float64(n)*0.40, 3.5, 8.0)
	coreHalfWidth := clampFloat(bandHalfWidth*0.34, 1.15, 2.8)

	baseColor := palette[0]
	shoulderColor := palette[1]
	coreColor := palette[2]

	var o strings.Builder
	for i, cluster := range clusters {
		iPos := float64(i + padding)
		dist := math.Abs(iPos - pos)

		// Broad silver shoulder.
		shoulder := math.Pow(cosineBell(dist, bandHalfWidth), 1.12)

		// Bright tighter center.
		core := math.Pow(cosineBell(dist, coreHalfWidth), 1.04)

		// Build the shimmer in layers:
		// base -> grey shoulder -> soft-white core.
		c := lerpShimmer(baseColor, shoulderColor, shoulder*0.78)
		c = lerpShimmer(c, coreColor, core*0.96)

		style := t.Base.Foreground(shimmerColor(c)).Bold(true)
		fmt.Fprint(&o, style.Render(cluster))
	}

	return o.String()
}

// Neutral palette: off-white resting text with a cool grey/silver shimmer.
var shimmerNeutralPalette = []shimmerRGB{
	{245, 245, 244}, // base: neutral-100
	{214, 211, 209}, // shoulder: neutral-300
	{255, 255, 253}, // core: soft white
}

// Warm palette: slightly warmer off-white for softer UI surfaces.
var shimmerWarmPalette = []shimmerRGB{
	{245, 240, 235}, // base
	{221, 210, 198}, // shoulder
	{255, 249, 242}, // core
}

func ShimmerText(t *Styles, input string, shift int) string {
	_ = shift
	return ShimmerTextCodex(t, input)
}

func ShimmerTextNeutral(t *Styles, input string, shift int) string {
	_ = shift
	return ShimmerTextCodex(t, input)
}

func ShimmerTextWarm(t *Styles, input string, shift int) string {
	_ = shift
	return ShimmerTextCodex(t, input)
}

// blendColors returns a slice of colors blended between the given keys.
// Blending is done in RGB space to avoid hue shifts through purple.
func blendColors(size int, stops ...color.Color) []color.Color {
	if size <= 0 || len(stops) < 2 {
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
	for i := 0; i < numSegments; i++ {
		segmentSizes[i] = baseSize
		if i < remainder {
			segmentSizes[i]++
		}
	}

	// Generate colors for each segment using RGB blending to avoid purple shift.
	for i := 0; i < numSegments; i++ {
		c1 := stopsPrime[i]
		c2 := stopsPrime[i+1]
		segmentSize := segmentSizes[i]

		for j := 0; j < segmentSize; j++ {
			var t float64
			if segmentSize > 1 {
				t = float64(j) / float64(segmentSize-1)
			}
			c := c1.BlendRgb(c2, t)
			blended = append(blended, c)
		}
	}

	return blended
}
