package tools

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/permission"
)

type InstallMCPParams struct {
	MCPName string `json:"mcp_name" description:"The exact MCP server name to install from list_available_mcps"`
}

type InstallMCPPermissionsParams struct {
	MCPName string `json:"mcp_name"`
}

const InstallMCPToolName = "install_mcp"

//go:embed install_mcp.md
var installMCPDescription []byte

func NewInstallMCPTool(cfg *config.Config, permissions permission.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		InstallMCPToolName,
		string(installMCPDescription),
		func(ctx context.Context, params InstallMCPParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			params.MCPName = strings.TrimSpace(params.MCPName)
			if params.MCPName == "" {
				return fantasy.NewTextErrorResponse("mcp_name parameter is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for installing MCP servers")
			}

			path := filepathext.SmartJoin(cfg.WorkingDir(), params.MCPName)
			granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        path,
				ToolCallID:  call.ID,
				ToolName:    InstallMCPToolName,
				Action:      "execute",
				Description: fmt.Sprintf("Install MCP server %s", params.MCPName),
				Params:      InstallMCPPermissionsParams(params),
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !granted {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			mcpCfg, found, created, err := installMCPConfig(ctx, cfg, params.MCPName)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("OPERATIONAL FAILURE: %w", err)
			}
			if !found {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("MCP %q was not found in the registry. Call list_available_mcps and use an exact MCP name from that output.", params.MCPName)), nil
			}

			missing := missingEnvKeys(mcpCfg.Env)
			sort.Strings(missing)
			if len(missing) == 0 && mcpCfg.Disabled && canEnableInstalledMCP(mcpCfg.Env) {
				mcpCfg.Disabled = false
				if err := cfg.UpsertMCPConfig(params.MCPName, mcpCfg); err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
			}

			status := "Already installed"
			if created {
				status = "Installed"
			}
			if len(missing) > 0 {
				return fantasy.NewTextResponse(fmt.Sprintf("%s %q in disabled state. Missing environment variables: %s. Set them, then call connect_mcp.", status, params.MCPName, strings.Join(missing, ", "))), nil
			}

			return fantasy.NewTextResponse(fmt.Sprintf("%s %q. Next: call connect_mcp with the same mcp_name.", status, params.MCPName)), nil
		},
	)
}
