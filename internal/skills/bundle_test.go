package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureProjectSkillsPrunesLegacyBundledCopies(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	projectRoot := ProjectSkillsDir(dataDir)
	require.NoError(t, os.MkdirAll(projectRoot, 0o755))

	embeddedBackend, err := bundledSkills.ReadFile("bundled/backend/SKILL.md")
	require.NoError(t, err)
	embeddedBackendDir := filepath.Join(projectRoot, "backend")
	require.NoError(t, os.MkdirAll(embeddedBackendDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(embeddedBackendDir, SkillFileName), embeddedBackend, 0o644))

	customDir := filepath.Join(projectRoot, "autolearn-custom")
	require.NoError(t, os.MkdirAll(customDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(customDir, SkillFileName), []byte(`---
name: autolearn-custom
description: Custom learned skill
---
Use the learned custom route.`), 0o644))

	require.NoError(t, EnsureProjectSkills(dataDir))

	_, err = os.Stat(filepath.Join(projectRoot, "backend"))
	require.ErrorIs(t, err, os.ErrNotExist)
	require.FileExists(t, filepath.Join(customDir, SkillFileName))
}

func TestDiscoverBundledSkillsToleratesEmbeddedFormats(t *testing.T) {
	t.Parallel()

	discovered := DiscoverBundledSkills()
	require.NotEmpty(t, discovered)

	byPath := make(map[string]*Skill, len(discovered))
	for _, skill := range discovered {
		byPath[skill.SkillFilePath] = skill
		require.True(t, skill.IsInternal)
		require.NotEmpty(t, skill.Name)
		require.NotEmpty(t, skill.Description)
		require.NotEmpty(t, skill.Instructions)
	}

	require.Contains(t, byPath, "bundled/sequential-thinking/SKILL.md")
	require.Contains(t, byPath["bundled/sequential-thinking/SKILL.md"].Instructions, "PROTOCOL")

	require.Contains(t, byPath, "bundled/backend/SKILL.md")
	require.Equal(t, "elite-backend-engineering", byPath["bundled/backend/SKILL.md"].Name)

	slackSkill, err := LoadBundledSkill("slack")
	require.NoError(t, err)
	require.Equal(t, "slack", slackSkill.Name)
	require.Contains(t, slackSkill.Instructions, "Slack Actions")
}
