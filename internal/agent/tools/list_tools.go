package tools

import (
	"context"
	_ "embed"
	"sort"
	"strings"

	"charm.land/fantasy"
)

type ListToolsParams struct {
	Query string `json:"query,omitempty" description:"Optional substring to filter tool names"`
	Limit int    `json:"limit,omitempty" description:"Maximum number of tools to return"`
}

const ListToolsToolName = "list_tools"

//go:embed list_tools.md
var listToolsDescription []byte

func NewListToolsTool(list func() []string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ListToolsToolName,
		string(listToolsDescription),
		func(_ context.Context, params ListToolsParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			names := list()
			if len(names) == 0 {
				return fantasy.NewTextResponse("No tools available."), nil
			}

			query := strings.ToLower(strings.TrimSpace(params.Query))
			filtered := make([]string, 0, len(names))
			for _, name := range names {
				if query == "" || strings.Contains(strings.ToLower(name), query) {
					filtered = append(filtered, name)
				}
			}
			if len(filtered) == 0 {
				return fantasy.NewTextResponse("No tools matched the query."), nil
			}

			sort.Strings(filtered)
			if params.Limit > 0 && params.Limit < len(filtered) {
				filtered = filtered[:params.Limit]
			}

			var sb strings.Builder
			sb.WriteString("Available tools:\n")
			for _, name := range filtered {
				sb.WriteString(name + "\n")
			}

			return fantasy.NewTextResponse(strings.TrimSpace(sb.String())), nil
		},
	)
}
