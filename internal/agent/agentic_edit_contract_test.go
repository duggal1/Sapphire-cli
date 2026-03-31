package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/stretchr/testify/require"
)

func TestAgenticEditPromptContractIsAligned(t *testing.T) {
	t.Parallel()

	history, _ := (&sessionAgent{}).preparePrompt(planmode.DefaultSessionMode, nil, "architecture changes across the codebase touching multiple files")
	require.NotEmpty(t, history)

	var reminders []string
	for _, msg := range history {
		if msg.Role != fantasy.MessageRoleUser {
			continue
		}
		for _, part := range msg.Content {
			text, ok := fantasy.AsMessagePart[fantasy.TextPart](part)
			if ok {
				reminders = append(reminders, text.Text)
			}
		}
	}

	joined := strings.Join(reminders, "\n")
	require.Contains(t, joined, `For any complex task, read the main relevant files deeply enough to understand the real behavior, architecture, and integration points, then create a concrete update_plan checklist before mutating repository files or starting execution-heavy commands.`)
	require.Contains(t, joined, `This checklist flow is normal execution mode, not Plan Mode; once the plan is clear, execute it autonomously without asking permission.`)
	require.Contains(t, joined, `Complex-task checklists should usually contain 6-10 short, verifiable steps and must stay synchronized after every state change.`)
	require.Contains(t, joined, `In very large repos, use "tool_search" as a bounded locator when you need the exact file, symbol, or code region before reading. Start with one focused query, refine at most 1-2 times, stop once you have a small set of strong candidates, then switch to "agentic_view" by default. Use "single_view" only if the resulting read is explicitly narrow and trivial.`)
	require.Contains(t, joined, `These search tools are not interchangeable: unknown location -> "tool_search"; known path shape -> "rg_files"; known exact text or symbol string -> "rg"; line counts -> "wc_l"; size or density -> "wc".`)
	require.Contains(t, joined, `"agentic_view" is the default repository read tool, including one-file reads when you want the default path or scope may expand. Use "single_view" only as an extreme fallback for an explicitly user-narrowed or guaranteed-trivial one-file read. Normal general or semi-complex repo investigation should start with an "agentic_view" sweep of about 12-20 relevant files. Initialization, AGENTS generation, broad codebase mapping, or wide subsystem work should use aggressive "agentic_view" sweeps of about 20-30 relevant files and continue with additional sweeps until the major domains are actually covered. For a narrow but complex task, read all main relevant files tied to the task before editing. If the repo has fewer meaningful files, read all of them.`)
	require.Contains(t, joined, `Use "agentic_edit" for any multi-line or multi-file change. Use "single_edit" only for a trivial one-line tweak in one file. Use "apply_patch" only for an exact unified-diff patch, or when add/delete/move semantics are required.`)
	require.Contains(t, joined, `After every edit, read the full current-file diagnostics and keep repairing that file until current-file errors and warnings are zero. Use exact reported lines and messages; never guess.`)
}

func TestPlanModePromptContractRemovesExecutionChecklistReminder(t *testing.T) {
	t.Parallel()

	history, _ := (&sessionAgent{}).preparePrompt(planmode.PlanMode, nil, "plan the refactor")
	require.NotEmpty(t, history)

	var reminders []string
	for _, msg := range history {
		if msg.Role != fantasy.MessageRoleUser {
			continue
		}
		for _, part := range msg.Content {
			text, ok := fantasy.AsMessagePart[fantasy.TextPart](part)
			if ok {
				reminders = append(reminders, text.Text)
			}
		}
	}

	joined := strings.Join(reminders, "\n")
	require.Contains(t, joined, "<proposed_plan>")
	require.NotContains(t, joined, "use update_plan before execution")
	require.NotContains(t, joined, "Initialize plan with update_plan")
}

func TestAgenticEditDocsMatchRuntimeContract(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	templatePath := filepath.Join(filepath.Dir(currentFile), "templates", "coder.md.tpl")
	templateBody, err := os.ReadFile(templatePath)
	require.NoError(t, err)
	templateText := string(templateBody)
	require.Contains(t, templateText, "<editing_files>")
	require.Contains(t, templateText, "- `agentic_view` as the default read tool for one or more target files.")
	require.Contains(t, templateText, "- `single_view` only for an explicit trivial one-file read.")
	require.Contains(t, templateText, "- `single_edit` for exactly 1 target file.")
	require.Contains(t, templateText, "- `agentic_edit` for 2 or more target files.")
	require.Contains(t, templateText, "- `apply_patch` for surgical multi-hunk changes using the `*** Begin Patch` format.")
	require.NotContains(t, templateText, "Read exactly 1 repository file → use `single_view`.")
	require.NotContains(t, templateText, "Edit exactly 1 repository file → use `single_edit`.")
	require.Contains(t, templateText, "If the file has any error or warning, keep fixing that file until current-file errors and warnings are both zero.")
	require.Contains(t, templateText, "Add comments only when they explain genuinely non-obvious logic in a complex")

	toolDocPath := filepath.Join(filepath.Dir(currentFile), "tools", "agentic_edit.md")
	toolDocBody, err := os.ReadFile(toolDocPath)
	require.NoError(t, err)
	toolDocText := string(toolDocBody)
	require.Contains(t, toolDocText, "Makes batched edits across one or more files")
	require.Contains(t, toolDocText, "Compatible single-file shorthand")
}
