package tools

import (
	"context"
	_ "embed"
	"fmt"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/shell"
)

const (
	JobKillToolName = "job_kill"
)

//go:embed job_kill.md
var jobKillDescription []byte

type JobKillParams struct {
	ShellID string `json:"shell_id,omitempty" description:"The ID of the background shell to terminate (optional if using the most recent background job)"`
}

type JobKillResponseMetadata struct {
	ShellID     string `json:"shell_id"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

func NewJobKillTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobKillToolName,
		string(jobKillDescription),
		func(ctx context.Context, params JobKillParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ShellID == "" {
				params.ShellID = getLastBackgroundShellID(GetSessionFromContext(ctx))
			}
			if params.ShellID == "" {
				return fantasy.NewTextResponse("No background job is available to terminate."), nil
			}

			if fastShell, ok := shell.GetFastBackgroundShellManager().Get(params.ShellID); ok {
				metadata := JobKillResponseMetadata{
					ShellID:     params.ShellID,
					Command:     fastShell.Command,
					Description: fastShell.Description,
				}
				if err := shell.GetFastBackgroundShellManager().Kill(params.ShellID); err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				removeBackgroundShellID(GetSessionFromContext(ctx), params.ShellID)
				result := fmt.Sprintf("Background shell %s terminated successfully", params.ShellID)
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
			}

			bgManager := shell.GetBackgroundShellManager()
			bgShell, ok := bgManager.Get(params.ShellID)
			if !ok {
				return fantasy.NewTextResponse(fmt.Sprintf("No background job found for ID: %s", params.ShellID)), nil
			}

			metadata := JobKillResponseMetadata{
				ShellID:     params.ShellID,
				Command:     bgShell.Command,
				Description: bgShell.Description,
			}

			err := bgManager.Kill(params.ShellID)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			removeBackgroundShellID(GetSessionFromContext(ctx), params.ShellID)
			result := fmt.Sprintf("Background shell %s terminated successfully", params.ShellID)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		})
}
