package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/csync"
	"github.com/stretchr/testify/require"
)

func TestResolveSubAgentModelOverrideConcreteTaskUsesMedium(t *testing.T) {
	t.Parallel()

	coord := testReasoningCoordinator(config.SelectedModel{
		Provider: "openai",
		Model:    "gpt-5.4",
	}, map[string]config.ProviderConfig{
		"openai": {
			ID:   "openai",
			Type: "openai",
			Models: []catwalk.Model{
				{
					ID:                     "gpt-5.4",
					CanReason:              true,
					ReasoningLevels:        []string{"low", "medium", "high", "xhigh"},
					DefaultReasoningEffort: "medium",
				},
			},
		},
	})

	prompt := "Implement the auth retry fix in internal/agent/auth.go and update the related tests."
	override, err := coord.resolveSubAgentModelOverride("", "", prompt, evaluateSubAgentLaunch(prompt))
	require.NoError(t, err)
	require.NotNil(t, override)
	require.Equal(t, "medium", override.ReasoningEffort)
	require.Empty(t, override.ProviderOptions)
}

func TestResolveSubAgentModelOverrideReasoningHeavyTaskUsesHigh(t *testing.T) {
	t.Parallel()

	coord := testReasoningCoordinator(config.SelectedModel{
		Provider: "openai",
		Model:    "gpt-5.4",
	}, map[string]config.ProviderConfig{
		"openai": {
			ID:   "openai",
			Type: "openai",
			Models: []catwalk.Model{
				{
					ID:                     "gpt-5.4",
					CanReason:              true,
					ReasoningLevels:        []string{"low", "medium", "high", "xhigh"},
					DefaultReasoningEffort: "medium",
				},
			},
		},
	})

	prompt := "Analyze the root cause of the sub-agent orchestration regression and compare the safest fix options."
	override, err := coord.resolveSubAgentModelOverride("", "", prompt, evaluateSubAgentLaunch(prompt))
	require.NoError(t, err)
	require.NotNil(t, override)
	require.Equal(t, "high", override.ReasoningEffort)
}

func TestResolveSubAgentModelOverrideGemini25UsesHighBudgetThinking(t *testing.T) {
	t.Parallel()

	coord := testReasoningCoordinator(config.SelectedModel{
		Provider: "google",
		Model:    "gemini-2.5-pro",
	}, map[string]config.ProviderConfig{
		"google": {
			ID:   "google",
			Type: "google",
			Models: []catwalk.Model{
				{
					ID:                     "gemini-2.5-pro",
					CanReason:              true,
					DefaultReasoningEffort: "low",
				},
			},
		},
	})

	prompt := "Analyze the architecture hotspots in the orchestration layer and explain the risk tradeoffs."
	override, err := coord.resolveSubAgentModelOverride("", "", prompt, evaluateSubAgentLaunch(prompt))
	require.NoError(t, err)
	require.NotNil(t, override)
	require.Empty(t, override.ReasoningEffort)

	thinkingConfig, ok := override.ProviderOptions["thinking_config"].(map[string]any)
	require.True(t, ok)
	require.EqualValues(t, gemini25MaxThinkingBudget, thinkingConfig["thinking_budget"])
	require.Equal(t, true, thinkingConfig["include_thoughts"])
}

func TestResolveSubAgentModelOverrideExplicitReasoningWins(t *testing.T) {
	t.Parallel()

	coord := testReasoningCoordinator(config.SelectedModel{
		Provider: "openai",
		Model:    "gpt-5.4-pro",
	}, map[string]config.ProviderConfig{
		"openai": {
			ID:   "openai",
			Type: "openai",
			Models: []catwalk.Model{
				{
					ID:                     "gpt-5.4-pro",
					CanReason:              true,
					ReasoningLevels:        []string{"medium", "high", "xhigh"},
					DefaultReasoningEffort: "medium",
				},
			},
		},
	})

	prompt := "Analyze the coordination failure and propose the cleanest fix."
	override, err := coord.resolveSubAgentModelOverride("", "medium", prompt, evaluateSubAgentLaunch(prompt))
	require.NoError(t, err)
	require.NotNil(t, override)
	require.Equal(t, "medium", override.ReasoningEffort)
}

func TestApplyAgentModelOverrideResetsProviderSpecificReasoningOnModelChange(t *testing.T) {
	t.Parallel()

	base := config.SelectedModel{
		Provider:        "openai",
		Model:           "gpt-5.4",
		ReasoningEffort: "low",
		Think:           true,
		ProviderOptions: map[string]any{
			"thinking_config": map[string]any{"thinking_budget": int64(1024)},
		},
	}

	updated := applyAgentModelOverride(base, &agentModelOverride{
		Provider: "google",
		Model:    "gemini-2.5-pro",
		ProviderOptions: map[string]any{
			"thinking_config": map[string]any{"thinking_budget": gemini25MaxThinkingBudget},
		},
	})

	require.Equal(t, "google", updated.Provider)
	require.Equal(t, "gemini-2.5-pro", updated.Model)
	require.Empty(t, updated.ReasoningEffort)
	require.False(t, updated.Think)
	require.EqualValues(t, gemini25MaxThinkingBudget, updated.ProviderOptions["thinking_config"].(map[string]any)["thinking_budget"])
}

func testReasoningCoordinator(large config.SelectedModel, providers map[string]config.ProviderConfig) *coordinator {
	cfg := &config.Config{
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: large,
			config.SelectedModelTypeSmall: {
				Provider: large.Provider,
				Model:    large.Model,
			},
		},
		Providers: csync.NewMapFrom(providers),
	}
	return &coordinator{cfg: cfg}
}
