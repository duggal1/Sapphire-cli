package shimmer

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const indexingSweepDuration = 900 * time.Millisecond

type IndexingTickMsg time.Time

func IndexingTickCmd() tea.Cmd {
	return tea.Tick(shimmerTickInterval, func(t time.Time) tea.Msg {
		return IndexingTickMsg(t)
	})
}

func RenderIndexingText(text string, frame int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	mode := currentRenderMode()
	position := indexingPosition(len(runes), frame)
	var b strings.Builder
	for i, r := range runes {
		intensity := indexingIntensity(i, position)
		b.WriteString(renderIndexRune(mode, intensity, string(r)))
	}
	return b.String()
}

func indexingPosition(length, frame int) int {
	if length <= 0 {
		return 0
	}
	if frame < 0 {
		frame = 0
	}
	elapsed := time.Duration(frame) * shimmerTickInterval
	progress := math.Mod(elapsed.Seconds(), indexingSweepDuration.Seconds()) / indexingSweepDuration.Seconds()
	return int(progress * float64(length+12))
}

func indexingIntensity(index, position int) float32 {
	dist := float32(math.Abs(float64(index - position + 6)))
	if dist > 10 {
		return 0
	}
	x := math.Pi * float64(dist/10)
	return float32(0.5 * (1 + math.Cos(x)))
}

func renderIndexRune(mode renderMode, intensity float32, ch string) string {
	intensity = clampIndexing01(intensity)
	switch mode {
	case renderTrueColor:
		baseColor := [3]uint8{116, 116, 116}
		highlightColor := [3]uint8{230, 230, 230}
		r, g, b := blend(highlightColor, baseColor, intensity)
		return renderRGB(r, g, b, ch)
	case renderANSI256:
		palette := []int{239, 241, 244, 247, 250, 254}
		index := palette[min(len(palette)-1, max(0, int(math.Round(float64(intensity)*float64(len(palette)-1)))))]
		return fmt.Sprintf("\033[38;5;%dm%s\033[0m", index, ch)
	case renderDecorated:
		switch {
		case intensity < 0.18:
			return "\033[2m\033[38;5;239m" + ch + "\033[0m"
		case intensity < 0.45:
			return "\033[38;5;244m" + ch + "\033[0m"
		case intensity < 0.75:
			return "\033[38;5;250m" + ch + "\033[0m"
		default:
			return "\033[1m\033[38;5;254m" + ch + "\033[0m"
		}
	default:
		return ch
	}
}

func clampIndexing01(value float32) float32 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}
