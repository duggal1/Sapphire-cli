package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestAgenticEditPromptContractIsAligned(t *testing.T) {
	t.Parallel()

	history, _ := (&sessionAgent{}).preparePrompt(nil, "architecture changes across the codebase touching multiple files")
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
	require.Contains(t, joined, `Edit exactly 1 repository file with "single_edit". Edit 2 or more repository files with "agentic_edit". Keep each "agentic_edit" batch to 2–25 files and chunk larger edits into multiple batches.`)
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
	require.Contains(t, templateText, "Edit exactly 1 repository file → use `single_edit`.")
	require.Contains(t, templateText, "Edit 2 or more repository files → use `agentic_edit`.")
	require.Contains(t, templateText, "`agentic_edit` batching rule: edit 2–25 files per call.")

	toolDocPath := filepath.Join(filepath.Dir(currentFile), "tools", "agentic_edit.md")
	toolDocBody, err := os.ReadFile(toolDocPath)
	require.NoError(t, err)
	toolDocText := string(toolDocBody)
	require.Contains(t, toolDocText, "Makes batched edits across one or more files")
	require.Contains(t, toolDocText, "Compatible single-file shorthand")
}
