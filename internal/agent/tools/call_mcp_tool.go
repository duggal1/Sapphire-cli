package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools/mcp"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/permission"
)

type CallMCPToolParams struct {
	MCPName   string         `json:"mcp_name" description:"The MCP server name"`
	ToolName  string         `json:"tool_name" description:"The MCP tool name to execute"`
	Arguments map[string]any `json:"arguments,omitempty" description:"JSON arguments to pass to the MCP tool"`
}

func (p *CallMCPToolParams) UnmarshalJSON(data []byte) error {
	type rawCallMCPToolParams struct {
		MCPName    string          `json:"mcp_name,omitempty"`
		Server     string          `json:"server,omitempty"`
		ServerName string          `json:"server_name,omitempty"`
		ToolName   string          `json:"tool_name,omitempty"`
		MCPTool    string          `json:"mcp_tool,omitempty"`
		Arguments  json.RawMessage `json:"arguments,omitempty"`
		Args       json.RawMessage `json:"args,omitempty"`
		Params     json.RawMessage `json:"params,omitempty"`
		Parameters json.RawMessage `json:"parameters,omitempty"`
		Input      json.RawMessage `json:"input,omitempty"`
	}

	var raw rawCallMCPToolParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.MCPName = firstFetchString(raw.MCPName, raw.Server, raw.ServerName)
	p.ToolName = firstFetchString(raw.ToolName, raw.MCPTool)
	p.Arguments = decodeObjectRawMessage(raw.Arguments, raw.Args, raw.Params, raw.Parameters, raw.Input)
	return nil
}

type CallMCPToolPermissionsParams struct {
	MCPName   string         `json:"mcp_name"`
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func decodeObjectRawMessage(values ...json.RawMessage) map[string]any {
	for _, raw := range values {
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err == nil && obj != nil {
			return obj
		}
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err == nil {
			encoded = strings.TrimSpace(encoded)
			if encoded == "" {
				continue
			}
			if err := json.Unmarshal([]byte(encoded), &obj); err == nil && obj != nil {
				return obj
			}
		}
	}
	return nil
}

const CallMCPToolName = "call_mcp_tool"

//go:embed call_mcp_tool.md
var callMCPToolDescription []byte

func NewCallMCPTool(cfg *config.Config, permissions permission.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		CallMCPToolName,
		string(callMCPToolDescription),
		func(ctx context.Context, params CallMCPToolParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			params.MCPName = strings.TrimSpace(params.MCPName)
			params.ToolName = strings.TrimSpace(params.ToolName)
			if params.MCPName == "" {
				return fantasy.NewTextErrorResponse("mcp_name parameter is required"), nil
			}
			if params.ToolName == "" {
				return fantasy.NewTextErrorResponse("tool_name parameter is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for calling MCP tools")
			}

			path := filepathext.SmartJoin(cfg.WorkingDir(), params.MCPName)
			p, err := permissions.Request(ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        path,
					ToolCallID:  call.ID,
					ToolName:    CallMCPToolName,
					Action:      "execute",
					Description: fmt.Sprintf("Execute MCP tool %s on %s", params.ToolName, params.MCPName),
					Params: CallMCPToolPermissionsParams{
						MCPName:   params.MCPName,
						ToolName:  params.ToolName,
						Arguments: params.Arguments,
					},
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			if params.Arguments == nil {
				params.Arguments = map[string]any{}
			}
			payload, err := json.Marshal(params.Arguments)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			result, err := mcp.RunTool(ctx, cfg, params.MCPName, params.ToolName, string(payload))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			switch result.Type {
			case "image", "media":
				if !GetSupportsImagesFromContext(ctx) {
					modelName := GetModelNameFromContext(ctx)
					return fantasy.NewTextErrorResponse(fmt.Sprintf("This model (%s) does not support image data.", modelName)), nil
				}

				var response fantasy.ToolResponse
				if result.Type == "image" {
					response = fantasy.NewImageResponse(result.Data, result.MediaType)
				} else {
					response = fantasy.NewMediaResponse(result.Data, result.MediaType)
				}
				response.Content = result.Content
				return response, nil
			default:
				return fantasy.NewTextResponse(result.Content), nil
			}
		},
	)
}
