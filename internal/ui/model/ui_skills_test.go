package model

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/duggal1/Sapphire-cli/internal/app"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/csync"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/dialog"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestOpenSapphireAPIKeyDialogOpensDialog(t *testing.T) {
	t.Setenv("SAPPHIRE_API_KEY", "")

	sty := styles.DefaultStyles(false)
	cfg := &config.Config{
		Options:   &config.Options{},
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	a := &app.App{}
	field := reflect.ValueOf(a).Elem().FieldByName("config")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(cfg))

	m := &UI{
		com: &common.Common{
			App:    a,
			Styles: &sty,
		},
		dialog: dialog.NewOverlay(),
	}

	if cmd := m.openSapphireAPIKeyDialog(); cmd != nil {
		t.Fatalf("expected no command from opening the dialog, got %T", cmd)
	}
	if !m.dialog.ContainsDialog(dialog.SapphireAPIKeyInputID) {
		t.Fatal("expected Sapphire API key dialog to be opened")
	}
}
