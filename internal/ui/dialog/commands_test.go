package dialog

import (
	"reflect"
	"testing"
	"unsafe"

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

func TestDefaultCommandsIncludeIndexCodebase(t *testing.T) {
	t.Parallel()

	cmds := (&Commands{
		com: testCommandsCommon(t),
	}).defaultCommands()

	for _, item := range cmds {
		if item.ID() != "index_codebase" {
			continue
		}
		if item.title != "Index Codebase" {
			t.Fatalf("expected index codebase label, got %q", item.title)
		}
		if _, ok := item.Action().(ActionIndexCodebase); !ok {
			t.Fatalf("expected ActionIndexCodebase, got %T", item.Action())
		}
		return
	}

	t.Fatal("missing index codebase command")
}
