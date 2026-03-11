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
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	defs, err := config.FetchRegistryDefinitions(ctx)
	if err == nil && len(defs) > 0 {
		return defs
	}
	return config.RegistryMCPDefinitions
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
			cfgEntry = config.MCPConfig{
				Type:     def.Type,
				Command:  def.Command,
				Args:     append([]string{}, def.Args...),
				Env:      envMap,
				Disabled: len(def.EnvKeys) > 0 && len(missingEnv) > 0,
			}
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
