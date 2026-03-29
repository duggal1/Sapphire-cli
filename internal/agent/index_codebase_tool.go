package agent

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

type IndexCodebaseParams struct {
	Force     bool `json:"force,omitempty" description:"Rebuild the durable codebase graph even if it already exists"`
	SubAgents int  `json:"sub_agents,omitempty" description:"Optional number of AI sub-agents to use for the mandatory semantic codebase graph pass. Defaults to 3."`
}

//go:embed tools/index_codebase.md
var indexCodebaseDescription []byte

func (c *coordinator) indexCodebaseTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewAgentTool(
		tools.IndexCodebaseToolName,
		string(indexCodebaseDescription),
		func(ctx context.Context, params IndexCodebaseParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			stats, survey, err := c.indexCodebaseWithOptions(ctx, indexCodebaseOptions{
				Force:     params.Force,
				SessionID: tools.GetSessionFromContext(ctx),
				SubAgents: params.SubAgents,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			lastIndexed := "unknown"
			if !stats.LastIndexedAt.IsZero() {
				lastIndexed = stats.LastIndexedAt.UTC().Format("2006-01-02 15:04:05Z")
			}
			var out strings.Builder
			out.WriteString("Durable codebase graph indexed.\n")
			out.WriteString(fmt.Sprintf("- files: %d\n", stats.FileCount))
			out.WriteString("- last_indexed_at: " + lastIndexed)
			if survey != nil {
				out.WriteString(fmt.Sprintf("\n- ai_codebase_graph_status: %s", survey.Status))
				out.WriteString(fmt.Sprintf("\n- ai_codebase_graph_agents: %d", survey.AgentCount))
				out.WriteString(fmt.Sprintf("\n- ai_codebase_graph_shards: %d", survey.ShardCount))
				out.WriteString(fmt.Sprintf("\n- ai_codebase_graph_manifest: %s", survey.ManifestPath))
				out.WriteString(fmt.Sprintf("\n- ai_codebase_graph_overview: %s", survey.OverviewPath))
			}
			return fantasy.NewTextResponse(strings.TrimSpace(out.String())), nil
		},
	), nil
}
