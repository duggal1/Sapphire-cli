package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
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
	if !strings.Contains(out, "search installed plugins under the Sapphire data dir (`<data-dir>/plugins`)") || !strings.Contains(out, "install it, then use it") {
		t.Fatalf("expected plugin policy in coder prompt")
	}
	if strings.Contains(out, "Available skills: `architect`, `backend`, `debug`, `devops`, `frontend`, `security`.") {
		t.Fatalf("expected hardcoded skill routing to be removed from coder prompt")
	}
	if strings.Contains(out, "<available_skills>") {
		t.Fatalf("expected prompt to stop inlining full available skills inventory")
	}
}

func TestPromptsIncludeTemporalRealityGuardrails(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp("", "temporal-prompt-*")
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

	for _, tc := range []struct {
		name   string
		build  func() (string, error)
		needle []string
	}{
		{
			name: "coder",
			build: func() (string, error) {
				p, err := coderPrompt()
				if err != nil {
					return "", err
				}
				return p.Build(context.Background(), "", "", *cfg)
			},
			needle: []string{
				"Your knowledge cutoff is mid-2025.",
				"Today's date is in the runtime context below.",
				"If asked for today's date, day, or current time, answer from the runtime context, not model memory.",
				"For anything time-sensitive or likely to have changed since the cutoff, verify with tools or web search before answering.",
			},
		},
		{
			name: "task",
			build: func() (string, error) {
				p, err := taskPrompt()
				if err != nil {
					return "", err
				}
				return p.Build(context.Background(), "", "", *cfg)
			},
			needle: []string{
				"Your knowledge cutoff is mid-2025.",
				"Today's date is in the runtime context below.",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.build()
			if err != nil {
				t.Fatalf("build %s prompt: %v", tc.name, err)
			}
			for _, needle := range tc.needle {
				if !strings.Contains(out, needle) {
					t.Fatalf("expected %q in %s prompt", needle, tc.name)
				}
			}
		})
	}
}

func TestSubAgentOrchestratorPromptIsComposedFromModules(t *testing.T) {
	text := string(subAgentOrchestratorPrompt)
	for _, needle := range []string{
		"# Sub-Agent Contract",
		"Follow exactly.",
		"Use `agent_mail_send` for blockers, handoffs, or dependency requests.",
		"Silent waiting is failure.",
		"Do not widen scope without instruction.",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected %q in sub-agent orchestration prompt", needle)
		}
	}
}

func TestCoderPromptIncludesExtendedModeOverlays(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp("", "mode-prompt-*")
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

	cases := []struct {
		mode   planmode.SessionMode
		needle string
	}{
		{mode: planmode.PlanMode, needle: "# Plan Mode"},
		{mode: planmode.ArchitectureMode, needle: "# Architect Mode"},
		{mode: planmode.DebugMode, needle: "# Debug Mode"},
		{mode: planmode.SecurityMode, needle: "# Security Mode"},
		{mode: planmode.ReviewMode, needle: "# Review Mode"},
		{mode: planmode.OrchestratorMode, needle: "# Orchestrator Mode"},
	}

	for _, tc := range cases {
		p, err := coderPromptForMode(tc.mode)
		if err != nil {
			t.Fatalf("coder prompt for %s: %v", tc.mode, err)
		}
		out, err := p.Build(context.Background(), "", "", *cfg)
		if err != nil {
			t.Fatalf("build prompt for %s: %v", tc.mode, err)
		}
		if !strings.Contains(out, tc.needle) {
			t.Fatalf("expected %q in %s prompt", tc.needle, tc.mode)
		}
	}
}

func TestPlanModePromptRequiresFinalProposedPlanWhenReady(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp("", "plan-prompt-*")
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

	p, err := coderPromptForMode(planmode.PlanMode)
	if err != nil {
		t.Fatalf("coder prompt for plan: %v", err)
	}
	out, err := p.Build(context.Background(), "", "", *cfg)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}

	for _, needle := range []string{
		"do **not** stop at validation prose, status narration, or generic explanation",
		"Once you have enough information for a safe plan, produce it immediately.",
		`Execution-oriented narration such as "Initiating Task Execution", "implement the plan", or similar act-first wording while still in Plan Mode`,
		"Do **not** narrate execution, implementation progress, or task performance while in Plan Mode.",
		"If `agent.md` exists in the repository, read it first as a quick map of the codebase.",
		"Read the full relevant code files, not just snippets, when those files materially control the behavior being planned",
		"The final plan must be strongly structured Markdown in a neutral professional tone.",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("expected %q in plan prompt", needle)
		}
	}
	for _, forbidden := range []string{
		"Dry-run commands when they do not modify repo-tracked files",
		"Tests, builds, or checks that may write caches or build artifacts",
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("did not expect %q in plan prompt", forbidden)
		}
	}
}

func TestArchitectModePromptRequiresDeepCodebaseGrounding(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp("", "architect-prompt-*")
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

	p, err := coderPromptForMode(planmode.ArchitectureMode)
	if err != nil {
		t.Fatalf("coder prompt for architect: %v", err)
	}
	out, err := p.Build(context.Background(), "", "", *cfg)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}

	for _, needle := range []string{
		"If `agent.md` exists in the repository, read it first as a quick architectural map.",
		"read the full relevant implementation files",
		"use non-mutating tooling, including shell, Python, tests, and builds",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("expected %q in architect prompt", needle)
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
