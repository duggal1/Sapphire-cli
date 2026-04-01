package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestRegistryAgentToolInfoUnwrapsObjectSchema(t *testing.T) {
	t.Parallel()

	tool := registryAgentTool{
		spec: ToolSpec{
			Name:        "update_plan",
			Description: "Record the current implementation plan.",
			Parameters: objectSchema(map[string]any{
				"plan": stringSchema("plan payload"),
			}, "plan"),
			Required: []string{"plan"},
		},
	}

	info := tool.Info()
	if _, ok := info.Parameters["plan"]; !ok {
		t.Fatalf("expected unwrapped plan property, got %#v", info.Parameters)
	}
	if _, ok := info.Parameters["properties"]; ok {
		t.Fatalf("expected object schema to be unwrapped, got %#v", info.Parameters)
	}
	if len(info.Required) != 1 || info.Required[0] != "plan" {
		t.Fatalf("unexpected required fields: %#v", info.Required)
	}
}

func TestRegistryAgentToolRunAppliesHarnessPreflight(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.Register(ToolSpec{
		Name:        LSToolName,
		Description: "list directory",
		Handler: func(ctx context.Context, rawInput json.RawMessage, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	}))

	ctx := context.WithValue(context.Background(), HarnessRequirementContextKey, HarnessRequirement{
		Required:               true,
		Reason:                 "broad codebase initialization",
		ComplexityScore:        3,
		RequireBeforeDiscovery: true,
	})

	tool := registry.AgentTools(LSToolName)[0]
	_, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "ls-1",
		Name:  LSToolName,
		Input: `{}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Start with `run_harness` before repo-wide discovery")
}
