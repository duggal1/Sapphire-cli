package tools

import (
	"context"
	_ "embed"
	"sort"
	"strings"

	"charm.land/fantasy"
)

type SearchToolsParams struct {
	Query string `json:"query" description:"Search query for tool name, description, or parameters"`
	Limit int    `json:"limit,omitempty" description:"Maximum number of tools to return"`
}

const SearchToolsToolName = "search_tools"

//go:embed search_tools.md
var searchToolsDescription []byte

func NewSearchToolsTool(list func() []fantasy.ToolInfo) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SearchToolsToolName,
		string(searchToolsDescription),
		func(_ context.Context, params SearchToolsParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			query := strings.ToLower(strings.TrimSpace(params.Query))
			if query == "" {
				return fantasy.NewTextResponse("Query is required."), nil
			}

			infos := list()
			matches := make([]toolSearchMatch, 0, len(infos))
			for _, info := range infos {
				if toolMatchesQuery(info, query) {
					matches = append(matches, toolSearchMatch{Info: info, Score: matchScore(info, query)})
				}
			}
			if len(matches) == 0 {
				return fantasy.NewTextResponse("No tools matched the query."), nil
			}

			sort.SliceStable(matches, func(i, j int) bool {
				if matches[i].Score == matches[j].Score {
					return matches[i].Info.Name < matches[j].Info.Name
				}
				return matches[i].Score > matches[j].Score
			})

			limit := params.Limit
			if limit <= 0 || limit > len(matches) {
				limit = len(matches)
			}
			var sb strings.Builder
			sb.WriteString("Matching tools:\n")
			for i := 0; i < limit; i++ {
				info := matches[i].Info
				paramsSummary := summarizeToolParams(info)
				line := "- " + info.Name + ": " + strings.TrimSpace(info.Description)
				if paramsSummary != "" {
					line += " | params: " + paramsSummary
				}
				sb.WriteString(line + "\n")
			}
			return fantasy.NewTextResponse(strings.TrimSpace(sb.String())), nil
		},
	)
}

type toolSearchMatch struct {
	Info  fantasy.ToolInfo
	Score int
}

func toolMatchesQuery(info fantasy.ToolInfo, query string) bool {
	combined := strings.ToLower(info.Name + " " + info.Description + " " + toolParamText(info.Parameters))
	return strings.Contains(combined, query)
}

func matchScore(info fantasy.ToolInfo, query string) int {
	score := 0
	if strings.Contains(strings.ToLower(info.Name), query) {
		score += 3
	}
	if strings.Contains(strings.ToLower(info.Description), query) {
		score += 2
	}
	if strings.Contains(strings.ToLower(toolParamText(info.Parameters)), query) {
		score++
	}
	return score
}

func toolParamText(params map[string]any) string {
	if params == nil {
		return ""
	}
	props, _ := params["properties"].(map[string]any)
	if len(props) == 0 {
		return ""
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, " ")
}

func summarizeToolParams(info fantasy.ToolInfo) string {
	props, _ := info.Parameters["properties"].(map[string]any)
	if len(props) == 0 {
		return ""
	}
	requiredSet := make(map[string]struct{}, len(info.Required))
	for _, req := range info.Required {
		requiredSet[req] = struct{}{}
	}
	required := make([]string, 0, len(info.Required))
	optional := make([]string, 0, len(props))
	for key := range props {
		if _, ok := requiredSet[key]; ok {
			required = append(required, key)
		} else {
			optional = append(optional, key)
		}
	}
	sort.Strings(required)
	sort.Strings(optional)
	var sb strings.Builder
	if len(required) > 0 {
		sb.WriteString("required=" + strings.Join(required, ", "))
	}
	if len(optional) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString("optional=" + strings.Join(optional, ", "))
	}
	return sb.String()
}
