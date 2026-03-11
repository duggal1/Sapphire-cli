package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/config"
)

type mcpSelection struct {
	MCPServers []string      `json:"mcp_servers"`
	Plan       []mcpPlanStep `json:"plan,omitempty"`
}

type mcpPlanStep struct {
	Step      string   `json:"step"`
	MCP       string   `json:"mcp"`
	DependsOn []string `json:"depends_on,omitempty"`
}

func (c *coordinator) selectMCPServers(ctx context.Context, userPrompt string) (map[string]struct{}, []string, error) {
	available := connectedMCPNames()
	if len(available) == 0 {
		return nil, nil, nil
	}

	capabilityMap := buildMCPCapabilityMap()
	systemPrompt := "You are a routing assistant. From the MCP capability map, return JSON with keys: mcp_servers (array of server names from the map) and plan (optional array of steps with fields step, mcp, depends_on) when the task has multiple distinct goals. Use only server names from the map. If none are needed, return {\"mcp_servers\": []}.\n\n" + capabilityMap

	model := c.currentAgent.Model().Model
	if _, small, err := c.buildAgentModels(ctx, false); err == nil {
		model = small.Model
	}

	agent := fantasy.NewAgent(model, fantasy.WithSystemPrompt(systemPrompt))
	result, err := agent.Generate(ctx, fantasy.AgentCall{Prompt: userPrompt})
	if err != nil {
		return nil, nil, err
	}
	selection, err := parseMCPSelection(result.Response.Content.Text())
	if err != nil {
		return nil, nil, err
	}

	selected, missing := mapDomainsToServers(selection.MCPServers, available)
	selectedSet := make(map[string]struct{}, len(selected))
	for _, name := range selected {
		selectedSet[name] = struct{}{}
	}
	return selectedSet, missing, nil
}

func parseMCPSelection(text string) (mcpSelection, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return mcpSelection{}, fmt.Errorf("empty selection")
	}
	jsonText := extractJSONObject(text)
	if jsonText == "" {
		return mcpSelection{}, fmt.Errorf("no JSON in selection")
	}
	var sel mcpSelection
	if err := json.Unmarshal([]byte(jsonText), &sel); err != nil {
		return mcpSelection{}, err
	}
	return sel, nil
}

func extractJSONObject(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return text[start : end+1]
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

func selectRegistryByKeywords(prompt string, defs []config.RegistryMCPDefinition) []string {
	prompt = strings.ToLower(prompt)
	if prompt == "" || len(defs) == 0 {
		return nil
	}

	keywords := []string{
		"stripe", "supabase", "postgres", "postgresql", "mysql", "redis", "mongodb", "neon",
		"aws", "amazon", "s3", "gcp", "google cloud", "azure", "github", "gitlab", "bitbucket",
		"vercel", "netlify", "cloudflare", "docker", "kubernetes", "k8s", "paddle", "clerk",
		"auth", "oauth", "payments", "billing", "email", "twilio", "sendgrid", "sentry",
		"datadog", "grafana", "prometheus", "lambda", "sqs", "sns", "dynamodb", "rds",
		"bigquery", "firebase", "analytics", "monitoring", "logging", "observability",
	}

	matchedKeywords := []string{}
	for _, kw := range keywords {
		if strings.Contains(prompt, kw) {
			matchedKeywords = append(matchedKeywords, kw)
		}
	}
	if len(matchedKeywords) == 0 {
		return nil
	}

	selected := []string{}
	seen := map[string]struct{}{}
	for _, def := range defs {
		blob := strings.ToLower(def.Name + " " + def.Description)
		for _, kw := range matchedKeywords {
			if strings.Contains(blob, kw) {
				if _, ok := seen[def.Name]; ok {
					break
				}
				seen[def.Name] = struct{}{}
				selected = append(selected, def.Name)
				break
			}
		}
	}
	return selected
}
