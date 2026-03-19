package tools

import (
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/stretchr/testify/require"
)

func TestMCPDiscoveryToolsDoNotRequireOptionalParams(t *testing.T) {
	cfg := &config.Config{}
	perm := permission.NewPermissionService(t.TempDir(), true, nil)

	listMCPTools := NewListMCPToolsTool(cfg, perm)
	require.Empty(t, listMCPTools.Info().Required)

	listAvailable := NewListAvailableMCPsTool(cfg, perm)
	require.Empty(t, listAvailable.Info().Required)
}

func TestScoreMCPSnapshotMatchesAnyRelevantTerm(t *testing.T) {
	t.Parallel()

	snapshot := mcpServerSnapshot{
		Name:        "supabase",
		Category:    string(config.RegistryMCPCategoryDatabases),
		Description: "Database workflows for Supabase",
		Tags:        []string{"supabase", "database"},
		Entry: config.RegistryMCPInventoryEntry{
			Priority: 20,
		},
	}

	score, ok := scoreMCPSnapshot(snapshot, "aws stripe supabase", mcpQueryTerms("aws stripe supabase"))
	require.True(t, ok)
	require.Greater(t, score, snapshot.Entry.Priority)
}

func TestFormatMCPInventorySummaryIncludesCounts(t *testing.T) {
	t.Parallel()

	summary := formatMCPInventorySummary(mcpInventorySummary{
		RegistryCount:   1000,
		ConfiguredCount: 94,
		ConnectedCount:  0,
		StartingCount:   0,
		ErrorCount:      0,
	})

	require.True(t, strings.Contains(summary, "MCP support: built in"))
	require.True(t, strings.Contains(summary, "Registry-backed MCP inventory: 1000"))
	require.True(t, strings.Contains(summary, "Configured MCP servers: 94"))
}

func TestMCPQueryTermsExpandsCapabilityAliases(t *testing.T) {
	t.Parallel()

	terms := mcpQueryTerms("payments")
	require.Contains(t, terms, "payments")
	require.Contains(t, terms, "stripe")
	require.Contains(t, terms, "billing")
}

func TestScoreMCPSnapshotMatchesConnectedToolSurface(t *testing.T) {
	t.Parallel()

	snapshot := mcpServerSnapshot{
		Name:              "com.stripe/mcp",
		Category:          string(config.RegistryMCPCategoryPayments),
		Description:       "Stripe MCP server",
		Connected:         true,
		State:             "connected",
		ConnectedToolInfo: []string{"create_checkout_session Create a Stripe checkout session"},
		Entry: config.RegistryMCPInventoryEntry{
			Priority: 20,
		},
	}

	score, ok := scoreMCPSnapshot(snapshot, "checkout", mcpQueryTerms("checkout"))
	require.True(t, ok)
	require.Greater(t, score, snapshot.Entry.Priority)
}

func TestScoreConnectedToolMatchesServerAndToolQuery(t *testing.T) {
	t.Parallel()

	score, ok := scoreConnectedTool("com.stripe/mcp", "create_checkout_session", "Create a checkout session", "stripe checkout", mcpQueryTerms("stripe checkout"))
	require.True(t, ok)
	require.Greater(t, score, 0)
}
