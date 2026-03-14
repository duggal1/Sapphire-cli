package styles

import (
	"fmt"
	"image/color"
	"math"
	"strings"

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

type shimmerRGB struct {
	r float64
	g float64
	b float64
}

var shimmerNeutralPalette = []shimmerRGB{
	{255, 255, 255},
	{245, 245, 245},
	{230, 230, 230},
	{210, 210, 210},
	{190, 190, 190},
	{175, 175, 175},
	{160, 160, 160},
}

var shimmerWarmPalette = []shimmerRGB{
	{255, 255, 255},
	{248, 244, 238},
	{236, 228, 216},
	{222, 208, 190},
	{205, 185, 160},
	{188, 163, 135},
	{170, 142, 112},
}

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

func shimmerColorAt(dist, spread float64, palette []shimmerRGB) color.Color {
	if spread <= 0 || len(palette) == 0 {
		return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	if dist >= spread {
		base := palette[0]
		return color.RGBA{
			R: uint8(math.Round(base.r)),
			G: uint8(math.Round(base.g)),
			B: uint8(math.Round(base.b)),
			A: 0xff,
		}
	}

	t := dist / spread
	slots := float64(len(palette) - 1)
	idx := t * slots
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if hi >= len(palette) {
		hi = len(palette) - 1
	}
	if lo < 0 {
		lo = 0
	}
	frac := idx - float64(lo)
	from := palette[len(palette)-1-lo]
	to := palette[len(palette)-1-hi]
	c := lerpShimmer(from, to, frac)
	return color.RGBA{
		R: uint8(math.Round(c.r)),
		G: uint8(math.Round(c.g)),
		B: uint8(math.Round(c.b)),
		A: 0xff,
	}
}

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

	n := float64(len(clusters))
	spread := n * 0.75
	travel := n + spread*2
	const shimmerFPS = 20.0
	speed := travel / (durationSec * shimmerFPS)
	const shimmerStep = 2.5
	pos := travel - math.Mod(float64(shift)*speed*shimmerStep, travel) - spread

	var o strings.Builder
	for i, cluster := range clusters {
		dist := math.Abs(float64(i) - pos)
		style := t.Base.Foreground(shimmerColorAt(dist, spread, palette))
		fmt.Fprint(&o, style.Render(cluster))
	}
	return o.String()
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
