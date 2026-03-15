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
	"github.com/charmbracelet/sapphire/internal/agent/hyper"
	"github.com/charmbracelet/sapphire/internal/csync"
	"github.com/charmbracelet/sapphire/internal/home"
	"github.com/charmbracelet/x/etag"
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
		autoupdate := !cfg.Options.DisableProviderAutoUpdate
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

		augmentProviderCatalog(providerList)

		providerErr = errors.Join(errs...)
	})
	return providerList, providerErr
}

func augmentProviderCatalog(providers []catwalk.Provider) {
	for i := range providers {
		switch providers[i].ID {
		case "openrouter":
			ensureModels(&providers[i], openRouterModelAugments)
		}
	}
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

var openRouterModelAugments = []catwalk.Model{
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
		DefaultMaxTokens:   32768,
		CostPer1MIn:        0,
		CostPer1MOut:       0,
		CostPer1MInCached:  0,
		CostPer1MOutCached: 0,
		CanReason:          false,
		SupportsImages:     false,
		Options:            catwalk.ModelOptions{},
	},
}

func loadCachedCatwalkProviders() ([]catwalk.Provider, error) {
	cached, _, err := newCache[[]catwalk.Provider](cachePathFor("providers")).Get()
	return cached, err
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
