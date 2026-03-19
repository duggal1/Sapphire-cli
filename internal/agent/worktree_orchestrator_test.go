package agent

import "testing"

func TestNormalizeWorktreeTaskSpecDefaultsIsolationAndDropsInvalidBranch(t *testing.T) {
	task := normalizeWorktreeTaskSpec(WorktreeTaskSpec{
		Name:          "Backend metrics",
		Prompt:        "Do the work",
		Isolation:     "",
		Branch:        "feat/backend-health-metrics",
		WriteManifest: []string{"internal/agent"},
	})

	if task.Isolation != "worktree" {
		t.Fatalf("expected default isolation=worktree, got %q", task.Isolation)
	}
	if task.Branch != "" {
		t.Fatalf("expected invalid branch hint to be dropped, got %q", task.Branch)
	}
}

func TestNormalizeWorktreeTaskSpecKeepsValidAgentBranch(t *testing.T) {
	task := normalizeWorktreeTaskSpec(WorktreeTaskSpec{
		Name:          "Backend metrics",
		Prompt:        "Do the work",
		Isolation:     "worktree",
		Branch:        "agent/backend/health-metrics",
		WriteManifest: []string{"internal/agent"},
	})

	if task.Branch != "agent/backend/health-metrics" {
		t.Fatalf("expected valid agent branch to be preserved, got %q", task.Branch)
	}
}
