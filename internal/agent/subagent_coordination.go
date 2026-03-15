package agent

import (
	"fmt"
	"strings"
	"time"
)

type subAgentAssignment struct {
	ID              string
	ParentSessionID string
	Title           string
	Task            string
	TaskKey         string
	Domains         []string
	WorkDir         string
	WorktreePath    string
	Branch          string
	WriteManifest   []string
	DefinitionOfDone string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type subAgentReport struct {
	Status   string
	Summary  string
	Progress string
	Files    []string
	Commands []string
	Risks    string
	Next     string
	Blockers string
}

func buildSubAgentAssignment(parentSessionID, title, task, workDir string, decision subAgentLaunchDecision, manifest []string, branch string, definitionOfDone string) (subAgentAssignment, string) {
	now := time.Now()
	assignment := subAgentAssignment{
		ID:              fmt.Sprintf("subagent-%d", now.UnixNano()),
		ParentSessionID: parentSessionID,
		Title:           title,
		Task:            strings.TrimSpace(task),
		TaskKey:         decision.TaskKey,
		Domains:         decision.Domains,
		WorkDir:         workDir,
		WorktreePath:    workDir,
		Branch:          branch,
		WriteManifest:   append([]string{}, manifest...),
		DefinitionOfDone: strings.TrimSpace(definitionOfDone),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	domainLine := "general"
	if len(assignment.Domains) > 0 {
		domainLine = strings.Join(assignment.Domains, ", ")
	}

	builder := &strings.Builder{}
	builder.WriteString("You are a dedicated sub-agent. Execute the assignment below autonomously.\n\n")
	builder.WriteString(fmt.Sprintf("Assignment ID: %s\n", assignment.ID))
	builder.WriteString(fmt.Sprintf("Parent session: %s\n", assignment.ParentSessionID))
	if assignment.Title != "" {
		builder.WriteString(fmt.Sprintf("Title: %s\n", assignment.Title))
	}
	builder.WriteString(fmt.Sprintf("Workdir: %s\n", assignment.WorkDir))
	if assignment.Branch != "" {
		builder.WriteString(fmt.Sprintf("Branch: %s\n", assignment.Branch))
	}
	builder.WriteString(fmt.Sprintf("Domains: %s\n\n", domainLine))
	builder.WriteString("Task:\n")
	builder.WriteString(assignment.Task)
	if assignment.DefinitionOfDone != "" {
		builder.WriteString("\n\nDefinition of done:\n")
		builder.WriteString(assignment.DefinitionOfDone)
	}
	builder.WriteString("\n\nConstraints:\n")
	builder.WriteString("- Stay within the assigned domain and task scope.\n")
	builder.WriteString("- Use tools and terminal commands as needed; run commands inside the workdir.\n")
	if len(assignment.WriteManifest) > 0 {
		builder.WriteString("- Write access is restricted to the manifest below. Read access is unrestricted.\n")
		for _, entry := range assignment.WriteManifest {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			builder.WriteString(fmt.Sprintf("  - %s\n", entry))
		}
	} else {
		builder.WriteString("- Write access is restricted: no writes outside the provided manifest.\n")
	}
	builder.WriteString("- Report absolute file paths for any findings or edits.\n")
	builder.WriteString("- If blocked, say so explicitly and state the missing information.\n\n")
	builder.WriteString("Output format (strict):\n")
	builder.WriteString("STATUS: done | blocked | needs_followup\n")
	builder.WriteString("SUMMARY: <one paragraph>\n")
	builder.WriteString("PROGRESS: <short status update>\n")
	builder.WriteString("FILES: <comma-separated absolute paths or 'none'>\n")
	builder.WriteString("COMMANDS: <comma-separated commands or 'none'>\n")
	builder.WriteString("RISKS: <brief risks or 'none'>\n")
	builder.WriteString("NEXT: <next steps or 'none'>\n")
	builder.WriteString("BLOCKERS: <what is missing, or 'none'>\n")

	return assignment, builder.String()
}

func buildSubAgentFollowupPrompt(assignment subAgentAssignment, followup string, items []string) string {
	builder := &strings.Builder{}
	builder.WriteString("You are continuing a sub-agent assignment.\n\n")
	builder.WriteString(fmt.Sprintf("Assignment ID: %s\n", assignment.ID))
	if assignment.Title != "" {
		builder.WriteString(fmt.Sprintf("Title: %s\n", assignment.Title))
	}
	builder.WriteString(fmt.Sprintf("Workdir: %s\n", assignment.WorkDir))
	if assignment.Branch != "" {
		builder.WriteString(fmt.Sprintf("Branch: %s\n", assignment.Branch))
	}
	builder.WriteString("Original task:\n")
	builder.WriteString(assignment.Task)
	builder.WriteString("\n\nFollow-up request:\n")
	builder.WriteString(strings.TrimSpace(followup))
	if len(items) > 0 {
		builder.WriteString("\n\nStructured input items:\n")
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			builder.WriteString("- ")
			builder.WriteString(item)
			builder.WriteString("\n")
		}
	}
	if assignment.DefinitionOfDone != "" {
		builder.WriteString("\n\nDefinition of done:\n")
		builder.WriteString(assignment.DefinitionOfDone)
	}
	if len(assignment.WriteManifest) > 0 {
		builder.WriteString("\n\nWrite manifest:\n")
		for _, entry := range assignment.WriteManifest {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			builder.WriteString(fmt.Sprintf("- %s\n", entry))
		}
	}
	builder.WriteString("\n\nOutput format (strict):\n")
	builder.WriteString("STATUS: done | blocked | needs_followup\n")
	builder.WriteString("SUMMARY: <one paragraph>\n")
	builder.WriteString("PROGRESS: <short status update>\n")
	builder.WriteString("FILES: <comma-separated absolute paths or 'none'>\n")
	builder.WriteString("COMMANDS: <comma-separated commands or 'none'>\n")
	builder.WriteString("RISKS: <brief risks or 'none'>\n")
	builder.WriteString("NEXT: <next steps or 'none'>\n")
	builder.WriteString("BLOCKERS: <what is missing, or 'none'>\n")

	return builder.String()
}

func parseSubAgentReport(content string) subAgentReport {
	report := subAgentReport{}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "STATUS:") {
			report.Status = strings.TrimSpace(strings.TrimPrefix(trimmed, "STATUS:"))
			continue
		}
		if strings.HasPrefix(trimmed, "SUMMARY:") {
			report.Summary = strings.TrimSpace(strings.TrimPrefix(trimmed, "SUMMARY:"))
			continue
		}
		if strings.HasPrefix(trimmed, "PROGRESS:") {
			report.Progress = strings.TrimSpace(strings.TrimPrefix(trimmed, "PROGRESS:"))
			continue
		}
		if strings.HasPrefix(trimmed, "FILES:") {
			report.Files = splitCommaList(strings.TrimPrefix(trimmed, "FILES:"))
			continue
		}
		if strings.HasPrefix(trimmed, "COMMANDS:") {
			report.Commands = splitCommaList(strings.TrimPrefix(trimmed, "COMMANDS:"))
			continue
		}
		if strings.HasPrefix(trimmed, "RISKS:") {
			report.Risks = strings.TrimSpace(strings.TrimPrefix(trimmed, "RISKS:"))
			continue
		}
		if strings.HasPrefix(trimmed, "NEXT:") {
			report.Next = strings.TrimSpace(strings.TrimPrefix(trimmed, "NEXT:"))
			continue
		}
		if strings.HasPrefix(trimmed, "BLOCKERS:") {
			report.Blockers = strings.TrimSpace(strings.TrimPrefix(trimmed, "BLOCKERS:"))
			continue
		}
	}
	return report
}

func splitCommaList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "none") {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.EqualFold(part, "none") {
			continue
		}
		out = append(out, part)
	}
	return out
}
