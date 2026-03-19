package memory

import (
	"testing"
	"time"
)

func TestRolloutSummaryFileStemFromParts(t *testing.T) {
	// Example UUID from Codex-rs or a known one
	threadID := "019c6e27-e55b-73d1-87d8-4e01f1f75043"
	updatedAt, _ := time.Parse(time.RFC3339, "2026-03-18T10:00:00Z")
	slug := "my-test-slug"

	stem1 := RolloutSummaryFileStemFromParts(threadID, updatedAt, &slug)
	stem2 := RolloutSummaryFileStemFromParts(threadID, updatedAt, &slug)

	if stem1 != stem2 {
		t.Errorf("expected deterministic output, got %s and %s", stem1, stem2)
	}

	if len(stem1) == 0 {
		t.Error("expected non-empty stem")
	}

	t.Logf("Generated stem: %s", stem1)
}
