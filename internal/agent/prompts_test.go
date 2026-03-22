package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
)

func TestCoderPromptIncludesOrchestrationOverlay(t *testing.T) {
	dir, err := os.MkdirTemp("", "coder-prompt-*")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()
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
	if !strings.Contains(out, "# SOUL.md") {
		t.Fatalf("expected SOUL prompt section in coder prompt")
	}
	if !strings.Contains(out, "If there is even slight uncertainty about which skill applies, call `search_skills` first") {
		t.Fatalf("expected strict skill policy in coder prompt")
	}
	if !strings.Contains(out, "`install_mcp`") || !strings.Contains(out, "Verify before claim.") {
		t.Fatalf("expected MCP policy module in coder prompt")
	}
	if !strings.Contains(out, "`search_skills`") {
		t.Fatalf("expected search_skills guidance in coder prompt")
	}
	if strings.Contains(out, "Available skills: `architect`, `backend`, `debug`, `devops`, `frontend`, `security`.") {
		t.Fatalf("expected hardcoded skill routing to be removed from coder prompt")
	}
	if strings.Contains(out, "<available_skills>") {
		t.Fatalf("expected prompt to stop inlining full available skills inventory")
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

func TestPromptTemplatesDoNotMispositionWorktreeHelper(t *testing.T) {
	checks := []struct {
		name    string
		text    string
		forbid  []string
		require []string
	}{
		{
			name:   "initialize",
			text:   string(initializePromptTmpl),
			forbid: []string{"Worktree orchestration: `orchestrate_worktrees` (parallel worktrees, optional test runners, optional integration agent)."},
			require: []string{
				"`orchestrate_worktrees` is a batch helper for pre-scoped parallel worktrees.",
			},
		},
		{
			name:   "agentic_fetch",
			text:   string(agenticFetchPromptTmpl),
			forbid: []string{"orchestrate_worktrees"},
		},
	}

	for _, check := range checks {
		for _, forbidden := range check.forbid {
			if strings.Contains(check.text, forbidden) {
				t.Fatalf("%s prompt still contains forbidden text %q", check.name, forbidden)
			}
		}
		for _, required := range check.require {
			if !strings.Contains(check.text, required) {
				t.Fatalf("%s prompt missing required text %q", check.name, required)
			}
		}
	}
}
