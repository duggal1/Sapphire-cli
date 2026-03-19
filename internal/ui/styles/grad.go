package styles

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/charmbracelet/sapphire/internal/ui/shimmer"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"
)


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

// ── shimmer wrappers ──────────────────────────────────────────────────────────

func ShimmerTextCodex(t *Styles, input string) string {
	_ = t
	return shimmer.ShimmerText(input)
}

func ShimmerTextWithDot(t *Styles, input string) string {
	_ = t
	return shimmer.ShimmerWithDot(input)
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

// ── internal helpers ──────────────────────────────────────────────────────────

func splitGraphemeClusters(input string) []string {
	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}
	return clusters
}

// blendColors returns a slice of colors blended between the given stops.
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

	segmentSizes := make([]int, numSegments)
	baseSize := size / numSegments
	remainder := size % numSegments

	for i := 0; i < numSegments; i++ {
		segmentSizes[i] = baseSize
		if i < remainder {
			segmentSizes[i]++
		}
	}

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