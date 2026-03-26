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
