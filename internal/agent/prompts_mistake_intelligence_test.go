package agent

import (
	"context"
	"os"
	"testing"

	promptpkg "github.com/duggal1/Sapphire-cli/internal/agent/prompt"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCoderPromptIncludesMistakeIntelligenceProtocol(t *testing.T) {
	workingDir, err := os.MkdirTemp("", "sapphire-prompt-mistake-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(workingDir) })
	cfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	prompt, err := coderPrompt(promptpkg.WithWorkingDir(workingDir))
	require.NoError(t, err)

	built, err := prompt.Build(context.Background(), "", "", *cfg)
	require.NoError(t, err)
	require.Contains(t, built, ".sapphire/mistake.md")
	require.Contains(t, built, "The agent writes `MISTAKES.md`.")
	require.Contains(t, built, "update that entry with stronger evidence")
	require.Contains(t, built, "`improvement_eval`")
	require.Contains(t, built, "`strategy_pattern`")
	require.Contains(t, built, "Before a high-risk, repeated, or structurally similar task")
}
