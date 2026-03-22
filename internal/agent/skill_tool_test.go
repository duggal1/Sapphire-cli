package agent

import (
	"context"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/skills"
	"github.com/stretchr/testify/require"
)

// TestSkillToolsRegistered verifies that load_skill, list_skills, and search_skills tools are registered.
func TestSkillToolsRegistered(t *testing.T) {
	cfg := &config.Config{
		Options: &config.Options{
			DisabledTools: []string{},
			SkillsPaths:   []string{}, // Empty is fine for this test
		},
	}

	cfg.SetupAgents()
	coderAgent := cfg.Agents[config.AgentCoder]

	// Verify skill tools are in allowed tools
	require.Contains(t, coderAgent.AllowedTools, "load_skill", "load_skill should be in allowed tools")
	require.Contains(t, coderAgent.AllowedTools, "list_skills", "list_skills should be in allowed tools")
	require.Contains(t, coderAgent.AllowedTools, "search_skills", "search_skills should be in allowed tools")

	// Verify task agent also has skill tools (but not agent tool)
	taskAgent := cfg.Agents[config.AgentTask]
	require.Contains(t, taskAgent.AllowedTools, "load_skill", "load_skill should be in task agent tools")
	require.Contains(t, taskAgent.AllowedTools, "list_skills", "list_skills should be in task agent tools")
	require.Contains(t, taskAgent.AllowedTools, "search_skills", "search_skills should be in task agent tools")
	require.NotContains(t, taskAgent.AllowedTools, "agent", "task agent should not have agent tool")
}

// TestSkillToolExecution verifies skill tools can be created and executed.
func TestSkillToolExecution(t *testing.T) {
	ctx := context.Background()

	// Create a minimal coordinator for testing
	// Note: This is a basic smoke test - full integration tests require more setup
	c := &coordinator{
		cfg: &config.Config{
			Options: &config.Options{
				SkillsPaths: []string{},
			},
		},
		discoveredSkills:          []*skills.Skill{}, // Empty skills list
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
	}

	// Test list_skills tool creation
	listTool, err := c.listSkillsTool(ctx)
	require.NoError(t, err)
	require.NotNil(t, listTool)
	require.Equal(t, "list_skills", listTool.Info().Name)

	searchTool, err := c.searchSkillsTool(ctx)
	require.NoError(t, err)
	require.NotNil(t, searchTool)
	require.Equal(t, "search_skills", searchTool.Info().Name)

	// Test load_skill tool creation
	loadTool, err := c.loadSkillTool(ctx)
	require.NoError(t, err)
	require.NotNil(t, loadTool)
	require.Equal(t, "load_skill", loadTool.Info().Name)
}
