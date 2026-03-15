package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

func TestBashOutputIsCollapsedByDefault(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewBashToolMessageItem(&sty, message.ToolCall{
		ID:       "bash-1",
		Name:     "bash",
		Input:    `{"command":"go test ./..."}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "bash-1",
		Name:       "bash",
		Content:    "line one\nline two\nline three",
		Metadata:   `{"output":"line one\nline two\nline three"}`,
	}, false)

	rendered := item.Render(100)
	if strings.Contains(rendered, "line two") {
		t.Fatalf("expected raw bash output to be hidden by default, got %q", rendered)
	}
	if !strings.Contains(rendered, "Output hidden by default") {
		t.Fatalf("expected collapsed bash summary, got %q", rendered)
	}
}
