package config

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/catwalk/pkg/embedded"
	"github.com/charmbracelet/x/etag"
	"github.com/duggal1/Sapphire-cli/internal/agent/hyper"
	"github.com/duggal1/Sapphire-cli/internal/csync"
	"github.com/duggal1/Sapphire-cli/internal/home"
)

type syncer[T any] interface {
	Get(context.Context) (T, error)
}

var (
	providerOnce sync.Once
	providerList []catwalk.Provider
	providerErr  error
)

const providerBackgroundTimeout = 3 * time.Second

func shouldAutoUpdateProviders(cfg *Config) bool {
	if cfg == nil || cfg.Options == nil {
		return strings.TrimSpace(os.Getenv("SAPPHIRE_NON_INTERACTIVE")) != "1"
	}
	if cfg.Options.DisableProviderAutoUpdate {
		return false
	}
	return strings.TrimSpace(os.Getenv("SAPPHIRE_NON_INTERACTIVE")) != "1"
}

// file to cache provider data
func cachePathFor(name string) string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		return filepath.Join(xdgDataHome, appName, name+".json")
	}

	// return the path to the main data directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.local/share/crush/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName, name+".json")
	}

	return filepath.Join(home.Dir(), ".local", "share", appName, name+".json")
}

// UpdateProviders updates the Catwalk providers list from a specified source.
func UpdateProviders(pathOrURL string) error {
	var providers []catwalk.Provider
	pathOrURL = cmp.Or(pathOrURL, os.Getenv("CATWALK_URL"), defaultCatwalkURL)

	switch {
	case pathOrURL == "embedded":
		providers = embedded.GetAll()
	case strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://"):
		var err error
		providers, err = catwalk.NewWithURL(pathOrURL).GetProviders(context.Background(), "")
		if err != nil {
			return fmt.Errorf("failed to fetch providers from Catwalk: %w", err)
		}
	default:
		content, err := os.ReadFile(pathOrURL)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if err := json.Unmarshal(content, &providers); err != nil {
			return fmt.Errorf("failed to unmarshal provider data: %w", err)
		}
		if len(providers) == 0 {
			return fmt.Errorf("no providers found in the provided source")
		}
	}

	if err := newCache[[]catwalk.Provider](cachePathFor("providers")).Store(providers); err != nil {
		return fmt.Errorf("failed to save providers to cache: %w", err)
	}

	slog.Info("Providers updated successfully", "count", len(providers), "from", pathOrURL, "to", cachePathFor)
	return nil
}

// UpdateHyper updates the Hyper provider information from a specified URL.
func UpdateHyper(pathOrURL string) error {
	if !hyper.Enabled() {
		return fmt.Errorf("hyper not enabled")
	}
	var provider catwalk.Provider
	pathOrURL = cmp.Or(pathOrURL, hyper.BaseURL())

	switch {
	case pathOrURL == "embedded":
		provider = hyper.Embedded()
	case strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://"):
		client := realHyperClient{baseURL: pathOrURL}
		var err error
		provider, err = client.Get(context.Background(), "")
		if err != nil {
			return fmt.Errorf("failed to fetch provider from Hyper: %w", err)
		}
	default:
		content, err := os.ReadFile(pathOrURL)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if err := json.Unmarshal(content, &provider); err != nil {
			return fmt.Errorf("failed to unmarshal provider data: %w", err)
		}
	}

	if err := newCache[catwalk.Provider](cachePathFor("hyper")).Store(provider); err != nil {
		return fmt.Errorf("failed to save Hyper provider to cache: %w", err)
	}

	slog.Info("Hyper provider updated successfully", "from", pathOrURL, "to", cachePathFor("hyper"))
	return nil
}

var (
	catwalkSyncer = &catwalkSync{}
	hyperSyncer   = &hyperSync{}
)

