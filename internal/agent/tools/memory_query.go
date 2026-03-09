package tools

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/agent/memory"
)

type MemoryQueryParams struct {
	Query string `json:"query" description:"The search query for memory (e.g., 'how did we fix the auth bug?')"`
	Type  string `json:"type,omitempty" description:"The type of memory to query (one of: 'summaries', 'codebase', 'history'). Defaults to all."`
}

const MemoryQueryToolName = "memory_query"

func NewMemoryQueryTool(mem memory.MemoryService) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MemoryQueryToolName,
		"Query the agent's long-term memory (cold memory), including past session summaries and codebase knowledge.",
		func(ctx context.Context, params MemoryQueryParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			var sb strings.Builder

			if params.Type == "" || params.Type == "summaries" {
				summaries, err := mem.ListStructuredSummaries(ctx, 10) // Get latest 10
				if err == nil && len(summaries) > 0 {
					sb.WriteString("### Past Session Summaries\n")
					for _, s := range summaries {
						sb.WriteString(fmt.Sprintf("#### Session: %s\n%s\n", s.SessionID, s.SummaryData))
					}
				}
			}

			if params.Type == "" || params.Type == "codebase" {
				knowledge, err := mem.SearchCodebaseKnowledge(ctx, params.Query, 20)
				if err == nil && len(knowledge) > 0 {
					sb.WriteString("\n### Codebase Knowledge\n")
					for _, k := range knowledge {
						sb.WriteString(fmt.Sprintf("- **%s** (%s) in %s\n  %s\n", k.SymbolName, k.SymbolType, k.FilePath, k.Documentation.String))
					}
				}
			}

			output := sb.String()
			if output == "" {
				return fantasy.NewTextResponse("No relevant memory found."), nil
			}

			return fantasy.NewTextResponse(output), nil
		},
	)
}
