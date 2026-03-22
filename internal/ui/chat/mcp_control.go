package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

type MCPControlToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*MCPControlToolMessageItem)(nil)

func NewMCPControlToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &MCPControlToolRenderContext{}, canceled)
}

type MCPControlToolRenderContext struct{}

func (m *MCPControlToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	title, mode, detail := mcpControlPresentation(opts.ToolCall)
	header := toolHeader(sty, opts.Status, "MCP", cappedWidth, opts.Compact, title)
	if opts.Compact {
		return header
	}

	tag := sty.TagInfo.Render(mode)
	contentWidth := cappedWidth - lipgloss.Width(tag) - 5
	if contentWidth < 10 {
		contentWidth = 10
	}
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		sty.PanelPadded.Render(
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				tag,
				" ",
				sty.Base.Width(contentWidth).Render(detail),
			),
		),
	)

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(body, earlyState)
	}
	if !opts.HasResult() || strings.TrimSpace(opts.Result.Content) == "" {
		return body
	}
	output := sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent))
	return joinToolParts(body, output)
}

func mcpControlPresentation(call message.ToolCall) (string, string, string) {
	type namedParams struct {
		MCPName string `json:"mcp_name"`
		Query   string `json:"query"`
		URI     string `json:"uri"`
	}
	var params namedParams
	_ = json.Unmarshal([]byte(call.Input), &params)

	name := strings.TrimSpace(params.MCPName)
	switch call.Name {
	case tools.InstallMCPToolName:
		if name == "" {
			return "Installing MCP", "Install", "Installing MCP configuration"
		}
		return "Installing MCP", "Install", fmt.Sprintf("Installing MCP configuration: %s", name)
	case tools.ConnectMCPToolName:
		if name == "" {
			return "Connecting MCP", "Connect", "Connecting installed MCP server"
		}
		return "Connecting MCP", "Connect", fmt.Sprintf("Connecting MCP server: %s", name)
	case tools.ListAvailableMCPsToolName:
		if query := strings.TrimSpace(params.Query); query != "" {
			return "Searching MCP Registry", "Registry", fmt.Sprintf("Searching MCP inventory for: %s", query)
		}
		return "Reading MCP Registry", "Registry", "Listing MCP inventory and local installation state"
	case tools.ListMCPToolsToolName:
		if name == "" {
			return "Reading MCP Tools", "Tools", "Listing tools exposed by the selected MCP"
		}
		return "Reading MCP Tools", "Tools", fmt.Sprintf("Listing tools exposed by: %s", name)
	case tools.ListMCPResourcesToolName:
		if name == "" {
			return "Reading MCP Resources", "Resources", "Listing resources exposed by the selected MCP"
		}
		return "Reading MCP Resources", "Resources", fmt.Sprintf("Listing resources exposed by: %s", name)
	case tools.ReadMCPResourceToolName:
		if name == "" {
			return "Reading MCP Resource", "Resource", "Reading MCP resource content"
		}
		return "Reading MCP Resource", "Resource", fmt.Sprintf("Reading an MCP resource from: %s", name)
	default:
		return "MCP", "MCP", "Executing MCP control operation"
	}
}
