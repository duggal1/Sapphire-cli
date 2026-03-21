package tools

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/codeindex"
)

const SemanticSearchToolName = "semantic_search"

type SemanticSearchParams struct {
	Query string `json:"query" description:"Natural-language code search query"`
	Limit int    `json:"limit,omitempty" description:"Maximum number of matching chunks to return"`
}

func NewSemanticSearchTool(index *codeindex.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SemanticSearchToolName,
		"Search the indexed codebase using real embeddings. Use this when you need retrieval across the repository without manually reading every file first.",
		func(ctx context.Context, params SemanticSearchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if index == nil {
				return fantasy.NewTextErrorResponse("semantic search is not configured"), nil
			}
			query := strings.TrimSpace(params.Query)
			if query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}
			limit := params.Limit
			if limit <= 0 {
				limit = 8
			}
			results, err := index.Search(ctx, query, limit)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if len(results) == 0 {
				return fantasy.NewTextResponse("No indexed matches found."), nil
			}
			var out strings.Builder
			out.WriteString("Indexed matches:\n")
			for _, result := range results {
				out.WriteString(fmt.Sprintf("- %s:%d-%d | score %.3f | %s | %s\n  %s\n",
					result.Path,
					result.StartLine,
					result.EndLine,
					result.Score,
					result.Language,
					result.Kind,
					result.Snippet,
				))
			}
			return fantasy.NewTextResponse(strings.TrimSpace(out.String())), nil
		},
	)
}

