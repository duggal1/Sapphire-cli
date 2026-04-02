package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func resetProviderState() {
	providerOnce = sync.Once{}
	providerList = nil
	providerErr = nil
	catwalkSyncer = &catwalkSync{}
	hyperSyncer = &hyperSync{}
}

func TestProviders_Integration_AutoUpdateDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Use a test-specific instance to avoid global state interference.
	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	originalCatwalSyncer := catwalkSyncer
	originalHyperSyncer := hyperSyncer
	defer func() {
		catwalkSyncer = originalCatwalSyncer
		hyperSyncer = originalHyperSyncer
	}()

	catwalkSyncer = testCatwalkSyncer
	hyperSyncer = testHyperSyncer

	resetProviderState()
	defer resetProviderState()

	cfg := &Config{
		Options: &Options{
			DisableProviderAutoUpdate: true,
		},
	}

	providers, err := Providers(cfg)
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.Greater(t, len(providers), 5, "Expected embedded providers")

	var openRouter *catwalk.Provider
	for i := range providers {
		if providers[i].ID == "openrouter" {
			openRouter = &providers[i]
			break
		}
	}
	require.NotNil(t, openRouter)

	modelIDs := make([]string, 0, len(openRouter.Models))
	for _, model := range openRouter.Models {
		modelIDs = append(modelIDs, model.ID)
	}

	require.Contains(t, modelIDs, "nvidia/nemotron-3-super-120b-a12b:free")
	require.Contains(t, modelIDs, "nvidia/nemotron-3-nano-30b-a3b:free")
	require.Contains(t, modelIDs, "nousresearch/hermes-3-llama-3.1-405b:free")
	require.Contains(t, modelIDs, "cognitivecomputations/dolphin-mistral-24b-venice-edition:free")
	require.Contains(t, modelIDs, "minimax/minimax-m2.5:free")
	require.Contains(t, modelIDs, "minimax/minimax-m2.5")
	require.Contains(t, modelIDs, "minimax/minimax-m2.5:nitro")
	require.Contains(t, modelIDs, "arcee-ai/trinity-mini:free")
	require.Contains(t, modelIDs, "arcee-ai/trinity-large-preview:free")
	require.Contains(t, modelIDs, "openai/gpt-oss-120b:free")
	require.Contains(t, modelIDs, "qwen/qwen3.6-plus-preview:free")
	require.Contains(t, modelIDs, "z-ai/glm-5")
	require.Contains(t, modelIDs, "z-ai/glm-5-turbo")

	providerIDs := make([]string, 0, len(providers))
	for _, provider := range providers {
		providerIDs = append(providerIDs, string(provider.ID))
	}
	require.Contains(t, providerIDs, "zai")
	require.Contains(t, providerIDs, "cerebras")
	require.Contains(t, providerIDs, "aihubmix")

	var miniMaxFree *catwalk.Model
	for i := range openRouter.Models {
		if openRouter.Models[i].ID == "minimax/minimax-m2.5:free" {
			miniMaxFree = &openRouter.Models[i]
			break
		}
	}
	require.NotNil(t, miniMaxFree)
	require.NotNil(t, miniMaxFree.Options.ProviderOptions)
	providerOpt, ok := miniMaxFree.Options.ProviderOptions["provider"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "deny", providerOpt["data_collection"])
}

func TestShouldAutoUpdateProviders_DisablesHeadlessRefresh(t *testing.T) {
	t.Setenv("SAPPHIRE_NON_INTERACTIVE", "1")
	cfg := &Config{
		Options: &Options{},
	}
	require.False(t, shouldAutoUpdateProviders(cfg))
}

func TestShouldAutoUpdateProviders_AllowsInteractiveRefreshWhenEnabled(t *testing.T) {
	t.Setenv("SAPPHIRE_NON_INTERACTIVE", "")
	cfg := &Config{
		Options: &Options{},
	}
	require.True(t, shouldAutoUpdateProviders(cfg))
}

