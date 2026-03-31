package agent

import (
	"context"
	_ "embed"

	"charm.land/fantasy"
	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

//go:embed tools/tool_search.md
var toolSearchDescription []byte

func (c *coordinator) toolSearchTool(_ context.Context, workingDir string) (fantasy.AgentTool, error) {
	indexLookup := func(ctx context.Context, query string, limit int) (tools.ToolSearchIndexedResult, error) {
		if c.memoryCompiler == nil {
			return tools.ToolSearchIndexedResult{
				Available: false,
				Message:   "Durable codebase graph is not available.",
			}, nil
		}

		status, matches, err := c.memoryCompiler.ToolSearch(ctx, workingDir, query, limit)
		if err != nil {
			return tools.ToolSearchIndexedResult{
				Available: status.Available,
				Message:   err.Error(),
			}, nil
		}

		indexedMatches := make([]tools.ToolSearchIndexedMatch, 0, len(matches))
		for _, match := range matches {
			indexedMatches = append(indexedMatches, tools.ToolSearchIndexedMatch{
				Kind:      match.Kind,
				Path:      match.Path,
				Name:      match.Name,
				Signature: match.Signature,
				Snippet:   match.Snippet,
				Language:  match.Language,
				Role:      match.Role,
				StartLine: match.StartLine,
				EndLine:   match.EndLine,
				Score:     match.Score,
			})
		}

		return tools.ToolSearchIndexedResult{
			Available: status.Available,
			Message:   "Durable codebase graph matches.",
			Matches:   indexedMatches,
		}, nil
	}

	return tools.NewToolSearchTool(string(toolSearchDescription), workingDir, indexLookup), nil
}

var _ agentmemory.ToolSearchMatch
