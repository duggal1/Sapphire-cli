package chat

import (
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

func TestAssistantThinkingRenderUsesThinkingBoxPresentation(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewAssistantMessageItem(&sty, &message.Message{
		ID:   "assistant-thinking",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{
				Thinking: "## Analyzing\n\n- first\n- second",
			},
		},
	})

	rendered := ansi.Strip(item.Render(100))
	if !strings.Contains(rendered, "Thinking") {
		t.Fatalf("expected thinking label, got %q", rendered)
	}
	if !strings.Contains(rendered, "Analyzing") || !strings.Contains(rendered, "first") {
		t.Fatalf("expected rendered thinking markdown, got %q", rendered)
	}
}
