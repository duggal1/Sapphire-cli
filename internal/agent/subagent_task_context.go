package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const subAgentTaskFileName = "TASK.md"

func writeSubAgentTaskContext(workDir string, assignment subAgentAssignment) error {
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("workdir is required")
	}

	builder := &strings.Builder{}
	builder.WriteString("# Task: " + assignment.Title + "\n\n")
	builder.WriteString(fmt.Sprintf("Assignment ID: %s\n", assignment.ID))
	builder.WriteString(fmt.Sprintf("Parent session: %s\n", assignment.ParentSessionID))
	if assignment.WorkDir != "" {
		builder.WriteString(fmt.Sprintf("Workdir: %s\n", assignment.WorkDir))
	}
	if assignment.Branch != "" {
		builder.WriteString(fmt.Sprintf("Branch: %s\n", assignment.Branch))
	}

	// Orchestrator Protocol Section
	if orchestrator := activeSubAgentOrchestratorPrompt(); orchestrator != "" {
		builder.WriteString("\n## Orchestrator Protocol\n")
		builder.WriteString("This worktree is governed by strict orchestrator rules:\n")
		builder.WriteString("- Autonomy: Use tools and terminal to solve blockers.\n")
		builder.WriteString("- Precision: Edits must be character-perfect.\n")
		builder.WriteString("- Reporting: STATUS, SUMMARY, PROGRESS, FILES, COMMANDS are mandatory.\n")
	}

	builder.WriteString("\n## Assignment\n")
	builder.WriteString(strings.TrimSpace(assignment.Task))
	builder.WriteString("\n")

	if assignment.DefinitionOfDone != "" {
		builder.WriteString("\n## Definition of Done\n")
		builder.WriteString(strings.TrimSpace(assignment.DefinitionOfDone))
		builder.WriteString("\n")
	}
	if assignment.TestCommand != "" {
		builder.WriteString("\n## Test Command\n")
		builder.WriteString(strings.TrimSpace(assignment.TestCommand))
		builder.WriteString("\n")
	}

	builder.WriteString("\n## Validation Gate Expectations\n")
	builder.WriteString("After you report STATUS: done, a validation gate will run:\n")
	builder.WriteString("1. `git diff --stat`: Verifies semantic changes.\n")
	builder.WriteString("2. Build Verification: Project must build successfully.\n")
	builder.WriteString("3. Test Verification: Existing and new tests must pass.\n")
	builder.WriteString("4. Lint Verification: Lint checks must pass if available.\n")
	builder.WriteString("5. Security Verification: Security scan must pass if available.\n")
	builder.WriteString("\n**IMPORTANT**: If you fail validation, this worktree will be quarantined for audit. Do not submit until the project is stable.\n")

	builder.WriteString("\n## Memory Protocol\n")
	builder.WriteString("- Significant decisions should be captured in the SUMMARY.\n")
	builder.WriteString("- Large file changes will trigger automated memory extraction.\n")

	builder.WriteString("\n## Write Manifest\n")
	if len(assignment.WriteManifest) == 0 {
		builder.WriteString("- (read-only)\n")
	} else {
		for _, entry := range assignment.WriteManifest {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			builder.WriteString(fmt.Sprintf("- %s\n", entry))
		}
	}

	taskPath := filepath.Join(workDir, subAgentTaskFileName)
	return os.WriteFile(taskPath, []byte(builder.String()), 0o644)
}
