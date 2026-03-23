package formula

import "testing"

func TestRenderTemplateSupportsMapAndBareVariableSyntax(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		"task":      "Fix plan mode",
		"task_slug": "fix-plan-mode",
	}

	got, err := renderTemplate("{{task}} :: {{ .task_slug }}", vars)
	if err != nil {
		t.Fatalf("renderTemplate returned error: %v", err)
	}
	if got != "Fix plan mode :: fix-plan-mode" {
		t.Fatalf("unexpected render output: %q", got)
	}
}
