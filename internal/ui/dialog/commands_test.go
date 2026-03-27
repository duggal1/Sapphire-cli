package dialog

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	apppkg "github.com/duggal1/Sapphire-cli/internal/app"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/csync"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func testCommandsCommon(t *testing.T) *common.Common {
	t.Helper()

	sty := styles.DefaultStyles(false)
	cfg := &config.Config{
		Options:   &config.Options{},
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	a := &apppkg.App{}
	field := reflect.ValueOf(a).Elem().FieldByName("config")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(cfg))

	return &common.Common{
		App:    a,
		Styles: &sty,
	}
}

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

func TestDefaultCommandsIncludeExtendedSkills(t *testing.T) {
	t.Parallel()

	cmds := (&Commands{
		com: testCommandsCommon(t),
	}).defaultCommands()

	for _, item := range cmds {
		if item.ID() != "extended_skills" {
			continue
		}
		if item.title != "Extended Skills" {
			t.Fatalf("expected extended skills label, got %q", item.title)
		}
		if _, ok := item.Action().(ActionOpenSkillsMarketplace); !ok {
			t.Fatalf("expected ActionOpenSkillsMarketplace, got %T", item.Action())
		}
		return
	}

	t.Fatal("missing extended skills command")
}
