package tools

import "testing"

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
