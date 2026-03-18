package chat

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/sapphire/internal/agent"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
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

	if opts.Status == ToolStatusError {
		return joinToolParts(header, sty.Tool.ErrorMessage.Render("Sub-agent launch rejected"))
	}

	if !opts.HasResult() || opts.Result.Content == "" {
		return header
	}

	var payload agent.SpawnAgentParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &payload)

	status := "Sub-agent launched"
	if payload.Title != "" {
		status = fmt.Sprintf("Sub-agent launched: %s", payload.Title)
	}

	return joinToolParts(header, sty.Tool.ContentLine.Render(status))
}
