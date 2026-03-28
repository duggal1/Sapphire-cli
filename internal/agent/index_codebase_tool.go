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
	Force bool `json:"force,omitempty" description:"Rebuild the durable codebase graph even if it already exists"`
}

//go:embed tools/index_codebase.md
var indexCodebaseDescription []byte

func (c *coordinator) indexCodebaseTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewAgentTool(
		tools.IndexCodebaseToolName,
		string(indexCodebaseDescription),
		func(ctx context.Context, params IndexCodebaseParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			stats, err := c.IndexCodebase(ctx, params.Force)
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
			return fantasy.NewTextResponse(strings.TrimSpace(out.String())), nil
		},
	), nil
}
