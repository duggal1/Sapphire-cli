package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestReasoningChoicesForGemini25(t *testing.T) {
	model := &catwalk.Model{
		ID:        "gemini-2.5-flash",
		CanReason: true,
	}

	choices := ReasoningChoicesForModel(model)
	if len(choices) != 2 || choices[0] != "thinking_on" || choices[1] != "thinking_off" {
		t.Fatalf("unexpected choices: %#v", choices)
	}
}

func TestApplyReasoningSelectionForGemini25(t *testing.T) {
	model := &catwalk.Model{
		ID:        "gemini-2.5-pro",
		CanReason: true,
	}

	selected := ApplyReasoningSelection(model, SelectedModel{ReasoningEffort: "medium", Think: true}, "thinking_off")
	if selected.Think {
		t.Fatalf("expected thinking to be disabled")
	}
	if selected.ReasoningEffort != "" {
		t.Fatalf("expected reasoning effort to be cleared, got %q", selected.ReasoningEffort)
	}
}

func TestNormalizeSelectedModelForGemini3(t *testing.T) {
	model := &catwalk.Model{
		ID:                     "gemini-3-flash",
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
	}

	selected := NormalizeSelectedModelForModel(model, SelectedModel{Think: true})
	if selected.Think {
		t.Fatalf("expected think to be cleared for gemini 3")
	}
	if selected.ReasoningEffort != "medium" {
		t.Fatalf("expected default reasoning effort, got %q", selected.ReasoningEffort)
	}
}

func TestReasoningChoicesForGemini3Flash(t *testing.T) {
	model := &catwalk.Model{
		ID:        "gemini-3-flash",
		CanReason: true,
	}

	choices := ReasoningChoicesForModel(model)
	expected := []string{"minimal", "low", "medium", "high"}
	if len(choices) != len(expected) {
		t.Fatalf("unexpected choices: %#v", choices)
	}
	for i, choice := range expected {
		if choices[i] != choice {
			t.Fatalf("unexpected choices: %#v", choices)
		}
	}
}

func TestReasoningChoicesForGemini3Pro(t *testing.T) {
	model := &catwalk.Model{
		ID:        "gemini-3-pro-preview",
		CanReason: true,
	}

	choices := ReasoningChoicesForModel(model)
	expected := []string{"low", "medium", "high"}
	if len(choices) != len(expected) {
		t.Fatalf("unexpected choices: %#v", choices)
	}
	for i, choice := range expected {
		if choices[i] != choice {
			t.Fatalf("unexpected choices: %#v", choices)
		}
	}
}
