package tools

import (
	"fmt"
	"slices"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools/mcp"
	"github.com/duggal1/Sapphire-cli/internal/config"
)

type mcpServerSnapshot struct {
	Name              string
	Category          string
	Type              string
	Description       string
	Instructions      string
	Tags              []string
	EnvKeys           []string
	MissingEnvKeys    []string
	Configured        bool
	State             string
	Connected         bool
	Starting          bool
	Errored           bool
	ToolCount         int
	PromptCount       int
	ResourceCount     int
	ConnectedToolInfo []string
	ResourceNames     []string
	PromptNames       []string
	Entry             config.RegistryMCPInventoryEntry
}

func buildMCPSnapshots(catalog []config.RegistryMCPInventoryEntry, cfg *config.Config) []mcpServerSnapshot {
	toolInfoByServer := map[string][]string{}
	for name, tools := range mcp.Tools() {
		for _, tool := range tools {
			if tool == nil {
				continue
			}
			entry := tool.Name
			if desc := strings.TrimSpace(tool.Description); desc != "" {
				entry += " " + desc
			}
			toolInfoByServer[name] = append(toolInfoByServer[name], entry)
		}
		slices.Sort(toolInfoByServer[name])
	}

	resourceNamesByServer := map[string][]string{}
	for name, resources := range mcp.Resources() {
		for _, resource := range resources {
			if resource == nil {
				continue
			}
			title := strings.TrimSpace(resource.Title)
			if title == "" {
				title = strings.TrimSpace(resource.Name)
			}
			if title == "" {
				title = strings.TrimSpace(resource.URI)
			}
			if title != "" {
				resourceNamesByServer[name] = append(resourceNamesByServer[name], title)
			}
		}
		slices.Sort(resourceNamesByServer[name])
	}

	promptNamesByServer := map[string][]string{}
	for name, prompts := range mcp.Prompts() {
		for _, prompt := range prompts {
			if prompt == nil {
				continue
			}
			title := strings.TrimSpace(prompt.Name)
			if title != "" {
				promptNamesByServer[name] = append(promptNamesByServer[name], title)
			}
		}
		slices.Sort(promptNamesByServer[name])
	}

	snapshots := make([]mcpServerSnapshot, 0, len(catalog))
	for _, entry := range catalog {
		def := entry.Definition
		snapshot := mcpServerSnapshot{
			Name:         def.Name,
			Category:     string(entry.Category),
			Type:         string(def.Type),
			Description:  strings.TrimSpace(def.Description),
			Instructions: config.RegistryEntryInstructions(entry),
			Tags:         append([]string(nil), entry.Tags...),
			EnvKeys:      append([]string(nil), def.EnvKeys...),
			Entry:        entry,
		}

		if cfg != nil {
			if _, ok := cfg.MCP[def.Name]; ok {
				snapshot.Configured = true
			}
		}

		if state, ok := mcp.GetState(def.Name); ok {
			snapshot.State = state.State.String()
			snapshot.ToolCount = state.Counts.Tools
			snapshot.PromptCount = state.Counts.Prompts
			snapshot.ResourceCount = state.Counts.Resources
			switch state.State {
			case mcp.StateConnected:
				snapshot.Connected = true
			case mcp.StateStarting:
				snapshot.Starting = true
			case mcp.StateError:
				snapshot.Errored = true
			}
		} else if snapshot.Configured {
			snapshot.State = mcp.StateDisabled.String()
		} else {
			snapshot.State = "discoverable"
		}

		if snapshot.Configured && cfg != nil {
			if mcpCfg, ok := cfg.MCP[def.Name]; ok {
				snapshot.MissingEnvKeys = missingEnvKeys(mcpCfg.Env)
				slices.Sort(snapshot.MissingEnvKeys)
			}
		}

		snapshot.ConnectedToolInfo = append([]string(nil), toolInfoByServer[def.Name]...)
		snapshot.ResourceNames = append([]string(nil), resourceNamesByServer[def.Name]...)
		snapshot.PromptNames = append([]string(nil), promptNamesByServer[def.Name]...)
		snapshots = append(snapshots, snapshot)
	}

	return snapshots
}

