package agent

import "charm.land/fantasy"

type mcpToolInfo interface {
	MCP() string
}

func buildActiveToolNames(allTools []fantasy.AgentTool, selectedMCP map[string]struct{}) []string {
	if selectedMCP == nil {
		return nil
	}
	active := make([]string, 0, len(allTools))
	for _, tool := range allTools {
		if mcpTool, ok := tool.(mcpToolInfo); ok {
			if _, allowed := selectedMCP[mcpTool.MCP()]; !allowed {
				continue
			}
		}
		active = append(active, tool.Info().Name)
	}
	return active
}
