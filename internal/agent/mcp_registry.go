package agent

import (
	"context"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/sapphire/internal/agent/tools/mcp"
	"github.com/charmbracelet/sapphire/internal/config"
)

func (c *coordinator) loadRegistryDefinitions(ctx context.Context) []config.RegistryMCPDefinition {
	const refreshInterval = 30 * time.Minute

	c.mcpRegistryMu.Lock()
	cached := append([]config.RegistryMCPDefinition(nil), c.mcpRegistryDefs...)
	lastFetch := c.mcpRegistryLastFetch
	inFlight := c.mcpRegistryFetchInFlight
	c.mcpRegistryMu.Unlock()

	if len(cached) > 0 && time.Since(lastFetch) < refreshInterval {
		return cached
	}

	if !inFlight {
		c.mcpRegistryMu.Lock()
		if !c.mcpRegistryFetchInFlight {
			c.mcpRegistryFetchInFlight = true
			c.mcpRegistryMu.Unlock()
			go c.refreshRegistryDefinitions()
		} else {
			c.mcpRegistryMu.Unlock()
		}
	}

	if len(cached) > 0 {
		return cached
	}

	return config.CuratedRegistryDefinitions(config.RegistryMCPDefinitions)
}

func (c *coordinator) refreshRegistryDefinitions() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	defs := config.DefaultRegistryDefinitions(ctx)

	c.mcpRegistryMu.Lock()
	if len(defs) > 0 {
		c.mcpRegistryDefs = defs
		c.mcpRegistryLastFetch = time.Now()
	}
	c.mcpRegistryFetchInFlight = false
	c.mcpRegistryMu.Unlock()
}

func (c *coordinator) ensureMCPInstalled(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	cfg := c.cfg
	if cfg.MCP == nil {
		cfg.MCP = make(map[string]config.MCPConfig)
	}

	defs := c.loadRegistryDefinitions(ctx)
	defMap := make(map[string]config.RegistryMCPDefinition, len(defs))
	for _, def := range defs {
		defMap[def.Name] = def
	}

	missing := []string{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		cfgEntry, ok := cfg.MCP[name]
		if !ok {
			def, ok := defMap[name]
			if !ok {
				missing = append(missing, name)
				continue
			}
			envMap, missingEnv := buildEnvPlaceholders(def.EnvKeys)
			cfgEntry = config.RegistryDefinitionToMCPConfig(def, false)
			cfgEntry.Env = envMap
			cfgEntry.Disabled = len(def.EnvKeys) > 0 && len(missingEnv) > 0
			if err := cfg.UpsertMCPConfig(def.Name, cfgEntry); err != nil {
				return missing, err
			}
		}

		if cfgEntry.Disabled {
			if canEnableMCP(cfgEntry) {
				cfgEntry.Disabled = false
				if err := cfg.UpsertMCPConfig(name, cfgEntry); err != nil {
					return missing, err
				}
			} else {
				continue
			}
		}

		_ = mcp.ApplyConfig(ctx, cfg, name)
	}

	return missing, nil
}

func buildEnvPlaceholders(keys []string) (map[string]string, []string) {
	if len(keys) == 0 {
		return nil, nil
	}
	missing := make([]string, 0, len(keys))
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = "$" + key
		if _, ok := os.LookupEnv(key); !ok {
			missing = append(missing, key)
		}
	}
	slices.Sort(missing)
	return out, missing
}

func canEnableMCP(cfg config.MCPConfig) bool {
	if len(cfg.Env) == 0 {
		return true
	}
	for key := range cfg.Env {
		if _, ok := os.LookupEnv(key); !ok {
			return false
		}
	}
	return true
}
