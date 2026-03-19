package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/session"
)

func (c *coordinator) validateSubAgentLaunch(ctx context.Context, sessionID, prompt string) (subAgentLaunchDecision, error) {
	decision := evaluateSubAgentLaunch(prompt)

	if sessionID != "" {
		if err := c.ensureSubAgentDepth(ctx, sessionID, 1); err != nil {
			return decision, err
		}
		active := c.activeSubAgentCount(sessionID)
		limit := c.subAgentThreadLimit()
		if limit > 0 && active >= limit {
			return decision, fmt.Errorf("sub-agent launch rejected: %d active sub-agents already running", active)
		}
	}

	return decision, nil
}

func (c *coordinator) validateSubAgentResume(ctx context.Context, parentSessionID string, sess session.Session) error {
	if parentSessionID != "" && sess.ParentSessionID != "" && sess.ParentSessionID != parentSessionID {
		return fmt.Errorf("sub-agent resume rejected: agent belongs to different parent session")
	}
	if err := c.ensureSubAgentDepth(ctx, sess.ID, 0); err != nil {
		return err
	}
	if parentSessionID != "" {
		active := c.activeSubAgentCount(parentSessionID)
		limit := c.subAgentThreadLimit()
		if limit > 0 && active >= limit {
			return fmt.Errorf("sub-agent resume rejected: %d active sub-agents already running", active)
		}
	}
	return nil
}

func (c *coordinator) subAgentThreadLimit() int {
	if c == nil || c.cfg == nil || c.cfg.Options == nil {
		return 0
	}
	if c.cfg.Options.AgentMaxThreads <= 0 {
		return 0
	}
	return c.cfg.Options.AgentMaxThreads
}

func (c *coordinator) ensureSubAgentDepth(ctx context.Context, sessionID string, additional int) error {
	if c == nil || c.cfg == nil || c.cfg.Options == nil {
		return nil
	}
	limit := c.cfg.Options.AgentMaxDepth
	if limit <= 0 {
		return nil
	}
	depthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	depth, err := c.sessionDepth(depthCtx, sessionID)
	if err != nil {
		return fmt.Errorf("sub-agent depth check failed: %w", err)
	}
	if depth+additional > limit {
		return fmt.Errorf("sub-agent launch rejected: depth %d exceeds max %d", depth+additional, limit)
	}
	return nil
}

func (c *coordinator) activeSubAgentCount(sessionID string) int {
	count := 0
	for _, runner := range c.ensureSubAgentRegistry().list() {
		runner.mu.Lock()
		parentSession := runner.parentSession
		status := runner.status
		runner.mu.Unlock()
		if parentSession != sessionID {
			continue
		}
		if isSubAgentActiveStatus(status) {
			count++
		}
	}
	return count
}

func (c *coordinator) hasDuplicateSubAgent(sessionID, taskKey string) bool {
	for _, runner := range c.ensureSubAgentRegistry().list() {
		runner.mu.Lock()
		parentSession := runner.parentSession
		runnerTaskKey := runner.assignment.TaskKey
		status := runner.status
		runner.mu.Unlock()
		if parentSession != sessionID {
			continue
		}
		if runnerTaskKey == taskKey && taskKey != "" {
			if isSubAgentActiveStatus(status) {
				return true
			}
		}
	}
	return false
}

func (c *coordinator) buildSubAgentStatusContext(sessionID string) string {
	snapshots := c.listSubAgentSnapshots(sessionID)
	if len(snapshots) == 0 {
		return ""
	}
	builder := &strings.Builder{}
	builder.WriteString("Sub-agent status:\n")
	for _, snap := range snapshots {
		line := fmt.Sprintf("- %s: %s", snap.ID, snap.Status)
		if snap.Task != "" {
			line += fmt.Sprintf(" | task: %s", truncateForContext(snap.Task, 120))
		}
		if snap.LastProgress != "" {
			line += fmt.Sprintf(" | progress: %s", truncateForContext(snap.LastProgress, 80))
		}
		if !snap.UpdatedAt.IsZero() {
			line += fmt.Sprintf(" | updated %s ago", time.Since(snap.UpdatedAt).Truncate(time.Second))
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func (c *coordinator) listSubAgentSnapshots(sessionID string) []subAgentSnapshot {
	runners := c.ensureSubAgentRegistry().list()
	if len(runners) == 0 {
		return nil
	}
	snapshots := make([]subAgentSnapshot, 0, len(runners))
	for _, runner := range runners {
		if sessionID != "" && runner.parentSession != sessionID {
			continue
		}
		snap := runner.snapshot()
		if snap.Status == subAgentStatusClosed {
			continue
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots
}

func truncateForContext(input string, max int) string {
	trimmed := strings.TrimSpace(input)
	if max <= 0 || len(trimmed) <= max {
		return trimmed
	}
	if max <= 1 {
		return trimmed[:max]
	}
	return trimmed[:max-1] + "…"
}

func (c *coordinator) sessionDepth(ctx context.Context, sessionID string) (int, error) {
	if sessionID == "" {
		return 0, nil
	}
	depth := 0
	current := sessionID
	for current != "" {
		sess, err := c.sessions.Get(ctx, current)
		if err != nil {
			return depth, err
		}
		if sess.ParentSessionID == "" {
			return depth, nil
		}
		depth++
		current = sess.ParentSessionID
	}
	return depth, nil
}
