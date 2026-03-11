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
	MCPName string `json:"mcp_name" description:"Optional MCP server name"`
}

type ListMCPToolsPermissionsParams struct {
	MCPName string `json:"mcp_name"`
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

			lines := collectMCPTools(params.MCPName)
			if len(lines) == 0 {
				return fantasy.NewTextResponse("No MCP tools found"), nil
			}

			sort.Strings(lines)
			return fantasy.NewTextResponse(strings.Join(lines, "\n")), nil
		},
	)
}

func collectMCPTools(mcpName string) []string {
	lines := []string{}
	for name, tools := range mcp.Tools() {
		if mcpName != "" && name != mcpName {
			continue
		}
		if len(tools) == 0 {
			continue
		}
		lines = append(lines, name+":")
		for _, tool := range tools {
			if tool == nil {
				continue
			}
			line := fmt.Sprintf("- %s", tool.Name)
			if strings.TrimSpace(tool.Description) != "" {
				line = fmt.Sprintf("%s: %s", line, tool.Description)
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
