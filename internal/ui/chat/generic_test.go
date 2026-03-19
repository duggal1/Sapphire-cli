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

func TestOrchestrateWorktreesRendersStructuredTree(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewGenericToolMessageItem(&sty, message.ToolCall{
		ID:       "orchestrate-1",
		Name:     "orchestrate_worktrees",
		Input:    `{"tasks":[{"agent":"coder","branch":"agent/backend-metrics/feat-health-stats","definition_of_done":"implement health metrics","worktree_path":".sapphire/worktrees/agent/42/backend-metrics","task":"Build backend metrics"}],"test_command":"go test ./...","integration_branch":"agent/integration/backend-metrics"}`,
		Finished: false,
	}, nil, false)

	rendered := item.Render(120)
	if !strings.Contains(rendered, "Worktree Plan") || !strings.Contains(rendered, "Tasks") {
		t.Fatalf("expected structured tree root, got %q", rendered)
	}
	if !strings.Contains(rendered, "Branch: agent/backend-metrics/feat-health-stats") {
		t.Fatalf("expected branch metadata, got %q", rendered)
	}
	if !strings.Contains(rendered, "Worktree: .sapphire/worktrees/agent/42/backend-metrics") {
		t.Fatalf("expected worktree metadata, got %q", rendered)
	}
}
