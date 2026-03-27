package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestSapphireAPIKeyInputSubmitReturnsSaveAction(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	dlg := NewSapphireAPIKeyInput(&common.Common{
		Styles: &sty,
	})
	dlg.input.SetValue("  secret-key  ")

	action := dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	save, ok := action.(ActionSaveSapphireAPIKey)
	if !ok {
		t.Fatalf("expected ActionSaveSapphireAPIKey, got %T", action)
	}
	if save.APIKey != "secret-key" {
		t.Fatalf("expected trimmed API key, got %q", save.APIKey)
	}
}
