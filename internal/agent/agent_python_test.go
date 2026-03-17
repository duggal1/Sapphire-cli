package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy/providers/google"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestIsGeminiCodeExecutionModel(t *testing.T) {
	t.Parallel()

	assert.True(t, isGeminiCodeExecutionModel(Model{
		CatwalkCfg: catwalk.Model{ID: "gemini-3-flash"},
		ModelCfg: config.SelectedModel{
			Provider: google.Name,
			Model:    "gemini-3-flash",
		},
	}))

	assert.True(t, isGeminiCodeExecutionModel(Model{
		CatwalkCfg: catwalk.Model{},
		ModelCfg: config.SelectedModel{
			Provider: "google-vertex",
			Model:    "gemini-3-pro-preview",
		},
	}))

	assert.False(t, isGeminiCodeExecutionModel(Model{
		CatwalkCfg: catwalk.Model{ID: "gemini-3-flash"},
		ModelCfg: config.SelectedModel{
			Provider: "openrouter",
			Model:    "google/gemini-3-flash",
		},
	}))

	assert.False(t, isGeminiCodeExecutionModel(Model{
		CatwalkCfg: catwalk.Model{ID: "claude-sonnet-4-5"},
		ModelCfg: config.SelectedModel{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-5",
		},
	}))
}
