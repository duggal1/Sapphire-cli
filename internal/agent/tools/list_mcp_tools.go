package tools

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/agent/tools/mcp"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/filepathext"
	"github.com/charmbracelet/sapphire/internal/permission"
)

type ListMCPToolsParams struct {
	MCPName string `json:"mcp_name,omitempty" description:"Optional MCP server name"`
	Query   string `json:"query,omitempty" description:"Optional search query over MCP tool names and descriptions"`
	Limit   int    `json:"limit,omitempty" description:"Maximum number of tools to return"`
}

type ListMCPToolsPermissionsParams struct {
	MCPName string `json:"mcp_name"`
	Query   string `json:"query,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

const ListMCPToolsToolName = "list_mcp_tools"

//go:embed list_mcp_tools.md
var listMCPToolsDescription []byte

func NewListMCPToolsTool(cfg *config.Config, permissions permission.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		ListMCPToolsToolName,
		string(listMCPToolsDescription),
		func(ctx context.Context, params ListMCPToolsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			params.MCPName = strings.TrimSpace(params.MCPName)
			params.Query = strings.TrimSpace(params.Query)

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for listing MCP tools")
			}

			path := filepathext.SmartJoin(cfg.WorkingDir(), cmpOr(params.MCPName, "mcp"))
			p, err := permissions.Request(ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        path,
					ToolCallID:  call.ID,
					ToolName:    ListMCPToolsToolName,
					Action:      "list",
					Description: "List MCP tools",
					Params:      ListMCPToolsPermissionsParams(params),
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			lines := collectMCPTools(params)
			if len(lines) == 0 {
				if params.Query != "" {
					return fantasy.NewTextResponse("No MCP tools matched the query"), nil
				}
				return fantasy.NewTextResponse("No MCP tools found"), nil
			}

			return fantasy.NewTextResponse(strings.Join(lines, "\n")), nil
		},
	)
}

func collectMCPTools(params ListMCPToolsParams) []string {
	type toolLine struct {
		server string
		text   string
		score  int
	}

	lines := []toolLine{}
	queryTerms := mcpQueryTerms(params.Query)
	for name, tools := range mcp.Tools() {
		if params.MCPName != "" && name != params.MCPName {
			continue
		}
		if len(tools) == 0 {
			continue
		}
		headerAdded := false
		for _, tool := range tools {
			if tool == nil {
				continue
			}
			score, ok := scoreConnectedTool(name, tool.Name, tool.Description, params.Query, queryTerms)
			if len(queryTerms) > 0 && !ok {
				continue
			}
			if !headerAdded {
				lines = append(lines, toolLine{server: name, text: name + ":", score: 1 << 30})
				headerAdded = true
			}

			line := fmt.Sprintf("- %s", tool.Name)
			description := strings.TrimSpace(tool.Description)
			if description != "" {
				line = fmt.Sprintf("%s: %s", line, description)
			}
			lines = append(lines, toolLine{server: name, text: line, score: score})
		}
	}

	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].server != lines[j].server {
			return lines[i].server < lines[j].server
		}
		if lines[i].score != lines[j].score {
			return lines[i].score > lines[j].score
		}
		return lines[i].text < lines[j].text
	})

	if params.Limit > 0 {
		result := make([]string, 0, len(lines))
		toolCount := 0
		for _, line := range lines {
			if strings.HasSuffix(line.text, ":") {
				if toolCount >= params.Limit {
					continue
				}
				result = append(result, line.text)
				continue
			}
			if toolCount >= params.Limit {
				continue
			}
			result = append(result, line.text)
			toolCount++
		}
		return trimTrailingHeaders(result)
	}

	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, line.text)
	}
	return trimTrailingHeaders(result)
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func scoreConnectedTool(serverName, toolName, description, query string, terms []string) (int, bool) {
	if len(terms) == 0 {
		return 1, true
	}
	blob := strings.ToLower(strings.Join([]string{serverName, toolName, description}, " "))
	score := 0
	phrase := strings.ToLower(strings.TrimSpace(query))
	if phrase != "" {
		switch {
		case strings.Contains(strings.ToLower(toolName), phrase):
			score += 30
		case strings.Contains(blob, phrase):
			score += 15
		}
	}
	matched := 0
	for _, term := range terms {
		switch {
		case strings.Contains(strings.ToLower(toolName), term):
			score += 12
			matched++
		case strings.Contains(blob, term):
			score += 8
			matched++
		}
	}
	if matched == 0 {
		return 0, false
	}
	return score + matched*2, true
}

func trimTrailingHeaders(lines []string) []string {
	for len(lines) > 0 && strings.HasSuffix(lines[len(lines)-1], ":") {
		lines = lines[:len(lines)-1]
	}
	return lines
}
