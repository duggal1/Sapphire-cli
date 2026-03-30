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
	require.Contains(t, joined, `Read exactly 1 repository file with "single_view". Read 2 or more repository files with "agentic_view". Keep each "agentic_view" batch to 2–30 files and chunk larger reads into multiple batches.`)
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
	require.Contains(t, templateText, "Read exactly 1 repository file → use `single_view`.")
	require.Contains(t, templateText, "Read 2 or more repository files → use `agentic_view`.")
	require.Contains(t, templateText, "`agentic_edit`: preferred edit path for any multi-line or multi-file change.")
	require.Contains(t, templateText, "`single_edit`: trivial one-line tweak in one file.")
	require.Contains(t, templateText, "`apply_patch`: exact unified-diff patching when patch format is required.")
	require.Contains(t, templateText, "If the file has any error or warning, keep fixing that file until current-file errors and warnings are both zero.")
	require.Contains(t, templateText, "Add comments only when they explain genuinely non-obvious logic in a complex")

	toolDocPath := filepath.Join(filepath.Dir(currentFile), "tools", "agentic_edit.md")
	toolDocBody, err := os.ReadFile(toolDocPath)
	require.NoError(t, err)
	toolDocText := string(toolDocBody)
	require.Contains(t, toolDocText, "Makes batched edits across one or more files")
	require.Contains(t, toolDocText, "Compatible single-file shorthand")
}
