package shimmer

import (
	"strings"
	"time"
)

func renderPlanRune(mode renderMode, intensity float32, ch string) string {
	switch mode {
	case renderTrueColor:
		baseColor := [3]uint8{44, 109, 214}
		midColor := [3]uint8{88, 180, 255}
		highlightColor := [3]uint8{214, 241, 255}
		intensity = clamp01(intensity)
		var r, g, b uint8
		if intensity < 0.6 {
			t := intensity / 0.6
			r, g, b = blend(midColor, baseColor, t)
		} else {
			t := (intensity - 0.6) / 0.4
			r, g, b = blend(highlightColor, midColor, t)
		}
		return renderRGB(r, g, b, ch)
	case renderANSI256:
		switch {
		case intensity < 0.18:
			return renderIndexedColor(25, false, ch)
		case intensity < 0.45:
			return renderIndexedColor(33, false, ch)
		case intensity < 0.75:
			return renderIndexedColor(45, true, ch)
		default:
			return renderIndexedColor(117, true, ch)
		}
	case renderDecorated:
		switch {
		case intensity < 0.18:
			return "\033[2m\033[38;5;25m" + ch + "\033[0m"
		case intensity < 0.45:
			return "\033[38;5;33m" + ch + "\033[0m"
		case intensity < 0.75:
			return "\033[1m\033[38;5;45m" + ch + "\033[0m"
		default:
			return "\033[1m\033[38;5;117m" + ch + "\033[0m"
		}
	default:
		return ch
	}
}

func renderPlanSpinnerFrame(mode renderMode, frame string, intensity float32) string {
	intensity = clamp01(intensity)
	switch mode {
	case renderTrueColor:
		baseColor := [3]uint8{39, 100, 201}
		highlightColor := [3]uint8{214, 241, 255}
		r, g, b := blend(highlightColor, baseColor, intensity)
		return renderRGB(r, g, b, frame)
	case renderANSI256:
		switch {
		case intensity < 0.2:
			return renderIndexedColor(25, false, frame)
		case intensity < 0.45:
			return renderIndexedColor(33, false, frame)
		case intensity < 0.75:
			return renderIndexedColor(45, true, frame)
		default:
			return renderIndexedColor(117, true, frame)
		}
	case renderDecorated:
		switch {
		case intensity < 0.2:
			return "\033[2m\033[38;5;25m" + frame + "\033[0m"
		case intensity < 0.45:
			return "\033[38;5;33m" + frame + "\033[0m"
		case intensity < 0.75:
			return "\033[1m\033[38;5;45m" + frame + "\033[0m"
		default:
			return "\033[1m\033[38;5;117m" + frame + "\033[0m"
		}
	default:
		return frame
	}
}

func planShimmerSpansAt(text string, elapsed time.Duration, mode renderMode) []string {
	chars := []rune(text)
	if len(chars) == 0 {
		return nil
	}

	position := shimmerPosition(len(chars), elapsed)
	spans := make([]string, 0, len(chars))
	for i, ch := range chars {
		spans = append(spans, renderPlanRune(mode, intensityAt(i, position), string(ch)))
	}
	return spans
}

func PlanSpans(text string) []string {
	return planShimmerSpansAt(text, elapsedSinceStart(), currentRenderMode())
}

func PlanText(text string) string {
	return strings.Join(PlanSpans(text), "")
}

func PlanWithDot(text string) string {
	processStartOnce.Do(func() { processStart = time.Now() })
	dot := CurrentSpinnerFrameAt(&processStart)
	if mode := currentRenderMode(); mode != renderPlain {
		dot = renderPlanSpinnerFrame(mode, dot, spinnerIntensityAt(&processStart))
	}
	if strings.TrimSpace(text) == "" {
		return dot
	}
	return dot + " " + PlanText(text)
}
