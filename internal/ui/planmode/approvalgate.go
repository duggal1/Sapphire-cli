package planmode

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type ApprovalGate struct {
	PlanSummary string
}

func (g ApprovalGate) Render() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("#F0ABFC")).Bold(true)
	button := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#9333EA")).
		Padding(0, 1)

	var builder strings.Builder
	builder.WriteString(title.Render("Plan is ready. Ready to implement?"))
	if strings.TrimSpace(g.PlanSummary) != "" {
		builder.WriteString("\n\n")
		builder.WriteString(g.PlanSummary)
	}
	builder.WriteString("\n\n")
	builder.WriteString(button.Render("Implement This Plan"))
	builder.WriteString("  ")
	builder.WriteString(button.Render("Suggest Changes"))
	return builder.String()
}
