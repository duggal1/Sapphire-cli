package memory

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAppendMistakeAndReadPreventionRules(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	require.NoError(t, EnsureMistakeProtocol(repoRoot))
	require.FileExists(t, MistakeProtocolPath(repoRoot))

	_, appended, err := AppendMistake(repoRoot, MistakeLogInput{
		Fingerprint:    "failure:test-1",
		Date:           time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC),
		Task:           "Auth refactor",
		TaskDomain:     "auth",
		Agent:          "agent-1",
		Model:          "gpt-test",
		Worktree:       "shared",
		WhatHappened:   "Broke downstream auth checks",
		RootCauseClass: MistakeRootCauseContextGap,
		RootCause:      "Missed downstream consumers outside the boot slice.",
		DeepAnalysis:   "The dependency graph slice omitted the validator file.",
		WhyThisClass:   "The missing file existed locally but was absent from the initial context.",
		Severity:       MistakeSeverityCritical,
		SolutionSteps:  []string{"Read all downstream auth consumers before editing.", "Expand required reads for auth changes."},
		PreventionRule: "Never edit auth-adjacent code without reading all downstream consumers first.",
		StatusNote:     "Prevention rule persisted to durable memory.",
		Resolved:       true,
	})
	require.NoError(t, err)
	require.True(t, appended)

	_, appended, err = AppendMistake(repoRoot, MistakeLogInput{
		Fingerprint:    "failure:test-2",
		Date:           time.Date(2026, 3, 29, 13, 0, 0, 0, time.UTC),
		Task:           "Imaginary failure",
		TaskDomain:     "general",
		Agent:          "agent-2",
		Model:          "gpt-test",
		Worktree:       "shared",
		WhatHappened:   "Invented a file that did not exist.",
		RootCauseClass: MistakeRootCauseHallucination,
		RootCause:      "Model invented a fact.",
		Severity:       MistakeSeverityHigh,
		IsIgnorable:    true,
		StatusNote:     "Logged for reference only. No structural prevention rule was persisted.",
		Resolved:       false,
	})
	require.NoError(t, err)
	require.True(t, appended)

	register, err := LoadMistakeRegister(repoRoot)
	require.NoError(t, err)
	require.Len(t, register.Entries, 2)
	require.Equal(t, "failure:test-1", register.Entries[0].Fingerprint)
	require.Equal(t, MistakeRootCauseContextGap, register.Entries[0].RootCauseClass)
	require.Equal(t, filepath.Join(repoRoot, MistakesFileName), MistakesPath(repoRoot))
	require.True(t, HasLoggedMistakeFingerprint(repoRoot, "failure:test-1"))
	require.False(t, HasLoggedMistakeFingerprint(repoRoot, "activity:missing"))

	rules := PreventionRules(repoRoot, 10)
	require.Equal(t, []string{"RULE-001: Never edit auth-adjacent code without reading all downstream consumers first."}, rules)

	block := RenderPreventionRulesBlock(repoRoot, 10)
	require.Contains(t, block, "### Prevention Rules From MISTAKES.md")
	require.Contains(t, block, "RULE-001: Never edit auth-adjacent code without reading all downstream consumers first.")
}
