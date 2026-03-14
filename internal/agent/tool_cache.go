package agent

import "charm.land/fantasy"

func (c *coordinator) setToolCache(tools []fantasy.AgentTool) {
	if c == nil {
		return
	}
	cloned := append([]fantasy.AgentTool(nil), tools...)
	names := make([]string, 0, len(cloned))
	for _, tool := range cloned {
		names = append(names, tool.Info().Name)
	}
	c.toolCacheMu.Lock()
	c.cachedTools = cloned
	c.cachedToolNames = names
	c.toolCacheMu.Unlock()
}

func (c *coordinator) getToolCache() ([]fantasy.AgentTool, []string) {
	if c == nil {
		return nil, nil
	}
	c.toolCacheMu.RLock()
	defer c.toolCacheMu.RUnlock()
	clonedTools := append([]fantasy.AgentTool(nil), c.cachedTools...)
	clonedNames := append([]string(nil), c.cachedToolNames...)
	return clonedTools, clonedNames
}
