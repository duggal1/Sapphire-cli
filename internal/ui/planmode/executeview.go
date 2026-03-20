package planmode

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

type ExecuteStep struct {
	Label  string
	Status string
}

type ExecuteView struct {
	Steps       []ExecuteStep
	CurrentFile string
	Output      string
}

func (v ExecuteView) Render() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("#E879F9")).Bold(true)
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7E22CE")).
		Padding(1, 2)

	var builder strings.Builder
	builder.WriteString(title.Render("Execution"))
	builder.WriteString("\n\n")
	for _, step := range v.Steps {
		builder.WriteString(fmt.Sprintf("%s [%s]\n", step.Label, step.Status))
	}
	if strings.TrimSpace(v.CurrentFile) != "" {
		builder.WriteString("\nCurrent file: " + v.CurrentFile + "\n")
	}
	if strings.TrimSpace(v.Output) != "" {
		builder.WriteString("\n")
		builder.WriteString(panel.Render(v.Output))
	}
	return strings.TrimSpace(builder.String())
}
