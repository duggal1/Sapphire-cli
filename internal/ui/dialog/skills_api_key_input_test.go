package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestSkillsMPAPIKeyInputSubmitReturnsSaveAction(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	dlg := NewSkillsMPAPIKeyInput(&common.Common{
		Styles: &sty,
	})
	dlg.input.SetValue("  secret-key  ")

	action := dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	save, ok := action.(ActionSaveSkillsMPAPIKey)
	if !ok {
		t.Fatalf("expected ActionSaveSkillsMPAPIKey, got %T", action)
	}
	if save.APIKey != "secret-key" {
		t.Fatalf("expected trimmed API key, got %q", save.APIKey)
	}
}
