package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestReasoningChoicesForGemini3(t *testing.T) {
	model := &catwalk.Model{
		ID:        "gemini-3-flash-preview",
		CanReason: true,
	}

	choices := ReasoningChoicesForModel(model)
	// Gemini 3 Flash has minimal, low, medium, high
	if len(choices) != 4 || choices[2] != "medium" {
		t.Fatalf("unexpected choices: %#v", choices)
	}
}

func TestApplyReasoningSelectionForGemini3(t *testing.T) {
	model := &catwalk.Model{
		ID:        "gemini-3-pro",
		CanReason: true,
	}

	selected := ApplyReasoningSelection(model, SelectedModel{ReasoningEffort: "medium", Think: false}, "high")
	if selected.ReasoningEffort != "high" {
		t.Fatalf("expected reasoning effort to be high, got %q", selected.ReasoningEffort)
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
