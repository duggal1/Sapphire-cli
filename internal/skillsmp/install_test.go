package skillsmp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstallerWritesSkillAndPlugin(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	installer := NewInstaller(dataDir)
	skill := Skill{
		Name:        "security-audit",
		Owner:       "trail-of-bits",
		Repo:        "security-audit",
		FilePath:    "SKILL.md",
		GithubURL:   "https://github.com/trail-of-bits/security-audit/blob/main/SKILL.md",
		Description: "Security audit workflow",
	}
	body := []byte(`---
name: security-audit
description: Security audit workflow
---
Audit the codebase for security issues.
`)

	require.NoError(t, installer.Install(skill, body))

	skillPath := filepath.Join(dataDir, "skills", skill.Name, "SKILL.md")
	pluginPath := filepath.Join(dataDir, "plugins", skill.Name, "SKILL.md")
	manifestPath := filepath.Join(dataDir, "plugins", skill.Name, "plugin.json")

	_, err := os.Stat(skillPath)
	require.NoError(t, err)
	_, err = os.Stat(pluginPath)
	require.NoError(t, err)
	manifest, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.Contains(t, string(manifest), `"name": "security-audit"`)
	require.Contains(t, string(manifest), `"path": "./SKILL.md"`)
	require.Equal(t, body, mustRead(t, skillPath))
	require.Equal(t, body, mustRead(t, pluginPath))
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