func TestAugmentProviderCatalog_AddsMissingOpenRouterModelsWithoutDuplication(t *testing.T) {
	providers := []catwalk.Provider{
		{
			ID: "openrouter",
			Models: []catwalk.Model{
				{ID: "arcee-ai/trinity-large-preview:free", Name: "Arcee AI: Trinity Large Preview (free)"},
				{ID: "qwen/qwen3-coder:free", Name: "Qwen: Qwen3 Coder 480B A35B (free)"},
				{ID: "qwen/qwen3.6-plus-preview:free", Name: "Qwen: Qwen3.6 Plus Preview (free)"},
				{ID: "z-ai/glm-5", Name: "Z.ai: GLM 5"},
				{ID: "z-ai/glm-5-turbo", Name: "Z.ai: GLM 5 Turbo"},
				{ID: "nousresearch/hermes-3-llama-3.1-405b:free", Name: "Nous: Hermes 3 405B Instruct (free)"},
			},
		},
	}

	providers = augmentProviderCatalog(providers)

	modelIDs := make([]string, 0, len(providers[0].Models))
	for _, model := range providers[0].Models {
		modelIDs = append(modelIDs, model.ID)
	}

	require.Len(t, providers[0].Models, 14)
	require.Contains(t, modelIDs, "minimax/minimax-m2.5")
	require.Contains(t, modelIDs, "minimax/minimax-m2.5:nitro")
	require.Contains(t, modelIDs, "nvidia/nemotron-3-nano-30b-a3b:free")
	require.Contains(t, modelIDs, "nvidia/nemotron-3-super-120b-a12b:free")
	require.Contains(t, modelIDs, "cognitivecomputations/dolphin-mistral-24b-venice-edition:free")
	require.Contains(t, modelIDs, "minimax/minimax-m2.5:free")
	require.Contains(t, modelIDs, "arcee-ai/trinity-mini:free")
	require.Contains(t, modelIDs, "openai/gpt-oss-120b:free")
	require.Contains(t, modelIDs, "z-ai/glm-5")
	require.Contains(t, modelIDs, "z-ai/glm-5-turbo")

	trinityLargeCount := 0
	qwenCoderCount := 0
	qwen36Count := 0
	glm5Count := 0
	glm5TurboCount := 0
	hermesCount := 0
	for _, modelID := range modelIDs {
		if modelID == "arcee-ai/trinity-large-preview:free" {
			trinityLargeCount++
		}
		if modelID == "qwen/qwen3-coder:free" {
			qwenCoderCount++
		}
		if modelID == "qwen/qwen3.6-plus-preview:free" {
			qwen36Count++
		}
		if modelID == "z-ai/glm-5" {
			glm5Count++
		}
		if modelID == "z-ai/glm-5-turbo" {
			glm5TurboCount++
		}
		if modelID == "nousresearch/hermes-3-llama-3.1-405b:free" {
			hermesCount++
		}
	}
	require.Equal(t, 1, trinityLargeCount)
	require.Equal(t, 1, qwenCoderCount)
	require.Equal(t, 1, qwen36Count)
	require.Equal(t, 1, glm5Count)
	require.Equal(t, 1, glm5TurboCount)
	require.Equal(t, 1, hermesCount)
}

