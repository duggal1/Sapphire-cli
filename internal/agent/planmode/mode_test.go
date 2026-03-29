package planmode

import "testing"

func TestAvailableModesIncludeSemanticAccents(t *testing.T) {
	t.Parallel()

	want := map[SessionMode]string{
		DefaultSessionMode: "#8B8B98",
		PlanMode:           "#A855F7",
		ArchitectureMode:   "#F59E0B",
		DebugMode:          "#06B6D4",
		SecurityMode:       "#EF4444",
		ReviewMode:         "#3B82F6",
		OrchestratorMode:   "#14B8A6",
	}

	for mode, expected := range want {
		if got := mode.AccentColor(); got != expected {
			t.Fatalf("expected accent %q for %s, got %q", expected, mode, got)
		}
	}
}

func TestAvailableModesDescriptionsAreConcise(t *testing.T) {
	t.Parallel()

	for _, mode := range AvailableModes() {
		if mode.Mode == DefaultSessionMode {
			continue
		}
		if len(mode.Description) > 80 {
			t.Fatalf("expected concise description for %s, got %q", mode.Mode, mode.Description)
		}
	}
}

func TestSelectableModesHideArchitectEntry(t *testing.T) {
	t.Parallel()

	for _, mode := range SelectableModes() {
		if mode.Mode == ArchitectureMode {
			t.Fatal("architecture mode should not appear in user-selectable modes")
		}
	}
}

func TestModeToolPoliciesMatchRuntimeContract(t *testing.T) {
	t.Parallel()

	if IsToolAllowed(PlanMode, "bash") {
		t.Fatal("plan mode must reject bash")
	}
	if IsToolAllowed(PlanMode, "apply_patch") {
		t.Fatal("plan mode must reject apply_patch")
	}
	if !IsToolAllowed(PlanMode, "view") {
		t.Fatal("plan mode must allow read-only discovery tools")
	}
	if IsToolAllowed(PlanMode, "unknown_tool") {
		t.Fatal("plan mode must use a strict allowlist")
	}

	for _, mode := range []SessionMode{ArchitectureMode, SecurityMode, DebugMode, ReviewMode, OrchestratorMode} {
		if !IsToolAllowed(mode, "bash") {
			t.Fatalf("%s should allow bash for inspection and analysis", mode)
		}
		if IsToolAllowed(mode, "apply_patch") {
			t.Fatalf("%s must reject direct file mutation tools", mode)
		}
	}
}
