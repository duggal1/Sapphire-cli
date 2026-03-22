package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/config"
)

type scoredRegistryMCP struct {
	Entry   config.RegistryMCPInventoryEntry
	Score   int
	Reasons []string
}

var categoryIntentKeywords = map[config.RegistryMCPCategory][]string{
	config.RegistryMCPCategoryCloudInfrastructure: {
		"deploy", "deployment", "cloud", "infrastructure", "container", "containers",
		"kubernetes", "k8s", "docker", "cluster", "serverless", "cloud run", "gcp",
		"google cloud", "aws", "render", "netlify", "vercel", "cloudflare",
	},
	config.RegistryMCPCategoryDatabases: {
		"database", "sql", "postgres", "postgresql", "schema", "migration", "query",
		"table", "branch", "supabase", "neon",
	},
	config.RegistryMCPCategoryAIVectorSearch: {
		"rag", "retrieval", "retrieve", "vector", "embedding", "semantic search",
		"knowledge base",
	},
	config.RegistryMCPCategoryAuthentication: {
		"auth", "authentication", "oauth", "oidc", "identity", "sso", "login",
		"user management", "credentials", "secret",
	},
	config.RegistryMCPCategoryPayments: {
		"stripe", "payment", "payments", "billing", "checkout", "subscription",
		"invoice", "merchant",
	},
	config.RegistryMCPCategoryDevelopmentInfra: {
		"git", "github", "gitlab", "bitbucket", "repository", "repo", "pull request",
		"pr", "branch", "worktree", "workflow", "actions", "release", "code review",
		"code search", "code indexing",
	},
	config.RegistryMCPCategoryProductivity: {
		"notion", "docs", "documentation", "knowledge", "wiki", "spec",
	},
	config.RegistryMCPCategoryDesign: {
		"figma", "design", "mock", "prototype", "ui",
	},
	config.RegistryMCPCategoryTestingDebugging: {
		"ci", "cd", "pipeline", "incident", "observability", "monitoring",
		"logs", "trace", "tracing",
		"sentry", "datadog", "grafana", "prometheus",
	},
}

func (c *coordinator) selectMCPServers(ctx context.Context, userPrompt string) (map[string]struct{}, string, error) {
	defs := c.loadRegistryDefinitions(ctx)
	if len(defs) == 0 {
		return nil, "", nil
	}

	ranked := rankRegistryMCPsForPrompt(userPrompt, config.CuratedRegistryCatalog(defs))
	selectedEntries := selectTopRegistryMCPs(ranked)
	if len(selectedEntries) == 0 {
		return nil, "", nil
	}

	targets := make([]string, 0, len(selectedEntries))
	for _, item := range selectedEntries {
		targets = append(targets, item.Entry.Definition.Name)
	}
	if len(targets) == 0 {
		return nil, "", nil
	}

	selectedSet := make(map[string]struct{}, len(targets))
	for _, name := range targets {
		selectedSet[name] = struct{}{}
	}

	return selectedSet, buildMCPSelectionContext(selectedEntries, selectedSet), nil
}

func rankRegistryMCPsForPrompt(prompt string, entries []config.RegistryMCPInventoryEntry) []scoredRegistryMCP {
	prompt = sanitizeMCPPrompt(prompt)
	if prompt == "" || len(entries) == 0 {
		return nil
	}

	tokens := strings.Fields(prompt)
	tokenLookup := tokenSet(tokens)
	scored := make([]scoredRegistryMCP, 0, len(entries))
	for _, entry := range entries {
		score, reasons := scoreRegistryMCPForPrompt(prompt, tokenLookup, entry)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredRegistryMCP{
			Entry:   entry,
			Score:   score,
			Reasons: reasons,
		})
	}

	slices.SortFunc(scored, func(a, b scoredRegistryMCP) int {
		if a.Score != b.Score {
			return b.Score - a.Score
		}
		if a.Entry.Priority != b.Entry.Priority {
			return b.Entry.Priority - a.Entry.Priority
		}
		return strings.Compare(a.Entry.Definition.Name, b.Entry.Definition.Name)
	})
	return scored
}

