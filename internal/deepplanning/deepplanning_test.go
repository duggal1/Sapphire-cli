package deepplanning

import "testing"

func TestIsRequested(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  bool
	}{
		{input: "create a plan mode for this refactor", want: true},
		{input: "think extremely long before you touch code", want: true},
		{input: "deep planning", want: true},
		{input: "plan", want: true},
		{input: "fix the parser", want: false},
	}

	for _, tc := range cases {
		if got := IsRequested(tc.input); got != tc.want {
			t.Fatalf("IsRequested(%q) = %t, want %t", tc.input, got, tc.want)
		}
	}
}

func TestPendingAssistantPlaceholderID(t *testing.T) {
	t.Parallel()

	planningID := PendingAssistantPlaceholderID("session-1", true)
	if !IsPlanningAssistantPlaceholderID(planningID) {
		t.Fatalf("expected planning placeholder id %q to be recognized", planningID)
	}

	normalID := PendingAssistantPlaceholderID("session-1", false)
	if IsPlanningAssistantPlaceholderID(normalID) {
		t.Fatalf("did not expect non-planning placeholder id %q to be recognized", normalID)
	}
}
