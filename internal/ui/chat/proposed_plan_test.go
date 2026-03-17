package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

func TestExtractMessageItemsAddsDedicatedProposedPlanItem(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	msg := &message.Message{
		ID:   "assistant-1",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "<proposed_plan>\n## Summary\n- First\n</proposed_plan>"},
		},
	}

	items := ExtractMessageItems(&sty, msg, nil)
	if len(items) != 1 {
		t.Fatalf("expected 1 rendered item, got %d", len(items))
	}
	if items[0].ID() != ProposedPlanID(msg.ID) {
		t.Fatalf("expected proposed plan item, got %q", items[0].ID())
	}
}

func TestProposedPlanItemRendersWithoutRawTags(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewProposedPlanItem(&sty, "assistant-1", "## Summary\n- First step")
	rendered := ansi.Strip(item.Render(100))
	requireContainsAll(t, rendered, "Proposed Plan", "Summary", "First step")
	for _, s := range []string{"<proposed_plan>", "</proposed_plan>"} {
		if strings.Contains(rendered, s) {
			t.Fatalf("expected rendered output to omit %q, got %q", s, rendered)
		}
	}
}
