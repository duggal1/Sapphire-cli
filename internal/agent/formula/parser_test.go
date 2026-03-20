package formula

import "testing"

func TestParseWorkflowFormula(t *testing.T) {
	t.Parallel()

	formulaText := []byte(`
formula = "plan-mode"
type = "workflow"
version = 1

[[steps]]
id = "understand"
title = "Understand"
description = "first"

[[steps]]
id = "design"
title = "Design"
description = "second"
needs = ["understand"]
`)

	parsed, err := Parse(formulaText)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	ordered, err := parsed.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() error = %v", err)
	}
	if len(ordered) != 2 || ordered[0].ID != "understand" || ordered[1].ID != "design" {
		t.Fatalf("unexpected order: %#v", ordered)
	}
}

func TestParseRejectsCycles(t *testing.T) {
	t.Parallel()

	formulaText := []byte(`
formula = "plan-mode"
type = "workflow"
version = 1

[[steps]]
id = "a"
title = "A"
description = "a"
needs = ["b"]

[[steps]]
id = "b"
title = "B"
description = "b"
needs = ["a"]
`)

	if _, err := Parse(formulaText); err == nil {
		t.Fatal("expected cycle validation error")
	}
}
