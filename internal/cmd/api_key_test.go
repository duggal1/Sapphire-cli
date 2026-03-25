package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyCmd_DefaultsToOpenRouter(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "true")

	cmd := newAPIKeyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(bytes.NewReader(nil))

	err := cmd.RunE(cmd, []string{"sk-or-test-key"})
	require.NoError(t, err)

	data, err := os.ReadFile(config.GlobalConfigData())
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	providers, ok := decoded["providers"].(map[string]any)
	require.True(t, ok)
	openrouter, ok := providers["openrouter"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sk-or-test-key", openrouter["api_key"])
	require.Contains(t, out.String(), "OpenRouter API key updated successfully.")
}

func TestAPIKeyCmd_ExplicitProviderFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "true")

	cmd := newAPIKeyCmd()
	require.NoError(t, cmd.Flags().Set("provider", "openrouter"))
	require.NoError(t, cmd.Flags().Set("api-key", "sk-or-flag-key"))

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(bytes.NewReader(nil))

	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)

	data, err := os.ReadFile(config.GlobalConfigData())
	require.NoError(t, err)

	require.Contains(t, string(data), `"openrouter"`)
	require.Contains(t, string(data), `"api_key":"sk-or-flag-key"`)
	require.Contains(t, filepath.Base(config.GlobalConfigData()), "sapphire.json")
}
