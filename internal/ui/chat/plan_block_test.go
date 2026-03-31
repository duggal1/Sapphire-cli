package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestRenderPlanBlockUsesMarkdownRendering(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	rendered := ansi.Strip(RenderPlanBlock(&sty, "## Summary\n- first item\n### Key Changes\n[Spec](https://example.com)", 100))

	if !strings.Contains(rendered, "Plan") {
		t.Fatalf("expected plan title, got %q", rendered)
	}
	if strings.Contains(rendered, "## Summary") || strings.Contains(rendered, "### Key Changes") {
		t.Fatalf("expected markdown headings to be rendered structurally, got %q", rendered)
	}
	if !strings.Contains(rendered, "Summary") || !strings.Contains(rendered, "Key Changes") {
		t.Fatalf("expected markdown heading text, got %q", rendered)
	}
	if !strings.Contains(rendered, "first item") || !strings.Contains(rendered, "Spec") {
		t.Fatalf("expected markdown list and link text, got %q", rendered)
	}
}
