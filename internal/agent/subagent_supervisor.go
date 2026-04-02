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
			return decision, fmt.Errorf("sub-agent limit reached: system currently allows up to %d concurrent sub-agents; %d already active", limit, active)
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
			return fmt.Errorf("sub-agent limit reached: system currently allows up to %d concurrent sub-agents; %d already active", limit, active)
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
	if c == nil {
		return 0
	}
	c.subAgentsMu.Lock()
	defer c.subAgentsMu.Unlock()
	// Use cached count if available
	if c.subAgentActiveCount != nil {
		return c.subAgentActiveCount[sessionID]
	}
	// Fallback: count from registry (used in tests and cold starts)
	count := 0
	if c.subAgentRegistry != nil {
		c.subAgentRegistry.mu.Lock()
		for _, runner := range c.subAgentRegistry.agents {
			runner.mu.Lock()
			if runner.parentSession == sessionID && isSubAgentActiveStatus(runner.status) {
				count++
			}
			runner.mu.Unlock()
		}
		c.subAgentRegistry.mu.Unlock()
	}
	return count
}

func (c *coordinator) adjustActiveSubAgentCount(sessionID string, delta int) {
	if c == nil || sessionID == "" {
		return
	}
	c.subAgentsMu.Lock()
	defer c.subAgentsMu.Unlock()
	if c.subAgentActiveCount == nil {
		c.subAgentActiveCount = make(map[string]int)
	}
	c.subAgentActiveCount[sessionID] += delta
	if c.subAgentActiveCount[sessionID] < 0 {
		c.subAgentActiveCount[sessionID] = 0
	}
}

func (c *coordinator) hasDuplicateSubAgent(sessionID, taskKey string) bool {
	if taskKey == "" {
		// Fallback to full scan if no task key
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
	// O(1) lookup via task key index
	c.subAgentsMu.Lock()
	registry := c.subAgentRegistry
	c.subAgentsMu.Unlock()
	if registry == nil {
		return false
	}
	registry.mu.Lock()
	keys, ok := registry.taskKeyIndex[sessionID]
	registry.mu.Unlock()
	if !ok {
		return false
	}
	if _, hasKey := keys[taskKey]; hasKey {
		// Verify at least one active runner has this task key
		for _, runner := range c.ensureSubAgentRegistry().list() {
			runner.mu.Lock()
			parentSession := runner.parentSession
			runnerTaskKey := runner.assignment.TaskKey
			status := runner.status
			runner.mu.Unlock()
			if parentSession == sessionID && runnerTaskKey == taskKey && isSubAgentActiveStatus(status) {
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
	// Pre-allocate builder with estimated capacity
	builder := &strings.Builder{}
	builder.Grow(len(snapshots) * 180) // ~180 chars per line estimate
	builder.WriteString("Sub-agent status:\n")
	for _, snap := range snapshots {
		builder.WriteByte('-')
		builder.WriteByte(' ')
		builder.WriteString(snap.ID)
		builder.WriteString(": ")
		builder.WriteString(string(snap.Status))
		if snap.Task != "" {
			builder.WriteString(" | task: ")
			builder.WriteString(truncateForContext(snap.Task, 120))
		}
		if snap.LastProgress != "" {
			builder.WriteString(" | progress: ")
			builder.WriteString(truncateForContext(snap.LastProgress, 80))
		}
		if !snap.UpdatedAt.IsZero() {
			builder.WriteString(" | updated ")
			builder.WriteString(time.Since(snap.UpdatedAt).Truncate(time.Second).String())
			builder.WriteString(" ago")
		}
		builder.WriteByte('\n')
	}
	return strings.TrimRight(builder.String(), "\n")
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
	// Check bounded cache first
	c.subAgentsMu.Lock()
	if c.depthCache == nil {
		c.depthCache = make(map[string]int)
	}
	if depth, ok := c.depthCache[sessionID]; ok {
		c.subAgentsMu.Unlock()
		return depth, nil
	}
	c.subAgentsMu.Unlock()

	depth := 0
	current := sessionID
	limit := c.cfg.Options.AgentMaxDepth
	if limit <= 0 {
		limit = 10 // safety bound
	}
	for current != "" && depth < limit {
		sess, err := c.sessions.Get(ctx, current)
		if err != nil {
			return depth, err
		}
		if sess.ParentSessionID == "" {
			// Cache all depths along the chain
			c.subAgentsMu.Lock()
			if c.depthCache == nil {
				c.depthCache = make(map[string]int)
			}
			// Walk back and cache
			walkDepth := depth
			walkCurrent := current
			for walkCurrent != "" && walkDepth >= 0 {
				c.depthCache[walkCurrent] = walkDepth
				if walkDepth == 0 {
					break
				}
				// We can't walk back without storing the chain, so just cache the current
				break
			}
			c.depthCache[sessionID] = depth
			c.subAgentsMu.Unlock()
			return depth, nil
		}
		depth++
		current = sess.ParentSessionID
	}
	// Cache result
	c.subAgentsMu.Lock()
	if c.depthCache == nil {
		c.depthCache = make(map[string]int)
	}
	c.depthCache[sessionID] = depth
	c.subAgentsMu.Unlock()
	return depth, nil
}