func TestAugmentProviderCatalog_AddsMissingProviders(t *testing.T) {
	providers := []catwalk.Provider{
		{ID: "openrouter", Name: "OpenRouter"},
	}

	providers = augmentProviderCatalog(providers)

	byID := make(map[string]catwalk.Provider, len(providers))
	for _, provider := range providers {
		byID[string(provider.ID)] = provider
	}

	aihubmixProvider, ok := byID["aihubmix"]
	require.True(t, ok)
	require.Equal(t, "https://aihubmix.com/v1", aihubmixProvider.APIEndpoint)
	require.Equal(t, "$AIHUBMIX_API_KEY", aihubmixProvider.APIKey)
	require.Equal(t, "coding-glm-5.1-free", aihubmixProvider.DefaultLargeModelID)
	require.Equal(t, "coding-glm-5-turbo-free", aihubmixProvider.DefaultSmallModelID)
	require.Len(t, aihubmixProvider.Models, 5)
	require.Equal(t, []string{
		"coding-glm-5.1-free",
		"coding-minimax-m2.7-free",
		"coding-glm-5-free",
		"coding-glm-5-turbo-free",
		"crush-glm-5.1-free",
	}, []string{
		aihubmixProvider.Models[0].ID,
		aihubmixProvider.Models[1].ID,
		aihubmixProvider.Models[2].ID,
		aihubmixProvider.Models[3].ID,
		aihubmixProvider.Models[4].ID,
	})

	zaiProvider, ok := byID["zai"]
	require.True(t, ok)
	require.Equal(t, "https://api.z.ai/api/coding/paas/v4", zaiProvider.APIEndpoint)
	require.Equal(t, "$ZAI_API_KEY", zaiProvider.APIKey)
	require.Equal(t, "glm-5", zaiProvider.DefaultLargeModelID)
	require.Equal(t, "glm-5-turbo", zaiProvider.DefaultSmallModelID)
	require.Len(t, zaiProvider.Models, 3)

	cerebrasProvider, ok := byID["cerebras"]
	require.True(t, ok)
	require.Equal(t, "https://api.cerebras.ai/v1", cerebrasProvider.APIEndpoint)
	require.Equal(t, "$CEREBRAS_API_KEY", cerebrasProvider.APIKey)
	require.Equal(t, "gpt-oss-120b", cerebrasProvider.DefaultLargeModelID)
	require.Equal(t, "qwen-3-235b-a22b-instruct-2507", cerebrasProvider.DefaultSmallModelID)
	require.Equal(t, "crush", cerebrasProvider.DefaultHeaders["X-Cerebras-3rd-Party-Integration"])
	require.Len(t, cerebrasProvider.Models, 3)
}

func TestProviders_Integration_WithMockClients(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create fresh syncers for this test.
	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	// Initialize with mock clients.
	mockCatwalkClient := &mockCatwalkClient{
		providers: []catwalk.Provider{
			{Name: "Provider1", ID: "p1"},
			{Name: "Provider2", ID: "p2"},
		},
	}
	mockHyperClient := &mockHyperClient{
		provider: catwalk.Provider{
			Name: "Hyper",
			ID:   "hyper",
			Models: []catwalk.Model{
				{ID: "hyper-1", Name: "Hyper Model"},
			},
		},
	}

	catwalkPath := tmpDir + "/crush/providers.json"
	hyperPath := tmpDir + "/crush/hyper.json"

	testCatwalkSyncer.Init(mockCatwalkClient, catwalkPath, true)
	testHyperSyncer.Init(mockHyperClient, hyperPath, true)

	// Get providers from each syncer.
	catwalkProviders, err := testCatwalkSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Len(t, catwalkProviders, 2)

	hyperProvider, err := testHyperSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Hyper", hyperProvider.Name)

	// Verify total.
	allProviders := append(catwalkProviders, hyperProvider)
	require.Len(t, allProviders, 3)
}

func TestProviders_Integration_WithCachedData(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create cache files.
	catwalkPath := tmpDir + "/crush/providers.json"
	hyperPath := tmpDir + "/crush/hyper.json"

	require.NoError(t, os.MkdirAll(tmpDir+"/crush", 0o755))

	// Write Catwalk cache.
	catwalkProviders := []catwalk.Provider{
		{Name: "Cached1", ID: "c1"},
		{Name: "Cached2", ID: "c2"},
	}
	data, err := json.Marshal(catwalkProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(catwalkPath, data, 0o644))

	// Write Hyper cache.
	hyperProvider := catwalk.Provider{
		Name: "Cached Hyper",
		ID:   "hyper",
	}
	data, err = json.Marshal(hyperProvider)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(hyperPath, data, 0o644))

	// Create fresh syncers.
	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	// Mock clients that return ErrNotModified.
	mockCatwalkClient := &mockCatwalkClient{
		err: catwalk.ErrNotModified,
	}
	mockHyperClient := &mockHyperClient{
		err: catwalk.ErrNotModified,
	}

	testCatwalkSyncer.Init(mockCatwalkClient, catwalkPath, true)
	testHyperSyncer.Init(mockHyperClient, hyperPath, true)

	// Get providers - should use cached.
	catwalkResult, err := testCatwalkSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Len(t, catwalkResult, 2)
	require.Equal(t, "Cached1", catwalkResult[0].Name)

	hyperResult, err := testHyperSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Cached Hyper", hyperResult.Name)
}

