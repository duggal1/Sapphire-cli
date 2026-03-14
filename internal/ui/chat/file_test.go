package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

func TestAgenticViewHidesFileContents(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewViewToolMessageItem(&sty, message.ToolCall{
		ID:       "view-1",
		Name:     "agentic_view",
		Input:    `{"file_paths":["/tmp/a.go"]}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "view-1",
		Name:       "agentic_view",
		Metadata:   `{"files":[{"file_path":"/tmp/a.go","content":"package main\n\nfunc main() {}"}]}`,
	}, false)

	rendered := ansi.Strip(item.Render(100))
	if strings.Contains(rendered, "func main()") {
		t.Fatalf("expected agentic view to hide file content, got %q", rendered)
	}
	if !strings.Contains(rendered, "Agent read file:") || !strings.Contains(rendered, "a.go") {
		t.Fatalf("expected agentic view to show file summary, got %q", rendered)
	}
}

func TestSingleViewHidesFileContents(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewViewToolMessageItem(&sty, message.ToolCall{
		ID:       "view-2",
		Name:     "single_view",
		Input:    `{"file_path":"/tmp/a.go"}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "view-2",
		Name:       "single_view",
		Metadata:   `{"file_path":"/tmp/a.go","content":"package main\n\nfunc main() {}"}`,
	}, false)

	rendered := ansi.Strip(item.Render(100))
	if strings.Contains(rendered, "func main()") {
		t.Fatalf("expected single view to hide file content, got %q", rendered)
	}
	if !strings.Contains(rendered, "Agent read file:") || !strings.Contains(rendered, "a.go") {
		t.Fatalf("expected single view to show file summary, got %q", rendered)
	}
}
