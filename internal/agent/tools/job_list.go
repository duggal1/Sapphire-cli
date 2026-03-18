package tools

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/shell"
)

const (
	JobListToolName = "job_list"
)

type JobListParams struct{}

type jobListEntry struct {
	ShellID     string `json:"shell_id"`
	Command     string `json:"command"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
}

func NewJobListTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobListToolName,
		"List all background jobs for the current session. Returns each job's ID, command, description, and completion status.",
		func(ctx context.Context, params JobListParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			ids := listBackgroundShellIDs(sessionID)
			if len(ids) == 0 {
				return fantasy.NewTextResponse("No background jobs are running."), nil
			}

			var lines []string
			for _, id := range ids {
				entry := jobListEntry{ShellID: id}
				if fastShell, ok := shell.GetFastBackgroundShellManager().Get(id); ok {
					_, _, done, _ := fastShell.GetOutput()
					entry.Command = fastShell.Command
					entry.Description = fastShell.Description
					entry.Done = done
				} else if bgShell, ok := shell.GetBackgroundShellManager().Get(id); ok {
					_, _, done, _ := bgShell.GetOutput()
					entry.Command = bgShell.Command
					entry.Description = bgShell.Description
					entry.Done = done
				} else {
					continue
				}
				status := "running"
				if entry.Done {
					status = "completed"
				}
				lines = append(lines, fmt.Sprintf("- [%s] %s (%s) — %s", entry.ShellID, entry.Description, status, entry.Command))
			}

			if len(lines) == 0 {
				return fantasy.NewTextResponse("No background jobs are running."), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Background jobs (%d):\n%s", len(lines), strings.Join(lines, "\n"))), nil
		})
}
