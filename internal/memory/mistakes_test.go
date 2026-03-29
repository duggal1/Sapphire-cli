package memory

import (
	"os"
	"path/filepath"
	"strings"
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

func TestAppendMistakeEnsuresProtocolFile(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()

	_, appended, err := AppendMistake(repoRoot, MistakeLogInput{
		Fingerprint:    "failure:auto-protocol",
		Date:           time.Date(2026, 3, 29, 14, 0, 0, 0, time.UTC),
		Task:           "Bootstrap mistake logging",
		TaskDomain:     "memory",
		Agent:          "agent-1",
		Model:          "gemini-test",
		Worktree:       "shared",
		WhatHappened:   "The protocol file was missing in a fresh repo.",
		RootCauseClass: MistakeRootCauseWrongAssumption,
		RootCause:      "The runtime assumed the protocol file already existed.",
		Severity:       MistakeSeverityHigh,
		SolutionSteps:  []string{"Materialize .sapphire/mistake.md before logging."},
		PreventionRule: "Always create .sapphire/mistake.md before attempting autonomous mistake logging.",
		Resolved:       true,
	})
	require.NoError(t, err)
	require.True(t, appended)
	require.FileExists(t, MistakeProtocolPath(repoRoot))
}

func TestAppendMistakeKeepsSingleAppendixBlock(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	for i := 0; i < 2; i++ {
		_, appended, err := AppendMistake(repoRoot, MistakeLogInput{
			Fingerprint:    "failure:appendix-" + string(rune('a'+i)),
			Date:           time.Date(2026, 3, 29, 15+i, 0, 0, 0, time.UTC),
			Task:           "Appendix stability",
			TaskDomain:     "memory",
			Agent:          "agent-1",
			Model:          "gemini-test",
			Worktree:       "shared",
			WhatHappened:   "A non-trivial failure was logged.",
			RootCauseClass: MistakeRootCauseWrongAssumption,
			RootCause:      "The system appended a new mistake entry.",
			Severity:       MistakeSeverityMedium,
			SolutionSteps:  []string{"Keep the appendix block only once."},
			PreventionRule: "Keep the appendix block at the end of MISTAKES.md exactly once.",
			Resolved:       true,
		})
		require.NoError(t, err)
		require.True(t, appended)
	}

	raw, err := os.ReadFile(MistakesPath(repoRoot))
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(raw), "## APPENDIX: ROOT CAUSE TAXONOMY"))
	require.Equal(t, 1, strings.Count(string(raw), "## APPENDIX: RESOLUTION PROTOCOL"))
}
