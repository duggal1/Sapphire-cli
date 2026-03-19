package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestShortenSubAgentID(t *testing.T) {
	t.Parallel()

	if got := shortenSubAgentID("agent-d5655f40-f83b-46af-8b5d-2ff2becf4b78"); got != "d5655f40" {
		t.Fatalf("expected shortened agent id, got %q", got)
	}
	if got := shortenSubAgentID("e29e755d-11da-4d84-99d0-7031eae4f338"); got != "e29e755d" {
		t.Fatalf("expected shortened submission id, got %q", got)
	}
}

func TestFormatSubAgentElapsedAt(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 3, 20, 1, 0, 0, 0, time.UTC)
	now := start.Add(79 * time.Second)
	if got := formatSubAgentElapsedAt(start, now); got != "1m 19s" {
		t.Fatalf("expected 1m 19s, got %q", got)
	}
}

func TestRenderSubAgentWaitBodyUsesFriendlySummaryLine(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	rendered := ansi.Strip(renderSubAgentWaitBody(&sty, subAgentWaitResult{
		Agents: []subAgentStatusEntry{
			{
				ID:        "agent-d5655f40-f83b-46af-8b5d-2ff2becf4b78",
				Status:    "running",
				StartedAt: time.Time{},
			},
		},
	}, 120))

	if !strings.Contains(rendered, "d5655f40") {
		t.Fatalf("expected shortened id, got %q", rendered)
	}
	if strings.Contains(rendered, "agent-d5655f40-f83b-46af-8b5d-2ff2becf4b78") {
		t.Fatalf("expected full agent id to be removed, got %q", rendered)
	}
	if !strings.Contains(rendered, "worktree/auto") {
		t.Fatalf("expected worktree summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "running") {
		t.Fatalf("expected running status, got %q", rendered)
	}
}
