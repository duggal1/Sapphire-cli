package tools

import (
	"context"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/memory"
)

type MemoryQueryParams struct {
	Query string `json:"query" description:"The search query for memory (e.g., 'how did we fix the auth bug?')"`
	Type  string `json:"type,omitempty" description:"The type of memory to query (one of: 'summaries', 'codebase', 'history'). Defaults to all."`
}

const MemoryQueryToolName = "memory_query"

func NewMemoryQueryTool(_ memory.MemoryService) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MemoryQueryToolName,
		"Query the agent's long-term memory. Session-history summaries are intentionally disabled. Codebase indexing is temporarily disabled.",
		func(ctx context.Context, params MemoryQueryParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Type == "summaries" || params.Type == "history" {
				return fantasy.NewTextResponse("Session-history summaries are disabled."), nil
			}
			if params.Type == "codebase" {
				return fantasy.NewTextResponse("Codebase retrieval is temporarily disabled."), nil
			}
			return fantasy.NewTextResponse("No relevant long-term memory found."), nil
		},
	)
}
