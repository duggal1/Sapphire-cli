package planmode

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

type PhaseProgress struct {
	Label   string
	Status  string
	Percent int
}

type Question struct {
	Header  string
	Prompt  string
	Options []string
}

type PlanView struct {
	CurrentPhase string
	Progress     []PhaseProgress
	PlanDraft    string
	Questions    []Question
}

func (v PlanView) Render() string {
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#C084FC")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA"))
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, 2)

	var builder strings.Builder
	builder.WriteString(accent.Render("Plan Mode"))
	if v.CurrentPhase != "" {
		builder.WriteString("\n")
		builder.WriteString(muted.Render("Phase: " + v.CurrentPhase))
	}
	builder.WriteString("\n\n")
	for _, phase := range v.Progress {
		builder.WriteString(fmt.Sprintf("%s [%s] %d%%\n", phase.Label, phase.Status, phase.Percent))
	}
	if len(v.Questions) > 0 {
		builder.WriteString("\nQuestions\n")
		for _, question := range v.Questions {
			builder.WriteString("- " + question.Header + ": " + question.Prompt + "\n")
			for _, option := range question.Options {
				builder.WriteString("  (" + option + ")\n")
			}
		}
	}
	if strings.TrimSpace(v.PlanDraft) != "" {
		builder.WriteString("\n")
		builder.WriteString(panel.Render(v.PlanDraft))
	}
	return strings.TrimSpace(builder.String())
}
