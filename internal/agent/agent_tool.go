package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"

	"github.com/charmbracelet/sapphire/internal/agent/prompt"
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
	agentCfg, ok := c.cfg.Agents[config.AgentTask]
	if !ok {
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

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			useWorktree := true
			if params.Worktree != nil {
				useWorktree = *params.Worktree
			}

			workDir := c.cfg.WorkingDir()
			cleanup := func() {}
			branch := strings.TrimSpace(params.Branch)
			if useWorktree {
				wtDir, wtBranch, wtCleanup, err := c.prepareSubAgentWorktree(ctx, sessionID, call.ID, subAgentWorktreeSpec{
					WorktreePath: params.WorktreePath,
					Branch:       branch,
					Reuse:        false,
					TaskKey:      decision.TaskKey,
				})
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				workDir = wtDir
				branch = wtBranch
				cleanup = wtCleanup
			}
			normalizedManifest := normalizeWriteManifest(c.cfg.WorkingDir(), workDir, params.WriteManifest)
			writeScope := tools.NewWriteScope(workDir, normalizedManifest)

			if params.Background {
				bgTask := autonomousSubAgentTask{
					Name:         "agent",
					SessionTitle: "Background Agent Session",
					Prompt:       params.Prompt,
				}
				go func() {
					bgCtx := context.Background()
					if err := c.addBackgroundTasks(bgCtx, sessionID, 1); err != nil {
						slog.Error("Failed to publish background sub-agent indicator", "error", err)
					}

					prompt, err := taskPrompt(prompt.WithWorkingDir(workDir))
					if err != nil {
						slog.Error("Failed to build background sub-agent prompt", "error", err)
						c.completeBackgroundTasks(bgCtx, sessionID, 1)
						cleanup()
						return
					}

					agent, err := c.buildAgentWithWorkingDirOverrides(bgCtx, prompt, agentCfg, true, workDir, nil, writeScope)
					if err != nil {
						slog.Error("Failed to build background sub-agent", "error", err)
						c.completeBackgroundTasks(bgCtx, sessionID, 1)
						cleanup()
						return
					}

					c.acquireBackgroundSubAgentSlot()
					defer c.releaseBackgroundSubAgentSlot()
					subCtx, cancel := context.WithTimeout(bgCtx, backgroundSubAgentTimeout)
					defer cancel()
					_, assignmentPrompt := buildSubAgentAssignment(sessionID, bgTask.SessionTitle, params.Prompt, workDir, decision, normalizedManifest, branch, params.DefinitionOfDone)
					resp, runErr := c.runSubAgent(subCtx, subAgentParams{
						Agent:          agent,
						SessionID:      sessionID,
						AgentMessageID: agentMessageID,
						ToolCallID:     call.ID,
						Prompt:         assignmentPrompt,
						SessionTitle:   bgTask.SessionTitle,
					})
					if subCtx.Err() == context.DeadlineExceeded {
						runErr = fmt.Errorf("background sub-agent timed out after %s", backgroundSubAgentTimeout)
						resp = fantasy.ToolResponse{}
					}
					cleanup()
					c.publishBackgroundSubAgentResult(bgCtx, sessionID, bgTask, resp, runErr)
					c.completeBackgroundTasks(bgCtx, sessionID, 1)
				}()
				return fantasy.NewTextResponse("running in background"), nil
			}

			// Build agent lazily when tool is executed to avoid recursive initialization.
			prompt, err := taskPrompt(prompt.WithWorkingDir(workDir))
			if err != nil {
				cleanup()
				return fantasy.ToolResponse{}, err
			}

			agent, err := c.buildAgentWithWorkingDirOverrides(ctx, prompt, agentCfg, true, workDir, nil, writeScope)
			if err != nil {
				cleanup()
				return fantasy.ToolResponse{}, err
			}

			_, assignmentPrompt := buildSubAgentAssignment(sessionID, "New Agent Session", params.Prompt, workDir, decision, normalizedManifest, branch, params.DefinitionOfDone)
			resp, err := c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         assignmentPrompt,
				SessionTitle:   "New Agent Session",
			})
			cleanup()
			return resp, err
		}), nil
}
