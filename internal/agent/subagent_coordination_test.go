package agent

import "testing"

func TestClassifySubAgentTurnBlocked(t *testing.T) {
	t.Parallel()

	outcome := classifySubAgentTurn(subAgentReport{
		Status:   "blocked",
		Summary:  "Need dependency state",
		Blockers: "Waiting on dependency",
	})
	if outcome.Status != subAgentStatusStuck {
		t.Fatalf("expected blocked report to map to stuck status, got %q", outcome.Status)
	}
	if outcome.ReportStatus != "blocked" {
		t.Fatalf("expected blocked report status, got %q", outcome.ReportStatus)
	}
	if outcome.ErrMsg == "" {
		t.Fatal("expected blocked report to produce an error message")
	}
}

func TestClassifySubAgentTurnNeedsFollowup(t *testing.T) {
	t.Parallel()

	outcome := classifySubAgentTurn(subAgentReport{Status: "needs_followup"})
	if outcome.Status != subAgentStatusCompleted {
		t.Fatalf("expected needs_followup to keep completed status, got %q", outcome.Status)
	}
	if outcome.ReportStatus != "needs_followup" {
		t.Fatalf("expected needs_followup report status, got %q", outcome.ReportStatus)
	}
}
