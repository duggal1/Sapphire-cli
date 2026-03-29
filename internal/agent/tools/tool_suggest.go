package tools

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/permission"
)

type ToolSuggestParams struct {
	Query string `json:"query" description:"What capability you need (e.g., 'payments', 'database', 'auth', 'observability')"`
	Limit int    `json:"limit,omitempty" description:"Maximum number of suggestions to return"`
}

type scoredEntry struct {
	entry mcpServerSnapshot
	score int
}

type toolSuggestion struct {
	Name        string
	Action      string
	Reason      string
	Category    string
	Description string
	EnvKeys     []string
	Status      string
}

//go:embed tool_suggest.md
var toolSuggestDescription []byte

func NewToolSuggestTool(cfg *config.Config, permissions permission.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		ToolSuggestToolName,
		string(toolSuggestDescription),
		func(ctx context.Context, params ToolSuggestParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			params.Query = strings.TrimSpace(params.Query)
			if params.Query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for tool suggestions")
			}

			path := filepathext.SmartJoin(cfg.WorkingDir(), "mcp-registry")
			granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        path,
				ToolCallID:  call.ID,
				ToolName:    ToolSuggestToolName,
				Action:      "list",
				Description: "Suggest MCP servers for the requested capability",
				Params:      map[string]any{"query": params.Query, "limit": params.Limit},
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !granted {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			registryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			defs := config.DefaultRegistryDefinitions(registryCtx)
			if len(defs) == 0 {
				return fantasy.NewTextResponse("No MCP registry entries available"), nil
			}

			catalog := config.CuratedRegistryCatalog(defs)
			catalog = includeUncategorizedDefs(defs, catalog)
			catalog = mergeConfigMCPs(catalog, cfg)
			scored := rankSuggestedMCPs(catalog, cfg, params.Query)
			if len(scored) == 0 {
				if liveDefs := loadLiveRegistryDefinitions(ctx); len(liveDefs) > 0 {
					liveCatalog := config.CuratedRegistryCatalog(liveDefs)
					liveCatalog = includeUncategorizedDefs(liveDefs, liveCatalog)
					liveCatalog = mergeConfigMCPs(liveCatalog, cfg)
					scored = rankSuggestedMCPs(liveCatalog, cfg, params.Query)
				}
			}

			if len(scored) == 0 {
				return fantasy.NewTextResponse("No MCP servers matched the query. Use list_available_mcps for a full list."), nil
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

			limit := params.Limit
			if limit <= 0 || limit > len(scored) {
				limit = minInt(len(scored), 5)
			}

			suggestions := make([]toolSuggestion, 0, limit)
			for i := 0; i < limit; i++ {
				entry := scored[i].entry
				reason := entry.Instructions
				if reason == "" {
					reason = fmt.Sprintf("Matches capability: %s", params.Query)
				}

				suggestions = append(suggestions, toolSuggestion{
					Name:        entry.Name,
					Action:      suggestedMCPAction(entry),
					Reason:      truncateText(reason, 160),
					Category:    entry.Category,
					Description: truncateText(entry.Description, 160),
					EnvKeys:     entry.EnvKeys,
					Status:      entry.State,
				})
			}

			var sb strings.Builder
			sb.WriteString("Suggested MCP servers:\n")
			for _, s := range suggestions {
				line := fmt.Sprintf("- %s [%s] [%s] action=%s", s.Name, s.Status, s.Category, s.Action)
				if s.Reason != "" {
					line += " | reason: " + s.Reason
				}
				if s.Description != "" {
					line += " | description: " + s.Description
				}
				if len(s.EnvKeys) > 0 {
					line += fmt.Sprintf(" | env: %s", strings.Join(s.EnvKeys, ", "))
				}
				sb.WriteString(line + "\n")
			}

			sb.WriteString("If the MCP is not installed, call install_mcp first. If it is already installed, call connect_mcp.")
			return fantasy.NewTextResponse(strings.TrimSpace(sb.String())), nil
		},
	)
}

func rankSuggestedMCPs(catalog []config.RegistryMCPInventoryEntry, cfg *config.Config, query string) []scoredEntry {
	snapshots := buildMCPSnapshots(catalog, cfg)
	queryTerms := mcpQueryTerms(query)
	scored := make([]scoredEntry, 0, len(snapshots))
	for _, entry := range snapshots {
		score, ok := scoreMCPSnapshot(entry, query, queryTerms)
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

	return scored
}

func suggestedMCPAction(entry mcpServerSnapshot) string {
	switch {
	case entry.Connected:
		return "list_mcp_tools"
	case entry.Configured:
		return "connect_mcp"
	default:
		return InstallMCPToolName
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
