package tools

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools/mcp"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/permission"
)

type ListAvailableMCPsParams struct {
	Query string `json:"query,omitempty" description:"Optional search query for MCP names or descriptions"`
	Limit int    `json:"limit,omitempty" description:"Maximum number of MCP servers to return"`
}

type ListAvailableMCPsPermissionsParams struct {
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type mcpInventorySummary struct {
	RegistryCount   int
	ConfiguredCount int
	ConnectedCount  int
	StartingCount   int
	ErrorCount      int
}

const ListAvailableMCPsToolName = "list_available_mcps"

//go:embed list_available_mcps.md
var listAvailableMCPsDescription []byte

func NewListAvailableMCPsTool(cfg *config.Config, permissions permission.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		ListAvailableMCPsToolName,
		string(listAvailableMCPsDescription),
		func(ctx context.Context, params ListAvailableMCPsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			params.Query = strings.TrimSpace(params.Query)
			if params.Limit < 0 {
				params.Limit = 0
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for listing MCP servers")
			}

			path := filepathext.SmartJoin(cfg.WorkingDir(), "mcp-registry")
			p, err := permissions.Request(ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        path,
					ToolCallID:  call.ID,
					ToolName:    ListAvailableMCPsToolName,
					Action:      "list",
					Description: "List available MCP servers",
					Params:      ListAvailableMCPsPermissionsParams(params),
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			registryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			defs := config.DefaultRegistryDefinitions(registryCtx)
			if len(defs) == 0 {
				return fantasy.NewTextResponse("No MCP servers available"), nil
			}
			catalog := config.CuratedRegistryCatalog(defs)
			catalog = includeUncategorizedDefs(defs, catalog)
			catalog = mergeConfigMCPs(catalog, cfg)
			summary := collectMCPInventorySummary(cfg, len(catalog))

			type scoredEntry struct {
				entry mcpServerSnapshot
				score int
			}

			snapshots := buildMCPSnapshots(catalog, cfg)
			queryTerms := mcpQueryTerms(params.Query)
			scored := make([]scoredEntry, 0, len(snapshots))
			for _, entry := range snapshots {
				score, ok := scoreMCPSnapshot(entry, params.Query, queryTerms)
				if len(queryTerms) > 0 && !ok {
					continue
				}
				scored = append(scored, scoredEntry{entry: entry, score: score})
			}

			sort.SliceStable(scored, func(i, j int) bool {
				if scored[i].score != scored[j].score {
					return scored[i].score > scored[j].score
				}
				if scored[i].entry.Entry.Priority != scored[j].entry.Entry.Priority {
					return scored[i].entry.Entry.Priority > scored[j].entry.Entry.Priority
				}
				return scored[i].entry.Name < scored[j].entry.Name
			})

			lines := make([]string, 0, len(scored))
			for _, item := range scored {
				lines = append(lines, describeMCPServer(item.entry))
				if params.Limit > 0 && len(lines) >= params.Limit {
					break
				}
			}

			var sb strings.Builder
			sb.WriteString(formatMCPInventorySummary(summary))
			if params.Query != "" {
				sb.WriteString("\n")
				sb.WriteString(fmt.Sprintf("Query: %s\n", params.Query))
			}

			if len(lines) == 0 {
				sb.WriteString("No MCP servers matched the query")
				return fantasy.NewTextResponse(strings.TrimSpace(sb.String())), nil
			}

			sb.WriteString(strings.Join(lines, "\n"))
			return fantasy.NewTextResponse(strings.TrimSpace(sb.String())), nil
		},
	)
}

func collectMCPInventorySummary(cfg *config.Config, registryCount int) mcpInventorySummary {
	summary := mcpInventorySummary{
		RegistryCount: registryCount,
	}
	if cfg != nil {
		summary.ConfiguredCount = len(cfg.MCP)
	}
	for _, state := range mcp.GetStates() {
		switch state.State {
		case mcp.StateConnected:
			summary.ConnectedCount++
		case mcp.StateStarting:
			summary.StartingCount++
		case mcp.StateError:
			summary.ErrorCount++
		}
	}
	return summary
}

func formatMCPInventorySummary(summary mcpInventorySummary) string {
	return fmt.Sprintf(
		"MCP support: built in\nRegistry-backed MCP inventory: %d\nConfigured MCP servers: %d\nConnected MCP servers: %d\nStarting MCP servers: %d\nErrored MCP servers: %d\n",
		summary.RegistryCount,
		summary.ConfiguredCount,
		summary.ConnectedCount,
		summary.StartingCount,
		summary.ErrorCount,
	)
}

func mergeConfigMCPs(catalog []config.RegistryMCPInventoryEntry, cfg *config.Config) []config.RegistryMCPInventoryEntry {
	if cfg == nil || len(cfg.MCP) == 0 {
		return catalog
	}

	known := make(map[string]struct{}, len(catalog))
	for _, entry := range catalog {
		known[entry.Definition.Name] = struct{}{}
	}

	type manualOverride struct {
		description string
		category    config.RegistryMCPCategory
	}

	manual := map[string]manualOverride{
		"com.neon/mcp": {
			description: "Neon MCP server for managing Neon Postgres projects, branches, and queries.",
			category:    config.RegistryMCPCategoryDatabases,
		},
		"com.paddle/mcp": {
			description: "Paddle MCP server for payments, subscriptions, and billing workflows.",
			category:    config.RegistryMCPCategoryPayments,
		},
	}

	for name, mcpCfg := range cfg.MCP {
		if _, exists := known[name]; exists {
			continue
		}

		envKeys := make([]string, 0, len(mcpCfg.Env))
		for key := range mcpCfg.Env {
			envKeys = append(envKeys, key)
		}
		sort.Strings(envKeys)

		def := config.RegistryMCPDefinition{
			Name:    name,
			Type:    mcpCfg.Type,
			Command: mcpCfg.Command,
			Args:    mcpCfg.Args,
			URL:     mcpCfg.URL,
			EnvKeys: envKeys,
		}

		if override, ok := manual[name]; ok {
			def.Description = override.description
		} else {
			def.Description = "User-configured MCP server."
		}

		entry, ok := config.CuratedRegistryEntry(def)
		if !ok {
			category := config.RegistryMCPCategoryDevelopmentInfra
			if override, ok := manual[name]; ok {
				category = override.category
			}
			entry = config.RegistryMCPInventoryEntry{
				Definition: def,
				Category:   category,
				Priority:   1,
			}
		} else if override, ok := manual[name]; ok {
			entry.Category = override.category
		}

		catalog = append(catalog, entry)
		known[name] = struct{}{}
	}

	return catalog
}

func includeUncategorizedDefs(defs []config.RegistryMCPDefinition, catalog []config.RegistryMCPInventoryEntry) []config.RegistryMCPInventoryEntry {
	if len(defs) == 0 {
		return catalog
	}
	known := make(map[string]struct{}, len(catalog))
	for _, entry := range catalog {
		known[entry.Definition.Name] = struct{}{}
	}
	for _, def := range defs {
		if _, exists := known[def.Name]; exists {
			continue
		}
		catalog = append(catalog, config.RegistryMCPInventoryEntry{
			Definition: def,
			Category:   config.RegistryMCPCategoryDevelopmentInfra,
			Priority:   0,
		})
	}
	return catalog
}

func truncateText(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit]) + "…"
}
