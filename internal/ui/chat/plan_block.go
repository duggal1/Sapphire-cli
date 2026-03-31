// Codex-style structured mode block TUI renderer
// Reference: Codex CLI PR #9712 "TUI: prompt to implement plan and switch to Execute"

package chat

import (
	"fmt"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

// RenderStructuredBlock renders a tagged mode block with restrained terminal formatting.
func RenderStructuredBlock(sty *styles.Styles, block *planmode.StructuredBlock, width int) string {
	if block == nil || strings.TrimSpace(block.Content) == "" {
		return ""
	}

	var b strings.Builder

	header := sty.Base.Foreground(sty.Primary).Bold(true).Render(block.Title)
	ruleWidth := max(0, width-len(block.Title)-2)
	if ruleWidth > 0 {
		header += " " + sty.Base.Foreground(sty.Border).Render(strings.Repeat("─", ruleWidth))
	}
	b.WriteString(header)

	renderer := common.MarkdownRenderer(sty, max(20, width-4))
	renderedContent, err := renderer.Render(strings.TrimSpace(block.Content))
	if err != nil {
		renderedContent = strings.TrimSpace(block.Content)
	}
	renderedContent = strings.TrimRight(renderedContent, "\n")
	if renderedContent != "" {
		rail := sty.Base.Foreground(sty.Border).Render("│")
		b.WriteString("\n")
		b.WriteString(prefixRenderedBlock(rail, renderedContent))
	}

	if hint := structuredBlockHint(block.Mode); hint != "" {
		rail := sty.Base.Foreground(sty.Border).Render("│")
		b.WriteString("\n")
		b.WriteString(prefixRenderedBlock(rail, sty.Muted.Render(hint)))
	}

	return strings.TrimRight(b.String(), "\n")
}

// RenderPlanBlock retains the old entrypoint for plan-only callers.
func RenderPlanBlock(sty *styles.Styles, planContent string, width int) string {
	block := &planmode.StructuredBlock{
		Mode:    planmode.PlanMode,
		Title:   "Plan",
		Content: planContent,
	}
	return RenderStructuredBlock(sty, block, width)
}

func structuredBlockHint(mode planmode.SessionMode) string {
	switch planmode.NormalizeMode(mode) {
	case planmode.PlanMode:
		return "Plan ready for implementation."
	case planmode.ArchitectureMode:
		return "Architecture spec captured."
	case planmode.DebugMode:
		return "Debug report captured."
	case planmode.ReviewMode:
		return "Review report captured."
	case planmode.SecurityMode:
		return "Security report captured."
	case planmode.OrchestratorMode:
		return "Execution orchestration captured."
	default:
		return ""
	}
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
