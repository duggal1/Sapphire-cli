package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSubAgentsCmdShowsCurrentLimit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "true")

	cmd := newSubAgentsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(bytes.NewReader(nil))

	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)
	require.Contains(t, out.String(), "Sub-agent limit: 6")
}

func TestSubAgentsCmdPersistsLimit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "true")

	cmd := newSubAgentsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(bytes.NewReader(nil))

	err := cmd.RunE(cmd, []string{"20"})
	require.NoError(t, err)

	data, err := os.ReadFile(config.GlobalConfigData())
	require.NoError(t, err)
	require.Contains(t, string(data), `"agent_max_threads":20`)
	require.Contains(t, out.String(), "Saved sub-agent limit: 20")
}

func TestSubAgentsCmdRejectsNonPositiveLimit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "true")

	cmd := newSubAgentsCmd()
	err := cmd.RunE(cmd, []string{"0"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "greater than zero")
}
