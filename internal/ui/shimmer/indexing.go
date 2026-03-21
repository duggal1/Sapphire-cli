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
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
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
	elapsed := time.Duration(frame*80) * time.Millisecond
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
	switch mode {
	case renderTrueColor:
		baseColor := [3]uint8{124, 58, 237}
		highlightColor := [3]uint8{216, 180, 254}
		r, g, b := blend(highlightColor, baseColor, clampIndexing01(intensity)*0.9)
		return renderRGB(r, g, b, ch)
	case renderANSI256:
		palette := []int{92, 93, 99, 135, 141, 177}
		index := palette[min(len(palette)-1, max(0, int(math.Round(float64(intensity)*float64(len(palette)-1)))))]
		return fmt.Sprintf("\033[1m\033[38;5;%dm%s\033[0m", index, ch)
	default:
		return renderBold(ch)
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
