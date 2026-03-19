package chat

import (
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
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
	if !strings.Contains(rendered, "Agentic View") || !strings.Contains(rendered, "Scope: tmp") || !strings.Contains(rendered, "Files: 1") || !strings.Contains(rendered, "Status: read") {
		t.Fatalf("expected agentic view metadata tree, got %q", rendered)
	}
	if !strings.Contains(rendered, "a.go") || !strings.Contains(rendered, "L1-L3") {
		t.Fatalf("expected agentic view to show file and line metadata, got %q", rendered)
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
	if !strings.Contains(rendered, "Single View") || !strings.Contains(rendered, "Scope: tmp") || !strings.Contains(rendered, "File: a.go L1-L3") || !strings.Contains(rendered, "Status: read") {
		t.Fatalf("expected single view metadata tree, got %q", rendered)
	}
}

func TestSingleViewPendingStillRendersMetadataTree(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewViewToolMessageItem(&sty, message.ToolCall{
		ID:       "view-pending",
		Name:     "single_view",
		Input:    `{"file_path":"internal/agent/agent.go","offset":12,"limit":77}`,
		Finished: false,
	}, nil, false)

	rendered := ansi.Strip(item.Render(100))
	if !strings.Contains(rendered, "Single View") || !strings.Contains(rendered, "Scope: internal/agent") {
		t.Fatalf("expected pending single view metadata, got %q", rendered)
	}
	if !strings.Contains(rendered, "File: agent.go") || !strings.Contains(rendered, "L13-L89") {
		t.Fatalf("expected pending single view file metadata, got %q", rendered)
	}
}
