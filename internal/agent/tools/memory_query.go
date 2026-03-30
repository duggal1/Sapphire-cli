package tools

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/memory"
)

type MemoryQueryParams struct {
	Query string `json:"query" description:"The search query for memory (e.g., 'how did we fix the auth bug?')"`
	Type  string `json:"type,omitempty" description:"The type of memory to query (one of: 'summaries', 'codebase', 'history'). Defaults to all."`
}

const MemoryQueryToolName = "memory_query"

func NewMemoryQueryTool(mem memory.MemoryService) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MemoryQueryToolName,
		"Query the agent's long-term memory. Session-history summaries are intentionally disabled. Durable codebase context is delivered through the compiled boot packet instead of this tool.",
		func(ctx context.Context, params MemoryQueryParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Type == "summaries" || params.Type == "history" {
				return fantasy.NewTextResponse("Session-history summaries are disabled."), nil
			}
			if mem != nil && strings.TrimSpace(params.Query) != "" && (params.Type == "" || params.Type == "codebase") {
				results, err := mem.SearchCodebaseKnowledge(ctx, params.Query, 5)
				if err == nil && len(results) > 0 {
					var sb strings.Builder
					sb.WriteString("Codebase Knowledge\n")
					for _, result := range results {
						sb.WriteString(fmt.Sprintf("- %s (%s) in %s", result.SymbolName, result.SymbolType, result.FilePath))
						if result.Documentation.Valid && strings.TrimSpace(result.Documentation.String) != "" {
							sb.WriteString(fmt.Sprintf(": %s", strings.TrimSpace(result.Documentation.String)))
						}
						sb.WriteString("\n")
					}
					return fantasy.NewTextResponse(strings.TrimSpace(sb.String())), nil
				}
			}
			if params.Type == "codebase" {
				return fantasy.NewTextResponse("Codebase retrieval is handled through the compiled boot packet when no matching durable codebase knowledge is stored. For a fresh repo-wide graph, call `index_codebase`."), nil
			}
			return fantasy.NewTextResponse("No relevant long-term memory found."), nil
		},
	)
}
