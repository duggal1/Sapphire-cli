package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/permission"
	"github.com/charmbracelet/sapphire/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestBashToolBackgroundJobLifecycle(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	permissions := permission.NewPermissionService(workingDir, true, []string{})
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-1")

	bashTool := NewBashTool(permissions, workingDir, &config.Attribution{}, "gemini-3-flash")
	jobOutputTool := NewJobOutputTool()
	jobKillTool := NewJobKillTool()

	startInput, err := json.Marshal(BashParams{
		Description:     "background echo",
		Command:         "printf 'alpha'; sleep 1; printf 'beta'",
		RunInBackground: true,
	})
	require.NoError(t, err)

	startResp, err := bashTool.Run(ctx, fantasy.ToolCall{
		ID:    "bash-1",
		Name:  BashToolName,
		Input: string(startInput),
	})
	require.NoError(t, err)
	require.False(t, startResp.IsError)

	var bashMeta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(startResp.Metadata), &bashMeta))
	require.True(t, bashMeta.Background)
	require.NotEmpty(t, bashMeta.ShellID)

	outputInput, err := json.Marshal(JobOutputParams{Wait: true})
	require.NoError(t, err)

	outputResp, err := jobOutputTool.Run(ctx, fantasy.ToolCall{
		ID:    "job-output-1",
		Name:  JobOutputToolName,
		Input: string(outputInput),
	})
	require.NoError(t, err)
	require.Contains(t, outputResp.Content, "Status: completed")
	require.Contains(t, outputResp.Content, "alphabeta")

	killInput, err := json.Marshal(JobKillParams{})
	require.NoError(t, err)

	killResp, err := jobKillTool.Run(ctx, fantasy.ToolCall{
		ID:    "job-kill-1",
		Name:  JobKillToolName,
		Input: string(killInput),
	})
	require.NoError(t, err)
	require.Contains(t, killResp.Content, bashMeta.ShellID)
}

func TestJobKillUsesMostRecentBackgroundShellWithoutExplicitID(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	permissions := permission.NewPermissionService(workingDir, true, []string{})
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-2")

	bashTool := NewBashTool(permissions, workingDir, &config.Attribution{}, "gemini-3-flash")
	jobKillTool := NewJobKillTool()

	startInput, err := json.Marshal(BashParams{
		Description:     "long running",
		Command:         "sleep 30",
		RunInBackground: true,
	})
	require.NoError(t, err)

	startResp, err := bashTool.Run(ctx, fantasy.ToolCall{
		ID:    "bash-2",
		Name:  BashToolName,
		Input: string(startInput),
	})
	require.NoError(t, err)

	var bashMeta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(startResp.Metadata), &bashMeta))
	require.NotEmpty(t, bashMeta.ShellID)

	t.Cleanup(func() {
		_ = shell.GetFastBackgroundShellManager().Kill(bashMeta.ShellID)
		_ = shell.GetBackgroundShellManager().Kill(bashMeta.ShellID)
	})

	time.Sleep(100 * time.Millisecond)

	killInput, err := json.Marshal(JobKillParams{})
	require.NoError(t, err)

	killResp, err := jobKillTool.Run(ctx, fantasy.ToolCall{
		ID:    "job-kill-2",
		Name:  JobKillToolName,
		Input: string(killInput),
	})
	require.NoError(t, err)
	require.Contains(t, killResp.Content, bashMeta.ShellID)
}

func TestBashToolAcceptsCommandAliasesAndRejectsEmptyCommand(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	permissions := permission.NewPermissionService(workingDir, true, []string{})
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-3")

	bashTool := NewBashTool(permissions, workingDir, &config.Attribution{}, "gemini-3-flash")

	aliasResp, err := bashTool.Run(ctx, fantasy.ToolCall{
		ID:    "bash-alias",
		Name:  BashToolName,
		Input: `{"cmd":"printf hello"}`,
	})
	require.NoError(t, err)
	require.False(t, aliasResp.IsError)
	require.Contains(t, aliasResp.Content, "hello")

	emptyResp, err := bashTool.Run(ctx, fantasy.ToolCall{
		ID:    "bash-empty",
		Name:  BashToolName,
		Input: `{"command":"   "}`,
	})
	require.NoError(t, err)
	require.True(t, emptyResp.IsError)
	require.Contains(t, emptyResp.Content, "command is required")
}
