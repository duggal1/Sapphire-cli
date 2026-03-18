// Codex-style <plan> block TUI renderer
// Reference: Codex CLI PR #9712 "TUI: prompt to implement plan and switch to Execute"

package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// RenderPlanBlock renders a <plan> block with Codex-style formatting
func RenderPlanBlock(sty *styles.Styles, planContent string, width int) string {
	if strings.TrimSpace(planContent) == "" {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(sty.Base.Foreground(sty.Primary).Bold(true).Render("Plan\n"))

	// Content with padding
	lines := strings.Split(planContent, "\n")
	for _, line := range lines {
		avail := width - 2
		if avail < 0 {
			avail = 0
		}
		truncated := ansi.Truncate(line, avail, "…")
		b.WriteString(sty.Base.Render("  "+truncated) + "\n")
	}

	// Action hint
	b.WriteString(sty.Muted.Render("\n  Plan ready for implementation. Switch to Pair Programming mode to begin.\n"))

	return b.String()
}

// FormatPlanStatus formats plan completion progress
func FormatPlanStatus(sty *styles.Styles, totalSteps, completedSteps int) string {
	if totalSteps == 0 {
		return ""
	}

	percentage := float64(completedSteps) / float64(totalSteps) * 100
	var icon string
	if completedSteps == totalSteps {
		icon = sty.Base.Foreground(sty.GreenLight).Render("✓ ")
	} else {
		icon = sty.Base.Foreground(sty.Primary).Render("◇ ")
	}

	return fmt.Sprintf("%sPlan: %d/%d completed (%.0f%%)", icon, completedSteps, totalSteps, percentage)
}
