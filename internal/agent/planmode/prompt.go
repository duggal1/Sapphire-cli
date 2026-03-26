// Codex Plan Mode prompt builder
// Based on Codex CLI plan mode system prompt architecture
//
// Reference: Codex CLI v0.88.0+ plan mode prompts

package planmode

import (
	_ "embed"
	"strings"
)

//go:embed plan.md
var planModePromptMarkdown []byte

// PlanModePrompt holds the plan mode system prompt components
type PlanModePrompt struct {
	// FullPrompt - The complete plan mode system prompt
	FullPrompt string

	// Restrictions - Tool restriction reminder
	Restrictions string

	// Role - The AI's role in plan mode
	Role string
}

// BuildPlanModePrompt builds the plan mode system prompt (Codex-inspired)
func BuildPlanModePrompt() *PlanModePrompt {
	prompt := strings.TrimSpace(string(planModePromptMarkdown))

	return &PlanModePrompt{
		FullPrompt: prompt,
		Restrictions: `**PLAN MODE ACTIVE** - You are FORBIDDEN from:
- File mutation tools (edit, single_edit, agentic_edit, multiedit, write, apply_patch)
- Shell, Python, tests, builds, and execution commands
- Background execution, sub-agent dispatch, and update_plan

This mode is for THINKING AND PLANNING ONLY.`,
		Role: `You are in PLAN MODE. Your role:
1. ORGANIZE - Structure requirements, surface dependencies, identify risks
2. PROPOSE - Suggest architectural decisions and approaches
3. DESIGN - Create detailed, step-by-step plans

CRITICAL: Do NOT execute code or edit files.`,
	}
}

// GetPlanModeInstructions returns a concise plan mode instruction string
func GetPlanModeInstructions() string {
	return `## Plan Mode

You are in PLAN MODE - for thinking and planning only.

FORBIDDEN:
- File mutation (edit, write, multiedit, apply_patch, etc.)
- Shell commands, Python, tests, builds, and execution steps
- Background execution, sub-agent dispatch, and update_plan

ALLOWED:
- Research (view, glob, grep, fetch, etc.)
- Analysis (lsp_diagnostics, search_tools, etc.)

Do not narrate execution. Create a complete plan inside <proposed_plan>...</proposed_plan>.
Ask clarifying questions when requirements are unclear.`
}

// AppendPlanModeNotice appends a plan mode notice to an existing prompt
func AppendPlanModeNotice(basePrompt string) string {
	prompt := BuildPlanModePrompt()
	return basePrompt + "\n\n" + prompt.Restrictions
}

// IsPlanModePrompt returns true if the prompt contains plan mode instructions
func IsPlanModePrompt(prompt string) bool {
	return strings.Contains(prompt, "PLAN MODE") ||
		strings.Contains(prompt, "thinking and planning only") ||
		strings.Contains(prompt, "FORBIDDEN")
}