// Providers returns the list of providers, taking into account cached results
// and whether or not auto update is enabled.
//
// It will:
// 1. if auto update is disabled, it'll return the embedded providers at the
// time of release.
// 2. load the cached providers
// 3. try to get the fresh list of providers, and return either this new list,
// the cached list, or the embedded list if all others fail.
func Providers(cfg *Config) ([]catwalk.Provider, error) {
	providerOnce.Do(func() {
		var errs []error
		providers := csync.NewSlice[catwalk.Provider]()
		autoupdate := shouldAutoUpdateProviders(cfg)
		customProvidersOnly := cfg.Options.DisableDefaultProviders

		// Fast path: load cached providers (or embedded) without network calls.
		if !customProvidersOnly {
			items, err := loadCachedCatwalkProviders()
			if err != nil {
				slog.Debug("Failed to read cached Catwalk providers", "error", err)
			}
			if len(items) == 0 {
				items = embedded.GetAll()
			}
			providers.Append(items...)

			if autoupdate {
				go refreshCatwalkProviders()
			}
		}

		if !customProvidersOnly && hyper.Enabled() {
			item, err := loadCachedHyperProvider()
			if err != nil {
				slog.Debug("Failed to read cached Hyper provider", "error", err)
			}
			if item.ID == "" {
				item = hyper.Embedded()
			}
			if item.ID != "" {
				providers.Append(item)
			}
			if autoupdate {
				go refreshHyperProvider()
			}
		}

		providerList = slices.Collect(providers.Seq())

		// Add Gemini 3 Flash Preview
		for i := range providerList {
			if providerList[i].ID == "gemini" {
				hasFlash := false
				for _, m := range providerList[i].Models {
					if m.ID == "gemini-3-flash-preview" {
						hasFlash = true
						break
					}
				}
				if !hasFlash {
					providerList[i].Models = append(providerList[i].Models, catwalk.Model{
						ID:                     "gemini-3-flash-preview",
						Name:                   "Gemini 3 Flash Preview",
						CostPer1MIn:            0.1,
						CostPer1MOut:           0.4,
						CostPer1MInCached:      0.01,
						CostPer1MOutCached:     0.0,
						ContextWindow:          1000000,
						DefaultMaxTokens:       8192,
						CanReason:              true,
						ReasoningLevels:        []string{"minimal", "low", "medium", "high"},
						DefaultReasoningEffort: "medium",
					})
				}
			}
		}

		providerList = augmentProviderCatalog(providerList)

		providerErr = errors.Join(errs...)
	})
	return providerList, providerErr
}

func augmentProviderCatalog(providers []catwalk.Provider) []catwalk.Provider {
	providers = ensureProviders(providers, providerCatalogAugments)
	for i := range providers {
		switch providers[i].ID {
		case "openrouter":
			ensureModels(&providers[i], openRouterModelAugments)
			for j := range providers[i].Models {
				id := strings.TrimPrefix(providers[i].Models[j].ID, "google/")
				if IsGeminiReasoningModel(id) {
					providers[i].Models[j].CanReason = true
				}
			}
		case "gemini", "google":
			for j := range providers[i].Models {
				if IsGeminiReasoningModel(providers[i].Models[j].ID) {
					providers[i].Models[j].CanReason = true
				}
			}
		}
	}
	return providers
}

func ensureProviders(providers []catwalk.Provider, expected []catwalk.Provider) []catwalk.Provider {
	if len(expected) == 0 {
		return providers
	}

	existing := make(map[catwalk.InferenceProvider]struct{}, len(providers))
	for _, provider := range providers {
		existing[provider.ID] = struct{}{}
	}

	for _, provider := range expected {
		if _, ok := existing[provider.ID]; ok {
			continue
		}
		providers = append(providers, provider)
		existing[provider.ID] = struct{}{}
	}
	return providers
}

func ensureModels(provider *catwalk.Provider, expected []catwalk.Model) {
	if provider == nil {
		return
	}

	existing := make(map[string]struct{}, len(provider.Models))
	for _, model := range provider.Models {
		existing[model.ID] = struct{}{}
	}

	for _, model := range expected {
		if _, ok := existing[model.ID]; ok {
			continue
		}
		provider.Models = append(provider.Models, model)
		existing[model.ID] = struct{}{}
	}
}

