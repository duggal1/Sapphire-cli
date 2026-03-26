package dialog

import "testing"

func TestBuildPlanApprovalPreviewUsesTitleAndSummary(t *testing.T) {
	t.Parallel()

	title, summary := buildPlanApprovalPreview(`# Trusted Vendor Whitelist Implementation Plan

## Summary
Implement a whitelist mechanism that only recommends skills from trusted vendors.

## Key Changes
- Add config
`)

	if title != "Trusted Vendor Whitelist Implementation Plan" {
		t.Fatalf("unexpected title: %q", title)
	}
	if summary != "Implement a whitelist mechanism that only recommends skills from trusted vendors." {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestBuildPlanApprovalPreviewFallsBackGracefully(t *testing.T) {
	t.Parallel()

	title, summary := buildPlanApprovalPreview("Plain response with no headings at all.")

	if title != "Structured plan ready" {
		t.Fatalf("unexpected fallback title: %q", title)
	}
	if summary != "Plain response with no headings at all." {
		t.Fatalf("unexpected fallback summary: %q", summary)
	}
}
