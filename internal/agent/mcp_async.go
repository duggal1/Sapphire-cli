package agent

import (
	"context"
	"errors"
	"strings"
)

type mcpPreflightSnapshot struct {
	promptKey string
	context   string
}

type mcpSelectionSnapshot struct {
	promptKey string
	selected  map[string]struct{}
	context   string
}

func normalizePromptKey(prompt string) string {
	return strings.ToLower(strings.TrimSpace(prompt))
}

func (c *coordinator) getMCPPreflightContext(sessionID, prompt string) string {
	if c == nil {
		return ""
	}
	promptKey := normalizePromptKey(prompt)
	c.mcpPreflightMu.Lock()
	defer c.mcpPreflightMu.Unlock()
	snap, ok := c.mcpPreflightCache[sessionID]
	if !ok || snap.promptKey != promptKey {
		return ""
	}
	return snap.context
}

func (c *coordinator) refreshMCPPreflightAsync(sessionID, prompt string) {
	if c == nil {
		return
	}
	if !requiresMCPDiscovery(prompt) {
		return
	}
	promptKey := normalizePromptKey(prompt)
	c.mcpPreflightMu.Lock()
	if c.mcpPreflightInFlight[sessionID] {
		c.mcpPreflightMu.Unlock()
		return
	}
	c.mcpPreflightInFlight[sessionID] = true
	c.mcpPreflightMu.Unlock()

	go func() {
		defer func() {
			c.mcpPreflightMu.Lock()
			c.mcpPreflightInFlight[sessionID] = false
			c.mcpPreflightMu.Unlock()
		}()

		ctx := context.Background()
		preflightContext, err := c.preflightMCPDiscovery(ctx, sessionID, prompt)
		if err != nil {
			if errors.Is(err, errMissingMCP) {
				c.mcpPreflightMu.Lock()
				c.mcpPreflightCache[sessionID] = mcpPreflightSnapshot{
					promptKey: promptKey,
					context:   "<mcp_missing>\n" + missingMCPMessage + "\n</mcp_missing>",
				}
				c.mcpPreflightMu.Unlock()
			}
			return
		}
		if strings.TrimSpace(preflightContext) == "" {
			return
		}
		c.mcpPreflightMu.Lock()
		c.mcpPreflightCache[sessionID] = mcpPreflightSnapshot{
			promptKey: promptKey,
			context:   preflightContext,
		}
		c.mcpPreflightMu.Unlock()
	}()
}

func (c *coordinator) getMCPSelection(sessionID, prompt string) (map[string]struct{}, string) {
	if c == nil {
		return nil, ""
	}
	promptKey := normalizePromptKey(prompt)
	c.mcpSelectionMu.Lock()
	defer c.mcpSelectionMu.Unlock()
	snap, ok := c.mcpSelectionCache[sessionID]
	if !ok || snap.promptKey != promptKey {
		return nil, ""
	}
	selected := make(map[string]struct{}, len(snap.selected))
	for key := range snap.selected {
		selected[key] = struct{}{}
	}
	return selected, snap.context
}

func (c *coordinator) refreshMCPSelectionAsync(sessionID, prompt string) {
	if c == nil {
		return
	}
	if !requiresMCPDiscovery(prompt) {
		return
	}
	promptKey := normalizePromptKey(prompt)
	c.mcpSelectionMu.Lock()
	if c.mcpSelectionInFlight[sessionID] {
		c.mcpSelectionMu.Unlock()
		return
	}
	c.mcpSelectionInFlight[sessionID] = true
	c.mcpSelectionMu.Unlock()

	go func() {
		defer func() {
			c.mcpSelectionMu.Lock()
			c.mcpSelectionInFlight[sessionID] = false
			c.mcpSelectionMu.Unlock()
		}()

		ctx := context.Background()
		selected, ctxStr, err := c.selectMCPServers(ctx, prompt)
		if err != nil || len(selected) == 0 {
			return
		}
		c.mcpSelectionMu.Lock()
		c.mcpSelectionCache[sessionID] = mcpSelectionSnapshot{
			promptKey: promptKey,
			selected:  selected,
			context:   ctxStr,
		}
		c.mcpSelectionMu.Unlock()
	}()
}
