package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/google/uuid"
)

const missingMCPMessage = "This capability requires an MCP server that is not installed.\nPlease install the required MCP."

var errMissingMCP = errors.New("missing required mcp")

func requiresMCPDiscovery(prompt string) bool {
	prompt = sanitizeMCPPrompt(prompt)
	if prompt == "" {
		return false
	}
	tokens := tokenSet(strings.Fields(prompt))
	for _, keywords := range categoryIntentKeywords {
		for _, keyword := range keywords {
			if containsKeyword(prompt, tokens, keyword) {
				return true
			}
		}
	}
	for _, keyword := range []string{
		"integrate", "integration", "api", "webhook", "connect", "connection", "mcp",
		"automation", "orchestrate", "pipeline",
	} {
		if containsKeyword(prompt, tokens, keyword) {
			return true
		}
	}
	return false
}

func (c *coordinator) preflightMCPDiscovery(ctx context.Context, sessionID, prompt string) (string, error) {
	if !requiresMCPDiscovery(prompt) {
		return "", nil
	}

	inventoryRequest := isMCPInventoryPrompt(prompt)
	params := tools.ListAvailableMCPsParams{
		Query: strings.TrimSpace(prompt),
		Limit: 15,
	}
	if inventoryRequest {
		params.Query = ""
		params.Limit = 0
	}
	input, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	tool := tools.NewListAvailableMCPsTool(c.cfg, c.permissions)
	toolCtx := context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	resp, err := tool.Run(toolCtx, fantasy.ToolCall{
		ID:    "mcp-preflight-" + uuid.New().String(),
		Name:  tools.ListAvailableMCPsToolName,
		Input: string(input),
	})
	if err != nil {
		return "", err
	}
	if resp.IsError {
		return "", errors.New(resp.Content)
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" || strings.HasPrefix(content, "No MCP servers available") {
		if inventoryRequest {
			return "", nil
		}
		return "", errMissingMCP
	}

	candidates := parseMCPNames(content)
	if len(candidates) > 0 && !inventoryRequest {
		if _, err := c.ensureMCPInstalled(ctx, candidates); err != nil {
			return "", err
		}
	}

	return "<mcp_discovery>\n" + content + "\n</mcp_discovery>", nil
}

func parseMCPNames(content string) []string {
	lines := strings.Split(content, "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimSpace(line[2:])
		} else if strings.HasPrefix(line, "•") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "•"))
		}
		if line == "" {
			continue
		}
		name := line
		if idx := strings.Index(name, " ["); idx != -1 {
			name = strings.TrimSpace(name[:idx])
		}
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}
