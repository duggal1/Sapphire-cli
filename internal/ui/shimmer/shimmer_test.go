package shimmer

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
)

func TestDetectRenderModePromotesKnownTrueColorTerminal(t *testing.T) {
	t.Parallel()

	mode := detectRenderMode(
		[]string{
			"TERM=xterm-256color",
			"TERM_PROGRAM=ghostty",
		},
		colorprofile.ANSI256,
		colorprofile.ANSI256,
	)

	if mode != renderTrueColor {
		t.Fatalf("expected true color render mode, got %v", mode)
	}
}

func TestDetectRenderModeHonorsNoColor(t *testing.T) {
	t.Parallel()

	mode := detectRenderMode(
		[]string{
			"NO_COLOR=1",
			"COLORTERM=truecolor",
			"TERM=wezterm",
		},
		colorprofile.TrueColor,
		colorprofile.TrueColor,
	)

	if mode != renderDecorated {
		t.Fatalf("expected decorated render mode, got %v", mode)
	}
}

func TestShimmerTickIntervalTargetsEightyFPS(t *testing.T) {
	t.Parallel()

	if shimmerTickInterval != time.Second/80 {
		t.Fatalf("expected 80 FPS animation cadence, got %s", shimmerTickInterval)
	}
}

func TestShimmerSpansAtANSI256MovesBullet(t *testing.T) {
	t.Parallel()

	start := shimmerSpansAt("•", 0, renderANSI256)[0]
	lit := shimmerSpansAt("•", 700*time.Millisecond, renderANSI256)[0]

	if start == lit {
		t.Fatal("expected ANSI256 spinner frames to differ")
	}
	if !strings.Contains(start, "\033[38;5;") || !strings.Contains(lit, "\033[38;5;") {
		t.Fatal("expected ANSI256 frames to include indexed color escape sequences")
	}
}

func TestShimmerSpansAtDecoratedMovesText(t *testing.T) {
	t.Parallel()

	start := strings.Join(shimmerSpansAt("Thinking", 0, renderDecorated), "")
	lit := strings.Join(shimmerSpansAt("Thinking", 700*time.Millisecond, renderDecorated), "")

	if start == lit {
		t.Fatal("expected decorated shimmer frames to differ")
	}
}

func TestThinkingTextUsesDistinctHighContrastPalette(t *testing.T) {
	t.Parallel()

	start := strings.Join(thinkingShimmerSpansAt("Thinking...", 0, renderDecorated), "")
	lit := strings.Join(thinkingShimmerSpansAt("Thinking...", 700*time.Millisecond, renderDecorated), "")

	if start == lit {
		t.Fatal("expected thinking shimmer frames to differ")
	}
	if !strings.Contains(lit, "\033[1m") {
		t.Fatal("expected thinking shimmer to use bright emphasized frames")
	}
	if strings.Contains(lit, "\033[0m") {
		t.Fatal("expected thinking shimmer to preserve parent background instead of hard resetting it")
	}
}