func TestProviders_Integration_CatwalkFailsHyperSucceeds(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	// Catwalk fails, Hyper succeeds.
	mockCatwalkClient := &mockCatwalkClient{
		err: catwalk.ErrNotModified, // Will use embedded.
	}
	mockHyperClient := &mockHyperClient{
		provider: catwalk.Provider{
			Name: "Hyper",
			ID:   "hyper",
			Models: []catwalk.Model{
				{ID: "hyper-1", Name: "Hyper Model"},
			},
		},
	}

	catwalkPath := tmpDir + "/crush/providers.json"
	hyperPath := tmpDir + "/crush/hyper.json"

	testCatwalkSyncer.Init(mockCatwalkClient, catwalkPath, true)
	testHyperSyncer.Init(mockHyperClient, hyperPath, true)

	catwalkResult, err := testCatwalkSyncer.Get(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, catwalkResult) // Should have embedded.

	hyperResult, err := testHyperSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Hyper", hyperResult.Name)
}

func TestProviders_Integration_BothFail(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	// Both fail.
	mockCatwalkClient := &mockCatwalkClient{
		err: catwalk.ErrNotModified,
	}
	mockHyperClient := &mockHyperClient{
		provider: catwalk.Provider{}, // Empty provider.
	}

	catwalkPath := tmpDir + "/crush/providers.json"
	hyperPath := tmpDir + "/crush/hyper.json"

	testCatwalkSyncer.Init(mockCatwalkClient, catwalkPath, true)
	testHyperSyncer.Init(mockHyperClient, hyperPath, true)

	catwalkResult, err := testCatwalkSyncer.Get(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, catwalkResult) // Should fall back to embedded.

	hyperResult, err := testHyperSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Charm Hyper", hyperResult.Name) // Falls back to embedded when no models.
}

func TestCache_StoreAndGet(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := tmpDir + "/test.json"

	cache := newCache[[]catwalk.Provider](cachePath)

	providers := []catwalk.Provider{
		{Name: "Provider1", ID: "p1"},
		{Name: "Provider2", ID: "p2"},
	}

	// Store.
	err := cache.Store(providers)
	require.NoError(t, err)

	// Get.
	result, etag, err := cache.Get()
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "Provider1", result[0].Name)
	require.NotEmpty(t, etag)
}

func TestCache_GetNonExistent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := tmpDir + "/nonexistent.json"

	cache := newCache[[]catwalk.Provider](cachePath)

	_, _, err := cache.Get()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read provider cache file")
}

func TestCache_GetInvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := tmpDir + "/invalid.json"

	require.NoError(t, os.WriteFile(cachePath, []byte("invalid json"), 0o644))

	cache := newCache[[]catwalk.Provider](cachePath)

	_, _, err := cache.Get()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal provider data from cache")
}

func TestCachePathFor(t *testing.T) {
	tests := []struct {
		name        string
		xdgDataHome string
		expected    string
	}{
		{
			name:        "with XDG_DATA_HOME",
			xdgDataHome: "/custom/data",
			expected:    "/custom/data/sapphire/providers.json",
		},
		{
			name:        "without XDG_DATA_HOME",
			xdgDataHome: "",
			expected:    "", // Will use platform-specific default.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.xdgDataHome != "" {
				t.Setenv("XDG_DATA_HOME", tt.xdgDataHome)
			} else {
				t.Setenv("XDG_DATA_HOME", "")
			}

			result := cachePathFor("providers")
			if tt.expected != "" {
				require.Equal(t, tt.expected, filepath.ToSlash(result))
			} else {
				require.Contains(t, result, "sapphire")
				require.Contains(t, result, "providers.json")
			}
		})
	}
}
