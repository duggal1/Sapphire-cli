package agent

import (
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRankRegistryMCPsForPrompt_SelectsCloudTargets(t *testing.T) {
	entries := []config.RegistryMCPInventoryEntry{
		{
			Definition: config.RegistryMCPDefinition{
				Name:        "google-cloud-run",
				Description: "Deploy applications to Google Cloud Run",
			},
			Category: config.RegistryMCPCategoryCloudInfrastructure,
			Tags:     []string{"google cloud", "cloud run", "deploy"},
			Priority: 20,
		},
		{
			Definition: config.RegistryMCPDefinition{
				Name:        "github",
				Description: "Manage GitHub pull requests and actions",
			},
			Category: config.RegistryMCPCategoryDevelopmentInfra,
			Tags:     []string{"github", "pull request"},
			Priority: 18,
		},
	}

	ranked := rankRegistryMCPsForPrompt("Deploy this project to Google Cloud Run", entries)
	require.NotEmpty(t, ranked)
	require.Equal(t, "google-cloud-run", ranked[0].Entry.Definition.Name)
}

func TestRankRegistryMCPsForPrompt_SelectsPaymentsTargets(t *testing.T) {
	entries := []config.RegistryMCPInventoryEntry{
		{
			Definition: config.RegistryMCPDefinition{
				Name:        "stripe",
				Description: "Stripe billing and checkout automation",
			},
			Category: config.RegistryMCPCategoryPayments,
			Tags:     []string{"stripe", "billing", "checkout"},
			Priority: 20,
		},
		{
			Definition: config.RegistryMCPDefinition{
				Name:        "neon",
				Description: "Neon Postgres projects and branches",
			},
			Category: config.RegistryMCPCategoryDatabases,
			Tags:     []string{"neon", "postgres"},
			Priority: 18,
		},
	}

	ranked := rankRegistryMCPsForPrompt("Set up Stripe billing and subscriptions", entries)
	require.NotEmpty(t, ranked)
	require.Equal(t, "stripe", ranked[0].Entry.Definition.Name)
}
