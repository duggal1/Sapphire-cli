package agent

import (
	"fmt"
	"strings"
	"time"
)

type subAgentAssignment struct {
	ID                 string
	ParentSessionID    string
	Title              string
	Task               string
	TaskKey            string
	Domains            []string
	WorkDir            string
	WorktreePath       string
	Branch             string
	WriteManifest      []string
	DefinitionOfDone   string
	TestCommand        string
	LongHorizonContext string
	CreatedAt          time.Time
	UpdatedAt          time.Time
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

func activeSubAgentOrchestratorPrompt() string {
	if len(subAgentOrchestratorPrompt) > 0 {
		return string(subAgentOrchestratorPrompt)
	}
	if len(orchestratorPrompt) > 0 {
		return string(orchestratorPrompt)
	}
	return ""
}

func buildSubAgentAssignment(assignmentID, parentSessionID, title, task, workDir string, decision subAgentLaunchDecision, manifest []string, branch string, definitionOfDone string, testCommand string, longHorizonContext string) (subAgentAssignment, string) {
	now := time.Now()
	if strings.TrimSpace(assignmentID) == "" {
		assignmentID = fmt.Sprintf("subagent-%d", now.UnixNano())
	}
	assignment := subAgentAssignment{
		ID:                 assignmentID,
		ParentSessionID:    parentSessionID,
		Title:              title,
		Task:               strings.TrimSpace(task),
		TaskKey:            decision.TaskKey,
		Domains:            decision.Domains,
		WorkDir:            workDir,
		WorktreePath:       workDir,
		Branch:             branch,
		WriteManifest:      append([]string{}, manifest...),
		DefinitionOfDone:   strings.TrimSpace(definitionOfDone),
		TestCommand:        strings.TrimSpace(testCommand),
		LongHorizonContext: longHorizonContext,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	domainLine := "general"
	if len(assignment.Domains) > 0 {
		domainLine = strings.Join(assignment.Domains, ", ")
	}

	builder := &strings.Builder{}

	// Inject orchestrator prompt as system context for the sub-agent
	if orchestrator := activeSubAgentOrchestratorPrompt(); orchestrator != "" {
		builder.WriteString("<orchestrator_protocol>\n")
		builder.WriteString(orchestrator)
		builder.WriteString("\n</orchestrator_protocol>\n\n")
	}

	builder.WriteString("Role: sub-agent\n")
	builder.WriteString(fmt.Sprintf("Assignment ID: %s\n", assignment.ID))
	builder.WriteString(fmt.Sprintf("Parent session: %s\n", assignment.ParentSessionID))
	if assignment.Title != "" {
		builder.WriteString(fmt.Sprintf("Title: %s\n", assignment.Title))
	}
	builder.WriteString("Mode: execution\n")
	builder.WriteString("\nAssignment Objective:\n")
	builder.WriteString("- Own exactly this assignment. Do not widen scope.\n")
	builder.WriteString("- Complete the assigned task or return a precise blocker.\n")
	builder.WriteString("- If this run is part of a parallel batch, assume sibling agents own different scopes. Do not duplicate likely sibling work.\n")
	builder.WriteString("- Read the real files in scope before reporting, editing, or concluding.\n")
	builder.WriteString("\nAssigned Scope:\n")
	builder.WriteString(fmt.Sprintf("- Workdir: %s\n", assignment.WorkDir))
	if assignment.Branch != "" {
		builder.WriteString(fmt.Sprintf("- Branch: %s\n", assignment.Branch))
	}
	builder.WriteString(fmt.Sprintf("- Domains: %s\n", domainLine))
	builder.WriteString("\nPrimary Task:\n")
	builder.WriteString(assignment.Task)
	if assignment.DefinitionOfDone != "" {
		builder.WriteString("\n\nSuccess Criteria:\n")
		builder.WriteString("- ")
		builder.WriteString(strings.ReplaceAll(assignment.DefinitionOfDone, "\n", "\n- "))
	}
	if assignment.TestCommand != "" {
		builder.WriteString("\n\nValidation Command:\n")
		builder.WriteString("- ")
		builder.WriteString(assignment.TestCommand)
	}
	builder.WriteString("\n\nExecution Contract:\n")
	builder.WriteString("- Keep the plan concise, concrete, and task-oriented.\n")
	builder.WriteString("- Stay within the assigned task and scope.\n")
	builder.WriteString("- Run commands inside the workdir.\n")
	builder.WriteString("- Inspect before claiming.\n")
	builder.WriteString("- Do not return generic analysis. Report evidence from actual inspected files.\n")
	builder.WriteString("- If blocked or dependent on another agent, use durable mail.\n")
	if len(assignment.WriteManifest) > 0 {
		builder.WriteString("- Write access is restricted to the manifest below.\n")
		for _, entry := range assignment.WriteManifest {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			builder.WriteString(fmt.Sprintf("  - %s\n", entry))
		}
	} else {
		builder.WriteString("- No writes outside the provided manifest.\n")
	}
	builder.WriteString("- Report absolute file paths for findings and edits.\n")
	builder.WriteString("- If blocked, report the missing dependency or decision.\n\n")
	builder.WriteString("Deliverable:\n")
	builder.WriteString("- Return the strict report format below.\n")
	builder.WriteString("- Include only concrete progress, evidence, risks, and next step.\n")
	builder.WriteString("- Cite absolute file paths when claiming findings or edits.\n\n")
	builder.WriteString("Validation:\n")
	builder.WriteString("- A validation gate runs after the turn.\n")
	builder.WriteString("- Failed validation preserves the worktree for review.\n")
	builder.WriteString("- Do not report done if required validation is known to be failing.\n\n")
	builder.WriteString("Output format (strict):\n")
	builder.WriteString("STATUS: done | blocked | needs_followup\n")
	builder.WriteString("SUMMARY: <one concise paragraph>\n")
	builder.WriteString("PROGRESS: <concrete progress>\n")
	builder.WriteString("FILES: <comma-separated absolute paths or none>\n")
	builder.WriteString("COMMANDS: <comma-separated commands or none>\n")
	builder.WriteString("RISKS: <concise risks or none>\n")
	builder.WriteString("NEXT: <exact next step or none>\n")
	builder.WriteString("BLOCKERS: <exact blocker or none>\n")

	return assignment, builder.String()
}

func buildSubAgentFollowupPrompt(assignment subAgentAssignment, followup string, items []string) string {
	builder := &strings.Builder{}

	// Inject orchestrator context on followup as well
	if orchestrator := activeSubAgentOrchestratorPrompt(); orchestrator != "" {
		builder.WriteString("<orchestrator_protocol>\n")
		builder.WriteString(orchestrator)
		builder.WriteString("\n</orchestrator_protocol>\n\n")
	}

	builder.WriteString("Role: sub-agent\n")
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
	builder.WriteString("\n\nRules:\n")
	builder.WriteString("- Continue the current assignment only.\n")
	builder.WriteString("- Inspect before claiming.\n")
	builder.WriteString("- Use durable mail for blockers or dependency handoffs.\n")
	builder.WriteString("- Report concrete evidence.\n")
	builder.WriteString("\n\nOutput format (strict):\n")
	builder.WriteString("STATUS: done | blocked | needs_followup\n")
	builder.WriteString("SUMMARY: <one concise paragraph>\n")
	builder.WriteString("PROGRESS: <concrete progress>\n")
	builder.WriteString("FILES: <comma-separated absolute paths or none>\n")
	builder.WriteString("COMMANDS: <comma-separated commands or none>\n")
	builder.WriteString("RISKS: <concise risks or none>\n")
	builder.WriteString("NEXT: <exact next step or none>\n")
	builder.WriteString("BLOCKERS: <exact blocker or none>\n")

	return builder.String()
}

type subAgentTurnDisposition struct {
	Status       subAgentStatus
	ReportStatus string
	Stage        SubAgentLifecycleStage
	EventType    string
	ErrMsg       string
}

func classifySubAgentTurn(report subAgentReport) subAgentTurnDisposition {
	status := strings.ToLower(strings.TrimSpace(report.Status))
	switch status {
	case "blocked":
		return subAgentTurnDisposition{
			Status:       subAgentStatusBlocked,
			ReportStatus: "blocked",
			Stage:        SubAgentStageBlocked,
			EventType:    string(SubAgentBlockedEvent),
			ErrMsg:       firstNonEmptyString(strings.TrimSpace(report.Blockers), strings.TrimSpace(report.Summary), "sub-agent reported blocked"),
		}
	case "needs_followup":
		return subAgentTurnDisposition{
			Status:       subAgentStatusCompleted,
			ReportStatus: "needs_followup",
			Stage:        SubAgentStageCompleted,
			EventType:    string(SubAgentCompletedEvent),
		}
	default:
		return subAgentTurnDisposition{
			Status:       subAgentStatusCompleted,
			ReportStatus: "done",
			Stage:        SubAgentStageCompleted,
			EventType:    string(SubAgentCompletedEvent),
		}
	}
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
