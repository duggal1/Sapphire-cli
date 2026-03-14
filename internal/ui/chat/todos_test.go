package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

func TestTodosToolRenderTextInputWithoutMetadata(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewTodosToolMessageItem(&sty, message.ToolCall{
		ID:       "todos-1",
		Name:     "todos",
		Input:    `{"action":"create","tasks":[{"content":"Read codebase","status":"in_progress","active_form":"Reading codebase"},{"content":"Run tests","status":"pending","active_form":"Running tests"}]}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "todos-1",
		Name:       "todos",
		Content:    "Todo list updated successfully.",
	}, false)

	rendered := item.Render(100)
	requireContainsAll(t, rendered,
		"Reading codebase",
		"Run tests",
		"0/2",
	)
}

func TestTodosToolRenderItemsAliasWithoutMetadata(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewTodosToolMessageItem(&sty, message.ToolCall{
		ID:       "todos-2",
		Name:     "todos",
		Input:    `{"action":"create","tasks":[{"content":"Inspect prompts","status":"completed"},{"content":"Fix renderer","status":"in_progress","active_form":"Fixing renderer"}]}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "todos-2",
		Name:       "todos",
		Content:    "Todo list updated successfully.",
	}, false)

	rendered := item.Render(100)
	requireContainsAll(t, rendered,
		"Inspect prompts",
		"Fixing renderer",
		"1/2",
	)
}

func requireContainsAll(t *testing.T, rendered string, want ...string) {
	t.Helper()

	for _, s := range want {
		if !strings.Contains(rendered, s) {
			t.Fatalf("expected rendered output to contain %q, got %q", s, rendered)
		}
	}
}
