package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/sapphire/internal/agent/tools/mcp"
	"github.com/charmbracelet/sapphire/internal/config"
)

func (c *coordinator) buildMCPInventoryContext(ctx context.Context) string {
	defs := c.loadRegistryDefinitions(ctx)
	if len(defs) == 0 {
		return ""
	}
	catalog := config.CuratedRegistryCatalog(defs)
	if len(catalog) == 0 {
		return ""
	}

	connected := 0
	for _, state := range mcp.GetStates() {
		if state.State == mcp.StateConnected {
			connected++
		}
	}

	configured := 0
	if c.cfg != nil {
		configured = len(c.cfg.MCP)
	}

	categories := map[config.RegistryMCPCategory][]string{}
	for _, entry := range catalog {
		categories[entry.Category] = append(categories[entry.Category], entry.Definition.Name)
	}

	categoryOrder := []config.RegistryMCPCategory{
		config.RegistryMCPCategoryCloudInfrastructure,
		config.RegistryMCPCategoryAuthentication,
		config.RegistryMCPCategoryPayments,
		config.RegistryMCPCategoryDatabases,
		config.RegistryMCPCategoryAIVectorSearch,
		config.RegistryMCPCategoryDevelopmentInfra,
		config.RegistryMCPCategoryTestingDebugging,
		config.RegistryMCPCategoryDesign,
		config.RegistryMCPCategoryProductivity,
	}

	var sb strings.Builder
	sb.WriteString("<mcp_inventory>\n")
	sb.WriteString("MCP support: built in\n")
	sb.WriteString("The connected capability map is not the full inventory.\n")
	sb.WriteString(fmt.Sprintf("Registry-backed MCP inventory: %d\n", len(catalog)))
	sb.WriteString(fmt.Sprintf("Configured MCP servers in this terminal: %d\n", configured))
	sb.WriteString(fmt.Sprintf("Connected MCP servers right now: %d\n", connected))
	sb.WriteString("Category coverage:\n")
	for _, category := range categoryOrder {
		names := categories[category]
		if len(names) == 0 {
			sb.WriteString(fmt.Sprintf("- %s: 0\n", category))
			continue
		}
		slices.Sort(names)
		examples := names
		if len(examples) > 3 {
			examples = examples[:3]
		}
		sb.WriteString(fmt.Sprintf("- %s: %d (examples: %s)\n", category, len(names), strings.Join(examples, ", ")))
	}
	sb.WriteString("Use list_available_mcps to search the full registry and connect_mcp to start a server.\n")
	sb.WriteString("</mcp_inventory>")
	return sb.String()
}
