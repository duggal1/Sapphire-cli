package formula

import "testing"

func TestSynthesizeFindingsDeduplicatesAndProducesGoWithFixes(t *testing.T) {
	t.Parallel()

	results := []ExplorationResult{
		{
			AgentID: "a1",
			LegType: "requirements",
			Status:  "completed",
			Result: `## Verdict
PASS WITH NOTES

## Must Fix
- Add explicit rollout validation.

## Should Fix
- Define smoke tests.

## Observations
- Existing logging is decent.`,
		},
		{
			AgentID: "a2",
			LegType: "scope",
			Status:  "completed",
			Result: `## Verdict
PASS WITH NOTES

## Must Fix
- Add explicit rollout validation.

## Should Fix
- Split optional polish into follow-up work.

## Observations
- Scope is mostly controlled.`,
		},
	}

	synthesis, err := SynthesizeFindings(results)
	if err != nil {
		t.Fatalf("SynthesizeFindings() error = %v", err)
	}
	if synthesis.Verdict != VerdictGoWithFixes {
		t.Fatalf("expected verdict %q, got %q", VerdictGoWithFixes, synthesis.Verdict)
	}
	if len(synthesis.MustFix) != 1 {
		t.Fatalf("expected 1 deduplicated must-fix item, got %d", len(synthesis.MustFix))
	}
	if got := synthesis.MustFix[0].Sources; len(got) != 2 {
		t.Fatalf("expected merged sources for must-fix item, got %#v", got)
	}
}

func TestSynthesizeFindingsProducesNoGoOnFailedLeg(t *testing.T) {
	t.Parallel()

	results := []ExplorationResult{
		{
			AgentID: "a1",
			LegType: "feasibility",
			Status:  "failed",
			Error:   "repo scan timed out",
			Result: `## Verdict
FAIL

## Must Fix
- Confirm prerequisite service ownership.`,
		},
	}

	synthesis, err := SynthesizeFindings(results)
	if err != nil {
		t.Fatalf("SynthesizeFindings() error = %v", err)
	}
	if synthesis.Verdict != VerdictNoGo {
		t.Fatalf("expected verdict %q, got %q", VerdictNoGo, synthesis.Verdict)
	}
	if len(synthesis.MustFix) == 0 {
		t.Fatal("expected must-fix findings for failed leg")
	}
}

func TestSynthesizeFindingsParsesTaggedLegReport(t *testing.T) {
	t.Parallel()

	results := []ExplorationResult{
		{
			AgentID: "a1",
			LegType: "ambiguity",
			Status:  "completed",
			Result: `<verdict>PASS_WITH_NOTES</verdict>
<must_fix>
- Clarify the fallback order.
</must_fix>
<should_fix>
- Define empty state behavior.
</should_fix>
<observations>
- Naming is mostly consistent.
</observations>`,
		},
	}

	synthesis, err := SynthesizeFindings(results)
	if err != nil {
		t.Fatalf("SynthesizeFindings() error = %v", err)
	}
	if got := synthesis.LegReports[0].Verdict; got != "PASS_WITH_NOTES" {
		t.Fatalf("expected tagged verdict to parse, got %q", got)
	}
	if len(synthesis.MustFix) != 1 || synthesis.MustFix[0].Text != "Clarify the fallback order." {
		t.Fatalf("unexpected must-fix findings: %#v", synthesis.MustFix)
	}
}