func scoreRegistryMCPForPrompt(prompt string, tokens map[string]struct{}, entry config.RegistryMCPInventoryEntry) (int, []string) {
	blob := config.RegistryEntrySearchBlob(entry)
	score := 0
	reasons := []string{}
	seenReasons := map[string]struct{}{}
	addReason := func(reason string) {
		if _, ok := seenReasons[reason]; ok {
			return
		}
		seenReasons[reason] = struct{}{}
		reasons = append(reasons, reason)
	}

	for _, keyword := range categoryIntentKeywords[entry.Category] {
		if containsKeyword(prompt, tokens, keyword) {
			score += 4
			addReason(keyword)
		}
	}

	for _, tag := range entry.Tags {
		if containsKeyword(prompt, tokens, tag) {
			score += 8
			addReason(tag)
		}
	}

	for token := range tokens {
		if len(token) < 3 {
			continue
		}
		if strings.Contains(blob, token) {
			score++
		}
	}

	if score == 0 {
		return 0, nil
	}

	score += entry.Priority
	slices.Sort(reasons)
	return score, reasons
}

func selectTopRegistryMCPs(ranked []scoredRegistryMCP) []scoredRegistryMCP {
	if len(ranked) == 0 {
		return nil
	}

	selected := make([]scoredRegistryMCP, 0, min(4, len(ranked)))
	perCategory := map[config.RegistryMCPCategory]int{}
	seen := map[string]struct{}{}
	for _, item := range ranked {
		if len(selected) >= 4 {
			break
		}
		if item.Score < 14 {
			break
		}
		if _, ok := seen[item.Entry.Definition.Name]; ok {
			continue
		}

		maxPerCategory := 1
		if item.Score >= 24 {
			maxPerCategory = 2
		}
		if perCategory[item.Entry.Category] >= maxPerCategory {
			continue
		}

		seen[item.Entry.Definition.Name] = struct{}{}
		perCategory[item.Entry.Category]++
		selected = append(selected, item)
	}
	return selected
}

func buildMCPSelectionContext(selected []scoredRegistryMCP, active map[string]struct{}) string {
	if len(selected) == 0 || len(active) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<mcp_task_context>\n")
	sb.WriteString("Potential MCP servers for this task:\n")
	for _, item := range selected {
		name := item.Entry.Definition.Name
		if _, ok := active[name]; !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s [%s]", name, item.Entry.Category))
		if len(item.Reasons) > 0 {
			sb.WriteString(": " + strings.Join(item.Reasons, ", "))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Use list_available_mcps to confirm inventory. If the server is not installed, call install_mcp. Then call connect_mcp before execution.\n")
	sb.WriteString("</mcp_task_context>")
	return sb.String()
}

func mapDomainsToServers(domains []string, available []string) ([]string, []string) {
	if len(domains) == 0 {
		return nil, nil
	}
	availLower := make(map[string]string, len(available))
	for _, name := range available {
		availLower[strings.ToLower(name)] = name
	}

	selected := []string{}
	missing := []string{}
	seen := map[string]struct{}{}
	for _, domain := range domains {
		key := strings.ToLower(strings.TrimSpace(domain))
		if key == "" {
			continue
		}
		if name, ok := availLower[key]; ok {
			if _, exists := seen[name]; !exists {
				seen[name] = struct{}{}
				selected = append(selected, name)
			}
			continue
		}

		matched := ""
		for low, original := range availLower {
			if strings.Contains(low, key) || strings.Contains(key, low) {
				matched = original
				break
			}
		}
		if matched != "" {
			if _, exists := seen[matched]; !exists {
				seen[matched] = struct{}{}
				selected = append(selected, matched)
			}
			continue
		}
		missing = append(missing, domain)
	}

	return selected, missing
}

func normalizePrompt(prompt string) string {
	prompt = strings.ToLower(prompt)
	replacer := strings.NewReplacer(
		"/", " ",
		"-", " ",
		"_", " ",
		",", " ",
		".", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
	)
	return strings.Join(strings.Fields(replacer.Replace(prompt)), " ")
}
