package chat

import (
	"encoding/json"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

// SpawnAgentToolMessageItem renders spawn_agent without leaking raw errors.
type SpawnAgentToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*SpawnAgentToolMessageItem)(nil)

// NewSpawnAgentToolMessageItem creates a new spawn_agent message item.
func NewSpawnAgentToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &SpawnAgentToolRenderContext{}, canceled)
}

// SpawnAgentToolRenderContext renders spawn_agent tool messages.
type SpawnAgentToolRenderContext struct{}

// RenderTool implements the ToolRenderer interface.
func (s *SpawnAgentToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Spawn Agent", opts.Anim)
	}

	header := toolHeader(sty, opts.Status, "Spawn Agent", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	lower := strings.ToLower(resultContent(opts))
	if strings.Contains(lower, "launch rejected") || strings.Contains(lower, "too small for delegation") {
		root := &TreeNode{
			Label: sty.Tool.ListRoot.Render("Sub-Agent"),
			Children: []*TreeNode{
				{Label: subAgentKVLabel("Status", "handled locally")},
			},
		}
		body := strings.Join(renderTreeWithRoot(root, cappedWidth-toolBodyLeftPaddingTotal), "\n")
		return joinToolParts(header, sty.Tool.Body.Render(body))
	}

	var payload agent.SpawnAgentParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &payload)

	var result subAgentSpawnResult
	_ = json.Unmarshal([]byte(resultContent(opts)), &result)

	if opts.Status == ToolStatusError {
		errorText := strings.TrimSpace(resultContent(opts))
		if errorText == "" {
			errorText = "launch failed"
		}
		children := []*TreeNode{}
		if payload.Title != "" {
			children = append(children, &TreeNode{Label: subAgentKVLabel("Title", payload.Title)})
		}
		if payload.Agent != "" {
			children = append(children, &TreeNode{Label: subAgentKVLabel("Profile", payload.Agent)})
		}
		children = append(children,
			&TreeNode{Label: subAgentKVLabel("Status", "error")},
			&TreeNode{Label: subAgentKVLabel("Error", oneLine(errorText))},
		)
		root := &TreeNode{
			Label:    sty.Tool.ListRoot.Render("Sub-Agent"),
			Children: children,
		}
		body := strings.Join(renderTreeWithRoot(root, cappedWidth-toolBodyLeftPaddingTotal), "\n")
		return joinToolParts(header, sty.Tool.Body.Render(body))
	}

	body := renderSubAgentSpawnBody(sty, &payload, &result, cappedWidth-toolBodyLeftPaddingTotal)
	if body == "" {
		return header
	}
	return joinToolParts(header, sty.Tool.Body.Render(body))
}
