package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestListSearchAndLoadSkillTools(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	skillsDir := filepath.Join(workingDir, "skills", "backend")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(`---
name: backend
description: Backend implementation skill
---
Use strict backend workflows.`), 0o644))

	cfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)
	cfg.Options.SkillsPaths = []string{filepath.Join(workingDir, "skills")}

	coord := &coordinator{cfg: cfg}

	listTool, err := coord.listSkillsTool(t.Context())
	require.NoError(t, err)
	listResp := runAgentTool(t, listTool, "list_skills", struct{}{})
	require.Contains(t, listResp.Content, "backend")
	require.Contains(t, listResp.Content, "Backend implementation skill")

	searchTool, err := coord.searchSkillsTool(t.Context())
	require.NoError(t, err)
	searchResp := runAgentTool(t, searchTool, "search_skills", SearchSkillsParams{Query: "backend api server", Limit: 5})
	require.Contains(t, searchResp.Content, "backend")
	require.Contains(t, searchResp.Content, "Matching Skills")

	loadTool, err := coord.loadSkillTool(t.Context())
	require.NoError(t, err)
	loadResp := runAgentTool(t, loadTool, "load_skill", LoadSkillParams{Name: "backend"})
	require.Contains(t, loadResp.Content, "Successfully loaded project skill")
	require.Contains(t, loadResp.Content, "Use strict backend workflows.")
}

func runAgentTool[T any](t *testing.T, tool fantasy.AgentTool, name string, params T) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		ID:    name + "-1",
		Name:  name,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}
