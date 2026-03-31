package model

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	tea "charm.land/bubbletea/v2"
	"github.com/duggal1/Sapphire-cli/internal/app"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
)

func TestNewInitializeStateFocusesPromptAndHandlesEnter(t *testing.T) {
	t.Parallel()

	workingDir, err := os.MkdirTemp("", "sapphire-ui-working-*")
	if err != nil {
		t.Fatalf("create working dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workingDir) })
	if err := os.WriteFile(filepath.Join(workingDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write visible file: %v", err)
	}

	dataDir, err := os.MkdirTemp("", "sapphire-ui-data-*")
	if err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })

	cfg, err := config.Load(workingDir, dataDir, false)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Providers.Set("test", config.ProviderConfig{ID: "test"})

	a := &app.App{
		Permissions: permission.NewPermissionService(workingDir, true, nil),
	}
	field := reflect.ValueOf(a).Elem().FieldByName("config")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(cfg))

	ui := New(common.DefaultCommon(a))

	if ui.state != uiInitialize {
		t.Fatalf("expected initialize state, got %v", ui.state)
	}
	if ui.focus != uiFocusMain {
		t.Fatalf("expected initialize prompt to own focus, got %v", ui.focus)
	}
	if cmd := ui.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd == nil {
		t.Fatal("expected enter on initialize screen to trigger an action")
	}
}