var providerCatalogAugments = []catwalk.Provider{
	{
		Name:                "AIHubMix",
		ID:                  "aihubmix",
		Type:                catwalk.TypeOpenAICompat,
		APIKey:              "$AIHUBMIX_API_KEY",
		APIEndpoint:         "https://aihubmix.com/v1",
		DefaultLargeModelID: "coding-glm-5.1-free",
		DefaultSmallModelID: "coding-glm-5-turbo-free",
		Models: []catwalk.Model{
			{
				ID:                     "coding-glm-5.1-free",
				Name:                   "Coding GLM 5.1 (free)",
				CostPer1MIn:            0,
				CostPer1MOut:           0,
				CostPer1MInCached:      0,
				CostPer1MOutCached:     0,
				ContextWindow:          204800,
				DefaultMaxTokens:       65535,
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				SupportsImages:         false,
			},
			{
				ID:                     "coding-minimax-m2.7-free",
				Name:                   "Coding MiniMax M2.7 (free)",
				CostPer1MIn:            0,
				CostPer1MOut:           0,
				CostPer1MInCached:      0,
				CostPer1MOutCached:     0,
				ContextWindow:          204800,
				DefaultMaxTokens:       13100,
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				SupportsImages:         false,
			},
			{
				ID:                     "coding-glm-5-free",
				Name:                   "Coding GLM 5 (free)",
				CostPer1MIn:            0,
				CostPer1MOut:           0,
				CostPer1MInCached:      0,
				CostPer1MOutCached:     0,
				ContextWindow:          202752,
				DefaultMaxTokens:       65535,
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				SupportsImages:         false,
			},
			{
				ID:                     "coding-glm-5-turbo-free",
				Name:                   "Coding GLM 5 Turbo (free)",
				CostPer1MIn:            0,
				CostPer1MOut:           0,
				CostPer1MInCached:      0,
				CostPer1MOutCached:     0,
				ContextWindow:          202752,
				DefaultMaxTokens:       20275,
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				SupportsImages:         false,
			},
			{
				ID:                     "crush-glm-5.1-free",
				Name:                   "Crush GLM 5.1 (free)",
				CostPer1MIn:            0,
				CostPer1MOut:           0,
				CostPer1MInCached:      0,
				CostPer1MOutCached:     0,
				ContextWindow:          204800,
				DefaultMaxTokens:       65535,
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				SupportsImages:         false,
			},
		},
	},
	{
		Name:                "Z.AI",
		ID:                  "zai",
		Type:                catwalk.TypeOpenAICompat,
		APIKey:              "$ZAI_API_KEY",
		APIEndpoint:         "https://api.z.ai/api/coding/paas/v4",
		DefaultLargeModelID: "glm-5",
		DefaultSmallModelID: "glm-5-turbo",
		Models: []catwalk.Model{
			{
				ID:                     "glm-5.1",
				Name:                   "GLM-5.1",
				CostPer1MIn:            1.0,
				CostPer1MOut:           3.2,
				CostPer1MInCached:      0.2,
				ContextWindow:          204800,
				DefaultMaxTokens:       65536,
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				SupportsImages:         false,
			},
			{
				ID:                     "glm-5-turbo",
				Name:                   "GLM-5-Turbo",
				CostPer1MIn:            1.2,
				CostPer1MOut:           4.0,
				CostPer1MInCached:      0.24,
				ContextWindow:          200000,
				DefaultMaxTokens:       128000,
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				SupportsImages:         false,
			},
			{
				ID:                     "glm-5",
				Name:                   "GLM-5",
				CostPer1MIn:            1.0,
				CostPer1MOut:           3.2,
				CostPer1MInCached:      0.2,
				ContextWindow:          204800,
				DefaultMaxTokens:       65536,
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				SupportsImages:         false,
			},
		},
	},
	{
		Name:                "Cerebras",
		ID:                  "cerebras",
		Type:                catwalk.TypeOpenAICompat,
		APIKey:              "$CEREBRAS_API_KEY",
		APIEndpoint:         "https://api.cerebras.ai/v1",
		DefaultLargeModelID: "gpt-oss-120b",
		DefaultSmallModelID: "qwen-3-235b-a22b-instruct-2507",
		DefaultHeaders: map[string]string{
			"X-Cerebras-3rd-Party-Integration": "crush",
		},
		Models: []catwalk.Model{
			{
				ID:                     "gpt-oss-120b",
				Name:                   "OpenAI GPT OSS",
				CostPer1MIn:            0.35,
				CostPer1MOut:           0.75,
				ContextWindow:          131072,
				DefaultMaxTokens:       25000,
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				SupportsImages:         false,
			},
			{
				ID:               "qwen-3-235b-a22b-instruct-2507",
				Name:             "Qwen 3 235B Instruct",
				CostPer1MIn:      0.6,
				CostPer1MOut:     1.2,
				ContextWindow:    131072,
				DefaultMaxTokens: 25000,
				CanReason:        false,
				SupportsImages:   false,
			},
			{
				ID:               "zai-glm-4.7",
				Name:             "Z.ai GLM 4.7",
				CostPer1MIn:      2.25,
				CostPer1MOut:     2.75,
				ContextWindow:    131072,
				DefaultMaxTokens: 25000,
				CanReason:        false,
				SupportsImages:   false,
				Options: catwalk.ModelOptions{
					Temperature: floatPtr(1),
					TopP:        floatPtr(0.95),
				},
			},
		},
	},
}

