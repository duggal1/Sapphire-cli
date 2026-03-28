package agent

import (
	"testing"

	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIndexCodebaseToolRegistered(t *testing.T) {
	cfg := &config.Config{
		Options: &config.Options{
			DisabledTools: []string{},
		},
	}

	cfg.SetupAgents()
	coderAgent := cfg.Agents[config.AgentCoder]
	taskAgent := cfg.Agents[config.AgentTask]

	require.Contains(t, coderAgent.AllowedTools, agenttools.IndexCodebaseToolName)
	require.Contains(t, taskAgent.AllowedTools, agenttools.IndexCodebaseToolName)
}