func mcpQueryTerms(query string) []string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return nil
	}
	fields := strings.FieldsFunc(normalized, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	seen := map[string]struct{}{}
	terms := make([]string, 0, len(fields))
	appendTerm := func(term string) {
		term = strings.TrimSpace(term)
		if len(term) < 2 {
			return
		}
		if _, ok := seen[term]; ok {
			return
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	for _, field := range fields {
		appendTerm(field)
		for _, alias := range mcpSearchAliases[field] {
			appendTerm(alias)
		}
	}
	return terms
}

var mcpSearchAliases = map[string][]string{
	"aws":        {"amazon", "cloud", "infra"},
	"azure":      {"cloud", "infra"},
	"gcp":        {"google", "cloud", "infra"},
	"payments":   {"payment", "billing", "invoice", "checkout", "subscription", "stripe", "paddle"},
	"payment":    {"billing", "invoice", "checkout", "subscription", "stripe", "paddle"},
	"billing":    {"payment", "payments", "invoice", "subscription", "stripe", "paddle"},
	"database":   {"db", "postgres", "mysql", "redis", "supabase", "neon", "mongodb"},
	"db":         {"database", "postgres", "mysql", "redis", "supabase", "neon", "mongodb"},
	"auth":       {"authentication", "oauth", "sso", "identity", "clerk", "auth0"},
	"oauth":      {"auth", "authentication", "sso", "identity"},
	"deploy":     {"deployment", "cloud", "infra", "vercel", "netlify", "render", "railway"},
	"deployment": {"deploy", "cloud", "infra", "vercel", "netlify", "render", "railway"},
	"vector":     {"embedding", "search", "retrieval", "pinecone", "weaviate", "qdrant"},
	"search":     {"retrieval", "vector", "sourcegraph"},
}

func scoreMCPSnapshot(snapshot mcpServerSnapshot, query string, terms []string) (int, bool) {
	if len(terms) == 0 {
		base := snapshot.Entry.Priority
		if snapshot.Connected {
			base += 50
		} else if snapshot.Configured {
			base += 10
		}
		return base, true
	}

	blob := strings.ToLower(strings.Join([]string{
		snapshot.Name,
		snapshot.Description,
		snapshot.Category,
		strings.Join(snapshot.Tags, " "),
		snapshot.Instructions,
		snapshot.Type,
		strings.Join(snapshot.ConnectedToolInfo, " "),
		strings.Join(snapshot.ResourceNames, " "),
		strings.Join(snapshot.PromptNames, " "),
	}, " "))

	score := snapshot.Entry.Priority
	matched := 0
	phrase := strings.ToLower(strings.TrimSpace(query))
	if phrase != "" {
		if strings.Contains(strings.ToLower(snapshot.Name), phrase) {
			score += 40
		} else if strings.Contains(blob, phrase) {
			score += 20
		}
	}

	for _, term := range terms {
		if len(term) < 2 {
			continue
		}
		switch {
		case strings.Contains(strings.ToLower(snapshot.Name), term):
			score += 18
			matched++
		case strings.Contains(strings.ToLower(snapshot.Category), term):
			score += 14
			matched++
		case strings.Contains(blob, term):
			score += 10
			matched++
		}
	}

	if matched == 0 {
		return 0, false
	}
	if snapshot.Connected {
		score += 30
	} else if snapshot.Configured {
		score += 10
	}
	if len(snapshot.MissingEnvKeys) == 0 && len(snapshot.EnvKeys) > 0 {
		score += 4
	}
	score += matched * 2
	return score, true
}

func describeMCPServer(snapshot mcpServerSnapshot) string {
	line := fmt.Sprintf("- %s [%s] [%s] [%s]", snapshot.Name, snapshot.State, snapshot.Category, snapshot.Type)
	if len(snapshot.EnvKeys) > 0 {
		line += fmt.Sprintf(" [env: %s]", strings.Join(snapshot.EnvKeys, ", "))
	}
	if len(snapshot.MissingEnvKeys) > 0 {
		line += fmt.Sprintf(" [missing env: %s]", strings.Join(snapshot.MissingEnvKeys, ", "))
	}
	if snapshot.Connected {
		line += fmt.Sprintf(" [tools=%d prompts=%d resources=%d]", snapshot.ToolCount, snapshot.PromptCount, snapshot.ResourceCount)
	}
	if snapshot.Description != "" {
		line += ": " + snapshot.Description
	}
	if snapshot.Connected && len(snapshot.ConnectedToolInfo) > 0 {
		line += " | tools: " + truncateText(strings.Join(snapshot.ConnectedToolInfo, "; "), 200)
	}
	if snapshot.Instructions != "" {
		line += " | instructions: " + truncateText(snapshot.Instructions, 200)
	}
	return line
}