var openRouterModelAugments = []catwalk.Model{
	{
		ID:                     "qwen/qwen3.6-plus-preview:free",
		Name:                   "Qwen: Qwen3.6 Plus Preview (free)",
		ContextWindow:          1000000,
		DefaultMaxTokens:       32768,
		CostPer1MIn:            0,
		CostPer1MOut:           0,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options:                catwalk.ModelOptions{},
	},
	{
		ID:                     "z-ai/glm-5",
		Name:                   "Z.ai: GLM 5",
		CostPer1MIn:            1,
		CostPer1MOut:           3.2,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0.2,
		ContextWindow:          202800,
		DefaultMaxTokens:       65536,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options:                catwalk.ModelOptions{},
	},
	{
		ID:                     "z-ai/glm-5-turbo",
		Name:                   "Z.ai: GLM 5 Turbo",
		CostPer1MIn:            1.2,
		CostPer1MOut:           4,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0.24,
		ContextWindow:          262144,
		DefaultMaxTokens:       65536,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options:                catwalk.ModelOptions{},
	},
	{
		ID:                 "nvidia/nemotron-3-nano-30b-a3b:free",
		Name:               "Nemotron 3 Nano 30B A3B (free)",
		ContextWindow:      256000,
		DefaultMaxTokens:   256000,
		CostPer1MIn:        0,
		CostPer1MOut:       0,
		CostPer1MInCached:  0,
		CostPer1MOutCached: 0,
		CanReason:          false,
		SupportsImages:     false,
		Options:            catwalk.ModelOptions{},
	},
	{
		ID:                 "nvidia/nemotron-3-super-120b-a12b:free",
		Name:               "NVIDIA: Nemotron 3 Super (free)",
		ContextWindow:      262144,
		DefaultMaxTokens:   26214,
		CostPer1MIn:        0,
		CostPer1MOut:       0,
		CostPer1MInCached:  0,
		CostPer1MOutCached: 0,
		CanReason:          false,
		SupportsImages:     false,
		Options:            catwalk.ModelOptions{},
	},
	{
		ID:                     "nousresearch/hermes-3-llama-3.1-405b:free",
		Name:                   "Nous: Hermes 3 405B Instruct (free)",
		ContextWindow:          131072,
		DefaultMaxTokens:       32768,
		CostPer1MIn:            0,
		CostPer1MOut:           0,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options:                catwalk.ModelOptions{},
	},
	{
		ID:                 "cognitivecomputations/dolphin-mistral-24b-venice-edition:free",
		Name:               "Venice: Uncensored (free)",
		ContextWindow:      32768,
		DefaultMaxTokens:   32800,
		CostPer1MIn:        0,
		CostPer1MOut:       0,
		CostPer1MInCached:  0,
		CostPer1MOutCached: 0,
		CanReason:          false,
		SupportsImages:     false,
		Options:            catwalk.ModelOptions{},
	},
	{
		ID:                     "minimax/minimax-m2.5:free",
		Name:                   "MiniMax: MiniMax M2.5 (free)",
		ContextWindow:          196608,
		DefaultMaxTokens:       32768,
		CostPer1MIn:            0,
		CostPer1MOut:           0,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options: catwalk.ModelOptions{
			ProviderOptions: map[string]any{
				"provider": map[string]any{
					"data_collection": "deny",
				},
			},
		},
	},
	{
		ID:                     "minimax/minimax-m2.5",
		Name:                   "MiniMax: MiniMax M2.5",
		ContextWindow:          196608,
		DefaultMaxTokens:       32768,
		CostPer1MIn:            0.27,
		CostPer1MOut:           0.95,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options:                catwalk.ModelOptions{},
	},
	{
		ID:                     "minimax/minimax-m2.5:nitro",
		Name:                   "MiniMax: MiniMax M2.5 (nitro)",
		ContextWindow:          196608,
		DefaultMaxTokens:       32768,
		CostPer1MIn:            0.295,
		CostPer1MOut:           1.20,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options:                catwalk.ModelOptions{},
	},
	{
		ID:                     "arcee-ai/trinity-mini:free",
		Name:                   "Arcee AI: Trinity Mini (free)",
		ContextWindow:          131072,
		DefaultMaxTokens:       131100,
		CostPer1MIn:            0,
		CostPer1MOut:           0,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options:                catwalk.ModelOptions{},
	},
	{
		ID:                     "arcee-ai/trinity-large-preview:free",
		Name:                   "Arcee AI: Trinity Large Preview (free)",
		ContextWindow:          131000,
		DefaultMaxTokens:       131000,
		CostPer1MIn:            0,
		CostPer1MOut:           0,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options:                catwalk.ModelOptions{},
	},
	{
		ID:                     "openai/gpt-oss-120b:free",
		Name:                   "OpenAI: gpt-oss-120b (free)",
		ContextWindow:          131072,
		DefaultMaxTokens:       131100,
		CostPer1MIn:            0,
		CostPer1MOut:           0,
		CostPer1MInCached:      0,
		CostPer1MOutCached:     0,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
		Options:                catwalk.ModelOptions{},
	},
}

