package agent

import (
	"strings"
	"testing"
)

func TestBuildSubAgentAssignmentUsesStructuredTaskPacket(t *testing.T) {
	assignment, prompt := buildSubAgentAssignment(
		"work-auth",
		"parent-session",
		"Auth survey",
		"Read the auth-related code paths and summarize the exact responsibility split.",
		"/repo",
		subAgentLaunchDecision{
			TaskKey: "auth-survey",
			Domains: []string{"auth", "api"},
		},
		[]string{"internal/auth", "internal/api"},
		"",
		"Return the exact auth file map and main risks.",
		"go test ./internal/auth",
		"",
	)

	if assignment.ID != "work-auth" {
		t.Fatalf("expected assignment id to be preserved")
	}
	for _, needle := range []string{
		"Mode: execution",
		"Assignment Objective:",
		"Assigned Scope:",
		"Primary Task:",
		"Success Criteria:",
		"Validation Command:",
		"Execution Contract:",
		"Deliverable:",
		"Do not duplicate likely sibling work.",
		"Read the real files in scope before reporting, editing, or concluding.",
		"Do not return generic analysis. Report evidence from actual inspected files.",
		"Cite absolute file paths when claiming findings or edits.",
		"Output format (strict):",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("expected %q in assignment prompt", needle)
		}
	}
}
