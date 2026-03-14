package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefinitionFromServer_RemotesOnly(t *testing.T) {
	wrapped := registryServerWrapper{
		Server: registryServer{
			Name:        "example/remote",
			Description: "Hosted MCP server",
			Remotes: []registryTransport{
				{
					Type: "streamable-http",
					URL:  "https://api.example.com/mcp",
				},
			},
		},
		Meta: map[string]registryServerMeta{
			mcpRegistryMetaKey: {IsLatest: true},
		},
	}

	def, ok := definitionFromServer(wrapped)
	require.True(t, ok)
	require.Equal(t, "example/remote", def.Name)
	require.Equal(t, MCPHttp, def.Type)
	require.Equal(t, "https://api.example.com/mcp", def.URL)
	require.Empty(t, def.Command)
	require.Empty(t, def.Args)
}

func TestDefinitionFromServer_PyPIExecutableHint(t *testing.T) {
	wrapped := registryServerWrapper{
		Server: registryServer{
			Name:        "example/pathfinder",
			Description: "Python MCP server",
			Packages: []registryPackage{
				{
					RegistryType: "pypi",
					Identifier:   "codepathfinder",
					RuntimeHint:  "uvx",
					Transport: registryTransport{
						Type: "stdio",
					},
					PackageArguments: []registryArgument{
						{
							Type:    "positional",
							Default: "pathfinder",
						},
					},
				},
			},
		},
		Meta: map[string]registryServerMeta{
			mcpRegistryMetaKey: {IsLatest: true},
		},
	}

	def, ok := definitionFromServer(wrapped)
	require.True(t, ok)
	require.Equal(t, MCPStdio, def.Type)
	require.Equal(t, "uvx", def.Command)
	require.Equal(t, []string{"--from", "codepathfinder", "pathfinder"}, def.Args)
}

func TestCuratedRegistryEntry_EngineeringCategoriesOnly(t *testing.T) {
	supabase, ok := CuratedRegistryEntry(RegistryMCPDefinition{
		Name:        "com.supabase/mcp",
		Description: "MCP server for interacting with the Supabase platform",
	})
	require.True(t, ok)
	require.Equal(t, RegistryMCPCategoryDatabases, supabase.Category)

	github, ok := CuratedRegistryEntry(RegistryMCPDefinition{
		Name:        "io.github.Dave-London/github",
		Description: "MCP server for GitHub operations (PRs, issues, actions) with structured output",
	})
	require.True(t, ok)
	require.Equal(t, RegistryMCPCategoryDevelopmentInfra, github.Category)

	_, ok = CuratedRegistryEntry(RegistryMCPDefinition{
		Name:        "io.github.weather/mcp",
		Description: "An MCP server for weather information",
	})
	require.False(t, ok)
}

func TestCuratedRegistryDefinitions_LargeEngineeringInventory(t *testing.T) {
	curated := CuratedRegistryDefinitions(RegistryMCPDefinitions)
	require.GreaterOrEqual(t, len(curated), 100)
}

func TestCuratedRegistryDefinitions_CategoryCoverage(t *testing.T) {
	catalog := CuratedRegistryCatalog(RegistryMCPDefinitions)
	counts := map[RegistryMCPCategory]int{}
	for _, entry := range catalog {
		counts[entry.Category]++
	}

	categories := []RegistryMCPCategory{
		RegistryMCPCategoryCloudInfrastructure,
		RegistryMCPCategoryAuthentication,
		RegistryMCPCategoryPayments,
		RegistryMCPCategoryDatabases,
		RegistryMCPCategoryAIVectorSearch,
		RegistryMCPCategoryDevelopmentInfra,
		RegistryMCPCategoryTestingDebugging,
		RegistryMCPCategoryDesign,
		RegistryMCPCategoryProductivity,
	}

	for _, category := range categories {
		require.Greaterf(t, counts[category], 0, "missing MCP coverage for %s", category)
	}
}

func TestRegistryEntryInstructions_NotEmpty(t *testing.T) {
	catalog := CuratedRegistryCatalog(RegistryMCPDefinitions)
	for _, entry := range catalog {
		require.NotEmpty(t, RegistryEntryInstructions(entry), "missing instructions for %s", entry.Definition.Name)
	}
}
