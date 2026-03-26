package dialog

import (
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestModeCommandsIncludeEveryAvailableMode(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	cmds := (&Commands{
		com: &common.Common{
			Styles: &sty,
		},
	}).modeCommands(planmode.ReviewMode)

	if len(cmds) != len(planmode.AvailableModes()) {
		t.Fatalf("expected %d mode commands, got %d", len(planmode.AvailableModes()), len(cmds))
	}

	seen := make(map[planmode.SessionMode]bool, len(cmds))
	for _, item := range cmds {
		action, ok := item.Action().(ActionSelectMode)
		if !ok {
			t.Fatalf("expected ActionSelectMode, got %T", item.Action())
		}
		seen[action.Mode] = true
	}

	for _, mode := range planmode.AvailableModes() {
		if !seen[mode.Mode] {
			t.Fatalf("expected command item for mode %s", mode.Mode)
		}
	}
}

func TestModeCommandsMarkCurrentMode(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	cmds := (&Commands{
		com: &common.Common{
			Styles: &sty,
		},
	}).modeCommands(planmode.PlanMode)

	for _, item := range cmds {
		action, ok := item.Action().(ActionSelectMode)
		if !ok {
			t.Fatalf("expected ActionSelectMode, got %T", item.Action())
		}
		if action.Mode == planmode.PlanMode {
			if item.title != "Plan Mode (Current)" {
				t.Fatalf("expected current mode label, got %q", item.title)
			}
			return
		}
	}

	t.Fatal("missing current mode command")
}
