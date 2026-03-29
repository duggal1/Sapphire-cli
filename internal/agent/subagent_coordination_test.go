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
		"Assignment Objective:",
		"Assigned Scope:",
		"Primary Task:",
		"Success Criteria:",
		"Validation Command:",
		"Execution Contract:",
		"Deliverable:",
		"Do not duplicate likely sibling work.",
		"Output format (strict):",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("expected %q in assignment prompt", needle)
		}
	}
}
