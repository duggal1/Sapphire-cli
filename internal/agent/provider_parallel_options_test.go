package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	openai "charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
	"charm.land/fantasy/providers/vercel"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGetProviderOptionsEnablesParallelToolCallsForOpenAI(t *testing.T) {
	t.Parallel()

	c := &coordinator{}
	opts := c.getProviderOptions(
		Model{CatwalkCfg: catwalk.Model{ID: "gpt-4o"}},
		config.ProviderConfig{Type: catwalk.Type(openai.Name)},
		false,
	)

	switch parsed := opts[openai.Name].(type) {
	case *openai.ProviderOptions:
		require.NotNil(t, parsed.ParallelToolCalls)
		require.True(t, *parsed.ParallelToolCalls)
	case *openai.ResponsesProviderOptions:
		require.NotNil(t, parsed.ParallelToolCalls)
		require.True(t, *parsed.ParallelToolCalls)
	default:
		require.Failf(t, "unexpected openai provider options type", "%T", opts[openai.Name])
	}
}

func TestGetProviderOptionsEnablesParallelToolCallsForOpenRouter(t *testing.T) {
	t.Parallel()

	c := &coordinator{}
	opts := c.getProviderOptions(
		Model{CatwalkCfg: catwalk.Model{ID: "openrouter-test"}},
		config.ProviderConfig{Type: catwalk.Type(openrouter.Name)},
		false,
	)

	parsed, ok := opts[openrouter.Name].(*openrouter.ProviderOptions)
	require.True(t, ok)
	require.NotNil(t, parsed.ParallelToolCalls)
	require.True(t, *parsed.ParallelToolCalls)
}

func TestGetProviderOptionsEnablesParallelToolCallsForVercel(t *testing.T) {
	t.Parallel()

	c := &coordinator{}
	opts := c.getProviderOptions(
		Model{CatwalkCfg: catwalk.Model{ID: "vercel-test"}},
		config.ProviderConfig{Type: catwalk.Type(vercel.Name)},
		false,
	)

	parsed, ok := opts[vercel.Name].(*vercel.ProviderOptions)
	require.True(t, ok)
	require.NotNil(t, parsed.ParallelToolCalls)
	require.True(t, *parsed.ParallelToolCalls)
}
