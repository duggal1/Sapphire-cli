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
	ShellID      string `json:"shell_id" description:"The ID of the background shell to retrieve output from"`
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
				return fantasy.NewTextErrorResponse("missing shell_id"), nil
			}

			bgManager := shell.GetBackgroundShellManager()
			bgShell, ok := bgManager.Get(params.ShellID)
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("background shell not found: %s", params.ShellID)), nil
			}

			if params.Wait {
				bgShell.WaitContext(ctx)
			}

			stdout, newStdoutCursor, stdoutMissed, stderr, newStderrCursor, stderrMissed, done, err := bgShell.GetOutputSince(params.StdoutCursor, params.StderrCursor)

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
				if err != nil {
					exitCode := shell.ExitCode(err)
					if exitCode != 0 {
						outputParts = append(outputParts, fmt.Sprintf("Exit code %d", exitCode))
					}
				}
			}

			output := strings.Join(outputParts, "\n")

			metadata := JobOutputResponseMetadata{
				ShellID:          params.ShellID,
				Command:          bgShell.Command,
				Description:      bgShell.Description,
				Done:             done,
				WorkingDirectory: bgShell.WorkingDir,
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
