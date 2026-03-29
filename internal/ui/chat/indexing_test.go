package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/codeindex"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestIndexingMessageRendersSemanticAgentTree(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewIndexingMessageItem(&sty, codeindex.Progress{
		Workspace:       "/tmp/repo",
		Phase:           "semantic_graph",
		Message:         "AI codebase graph shards complete 1/3; parallel shard sub-agents still running",
		Active:          true,
		FilesDiscovered: 6187,
		FilesProcessed:  6187,
		FilesIndexed:    6187,
		Percent:         0.96,
		StartedAt:       time.Now().Add(-2 * time.Minute),
		UpdatedAt:       time.Now(),
		SemanticAgents: []codeindex.SemanticAgentProgress{
			{
				Label:     "Shard 1 (internal/agent, internal/memory)",
				Status:    "running",
				Task:      "Read assigned files and write shard graph",
				Scope:     "internal/agent, internal/memory",
				FileCount: 1204,
			},
			{
				Label:     "Shard 2 (internal/ui, internal/cmd)",
				Status:    "completed",
				Task:      "Read assigned files and write shard graph",
				Scope:     "internal/ui, internal/cmd",
				FileCount: 998,
			},
		},
	}, 0)

	rendered := ansi.Strip(item.Render(120))
	for _, expected := range []string{
		"AI sub-agents",
		"Shard 1 (internal/agent, internal/memory) · running · 1204 files",
		"internal/agent, internal/memory · Read assigned files and write shard graph",
		"Shard 2 (internal/ui, internal/cmd) · completed · 998 files",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected rendered indexing tree to contain %q, got %q", expected, rendered)
		}
	}
	if !strings.Contains(rendered, "├") && !strings.Contains(rendered, "└") {
		t.Fatalf("expected rendered indexing tree branches, got %q", rendered)
	}
}
