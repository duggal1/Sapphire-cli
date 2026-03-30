package logo

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestRenderFallsBackToNarrowLogoWhenFullLogoWouldClip(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	rendered := ansi.Strip(Render(&sty, "v0.1.0", false, Opts{
		FieldColor:   sty.LogoFieldColor,
		TitleColorA:  sty.LogoTitleColorA,
		TitleColorB:  sty.LogoTitleColorB,
		CharmColor:   sty.LogoCharmColor,
		VersionColor: sty.LogoVersionColor,
		Width:        64,
	}))

	if !strings.Contains(rendered, "v0.1.0") {
		t.Fatalf("expected narrow fallback to include version row, got %q", rendered)
	}
	if !strings.Contains(rendered, "╱") {
		t.Fatalf("expected narrow fallback to include diagonal field, got %q", rendered)
	}
	for _, line := range strings.Split(rendered, "\n") {
		if width := lipgloss.Width(line); width > 64 {
			t.Fatalf("expected line width <= 64, got %d for %q", width, line)
		}
	}
}
