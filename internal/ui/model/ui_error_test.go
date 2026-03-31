package model

import "testing"

func TestSplitPendingAssistantError(t *testing.T) {
	t.Parallel()

	title, details := splitPendingAssistantError("Too Many Requests\nRate limit exceeded")
	if title != "Too Many Requests" {
		t.Fatalf("expected first line as title, got %q", title)
	}
	if details != "Rate limit exceeded" {
		t.Fatalf("expected remaining lines as details, got %q", details)
	}
}

func TestSplitPendingAssistantErrorFallsBackForBlankInput(t *testing.T) {
	t.Parallel()

	title, details := splitPendingAssistantError(" \n ")
	if title != "Request failed" {
		t.Fatalf("expected fallback title, got %q", title)
	}
	if details != "" {
		t.Fatalf("expected empty details, got %q", details)
	}
}
