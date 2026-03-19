package chat

import (
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestGenericMarkdownOutputIsCollapsed(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewGenericToolMessageItem(&sty, message.ToolCall{
		ID:       "generic-1",
		Name:     "scale",
		Input:    `{"prompt":"test"}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "generic-1",
		Name:       "scale",
		Content:    "# Heading\n\n- item 1\n- item 2\n\nMore text here",
	}, false)

	rendered := item.Render(100)
	if strings.Contains(rendered, "Heading") {
		t.Fatalf("expected markdown body to be hidden, got %q", rendered)
	}
	if !strings.Contains(rendered, "Markdown output hidden") {
		t.Fatalf("expected collapsed markdown summary, got %q", rendered)
	}
}
