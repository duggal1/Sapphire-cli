package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
)

func TestCoderPromptIncludesOrchestrationOverlay(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir, "", false)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	p, err := coderPrompt()
	if err != nil {
		t.Fatalf("coder prompt: %v", err)
	}

	out, err := p.Build(context.Background(), "", "", *cfg)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}

	if !strings.Contains(out, "Treat persistent orchestration state as the source of truth.") {
		t.Fatalf("expected orchestration memory overlay in coder prompt")
	}
	if !strings.Contains(out, "Live recipients are nudged automatically by the control plane") {
		t.Fatalf("expected mail protocol overlay in coder prompt")
	}
}

func TestSubAgentOrchestratorPromptIsComposedFromModules(t *testing.T) {
	text := string(subAgentOrchestratorPrompt)
	for _, needle := range []string{
		"This is an operating manual. Follow it exactly.",
		"Use durable mail for dependency handoffs, blockers, completion notices, recovery notes, and requests for help.",
		"Healthy workers either make progress, report a blocker, or finish. Silent waiting is a failure.",
		"You own exactly one assignment at a time. Do not widen scope without instruction.",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected %q in sub-agent orchestration prompt", needle)
		}
	}
}
