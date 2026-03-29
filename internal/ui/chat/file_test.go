package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
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
	if !strings.Contains(rendered, "View") || !strings.Contains(rendered, "Scope: tmp") || !strings.Contains(rendered, "File: a.go L1-L3") || !strings.Contains(rendered, "Status: read") || !strings.Contains(rendered, "Purpose: inspect single-file context") {
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
	if !strings.Contains(rendered, "View") || !strings.Contains(rendered, "Scope: internal/agent") {
		t.Fatalf("expected pending single view metadata, got %q", rendered)
	}
	if !strings.Contains(rendered, "File: agent.go") || !strings.Contains(rendered, "L12-L88") || !strings.Contains(rendered, "Activity:") || !strings.Contains(rendered, "reading file") {
		t.Fatalf("expected pending single view file metadata, got %q", rendered)
	}
}

func TestSingleViewUsesAliasPathFields(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewViewToolMessageItem(&sty, message.ToolCall{
		ID:       "view-alias",
		Name:     "single_view",
		Input:    `{"path":"internal/ui/chat/file.go","offset":9,"limit":53}`,
		Finished: false,
	}, nil, false)

	rendered := ansi.Strip(item.Render(100))
	if !strings.Contains(rendered, "View") || !strings.Contains(rendered, "Scope: internal/ui/chat") {
		t.Fatalf("expected alias path single view metadata, got %q", rendered)
	}
	if !strings.Contains(rendered, "File: file.go L9-L61") {
		t.Fatalf("expected exact alias path line range, got %q", rendered)
	}
}

func TestAgenticViewPendingRendersLoaderActivity(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewViewToolMessageItem(&sty, message.ToolCall{
		ID:       "view-agentic-pending",
		Name:     "agentic_view",
		Input:    `{"file_paths":["internal/cmd/root.go","internal/ui/chat/file.go"]}`,
		Finished: false,
	}, nil, false)

	rendered := ansi.Strip(item.Render(100))
	if !strings.Contains(rendered, "Agentic View") || !strings.Contains(rendered, "Files: 2") {
		t.Fatalf("expected pending agentic view metadata, got %q", rendered)
	}
	if !strings.Contains(rendered, "Activity:") || !strings.Contains(rendered, "reading 2 files") {
		t.Fatalf("expected pending agentic view loader activity, got %q", rendered)
	}
}

func TestPendingViewInvalidatesOnEveryShimmerTick(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewViewToolMessageItem(&sty, message.ToolCall{
		ID:       "view-agentic-pending-tick",
		Name:     "agentic_view",
		Input:    `{"file_paths":["internal/cmd/root.go","internal/ui/chat/file.go"]}`,
		Finished: false,
	}, nil, false)

	tickable, ok := item.(ShimmerTickable)
	if !ok {
		t.Fatal("expected pending view item to support shimmer ticks")
	}

	if !tickable.OnShimmerTick() {
		t.Fatal("expected first shimmer tick to invalidate pending view loader")
	}
	if !tickable.OnShimmerTick() {
		t.Fatal("expected consecutive shimmer ticks to keep invalidating pending view loader")
	}
}
