package shimmer

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/colorprofile"
)

const (
    sweepDuration  = 750 * time.Millisecond  // was 1300 — faster sweep
    bandHalfWidth  = 14.0                     // was 5.0 — wider shimmer band
    shimmerPadding = 10
)

type renderMode uint8

const (
	renderPlain renderMode = iota
	renderDecorated
	renderANSI256
	renderTrueColor
)

var (
	processStart     time.Time
	processStartOnce sync.Once

	detectedRenderMode     renderMode
	detectedRenderModeOnce sync.Once
)

func elapsedSinceStart() time.Duration {
	processStartOnce.Do(func() { processStart = time.Now() })
	return time.Since(processStart)
}

func currentRenderMode() renderMode {
	detectedRenderModeOnce.Do(func() {
		env := os.Environ()
		detectedRenderMode = detectRenderMode(
			env,
			colorprofile.Env(env),
			colorprofile.Detect(os.Stdout, env),
		)
	})
	return detectedRenderMode
}

func detectRenderMode(env []string, envProfile, detectedProfile colorprofile.Profile) renderMode {
	vars := envMap(env)
	if noColorRequested(vars) {
		return renderDecorated
	}

	if forceLevel, ok := forceColorLevel(vars["FORCE_COLOR"]); ok {
		switch forceLevel {
		case 3:
			return renderTrueColor
		case 2:
			return renderANSI256
		case 1:
			return renderDecorated
		default:
			return renderPlain
		}
	}

	if hasTrueColorHint(vars) {
		return renderTrueColor
	}

	switch maxProfile(envProfile, detectedProfile) {
	case colorprofile.TrueColor:
		return renderTrueColor
	case colorprofile.ANSI256:
		return renderANSI256
	case colorprofile.ANSI:
		return renderDecorated
	default:
		if looksLikeInteractiveTerminal(vars) {
			return renderDecorated
		}
		return renderPlain
	}
}

func noColorRequested(env map[string]string) bool {
	term := strings.ToLower(env["TERM"])
	return env["NO_COLOR"] != "" || term == "dumb"
}

func forceColorLevel(value string) (int, bool) {
	if value == "" {
		return 0, false
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return 1, true
	case "false":
		return 0, true
	}

	level, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	if level < 0 {
		level = 0
	}
	if level > 3 {
		level = 3
	}
	return level, true
}

func hasTrueColorHint(env map[string]string) bool {
	if strings.Contains(strings.ToLower(env["COLORTERM"]), "truecolor") ||
		strings.Contains(strings.ToLower(env["COLORTERM"]), "24bit") {
		return true
	}
	if env["WT_SESSION"] != "" {
		return true
	}

	termProgram := strings.ToLower(env["TERM_PROGRAM"])
	switch termProgram {
	case "ghostty", "wezterm", "iterm.app", "vscode", "hyper", "rio", "tabby", "kitty", "warpterminal":
		return true
	}

	term := strings.ToLower(env["TERM"])
	for _, marker := range []string{
		"direct",
		"truecolor",
		"24bit",
		"ghostty",
		"wezterm",
		"kitty",
		"alacritty",
	} {
		if strings.Contains(term, marker) {
			return true
		}
	}

	return false
}

func looksLikeInteractiveTerminal(env map[string]string) bool {
	term := strings.ToLower(env["TERM"])
	return term != "" && term != "dumb"
}

func envMap(entries []string) map[string]string {
	vars := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			vars[key] = value
		}
	}
	return vars
}

func maxProfile(profiles ...colorprofile.Profile) colorprofile.Profile {
	best := colorprofile.Unknown
	for _, profile := range profiles {
		if profile > best {
			best = profile
		}
	}
	return best
}

func blend(a, b [3]uint8, t float32) (uint8, uint8, uint8) {
	clamp := func(v float32) uint8 {
		switch {
		case v < 0:
			return 0
		case v > 255:
			return 255
		default:
			return uint8(v)
		}
	}

	return clamp(float32(a[0])*t + float32(b[0])*(1-t)),
		clamp(float32(a[1])*t + float32(b[1])*(1-t)),
		clamp(float32(a[2])*t + float32(b[2])*(1-t))
}

