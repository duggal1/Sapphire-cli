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

func TestParseSynthesisResponseUsesTaggedMainAgentOutput(t *testing.T) {
	t.Parallel()

	baseline := SynthesisResult{
		Verdict: VerdictGoWithFixes,
		LegReports: []LegReport{
			{LegType: "requirements", Verdict: "PASS_WITH_NOTES"},
		},
		MustFix: []Finding{
			{Text: "Clarify rollout validation.", Severity: SeverityMustFix, Sources: []string{"requirements", "scope"}},
		},
		ShouldFix: []Finding{
			{Text: "Add smoke test coverage.", Severity: SeverityShouldFix, Sources: []string{"requirements"}},
		},
		Observations: []Finding{
			{Text: "Logging is already in place.", Severity: SeverityObservation, Sources: []string{"feasibility"}},
		},
		Summary: "Baseline summary.",
	}

	raw := `<synthesis>
<overall_verdict>GO_WITH_FIXES</overall_verdict>
<summary>Main agent consolidated the findings.</summary>
<must_fix_count>1</must_fix_count>
<should_fix_count>1</should_fix_count>
<observation_count>1</observation_count>
</synthesis>
<must_fix>
- Clarify rollout validation.
</must_fix>
<should_fix>
- Add smoke test coverage.
</should_fix>
<observations>
- Logging is already in place.
</observations>`

	result, err := ParseSynthesisResponse(raw, baseline)
	if err != nil {
		t.Fatalf("ParseSynthesisResponse() error = %v", err)
	}
	if result.Verdict != VerdictGoWithFixes {
		t.Fatalf("expected verdict %q, got %q", VerdictGoWithFixes, result.Verdict)
	}
	if result.Summary != "Main agent consolidated the findings." {
		t.Fatalf("unexpected summary %q", result.Summary)
	}
	if len(result.MustFix) != 1 || len(result.MustFix[0].Sources) != 2 {
		t.Fatalf("expected baseline sources to be preserved, got %#v", result.MustFix)
	}
}

func TestParseSynthesisResponseRejectsCountMismatch(t *testing.T) {
	t.Parallel()

	_, err := ParseSynthesisResponse(`<synthesis>
<overall_verdict>GO</overall_verdict>
<summary>Ready.</summary>
<must_fix_count>2</must_fix_count>
</synthesis>
<must_fix>
- One item.
</must_fix>`, SynthesisResult{})
	if err == nil {
		t.Fatal("expected count mismatch error")
	}
}
