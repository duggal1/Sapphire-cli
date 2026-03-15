package agent

import (
	"context"
	_ "embed"
	"errors"

	"charm.land/fantasy"

	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/config"
)

//go:embed templates/agent_tool.md
var agentToolDescription []byte

type AgentParams struct {
	Prompt           string   `json:"prompt" description:"The task for the agent to perform"`
	Background       bool     `json:"background,omitempty" description:"Run in the background and return immediately"`
	Worktree         *bool    `json:"worktree,omitempty" description:"Run in an isolated git worktree (default true)"`
	WorktreePath     string   `json:"worktree_path,omitempty" description:"Optional worktree path (defaults to repo-root/worktrees/<task>)"`
	Branch           string   `json:"branch,omitempty" description:"Optional branch name for the worktree"`
	WriteManifest    []string `json:"write_manifest,omitempty" description:"Allowed write paths (relative to repo root). Empty list = read-only."`
	DefinitionOfDone string   `json:"definition_of_done,omitempty" description:"Acceptance criteria for completion"`
}

const (
	AgentToolName = "agent"
)

func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
	if _, ok := c.cfg.Agents[config.AgentTask]; !ok {
		return nil, errors.New("task agent not configured")
	}

	return fantasy.NewParallelAgentTool(
		AgentToolName,
		string(agentToolDescription),
		func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Military-grade safeguard: immediate exit if context cancelled
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}

			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			decision := evaluateSubAgentLaunch(params.Prompt)
			if !decision.Allowed {
				msg := "sub-agent launch rejected"
				if decision.Reason != "" {
					msg = msg + ": " + decision.Reason
				}
				return fantasy.NewTextErrorResponse(msg), nil
			}

			useWorktree := true
			if params.Worktree != nil {
				useWorktree = *params.Worktree
			}
			control := c.subAgentControl()

			if params.Background {
				if err := c.addBackgroundTasks(context.Background(), sessionID, 1); err != nil {
					return fantasy.ToolResponse{}, err
				}
				agentID, _, err := control.spawn(ctx, sessionID, spawnAgentOptions{
					Prompt:           params.Prompt,
					Title:            "Background Agent Session",
					Worktree:         useWorktree,
					WorktreePath:     params.WorktreePath,
					Branch:           params.Branch,
					WriteManifest:    params.WriteManifest,
					DefinitionOfDone: params.DefinitionOfDone,
					AgentID:          config.AgentTask,
				})
				if err != nil {
					c.completeBackgroundTasks(context.Background(), sessionID, 1)
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				go c.monitorBackgroundSubAgent(context.Background(), sessionID, "agent", agentID)
				return fantasy.NewTextResponse("running in background"), nil
			}
			agentID, _, err := control.spawn(ctx, sessionID, spawnAgentOptions{
				Prompt:           params.Prompt,
				Title:            "New Agent Session",
				Worktree:         useWorktree,
				WorktreePath:     params.WorktreePath,
				Branch:           params.Branch,
				WriteManifest:    params.WriteManifest,
				DefinitionOfDone: params.DefinitionOfDone,
				AgentID:          config.AgentTask,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			statuses, timedOut := control.wait(ctx, []string{agentID}, 0)
			if timedOut || len(statuses) == 0 {
				return fantasy.NewTextErrorResponse("sub-agent did not finish cleanly"), nil
			}
			results := control.collectResult([]string{agentID})
			defer func() { _ = control.close(agentID) }()
			if len(results) == 0 {
				return fantasy.NewTextErrorResponse("sub-agent did not report a final result"), nil
			}
			result := results[0]
			if result.Error != "" {
				return fantasy.NewTextErrorResponse(result.Error), nil
			}
			if result.Result != "" {
				return fantasy.NewTextResponse(result.Result), nil
			}
			return fantasy.NewTextResponse(result.Progress), nil
		}), nil
}
