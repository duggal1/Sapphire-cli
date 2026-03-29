package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools/mcp"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/permission"
)

type ConnectMCPParams struct {
	MCPName string `json:"mcp_name" description:"The installed MCP server name to connect"`
}

type ConnectMCPPermissionsParams struct {
	MCPName string `json:"mcp_name"`
}

const ConnectMCPToolName = "connect_mcp"

//go:embed connect_mcp.md
var connectMCPDescription []byte

func NewConnectMCPTool(cfg *config.Config, permissions permission.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		ConnectMCPToolName,
		string(connectMCPDescription),
		func(ctx context.Context, params ConnectMCPParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			params.MCPName = strings.TrimSpace(params.MCPName)
			if params.MCPName == "" {
				return fantasy.NewTextErrorResponse("mcp_name parameter is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for connecting MCP servers")
			}

			path := filepathext.SmartJoin(cfg.WorkingDir(), params.MCPName)
			p, err := permissions.Request(ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        path,
					ToolCallID:  call.ID,
					ToolName:    ConnectMCPToolName,
					Action:      "execute",
					Description: fmt.Sprintf("Connect MCP server %s", params.MCPName),
					Params:      ConnectMCPPermissionsParams(params),
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			mcpCfg, ok := cfg.MCP[params.MCPName]
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("MCP %q is not installed. Call install_mcp first.", params.MCPName)), nil
			}

			if missing := missingEnvKeys(mcpCfg.Env); len(missing) > 0 {
				sort.Strings(missing)
				return fantasy.NewTextErrorResponse(fmt.Sprintf("MCP %q is installed but requires environment variables before it can connect: %s", params.MCPName, strings.Join(missing, ", "))), nil
			}

			mcpCfg.Disabled = false
			if err := cfg.UpsertMCPConfig(params.MCPName, mcpCfg); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			connectCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			if err := mcp.ApplyConfig(connectCtx, cfg, params.MCPName); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("MCP %q did not connect before the timeout. Inspect its configuration and retry.", params.MCPName)), nil
				}
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			state, ok := mcp.GetState(params.MCPName)
			if !ok {
				return fantasy.NewTextResponse(fmt.Sprintf("Configured MCP %q", params.MCPName)), nil
			}

			switch state.State {
			case mcp.StateConnected:
				return fantasy.NewTextResponse(fmt.Sprintf(
					"Connected %s (tools=%d prompts=%d resources=%d)",
					params.MCPName,
					state.Counts.Tools,
					state.Counts.Prompts,
					state.Counts.Resources,
				)), nil
			case mcp.StateError:
				if state.Error != nil {
					return fantasy.NewTextErrorResponse(state.Error.Error()), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("MCP %q failed to connect", params.MCPName)), nil
			default:
				return fantasy.NewTextResponse(fmt.Sprintf("Configured %s [%s]", params.MCPName, state.State.String())), nil
			}
		},
	)
}

func installMCPConfig(ctx context.Context, cfg *config.Config, name string) (config.MCPConfig, bool, bool, error) {
	if existing, ok := cfg.MCP[name]; ok {
		return existing, true, false, nil
	}

	registryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	for _, def := range config.DefaultRegistryDefinitions(registryCtx) {
		if def.Name != name {
			continue
		}
		installCfg := config.RegistryDefinitionToMCPConfig(def, false)
		if err := cfg.UpsertMCPConfig(name, installCfg); err != nil {
			return config.MCPConfig{}, false, false, err
		}
		return installCfg, true, true, nil
	}

	for _, def := range loadLiveRegistryDefinitions(ctx) {
		if def.Name != name {
			continue
		}
		installCfg := config.RegistryDefinitionToMCPConfig(def, false)
		if err := cfg.UpsertMCPConfig(name, installCfg); err != nil {
			return config.MCPConfig{}, false, false, err
		}
		return installCfg, true, true, nil
	}

	return config.MCPConfig{}, false, false, nil
}

func missingEnvKeys(envMap map[string]string) []string {
	missing := make([]string, 0, len(envMap))
	for key := range envMap {
		if _, ok := os.LookupEnv(key); ok {
			continue
		}
		missing = append(missing, key)
	}
	return missing
}

func canEnableInstalledMCP(envMap map[string]string) bool {
	if len(envMap) == 0 {
		return true
	}
	for key := range envMap {
		if _, ok := os.LookupEnv(key); !ok {
			return false
		}
	}
	return true
}