func renderRGB(r, g, b uint8, ch string) string {
	return fmt.Sprintf("\033[1m\033[38;2;%d;%d;%dm%s\033[0m", r, g, b, ch)
}

func renderIndexedColor(index int, bold bool, ch string) string {
	if bold {
		return fmt.Sprintf("\033[1m\033[38;5;%dm%s\033[0m", index, ch)
	}
	return fmt.Sprintf("\033[38;5;%dm%s\033[0m", index, ch)
}

func renderDim(ch string) string    { return "\033[2m" + ch + "\033[0m" }
func renderBold(ch string) string   { return "\033[1m" + ch + "\033[0m" }
func renderNormal(ch string) string { return ch }

func styleForIntensity(intensity float32) func(string) string {
	switch {
	case intensity < 0.2:
		return renderDim
	case intensity < 0.6:
		return renderNormal
	default:
		return renderBold
	}
}

func shimmerPosition(textLen int, elapsed time.Duration) int {
	period := textLen + shimmerPadding*2
	if period <= 0 {
		return 0
	}

	progress := math.Mod(elapsed.Seconds(), sweepDuration.Seconds()) / sweepDuration.Seconds()
	return int(progress * float64(period))
}

func intensityAt(index, position int) float32 {
	dist := float32(math.Abs(float64(index + shimmerPadding - position)))
	if dist > bandHalfWidth {
		return 0
	}

	x := math.Pi * float64(dist/bandHalfWidth)
	return float32(0.5 * (1 + math.Cos(x)))
}

func renderRune(mode renderMode, intensity float32, ch string) string {
	switch mode {
	case renderTrueColor:
		baseColor := [3]uint8{130, 130, 130}
		highlightColor := [3]uint8{220, 220, 220}
		r, g, b := blend(highlightColor, baseColor, clamp01(intensity)*0.9)
		return renderRGB(r, g, b, ch)
	case renderANSI256:
		index := 240 + int(math.Round(float64(intensity)*12))
		if index < 240 {
			index = 240
		}
		if index > 255 {
			index = 255
		}
		return renderIndexedColor(index, intensity >= 0.7, ch)
	case renderDecorated:
		return styleForIntensity(intensity)(ch)
	default:
		return ch
	}
}

func clamp01(value float32) float32 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func shimmerSpansAt(text string, elapsed time.Duration, mode renderMode) []string {
	chars := []rune(text)
	if len(chars) == 0 {
		return nil
	}

	position := shimmerPosition(len(chars), elapsed)
	spans := make([]string, 0, len(chars))
	for i, ch := range chars {
		spans = append(spans, renderRune(mode, intensityAt(i, position), string(ch)))
	}
	return spans
}

func ShimmerSpans(text string) []string {
	return shimmerSpansAt(text, elapsedSinceStart(), currentRenderMode())
}

func ShimmerText(text string) string {
	return strings.Join(ShimmerSpans(text), "")
}

func Spinner(startTime *time.Time, animationsEnabled bool) string {
	if !animationsEnabled {
		if currentRenderMode() == renderPlain {
			return "•"
		}
		return renderDim("•")
	}

	var elapsed time.Duration
	if startTime != nil {
		elapsed = time.Since(*startTime)
	} else {
		elapsed = elapsedSinceStart()
	}

	mode := currentRenderMode()
	if mode != renderPlain {
		spans := shimmerSpansAt("•", elapsed, mode)
		if len(spans) > 0 {
			return spans[0]
		}
	}

	if (elapsed.Milliseconds()/600)%2 == 0 {
		return "•"
	}
	return "◦"
}

func ShimmerWithDot(text string) string {
	processStartOnce.Do(func() { processStart = time.Now() })
	dot := Spinner(&processStart, true)
	if strings.TrimSpace(text) == "" {
		return dot
	}
	return dot + " " + ShimmerText(text)
}