func loadCachedCatwalkProviders() ([]catwalk.Provider, error) {
	cached, _, err := newCache[[]catwalk.Provider](cachePathFor("providers")).Get()
	return cached, err
}

func floatPtr(v float64) *float64 {
	return &v
}

func loadCachedHyperProvider() (catwalk.Provider, error) {
	cached, _, err := newCache[catwalk.Provider](cachePathFor("hyper")).Get()
	return cached, err
}

func refreshCatwalkProviders() {
	ctx, cancel := context.WithTimeout(context.Background(), providerBackgroundTimeout)
	defer cancel()
	catwalkURL := cmp.Or(os.Getenv("CATWALK_URL"), defaultCatwalkURL)
	client := catwalk.NewWithURL(catwalkURL)
	path := cachePathFor("providers")
	catwalkSyncer.Init(client, path, true)

	if _, err := catwalkSyncer.Get(ctx); err != nil {
		slog.Debug("Catwalk provider refresh failed", "error", err)
	}
}

func refreshHyperProvider() {
	if !hyper.Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), providerBackgroundTimeout)
	defer cancel()
	path := cachePathFor("hyper")
	hyperSyncer.Init(realHyperClient{baseURL: hyper.BaseURL()}, path, true)
	if _, err := hyperSyncer.Get(ctx); err != nil {
		slog.Debug("Hyper provider refresh failed", "error", err)
	}
}

type cache[T any] struct {
	path string
}

func newCache[T any](path string) cache[T] {
	return cache[T]{path: path}
}

func (c cache[T]) Get() (T, string, error) {
	var v T
	data, err := os.ReadFile(c.path)
	if err != nil {
		return v, "", fmt.Errorf("failed to read provider cache file: %w", err)
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return v, "", fmt.Errorf("failed to unmarshal provider data from cache: %w", err)
	}

	return v, etag.Of(data), nil
}

func (c cache[T]) Store(v T) error {
	slog.Info("Saving provider data to disk", "path", c.path)
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for provider cache: %w", err)
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write provider data to cache: %w", err)
	}
	return nil
}
