package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/shell"
)

const (
	JobOutputToolName = "job_output"
)

//go:embed job_output.md
var jobOutputDescription []byte

type JobOutputParams struct {
	ShellID      string `json:"shell_id,omitempty" description:"The ID of the background shell to retrieve output from (optional if using the most recent background job)"`
	Wait         bool   `json:"wait,omitempty" description:"If true, block until the background shell completes before returning output"`
	StdoutCursor int    `json:"stdout_cursor,omitempty" description:"The stdout cursor returned from a previous reading. Starts at 0."`
	StderrCursor int    `json:"stderr_cursor,omitempty" description:"The stderr cursor returned from a previous reading. Starts at 0."`
}

type JobOutputResponseMetadata struct {
	ShellID          string `json:"shell_id"`
	Command          string `json:"command"`
	Description      string `json:"description"`
	Done             bool   `json:"done"`
	WorkingDirectory string `json:"working_directory"`
	StdoutCursor     int    `json:"stdout_cursor"`
	StderrCursor     int    `json:"stderr_cursor"`
}

func NewJobOutputTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobOutputToolName,
		string(jobOutputDescription),
		func(ctx context.Context, params JobOutputParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ShellID == "" {
				params.ShellID = getLastBackgroundShellID(GetSessionFromContext(ctx))
			}
			if params.ShellID == "" {
				return fantasy.NewTextResponse("No background job is available to read."), nil
			}

			var (
				command         string
				description     string
				workingDir      string
				stdout          string
				stderr          string
				done            bool
				execErr         error
				stdoutMissed    bool
				stderrMissed    bool
				newStdoutCursor int
				newStderrCursor int
				found           bool
			)

			fastManager := shell.GetFastBackgroundShellManager()
			if fastShell, ok := fastManager.Get(params.ShellID); ok {
				found = true
				if params.Wait {
					fastShell.WaitContext(ctx)
				}
				stdout, newStdoutCursor, stdoutMissed, stderr, newStderrCursor, stderrMissed, done, execErr = fastShell.GetOutputSince(params.StdoutCursor, params.StderrCursor)
				command = fastShell.Command
				description = fastShell.Description
				workingDir = fastShell.WorkingDir
			}

			if !found {
				bgManager := shell.GetBackgroundShellManager()
				bgShell, ok := bgManager.Get(params.ShellID)
				if !ok {
					return fantasy.NewTextResponse(fmt.Sprintf("No background job found for ID: %s", params.ShellID)), nil
				}
				if params.Wait {
					bgShell.WaitContext(ctx)
				}
				stdout, newStdoutCursor, stdoutMissed, stderr, newStderrCursor, stderrMissed, done, execErr = bgShell.GetOutputSince(params.StdoutCursor, params.StderrCursor)
				command = bgShell.Command
				description = bgShell.Description
				workingDir = bgShell.WorkingDir
			}

			var outputParts []string
			if stdoutMissed || stderrMissed {
				outputParts = append(outputParts, "[Warning: Some output was truncated due to buffer limits. Reading from the oldest available line.]")
			}
			if stdout != "" {
				outputParts = append(outputParts, stdout)
			}
			if stderr != "" {
				outputParts = append(outputParts, stderr)
			}

			status := "running"
			if done {
				status = "completed"
				removeBackgroundShellID(GetSessionFromContext(ctx), params.ShellID)
				if execErr != nil {
					exitCode := shell.ExitCode(execErr)
					if exitCode != 0 {
						outputParts = append(outputParts, fmt.Sprintf("Exit code %d", exitCode))
					}
				}
			}

			output := strings.Join(outputParts, "\n")

			metadata := JobOutputResponseMetadata{
				ShellID:          params.ShellID,
				Command:          command,
				Description:      description,
				Done:             done,
				WorkingDirectory: workingDir,
				StdoutCursor:     newStdoutCursor,
				StderrCursor:     newStderrCursor,
			}

			if output == "" {
				output = BashNoOutput
			}

			result := fmt.Sprintf("Status: %s\n\n%s", status, output)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		})
}
