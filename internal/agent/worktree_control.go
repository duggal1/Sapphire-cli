package agent

import (
	"context"
	"fmt"
	"strings"

	promptpkg "github.com/duggal1/Sapphire-cli/internal/agent/prompt"
	"github.com/duggal1/Sapphire-cli/internal/config"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/duggal1/Sapphire-cli/internal/worktreepolicy"
)

func (c *coordinator) resolveSessionWorktreePolicy(ctx context.Context, sessionID string) worktreepolicy.Policy {
	if c == nil || c.sessions == nil || strings.TrimSpace(sessionID) == "" {
		return worktreepolicy.Default()
	}
	policy, err := c.sessions.GetWorktreePolicy(ctx, sessionID)
	if err != nil {
		return worktreepolicy.Default()
	}
	return worktreepolicy.Normalize(policy)
}

func (c *coordinator) resolveSpawnWorktreePolicy(ctx context.Context, sessionID string, opts spawnAgentOptions) worktreepolicy.Policy {
	if opts.WorktreeSet {
		if opts.Worktree {
			return worktreepolicy.Isolated
		}
		return worktreepolicy.SharedRepo
	}
	policy := c.resolveSessionWorktreePolicy(ctx, sessionID)
	if policy != worktreepolicy.Auto {
		return policy
	}
	if opts.ReuseWorktree || strings.TrimSpace(opts.WorktreePath) != "" || strings.TrimSpace(opts.Branch) != "" {
		return worktreepolicy.Isolated
	}
	if len(normalizeStringSlice(opts.WriteManifest)) > 0 {
		return worktreepolicy.Isolated
	}
	return worktreepolicy.SharedRepo
}

func (c *coordinator) resolveMainExecutionRoot(ctx context.Context, sessionID string) (string, string, error) {
	root := c.cfg.WorkingDir()
	baseRef := resolveWorktreeBaseRef(ctx, root)
	policy := c.resolveSessionWorktreePolicy(ctx, sessionID)
	if policy != worktreepolicy.Isolated {
		return root, baseRef, nil
	}

	title := sessionID
	if c.sessions != nil && strings.TrimSpace(sessionID) != "" {
		if sess, err := c.sessions.Get(ctx, sessionID); err == nil && strings.TrimSpace(sess.Title) != "" {
			title = sess.Title
		}
	}
	handle, err := c.worktreeManager.PrepareMain(ctx, sessionID, title, policy)
	if err != nil {
		return "", "", err
	}
	return handle.Run.WorktreePath, handle.Run.Branch, nil
}

func (c *coordinator) lookupMainAgent(sessionID string) SessionAgent {
	if c == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	c.mainAgentsMu.RLock()
	defer c.mainAgentsMu.RUnlock()
	return c.mainAgents[sessionID]
}

func (c *coordinator) mainAgentForSession(ctx context.Context, sessionID string) (SessionAgent, error) {
	if strings.TrimSpace(sessionID) == "" {
		if c.currentAgent == nil {
			return nil, fmt.Errorf("main agent not initialized")
		}
		return c.currentAgent, nil
	}
	if agent := c.lookupMainAgent(sessionID); agent != nil {
		return agent, nil
	}

	agentCfg, ok := c.cfg.Agents[config.AgentCoder]
	if !ok {
		return nil, fmt.Errorf("coder agent not configured")
	}
	prompt, err := coderPrompt(promptpkg.WithWorkingDir(c.mainWorkingDir()))
	if err != nil {
		return nil, err
	}
	agent, err := c.buildAgent(ctx, prompt, agentCfg, false)
	if err != nil {
		return nil, err
	}
	if concrete, ok := agent.(*sessionAgent); ok {
		concrete.setSessionID(sessionID)
	}
	c.mainAgentsMu.Lock()
	defer c.mainAgentsMu.Unlock()
	if existing := c.mainAgents[sessionID]; existing != nil {
		return existing, nil
	}
	c.mainAgents[sessionID] = agent
	return agent, nil
}

func (c *coordinator) prepareCurrentAgentForSession(ctx context.Context, sessionID string, agent SessionAgent) (string, string, error) {
	if c == nil || agent == nil {
		return c.cfg.WorkingDir(), "", nil
	}

	workingDir, branch, err := c.resolveMainExecutionRoot(ctx, sessionID)
	if err != nil {
		return "", "", err
	}

	agentCfg, ok := c.cfg.Agents[config.AgentCoder]
	if !ok {
		return "", "", fmt.Errorf("coder agent not configured")
	}
	prompt, err := coderPrompt(promptpkg.WithWorkingDir(workingDir))
	if err != nil {
		return "", "", err
	}
	model := agent.Model()
	systemPrompt, err := prompt.Build(ctx, model.Model.Provider(), model.Model.Model(), *c.cfg)
	if err != nil {
		return "", "", err
	}
	agentTools, err := c.buildToolsForWorkingDir(ctx, agentCfg, workingDir)
	if err != nil {
		return "", "", err
	}

	agent.SetWorkingDir(workingDir)
	agent.SetSystemPrompt(systemPrompt)
	agent.SetTools(agentTools)
	c.setToolCache(agentTools)
	return workingDir, branch, nil
}

func (c *coordinator) ListWorktrees(ctx context.Context, sessionID string, statuses []string, limit int) ([]orchestrationdb.WorktreeRun, error) {
	return c.worktreeManager.List(ctx, sessionID, statuses, limit)
}

func (c *coordinator) LandWorktree(ctx context.Context, idOrPath, strategy string) (orchestrationdb.WorktreeRun, error) {
	return c.worktreeManager.Land(ctx, idOrPath, strategy)
}

func (c *coordinator) RepairWorktree(ctx context.Context, idOrPath string) (orchestrationdb.WorktreeRun, error) {
	return c.worktreeManager.Repair(ctx, idOrPath)
}

func (c *coordinator) RemoveManagedWorktree(ctx context.Context, idOrPath string, force bool) (orchestrationdb.WorktreeRun, error) {
	run, err := c.worktreeManager.lookup(ctx, idOrPath)
	if err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	if err := c.worktreeManager.Remove(ctx, idOrPath, force); err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	run.Status = "removed"
	return run, nil
}
