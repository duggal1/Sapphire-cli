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
				Label:     "Sub-agent 1",
				Status:    "running",
				Task:      "Shard 1 (internal/agent, internal/memory)",
				Scope:     "internal/agent, internal/memory",
				FileCount: 1204,
			},
			{
				Label:     "Sub-agent 2",
				Status:    "completed",
				Task:      "Shard 2 (internal/ui, internal/cmd)",
				Scope:     "internal/ui, internal/cmd",
				FileCount: 998,
			},
		},
	}, 0)

	rendered := ansi.Strip(item.Render(120))
	for _, expected := range []string{
		"AI sub-agents",
		"Sub-agent 1 · shard 1 · internal/agent, internal/memory · running",
		"Sub-agent 2 · shard 2 · internal/ui, internal/cmd · complete",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected rendered indexing tree to contain %q, got %q", expected, rendered)
		}
	}
	if !strings.Contains(rendered, "├") && !strings.Contains(rendered, "└") {
		t.Fatalf("expected rendered indexing tree branches, got %q", rendered)
	}
}

func TestIndexingMessageRendersImmediateSuccessState(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewIndexingMessageItem(&sty, codeindex.Progress{
		Workspace:       "/tmp/repo",
		Phase:           "ready",
		Message:         "Indexing complete",
		Finished:        true,
		FilesDiscovered: 6187,
		FilesProcessed:  6187,
		FilesIndexed:    6187,
		Percent:         1,
		StartedAt:       time.Now().Add(-12 * time.Second),
		UpdatedAt:       time.Now(),
		SemanticAgents: []codeindex.SemanticAgentProgress{
			{Label: "Sub-agent 1", Status: "running", Task: "Shard 1 (internal/agent)", Scope: "internal/agent"},
		},
	}, 0)

	rendered := ansi.Strip(item.Render(120))
	if !strings.Contains(rendered, "✓ Indexing complete (12s)") {
		t.Fatalf("expected success title with elapsed time, got %q", rendered)
	}
	if strings.Contains(rendered, "AI sub-agents") {
		t.Fatalf("did not expect shard tree after completion, got %q", rendered)
	}
	if strings.Contains(rendered, "100%") {
		t.Fatalf("did not expect active progress row after completion, got %q", rendered)
	}
}

func TestRenderIndexingElapsedUsesLiveTimeWhileActive(t *testing.T) {
	t.Parallel()

	started := time.Now().Add(-5 * time.Second)
	got := renderIndexingElapsed(codeindex.Progress{
		Active:    true,
		StartedAt: started,
		UpdatedAt: started.Add(1 * time.Second),
	})
	if got == "1s" || got == "" {
		t.Fatalf("expected active elapsed time to use wall clock, got %q", got)
	}
}
