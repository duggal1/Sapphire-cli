package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/stretchr/testify/require"
)

func TestPrepareToolCallNormalizesEditAliases(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		EditToolName: editTool,
	}

	call := fantasy.ToolCall{
		ID:    "edit-1",
		Name:  EditToolName,
		Input: `{"path":"README.md","old":"alpha","replacement":"beta"}`,
	}

	prepared, _, err := PrepareToolCall(context.Background(), call, registry)
	require.NoError(t, err)
	require.Equal(t, EditToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "README.md", input["file_path"])
	require.Equal(t, "alpha", input["old_string"])
	require.Equal(t, "beta", input["new_string"])
}

func TestPrepareToolCallDoesNotRewriteEmptyEditPayloadToAgenticEdit(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	agenticEditTool := fantasy.NewAgentTool(
		AgenticEditToolName,
		"",
		func(ctx context.Context, params MultiEditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		EditToolName:        editTool,
		AgenticEditToolName: agenticEditTool,
	}

	_, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "edit-empty-1",
		Name:  EditToolName,
		Input: `{"file_edits":[]}`,
	}, registry)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "agentic_edit requires")
	require.Contains(t, err.Error(), "edit only accepts a single file_path plus old_string/new_string")
}

func TestPrepareToolCallUnwrapsArgumentsEnvelope(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		EditToolName: editTool,
	}

	call := fantasy.ToolCall{
		ID:   "edit-envelope-1",
		Name: EditToolName,
		Input: `{
			"name":"edit",
			"arguments":"{\"path\":\"README.md\",\"old\":\"alpha\",\"replacement\":\"beta\"}"
		}`,
	}

	prepared, _, err := PrepareToolCall(context.Background(), call, registry)
	require.NoError(t, err)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "README.md", input["file_path"])
	require.Equal(t, "alpha", input["old_string"])
	require.Equal(t, "beta", input["new_string"])
}

func TestPrepareToolCallStripsFencedJSONEnvelope(t *testing.T) {
	t.Parallel()

	viewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		SingleViewToolName: viewTool,
	}

	call := fantasy.ToolCall{
		ID:   "view-envelope-1",
		Name: SingleViewToolName,
		Input: "```json\n" +
			"{\"arguments\":{\"file_path\":\"README.md\",\"offset\":5}}\n" +
			"```",
	}

	prepared, _, err := PrepareToolCall(context.Background(), call, registry)
	require.NoError(t, err)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "README.md", input["file_path"])
	require.Equal(t, float64(5), input["offset"])
}

func TestPrepareToolCallExtractsJSONBlobFromProse(t *testing.T) {
	t.Parallel()

	searchTool := fantasy.NewAgentTool(
		WebSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		WebSearchToolName: searchTool,
	}

	call := fantasy.ToolCall{
		ID:    "web-search-blob-1",
		Name:  WebSearchToolName,
		Input: `Use this payload next: {"query":"agent loop detection"}`,
	}

	prepared, _, err := PrepareToolCall(context.Background(), call, registry)
	require.NoError(t, err)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "agent loop detection", input["query"])
}

func TestPrepareToolCallCoercesRunHarnessRawString(t *testing.T) {
	t.Parallel()

	runHarnessTool := fantasy.NewAgentTool(
		RunHarnessToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		RunHarnessToolName: runHarnessTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "harness-raw-1",
		Name:  RunHarnessToolName,
		Input: `Initialize the codebase thoroughly and lock the route first.`,
	}, registry)
	require.NoError(t, err)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "Initialize the codebase thoroughly and lock the route first.", input["task"])
}

func TestPrepareToolCallRejectsBashInPlanModeBeforeToolLookup(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), SessionModeContextKey, planmode.PlanMode)

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "plan-bash-1",
		Name:  "bash",
		Input: `{"command":"pwd"}`,
	}, map[string]fantasy.AgentTool{})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "plan mode restriction")
	require.Contains(t, err.Error(), `"bash"`)
	require.NotContains(t, err.Error(), "tool not found")
}

func TestPrepareToolCallRejectsApplyPatchAliasInPlanModeBeforeValidation(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), SessionModeContextKey, planmode.PlanMode)

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "plan-patch-1",
		Name:  "Apply Patch",
		Input: `{}`,
	}, map[string]fantasy.AgentTool{})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "plan mode restriction")
	require.Contains(t, err.Error(), `"apply_patch"`)
	require.NotContains(t, err.Error(), "unified_diff")
	require.NotContains(t, err.Error(), "tool not found")
}

func TestPrepareToolCallAllowsBashInArchitectMode(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	ctx := context.WithValue(context.Background(), SessionModeContextKey, planmode.ArchitectureMode)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "architect-bash-1",
		Name:  BashToolName,
		Input: `{"command":"pwd"}`,
	}, map[string]fantasy.AgentTool{BashToolName: bashTool})
	require.NoError(t, err)
	require.Equal(t, BashToolName, prepared.Name)
}

func TestPrepareToolCallBlocksExecutionDuringDeepPlanningBeforePlanPublished(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), DeepPlanningContextKey, true)
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, NewToolUsageState())

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "deep-plan-edit-1",
		Name:  EditToolName,
		Input: `{"file_path":"README.md","old_string":"old","new_string":"new"}`,
	}, map[string]fantasy.AgentTool{EditToolName: editTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Deep planning is active")
	require.Contains(t, err.Error(), "`update_plan`")
}

func TestPrepareToolCallAllowsStructuredReadDuringDeepPlanningBeforePlanPublished(t *testing.T) {
	t.Parallel()

	viewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), DeepPlanningContextKey, true)
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, NewToolUsageState())

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "deep-plan-view-1",
		Name:  SingleViewToolName,
		Input: `{"file_path":"README.md"}`,
	}, map[string]fantasy.AgentTool{SingleViewToolName: viewTool})
	require.NoError(t, err)
	require.Equal(t, SingleViewToolName, prepared.Name)
}

func TestPrepareToolCallAllowsExecutionAfterDeepPlanningPlanPublished(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.MarkPlanPublished()

	ctx := context.WithValue(context.Background(), DeepPlanningContextKey, true)
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "deep-plan-edit-2",
		Name:  EditToolName,
		Input: `{"file_path":"README.md","old_string":"old","new_string":"new"}`,
	}, map[string]fantasy.AgentTool{EditToolName: editTool})
	require.NoError(t, err)
	require.Equal(t, EditToolName, prepared.Name)
}

func TestPrepareToolCallRewritesBlockedDeepPlanningToolToUpdatePlanAfterEvidence(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	updatePlanTool := fantasy.NewAgentTool(
		UpdatePlanToolName,
		"",
		func(ctx context.Context, params UpdatePlanArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)

	ctx := context.WithValue(context.Background(), DeepPlanningContextKey, true)
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "deep-plan-bash-1",
		Name:  BashToolName,
		Input: `{"command":"git diff --stat","description":"inspect recent changes"}`,
	}, map[string]fantasy.AgentTool{BashToolName: bashTool, UpdatePlanToolName: updatePlanTool})
	require.NoError(t, err)
	require.Equal(t, UpdatePlanToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"Map the relevant architecture, constraints, and edge cases"`)
}

func TestPrepareToolCallRejectsDirectEditToolsInSecurityMode(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), SessionModeContextKey, planmode.SecurityMode)

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "security-patch-1",
		Name:  "apply_patch",
		Input: `{"file_path":"main.go","unified_diff":"diff --git a/main.go b/main.go"}`,
	}, map[string]fantasy.AgentTool{})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "security mode restriction")
	require.Contains(t, err.Error(), `"apply_patch"`)
}

func TestPrepareToolCallDoesNotRewriteUnreadEditToView(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	viewTool := fantasy.NewAgentTool(
		ViewToolName,
		"",
		func(ctx context.Context, params struct {
			FilePath string `json:"file_path"`
		}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		EditToolName: editTool,
		ViewToolName: viewTool,
	}

	call := fantasy.ToolCall{
		ID:    "edit-2",
		Name:  EditToolName,
		Input: `{"file_path":"README.md","old_string":"alpha","new_string":"beta"}`,
	}

	prepared, _, err := PrepareToolCall(context.Background(), call, registry)
	require.NoError(t, err)
	require.Equal(t, EditToolName, prepared.Name)
}

func TestPrepareToolCallNormalizesSaveMemoryAliases(t *testing.T) {
	t.Parallel()

	saveTool := fantasy.NewAgentTool(
		"save_memory",
		"",
		func(ctx context.Context, params struct {
			EventType string         `json:"event_type"`
			Content   map[string]any `json:"content"`
		}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		"save_memory": saveTool,
	}

	call := fantasy.ToolCall{
		ID:    "memory-1",
		Name:  "save_memory",
		Input: `{"type":"failure_mode","data":{"issue":"timeout"}}`,
	}

	prepared, _, err := PrepareToolCall(context.Background(), call, registry)
	require.NoError(t, err)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "failure_mode", input["event_type"])
	require.IsType(t, map[string]any{}, input["content"])
}

func TestPrepareToolCallBlocksProtectedToolsUntilHarnessRuns(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), MessageIDContextKey, "msg-harness-required")
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Required:        true,
		Reason:          "multi-phase non-trivial task",
		ComplexityScore: 5,
	})

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "edit-harness-1",
		Name:  EditToolName,
		Input: `{"file_path":"README.md","old_string":"alpha","new_string":"beta"}`,
	}, map[string]fantasy.AgentTool{EditToolName: editTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "call `run_harness`")
}

func TestPrepareToolCallRewritesProtectedToolsToHarnessWhenAvailable(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	runHarnessTool := fantasy.NewAgentTool(
		RunHarnessToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), MessageIDContextKey, "msg-harness-rewrite")
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Required:        true,
		Reason:          "multi-phase non-trivial task",
		ComplexityScore: 5,
		Task:            "Implement a safe multi-file change and verify the build.",
	})

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "edit-harness-rewrite-1",
		Name:  EditToolName,
		Input: `{"file_path":"README.md","old_string":"alpha","new_string":"beta"}`,
	}, map[string]fantasy.AgentTool{EditToolName: editTool, RunHarnessToolName: runHarnessTool})
	require.NoError(t, err)
	require.Equal(t, RunHarnessToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"task":"Implement a safe multi-file change and verify the build."`)
}

func TestPrepareToolCallRewritesProtectedToolsToStructuredDiscoveryBeforeContextRead(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		SingleEditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	searchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), MessageIDContextKey, "msg-context-rewrite")
	ctx = context.WithValue(ctx, LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:         "implementation/broad/backend",
		Reason:             "learned route policy for recurring implementation/broad/backend turns",
		RequireContextRead: true,
	})

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "context-rewrite-1",
		Name:  SingleEditToolName,
		Input: `{"file_path":"cmd/api/main.go","old_string":"before","new_string":"after"}`,
	}, map[string]fantasy.AgentTool{SingleEditToolName: editTool, ToolSearchToolName: searchTool})
	require.NoError(t, err)
	require.Equal(t, ToolSearchToolName, prepared.Name)
}

func TestPrepareToolCallRewritesProtectedToolsToReadWhenStructuredEvidenceExists(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		SingleEditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	viewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.MarkStructuredEvidence("cmd/api/main.go")

	ctx := context.WithValue(context.Background(), MessageIDContextKey, "msg-context-read-rewrite")
	ctx = context.WithValue(ctx, LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:         "implementation/broad/backend",
		Reason:             "learned route policy for recurring implementation/broad/backend turns",
		RequireContextRead: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "context-read-rewrite-1",
		Name:  SingleEditToolName,
		Input: `{"file_path":"cmd/api/main.go","old_string":"before","new_string":"after"}`,
	}, map[string]fantasy.AgentTool{SingleEditToolName: editTool, SingleViewToolName: viewTool})
	require.NoError(t, err)
	require.Equal(t, SingleViewToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"file_path":"cmd/api/main.go"`)
}

func TestPrepareToolCallAllowsProtectedToolsAfterHarnessDecision(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), MessageIDContextKey, "msg-harness-approved")
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Required:        true,
		Reason:          "multi-phase non-trivial task",
		ComplexityScore: 5,
	})
	RecordHarnessDecision(ctx, HarnessDecision{
		Required:        true,
		ComplexityScore: 5,
		Pattern:         "planner_executor_reviewer",
	})

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "edit-harness-2",
		Name:  EditToolName,
		Input: `{"file_path":"README.md","old_string":"alpha","new_string":"beta"}`,
	}, map[string]fantasy.AgentTool{EditToolName: editTool})
	require.NoError(t, err)
	require.Equal(t, EditToolName, prepared.Name)
}

func TestHarnessDecisionFallsBackToSessionContext(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session-harness")
	RecordHarnessDecision(ctx, HarnessDecision{
		Required:        true,
		ComplexityScore: 3,
		Pattern:         "planner_executor_reviewer",
	})

	decision, ok := GetHarnessDecision(ctx)
	require.True(t, ok)
	require.Equal(t, "planner_executor_reviewer", decision.Pattern)
}

func TestGetToolUsageStateFallsBackToSharedSessionState(t *testing.T) {
	t.Parallel()

	state := ResetSharedToolUsageState("session-usage")
	t.Cleanup(func() {
		ClearSharedToolUsageState("session-usage")
	})
	state.Increment(ToolSearchToolName)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session-usage")
	loaded := GetToolUsageStateFromContext(ctx)
	require.NotNil(t, loaded)
	require.Equal(t, 1, loaded.Count(ToolSearchToolName))
}

func TestPrepareToolCallRewritesDiscoveryToHarnessOnBroadInitialization(t *testing.T) {
	t.Parallel()

	searchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	runHarnessTool := fantasy.NewAgentTool(
		RunHarnessToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), MessageIDContextKey, "msg-harness-init")
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Required:               true,
		Reason:                 "broad codebase initialization",
		ComplexityScore:        3,
		Task:                   "Initialize the codebase thoroughly.",
		RequireBeforeDiscovery: true,
	})

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "search-harness-1",
		Name:  ToolSearchToolName,
		Input: `{"query":"routes"}`,
	}, map[string]fantasy.AgentTool{ToolSearchToolName: searchTool, RunHarnessToolName: runHarnessTool})
	require.NoError(t, err)
	require.Equal(t, RunHarnessToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"task":"Initialize the codebase thoroughly."`)
}

func TestPrepareToolCallRejectsLearnedDiscoveryBash(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:          "implementation/broad/backend",
		Reason:              "learned route policy for repeated large-repo implementation turns",
		ForbidBashDiscovery: true,
	})

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-bash-1",
		Name:  BashToolName,
		Input: `{"command":"find . -name '*.go'","description":"discover files"}`,
	}, map[string]fantasy.AgentTool{BashToolName: bashTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "discovery-oriented bash is blocked")
	require.Contains(t, err.Error(), "`tool_search`")
}

func TestPrepareToolCallRejectsRepeatedLSBeforeStructuredDiscovery(t *testing.T) {
	t.Parallel()

	lsTool := fantasy.NewAgentTool(
		LSToolName,
		"",
		func(ctx context.Context, params LSParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(LSToolName)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                "initialize/broad/codebase",
		Reason:                    "learned route policy for recurring initialize/broad/codebase turns",
		PreferStructuredDiscovery: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-ls-1",
		Name:  LSToolName,
		Input: `{"path":"."}`,
	}, map[string]fantasy.AgentTool{LSToolName: lsTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repeated `ls` browsing is blocked")
}

func TestPrepareToolCallRewritesLSDiscoveryToToolSearchWhenAvailable(t *testing.T) {
	t.Parallel()

	lsTool := fantasy.NewAgentTool(
		LSToolName,
		"",
		func(ctx context.Context, params LSParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	toolSearchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                "initialize/broad/codebase",
		Reason:                    "learned route policy for recurring initialize/broad/codebase turns",
		PreferStructuredDiscovery: true,
	})
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Task: "Initialize this codebase thoroughly.",
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, NewToolUsageState())

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-ls-rewrite-1",
		Name:  LSToolName,
		Input: `{"path":"."}`,
	}, map[string]fantasy.AgentTool{LSToolName: lsTool, ToolSearchToolName: toolSearchTool})
	require.NoError(t, err)
	require.Equal(t, ToolSearchToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"query":"Initialize this codebase thoroughly."`)
}

func TestPrepareToolCallRewritesLSDiscoveryToRGFilesWhenToolSearchUnavailable(t *testing.T) {
	t.Parallel()

	lsTool := fantasy.NewAgentTool(
		LSToolName,
		"",
		func(ctx context.Context, params LSParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	rgFilesTool := fantasy.NewAgentTool(
		RGFilesToolName,
		"",
		func(ctx context.Context, params RGFilesParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                "initialize/broad/codebase",
		Reason:                    "learned route policy for recurring initialize/broad/codebase turns",
		PreferStructuredDiscovery: true,
	})
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Task: "Initialize this codebase thoroughly.",
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, NewToolUsageState())

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-ls-rgfiles-1",
		Name:  LSToolName,
		Input: `{"path":"."}`,
	}, map[string]fantasy.AgentTool{LSToolName: lsTool, RGFilesToolName: rgFilesTool})
	require.NoError(t, err)
	require.Equal(t, RGFilesToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "readme agents package go mod cmd internal app src config main", input["query"])
	require.Equal(t, float64(40), input["limit"])
}

func TestPrepareToolCallRewritesInitializationSkillDetourToToolSearch(t *testing.T) {
	t.Parallel()

	searchSkillsTool := fantasy.NewAgentTool(
		"search_skills",
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	toolSearchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                "initialize/broad/codebase",
		Reason:                    "learned route policy for recurring initialize/broad/codebase turns",
		PreferStructuredDiscovery: true,
	})
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Task: "Initialize this codebase thoroughly.",
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, NewToolUsageState())

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-skill-rewrite-1",
		Name:  "search_skills",
		Input: `{"query":"architecture"}`,
	}, map[string]fantasy.AgentTool{"search_skills": searchSkillsTool, ToolSearchToolName: toolSearchTool})
	require.NoError(t, err)
	require.Equal(t, ToolSearchToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"query":"Initialize this codebase thoroughly."`)
}

func TestPrepareToolCallRewritesInitializationSkillDetourToRGFilesWhenToolSearchUnavailable(t *testing.T) {
	t.Parallel()

	searchSkillsTool := fantasy.NewAgentTool(
		"search_skills",
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	rgFilesTool := fantasy.NewAgentTool(
		RGFilesToolName,
		"",
		func(ctx context.Context, params RGFilesParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                "initialize/broad/codebase",
		Reason:                    "learned route policy for recurring initialize/broad/codebase turns",
		PreferStructuredDiscovery: true,
	})
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Task: "Initialize this codebase thoroughly.",
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, NewToolUsageState())

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-skill-rewrite-rgfiles-1",
		Name:  "search_skills",
		Input: `{"query":"architecture"}`,
	}, map[string]fantasy.AgentTool{"search_skills": searchSkillsTool, RGFilesToolName: rgFilesTool})
	require.NoError(t, err)
	require.Equal(t, RGFilesToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "readme agents package go mod cmd internal app src config main", input["query"])
	require.Equal(t, float64(40), input["limit"])
}

func TestPrepareToolCallRewritesRepeatedInitializationSkillToolToRGFiles(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	usage.Increment("list_skills")

	loadSkillTool := fantasy.NewAgentTool(
		LoadSkillToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	rgFilesTool := fantasy.NewAgentTool(
		RGFilesToolName,
		"",
		func(ctx context.Context, params RGFilesParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                "initialize/broad/codebase",
		Reason:                    "learned route policy for recurring initialize/broad/codebase turns",
		PreferStructuredDiscovery: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-skill-block-1",
		Name:  LoadSkillToolName,
		Input: `{"name":"architect"}`,
	}, map[string]fantasy.AgentTool{LoadSkillToolName: loadSkillTool, RGFilesToolName: rgFilesTool})
	require.NoError(t, err)
	require.Equal(t, RGFilesToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "readme agents package go mod cmd internal app src config main", input["query"])
	require.Equal(t, float64(40), input["limit"])
}

func TestPrepareToolCallBlocksRepeatedInitializationSkillToolWithoutStructuredFallback(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	usage.Increment("list_skills")

	loadSkillTool := fantasy.NewAgentTool(
		LoadSkillToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                "initialize/broad/codebase",
		Reason:                    "learned route policy for recurring initialize/broad/codebase turns",
		PreferStructuredDiscovery: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-skill-block-hard-1",
		Name:  LoadSkillToolName,
		Input: `{"name":"architect"}`,
	}, map[string]fantasy.AgentTool{LoadSkillToolName: loadSkillTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not auto-reroute")
}

func TestPrepareToolCallBlocksMutationUntilBroadInitializationContextExists(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:         "initialize/broad/codebase",
		Reason:             "learned route policy for recurring initialize/broad/codebase turns",
		RequireContextRead: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, NewToolUsageState())

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-edit-1",
		Name:  EditToolName,
		Input: `{"file_path":"AGENTS.md","old_string":"old","new_string":"new"}`,
	}, map[string]fantasy.AgentTool{EditToolName: editTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must gather repository evidence")
}

func TestPrepareToolCallAllowsMutationAfterBroadInitializationContextExists(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:         "initialize/broad/codebase",
		Reason:             "learned route policy for recurring initialize/broad/codebase turns",
		RequireContextRead: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-edit-2",
		Name:  EditToolName,
		Input: `{"file_path":"AGENTS.md","old_string":"old","new_string":"new"}`,
	}, map[string]fantasy.AgentTool{EditToolName: editTool})
	require.NoError(t, err)
	require.Equal(t, EditToolName, prepared.Name)
}

func TestPrepareToolCallBlocksNonAgentsWritesDuringBroadInitialization(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		RequireContextRead:           true,
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-edit-docs-1",
		Name:  EditToolName,
		Input: `{"file_path":"docs/overview.md","old_string":"old","new_string":"new"}`,
	}, map[string]fantasy.AgentTool{EditToolName: editTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not mutate unrelated files")
	require.Contains(t, err.Error(), "docs/overview.md")
}

func TestPrepareToolCallRewritesNonAgentsWriteDuringBroadInitializationToUpdatePlan(t *testing.T) {
	t.Parallel()

	writeTool := fantasy.NewAgentTool(
		WriteToolName,
		"",
		func(ctx context.Context, params WriteParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	updatePlanTool := fantasy.NewAgentTool(
		UpdatePlanToolName,
		"",
		func(ctx context.Context, params UpdatePlanArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		RequireContextRead:           true,
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-write-readme-1",
		Name:  WriteToolName,
		Input: `{"file_path":"README.md","content":"# wrong artifact"}`,
	}, map[string]fantasy.AgentTool{WriteToolName: writeTool, UpdatePlanToolName: updatePlanTool})
	require.NoError(t, err)
	require.Equal(t, UpdatePlanToolName, prepared.Name)
	require.Contains(t, prepared.Input, "AGENTS.md only")
	require.Contains(t, prepared.Input, "README.md")
}

func TestPrepareToolCallRewritesInitializationMemoryArtifactReadToToolSearch(t *testing.T) {
	t.Parallel()

	singleViewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	toolSearchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                "initialize/broad/codebase",
		Reason:                    "learned route policy for recurring initialize/broad/codebase turns",
		PreferStructuredDiscovery: true,
	})
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Task: "Initialize this codebase thoroughly.",
	})
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-memory-reroute-1",
		Name:  SingleViewToolName,
		Input: `{"file_path":"memory_summary.md"}`,
	}, map[string]fantasy.AgentTool{SingleViewToolName: singleViewTool, ToolSearchToolName: toolSearchTool})
	require.NoError(t, err)
	require.Equal(t, ToolSearchToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"query":"Initialize this codebase thoroughly."`)
}

func TestPrepareToolCallRewritesRedundantBroadInitializationLSAfterReadPhaseToUpdatePlan(t *testing.T) {
	t.Parallel()

	lsTool := fantasy.NewAgentTool(
		LSToolName,
		"",
		func(ctx context.Context, params LSParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	updatePlanTool := fantasy.NewAgentTool(
		UpdatePlanToolName,
		"",
		func(ctx context.Context, params UpdatePlanArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-discovery-cap-ls-1",
		Name:  LSToolName,
		Input: `{"path":"."}`,
	}, map[string]fantasy.AgentTool{LSToolName: lsTool, UpdatePlanToolName: updatePlanTool})
	require.NoError(t, err)
	require.Equal(t, UpdatePlanToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"Write or refine AGENTS.md only"`)
	require.Contains(t, prepared.Input, `"Verify AGENTS.md after writing"`)
}

func TestPrepareToolCallRewritesRedundantBroadInitializationDiscoveryToVerificationRead(t *testing.T) {
	t.Parallel()

	rgFilesTool := fantasy.NewAgentTool(
		RGFilesToolName,
		"",
		func(ctx context.Context, params RGFilesParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	singleViewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)
	usage.MarkPlanPublished()
	usage.MarkArtifactWrite("/repo/AGENTS.md")

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-discovery-cap-verify-1",
		Name:  RGFilesToolName,
		Input: `{"query":"*.go"}`,
	}, map[string]fantasy.AgentTool{RGFilesToolName: rgFilesTool, SingleViewToolName: singleViewTool})
	require.NoError(t, err)
	require.Equal(t, SingleViewToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"/repo/AGENTS.md"`)
}

func TestPrepareToolCallRewritesForbiddenBroadInitializationDiscoveryBashToToolSearch(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	toolSearchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		ForbidBashDiscovery:          true,
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Task: "Initialize this codebase thoroughly.",
	})

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-bash-reroute-1",
		Name:  BashToolName,
		Input: `{"command":"find . -maxdepth 3 -not -path '*/.*'","description":"discover repo files"}`,
	}, map[string]fantasy.AgentTool{BashToolName: bashTool, ToolSearchToolName: toolSearchTool})
	require.NoError(t, err)
	require.Equal(t, ToolSearchToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"query":"Initialize this codebase thoroughly."`)
}

func TestPrepareToolCallAllowsAgentsWriteDuringBroadInitialization(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		RequireContextRead:           true,
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-edit-agents-1",
		Name:  EditToolName,
		Input: `{"file_path":"AGENTS.md","old_string":"old","new_string":"new"}`,
	}, map[string]fantasy.AgentTool{EditToolName: editTool})
	require.NoError(t, err)
	require.Equal(t, EditToolName, prepared.Name)
}

func TestPrepareToolCallRequiresExplicitPlanAfterBroadDesignSeedContext(t *testing.T) {
	t.Parallel()

	searchTool := fantasy.NewAgentTool(
		RGFilesToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	updatePlanTool := fantasy.NewAgentTool(
		UpdatePlanToolName,
		"",
		func(ctx context.Context, params UpdatePlanArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:          "design/broad/backend+infra",
		Reason:              "learned route policy for recurring design/broad/backend+infra turns",
		RequireExplicitPlan: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-plan-1",
		Name:  RGFilesToolName,
		Input: `{"pattern":"*.go"}`,
	}, map[string]fantasy.AgentTool{RGFilesToolName: searchTool, UpdatePlanToolName: updatePlanTool})
	require.NoError(t, err)
	require.Equal(t, UpdatePlanToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"Collect initial repository evidence"`)
	require.Contains(t, prepared.Input, `"in_progress"`)
}

func TestPrepareToolCallRepairsEmptyUpdatePlanForBroadInitialization(t *testing.T) {
	t.Parallel()

	updatePlanTool := fantasy.NewAgentTool(
		UpdatePlanToolName,
		"",
		func(ctx context.Context, params UpdatePlanArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-empty-plan-init-1",
		Name:  UpdatePlanToolName,
		Input: `{"plan":[{"step":"","status":"in_progress"}]}`,
	}, map[string]fantasy.AgentTool{UpdatePlanToolName: updatePlanTool})
	require.NoError(t, err)
	require.Equal(t, UpdatePlanToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"Collect repository evidence for AGENTS.md"`)
	require.Contains(t, prepared.Input, `"Identify core entrypoints, domains, and constraints"`)
}

func TestPrepareToolCallAllowsBroadDesignContinuationAfterPlanPublished(t *testing.T) {
	t.Parallel()

	searchTool := fantasy.NewAgentTool(
		RGFilesToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)
	usage.MarkPlanPublished()

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:          "design/broad/backend+infra",
		Reason:              "learned route policy for recurring design/broad/backend+infra turns",
		RequireExplicitPlan: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "learned-plan-2",
		Name:  RGFilesToolName,
		Input: `{"pattern":"*.go"}`,
	}, map[string]fantasy.AgentTool{RGFilesToolName: searchTool})
	require.NoError(t, err)
	require.Equal(t, RGFilesToolName, prepared.Name)
}

func TestPrepareToolCallRewritesLateBroadImplementationDiscoveryToVerificationRead(t *testing.T) {
	t.Parallel()

	toolSearchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	singleViewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)
	usage.MarkPlanPublished()
	usage.MarkArtifactWrite("/repo/internal/platform/runtime.go")
	usage.RecordDeterministicToolCall(AgenticEditToolName)
	usage.RecordDeterministicWrite("/repo/internal/platform/runtime.go", false, false)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:          "implementation/broad/backend",
		Reason:              "learned route policy for recurring implementation/broad/backend turns",
		RequireContextRead:  true,
		RequireExplicitPlan: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, TurnStepOrdinalContextKey, 10)
	ctx = context.WithValue(ctx, TurnStepBudgetContextKey, 12)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "impl-late-discovery-1",
		Name:  ToolSearchToolName,
		Input: `{"query":"runtime config"}`,
	}, map[string]fantasy.AgentTool{ToolSearchToolName: toolSearchTool, SingleViewToolName: singleViewTool})
	require.NoError(t, err)
	require.Equal(t, SingleViewToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"/repo/internal/platform/runtime.go"`)
}

func TestShouldForceLateImplementationExecutionFocusWithoutWrites(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	usage.MarkReadEvidence("/repo/internal/platform/runtime.go")
	usage.MarkPlanPublished()
	usage.Increment(ToolSearchToolName)
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)
	usage.Increment(UpdatePlanToolName)
	usage.Increment(GrepToolName)
	usage.RecordDeterministicToolCall(ToolSearchToolName)
	usage.RecordDeterministicToolCall(AgenticViewToolName)
	usage.RecordDeterministicRead("/repo/internal/platform/runtime.go")
	usage.RecordDeterministicRead("/repo/internal/platform/runtime.go")

	ctx := context.WithValue(context.Background(), TurnStepOrdinalContextKey, 9)
	ctx = context.WithValue(ctx, TurnStepBudgetContextKey, 12)

	require.True(t, shouldForceLateImplementationExecutionFocus(ctx, usage))
}

func TestPrepareToolCallRewritesLateBroadImplementationEditChurnToVerificationRead(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		AgenticEditToolName,
		"",
		func(ctx context.Context, params MultiEditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	singleViewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)
	usage.MarkPlanPublished()
	usage.MarkArtifactWrite("/repo/internal/platform/runtime.go")
	usage.RecordDeterministicToolCall(AgenticEditToolName)
	usage.RecordDeterministicWrite("/repo/internal/platform/runtime.go", false, false)
	usage.RecordDeterministicToolCall(SingleEditToolName)
	usage.RecordDeterministicWrite("/repo/internal/platform/runtime.go", false, false)
	usage.RecordDeterministicToolCall(SingleEditToolName)
	usage.RecordDeterministicWrite("/repo/internal/platform/runtime.go", false, false)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:          "implementation/broad/backend",
		Reason:              "learned route policy for recurring implementation/broad/backend turns",
		RequireContextRead:  true,
		RequireExplicitPlan: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, TurnStepOrdinalContextKey, 11)
	ctx = context.WithValue(ctx, TurnStepBudgetContextKey, 12)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "impl-late-edit-1",
		Name:  AgenticEditToolName,
		Input: `{"file_path":"internal/platform/runtime.go","edits":[{"old_string":"old","new_string":"new"}]}`,
	}, map[string]fantasy.AgentTool{AgenticEditToolName: editTool, SingleViewToolName: singleViewTool})
	require.NoError(t, err)
	require.Equal(t, SingleViewToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"/repo/internal/platform/runtime.go"`)
}

func TestPrepareToolCallRewritesLateBroadInitializationCompoundBashToVerificationRead(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	singleViewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)
	usage.MarkPlanPublished()
	usage.MarkArtifactWrite("/repo/AGENTS.md")

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		ForbidBashDiscovery:          true,
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, TurnStepOrdinalContextKey, 11)
	ctx = context.WithValue(ctx, TurnStepBudgetContextKey, 12)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-late-bash-verify-1",
		Name:  BashToolName,
		Input: `{"command":"cd /repo && find . -maxdepth 3 -type f | head -n 20","description":"discover repo files"}`,
	}, map[string]fantasy.AgentTool{BashToolName: bashTool, SingleViewToolName: singleViewTool})
	require.NoError(t, err)
	require.Equal(t, SingleViewToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"/repo/AGENTS.md"`)
}

func TestPrepareToolCallRewritesLateBroadInitializationCompoundBashToUpdatePlanWithoutPendingArtifact(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	updatePlanTool := fantasy.NewAgentTool(
		UpdatePlanToolName,
		"",
		func(ctx context.Context, params UpdatePlanArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)
	usage.MarkPlanPublished()

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		ForbidBashDiscovery:          true,
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, TurnStepOrdinalContextKey, 11)
	ctx = context.WithValue(ctx, TurnStepBudgetContextKey, 12)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-late-bash-plan-1",
		Name:  BashToolName,
		Input: `{"command":"cd /repo && find . -maxdepth 3 -type f | head -n 20","description":"discover repo files"}`,
	}, map[string]fantasy.AgentTool{BashToolName: bashTool, UpdatePlanToolName: updatePlanTool})
	require.NoError(t, err)
	require.Equal(t, UpdatePlanToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"Write or refine AGENTS.md only"`)
}

func TestPrepareToolCallRewritesLateBroadInitializationGitLogBashToUpdatePlan(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	updatePlanTool := fantasy.NewAgentTool(
		UpdatePlanToolName,
		"",
		func(ctx context.Context, params UpdatePlanArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(ToolSearchToolName)
	usage.Increment(AgenticViewToolName)
	usage.MarkPlanPublished()

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		Reason:                       "learned route policy for recurring initialize/broad/codebase turns",
		ForbidBashDiscovery:          true,
		RequirePostWriteVerification: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, TurnStepOrdinalContextKey, 11)
	ctx = context.WithValue(ctx, TurnStepBudgetContextKey, 12)

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "init-late-gitlog-1",
		Name:  BashToolName,
		Input: `{"command":"git log --oneline -5","description":"check recent changes"}`,
	}, map[string]fantasy.AgentTool{BashToolName: bashTool, UpdatePlanToolName: updatePlanTool})
	require.NoError(t, err)
	require.Equal(t, UpdatePlanToolName, prepared.Name)
	require.Contains(t, prepared.Input, `"Write or refine AGENTS.md only"`)
}

func TestPrepareToolCallRewritesBroadDesignSkillDetourToStructuredDiscovery(t *testing.T) {
	t.Parallel()

	searchSkillsTool := fantasy.NewAgentTool(
		"search_skills",
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	toolSearchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:         "design/broad/backend",
		Reason:             "learned route policy for recurring design/broad/backend turns",
		RequireContextRead: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, NewToolUsageState())
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Task: "Architecture-only task: compare two designs for the singularity benchmark lane.",
	})

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "design-skill-detour-1",
		Name:  "search_skills",
		Input: `{"query":"architect"}`,
	}, map[string]fantasy.AgentTool{"search_skills": searchSkillsTool, ToolSearchToolName: toolSearchTool})
	require.NoError(t, err)
	require.Equal(t, ToolSearchToolName, prepared.Name)
	require.Contains(t, prepared.Input, "Architecture-only task")
}

func TestPrepareToolCallRewritesSecondBroadDesignReadDetourToStructuredDiscovery(t *testing.T) {
	t.Parallel()

	agenticViewTool := fantasy.NewAgentTool(
		AgenticViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	toolSearchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	usage := NewToolUsageState()
	usage.MarkReadEvidence("/repo/internal/agent/singularity_learning.go")

	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:         "design/broad/backend",
		Reason:             "learned route policy for recurring design/broad/backend turns",
		RequireContextRead: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Task: "Architecture-only task: compare two designs for the singularity benchmark lane.",
	})

	prepared, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "design-read-detour-1",
		Name:  AgenticViewToolName,
		Input: `{"file_paths":["internal/agent/singularity_admin.go"]}`,
	}, map[string]fantasy.AgentTool{AgenticViewToolName: agenticViewTool, ToolSearchToolName: toolSearchTool})
	require.NoError(t, err)
	require.Equal(t, ToolSearchToolName, prepared.Name)
	require.Contains(t, prepared.Input, "Architecture-only task")
}

func TestWrapRuntimePreflightToolsAppliesHarnessGuardrail(t *testing.T) {
	t.Parallel()

	searchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	runHarnessTool := fantasy.NewAgentTool(
		RunHarnessToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{Content: "harness ok"}, nil
		},
	)

	wrapped := WrapRuntimePreflightTools([]fantasy.AgentTool{searchTool, runHarnessTool})

	ctx := context.WithValue(context.Background(), MessageIDContextKey, "msg-runtime-harness")
	ctx = context.WithValue(ctx, HarnessRequirementContextKey, HarnessRequirement{
		Required:               true,
		Reason:                 "broad codebase initialization",
		ComplexityScore:        3,
		Task:                   "Initialize the codebase thoroughly.",
		RequireBeforeDiscovery: true,
	})

	resp, err := wrapped[0].Run(ctx, fantasy.ToolCall{
		ID:    "runtime-search-1",
		Name:  ToolSearchToolName,
		Input: `{"query":"router"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "harness ok", resp.Content)
	meta, ok := ParseRuntimeExecutionMetadata(resp.Metadata)
	require.True(t, ok)
	require.Equal(t, ToolSearchToolName, meta.RequestedToolName)
	require.Equal(t, RunHarnessToolName, meta.ExecutedToolName)
	require.True(t, meta.Rewritten)
}

func TestWrapRuntimePreflightToolsAnnotatesStructuredDiscoveryRewrite(t *testing.T) {
	t.Parallel()

	lsTool := fantasy.NewAgentTool(
		LSToolName,
		"",
		func(ctx context.Context, params LSParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	toolSearchTool := fantasy.NewAgentTool(
		ToolSearchToolName,
		"",
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{Content: "search ok"}, nil
		},
	)

	usage := NewToolUsageState()
	usage.Increment(LSToolName)
	ctx := context.WithValue(context.Background(), LearnedToolPolicyContextKey, LearnedToolPolicy{
		TaskFamily:                "initialize/broad/codebase",
		Reason:                    "learned route policy for recurring initialize/broad/codebase turns",
		PreferStructuredDiscovery: true,
	})
	ctx = context.WithValue(ctx, ToolUsageStateContextKey, usage)

	wrapped := WrapRuntimePreflightTools([]fantasy.AgentTool{lsTool, toolSearchTool})
	resp, err := wrapped[0].Run(ctx, fantasy.ToolCall{
		ID:    "runtime-ls-1",
		Name:  LSToolName,
		Input: `{"path":"."}`,
	})
	require.NoError(t, err)
	require.Equal(t, "search ok", resp.Content)

	meta, ok := ParseRuntimeExecutionMetadata(resp.Metadata)
	require.True(t, ok)
	require.Equal(t, LSToolName, meta.RequestedToolName)
	require.Equal(t, ToolSearchToolName, meta.ExecutedToolName)
	require.True(t, meta.Rewritten)
	require.NotEmpty(t, meta.ExecutedInput)
}

func TestPrepareToolCallBlocksMemoryReadToolsWhenPolicyDisallows(t *testing.T) {
	t.Parallel()

	viewMemoryTool := fantasy.NewAgentTool(
		"view_memory",
		"",
		func(ctx context.Context, params struct {
			Mode string `json:"mode,omitempty"`
		}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), TurnPolicyContextKey, TurnPolicy{
		AllowMemoryRead:          false,
		AllowMemoryWrite:         false,
		AllowAutoMemoryInjection: false,
	})

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "memory-read-1",
		Name:  "view_memory",
		Input: `{"mode":"recent"}`,
	}, map[string]fantasy.AgentTool{"view_memory": viewMemoryTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "durable memory reads are blocked")
}

func TestPrepareToolCallBlocksMemoryArtifactReadWhenPolicyDisallows(t *testing.T) {
	t.Parallel()

	singleViewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), TurnPolicyContextKey, TurnPolicy{
		AllowMemoryRead:          false,
		AllowMemoryWrite:         false,
		AllowAutoMemoryInjection: false,
	})
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "memory-file-1",
		Name:  SingleViewToolName,
		Input: `{"file_path":".sapphire-memory/memory_summary.md"}`,
	}, map[string]fantasy.AgentTool{SingleViewToolName: singleViewTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "durable memory file access is blocked")
}

func TestPrepareToolCallRejectsBareRepoMemoryAliasPaths(t *testing.T) {
	t.Parallel()

	singleViewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), TurnPolicyContextKey, DefaultTurnPolicy())
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "memory-file-2",
		Name:  SingleViewToolName,
		Input: `{"file_path":"memory_summary.md"}`,
	}, map[string]fantasy.AgentTool{SingleViewToolName: singleViewTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), ".sapphire-memory/memory_summary.md")
}

func TestPrepareToolCallBlocksToolsWhenTurnIsDirectResponseOnly(t *testing.T) {
	t.Parallel()

	viewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)

	ctx := context.WithValue(context.Background(), TurnPolicyContextKey, TurnPolicy{
		DirectResponseOnly:       true,
		AllowMemoryRead:          false,
		AllowMemoryWrite:         false,
		AllowAutoMemoryInjection: false,
	})

	_, _, err := PrepareToolCall(ctx, fantasy.ToolCall{
		ID:    "casual-1",
		Name:  SingleViewToolName,
		Input: `{"file_path":"README.md"}`,
	}, map[string]fantasy.AgentTool{SingleViewToolName: viewTool})
	require.Error(t, err)
	require.Contains(t, err.Error(), "casual conversation only")
}

func TestPrepareToolCallDoesNotRewriteSingleAgenticViewToView(t *testing.T) {
	t.Parallel()

	viewTool := fantasy.NewAgentTool(
		ViewToolName,
		"",
		func(ctx context.Context, params struct {
			FilePath string `json:"file_path"`
		}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	agenticViewTool := fantasy.NewAgentTool(
		AgenticViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		ViewToolName:        viewTool,
		AgenticViewToolName: agenticViewTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "view-1",
		Name:  AgenticViewToolName,
		Input: `{"file_path":"README.md"}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, AgenticViewToolName, prepared.Name)
}

func TestPrepareToolCallRewritesHeadBashToSingleView(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	singleViewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		BashToolName:       bashTool,
		SingleViewToolName: singleViewTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "bash-head-1",
		Name:  BashToolName,
		Input: `{"command":"head -n 80 AGENTS.md","description":"read file"}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, SingleViewToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "AGENTS.md", input["file_path"])
	require.Equal(t, float64(80), input["limit"])
}

func TestPrepareToolCallRewritesFindNameBashToRGFiles(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	rgFilesTool := fantasy.NewAgentTool(
		RGFilesToolName,
		"",
		func(ctx context.Context, params RGFilesParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		BashToolName:    bashTool,
		RGFilesToolName: rgFilesTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "bash-find-1",
		Name:  BashToolName,
		Input: `{"command":"find internal -name \"*mcp*\"","description":"discover files"}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, RGFilesToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "internal", input["path"])
	require.Equal(t, "*mcp*", input["query"])
}

func TestPrepareToolCallRewritesLSFlagsBashToStructuredLS(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	lsTool := fantasy.NewAgentTool(
		LSToolName,
		"",
		func(ctx context.Context, params LSParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		BashToolName: bashTool,
		LSToolName:   lsTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "bash-ls-1",
		Name:  BashToolName,
		Input: `{"command":"ls -la .sapphire","description":"inspect tree"}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, LSToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, ".sapphire", input["path"])
}

func TestPrepareToolCallRewritesRGShellSearchToRG(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	rgTool := fantasy.NewAgentTool(
		RGToolName,
		"",
		func(ctx context.Context, params RGParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		BashToolName: bashTool,
		RGToolName:   rgTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "bash-rg-1",
		Name:  BashToolName,
		Input: `{"command":"rg -l -i \"mistake\" --type go internal/agent","description":"search code"}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, RGToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "mistake", input["pattern"])
	require.Equal(t, "*.go", input["include"])
	require.Equal(t, "internal/agent", input["path"])
	require.Equal(t, false, input["case_sensitive"])
}

func TestPrepareToolCallRewritesCatMultiFileBashToAgenticView(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	agenticViewTool := fantasy.NewAgentTool(
		AgenticViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		BashToolName:        bashTool,
		AgenticViewToolName: agenticViewTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "bash-cat-1",
		Name:  BashToolName,
		Input: `{"command":"cat README.md AGENTS.md","description":"read files"}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, AgenticViewToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, []any{"README.md", "AGENTS.md"}, input["file_paths"])
}

func TestPrepareToolCallRewritesSedSliceBashToSingleView(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	singleViewTool := fantasy.NewAgentTool(
		SingleViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		BashToolName:       bashTool,
		SingleViewToolName: singleViewTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "bash-sed-1",
		Name:  BashToolName,
		Input: `{"command":"sed -n '12,20p' internal/agent/agent.go","description":"read slice"}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, SingleViewToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "internal/agent/agent.go", input["file_path"])
	require.Equal(t, float64(12), input["offset"])
	require.Equal(t, float64(9), input["limit"])
}

func TestPrepareToolCallRejectsBashRepoReadCompoundCommand(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		BashToolName: bashTool,
	}

	_, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "bash-reject-1",
		Name:  BashToolName,
		Input: `{"command":"find internal -name \"*mcp*\" && find internal -maxdepth 1 -type d","description":"inspect repo"}`,
	}, registry)
	require.Error(t, err)
	require.Contains(t, err.Error(), "do not use bash for repository discovery")
}

func TestPrepareToolCallCoercesRecallMemoryLimitStringToInt(t *testing.T) {
	t.Parallel()

	recallTool := fantasy.NewAgentTool(
		"recall_memory",
		"",
		func(ctx context.Context, params struct {
			Query string `json:"query"`
			Limit int    `json:"limit,omitempty"`
		}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		"recall_memory": recallTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "recall-memory-1",
		Name:  "recall_memory",
		Input: `{"query":"mistake","limit":"10"}`,
	}, registry)
	require.NoError(t, err)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "mistake", input["query"])
	require.Equal(t, float64(10), input["limit"])
}

func TestPrepareToolCallDoesNotRewriteMultiPathViewToAgenticView(t *testing.T) {
	t.Parallel()

	viewTool := fantasy.NewAgentTool(
		ViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	agenticViewTool := fantasy.NewAgentTool(
		AgenticViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		ViewToolName:        viewTool,
		AgenticViewToolName: agenticViewTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "view-2",
		Name:  ViewToolName,
		Input: `{"file_paths":["a.go","b.go"]}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, ViewToolName, prepared.Name)
}

func TestPrepareToolCallPromotesLargeViewBatchToAgenticView(t *testing.T) {
	t.Parallel()

	viewTool := fantasy.NewAgentTool(
		ViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	agenticViewTool := fantasy.NewAgentTool(
		AgenticViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		ViewToolName:        viewTool,
		AgenticViewToolName: agenticViewTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "view-2b",
		Name:  ViewToolName,
		Input: `{"file_paths":["a.go","b.go","c.go"]}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, AgenticViewToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	gotPaths, ok := input["file_paths"].([]any)
	require.True(t, ok)
	require.Len(t, gotPaths, 3)
}

func TestPrepareToolCallDoesNotRewriteSingleAgenticEditToEdit(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	agenticEditTool := fantasy.NewAgentTool(
		AgenticEditToolName,
		"",
		func(ctx context.Context, params MultiEditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		EditToolName:        editTool,
		AgenticEditToolName: agenticEditTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "edit-3",
		Name:  AgenticEditToolName,
		Input: `{"file_path":"README.md","old_string":"alpha","new_string":"beta"}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, AgenticEditToolName, prepared.Name)
}

func TestPrepareToolCallPromotesExplicitMultiEditShapeToAgenticEdit(t *testing.T) {
	t.Parallel()

	editTool := fantasy.NewAgentTool(
		EditToolName,
		"",
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	agenticEditTool := fantasy.NewAgentTool(
		AgenticEditToolName,
		"",
		func(ctx context.Context, params MultiEditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		EditToolName:        editTool,
		AgenticEditToolName: agenticEditTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "edit-4",
		Name:  EditToolName,
		Input: `{"file_path":"README.md","edits":[{"old_string":"alpha","new_string":"beta"},{"old_string":"gamma","new_string":"delta"}]}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, AgenticEditToolName, prepared.Name)
}

func TestPrepareToolCallDoesNotTruncateAgenticViewPaths(t *testing.T) {
	t.Parallel()

	agenticViewTool := fantasy.NewAgentTool(
		AgenticViewToolName,
		"",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		AgenticViewToolName: agenticViewTool,
	}

	paths := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		paths = append(paths, fmt.Sprintf("file_%02d.go", i))
	}
	inputBytes, err := json.Marshal(map[string]any{"file_paths": paths})
	require.NoError(t, err)

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "view-3",
		Name:  AgenticViewToolName,
		Input: string(inputBytes),
	}, registry)
	require.NoError(t, err)
	require.Equal(t, AgenticViewToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	gotPaths, ok := input["file_paths"].([]any)
	require.True(t, ok)
	require.Len(t, gotPaths, 30)
}

func TestPrepareToolCallPromotesTopLevelEditsFileEditShapeForAgenticEdit(t *testing.T) {
	t.Parallel()

	agenticEditTool := fantasy.NewAgentTool(
		AgenticEditToolName,
		"",
		func(ctx context.Context, params MultiEditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		AgenticEditToolName: agenticEditTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "edit-5",
		Name:  AgenticEditToolName,
		Input: `{"edits":[{"file_path":"README.md","old_string":"alpha","new_string":"beta"}]}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, AgenticEditToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	_, hasFileEdits := input["file_edits"]
	require.True(t, hasFileEdits)
	_, hasEdits := input["edits"]
	require.False(t, hasEdits)
}

func TestPrepareToolCallNormalizesFetchAndDownloadAliases(t *testing.T) {
	t.Parallel()

	fetchTool := fantasy.NewAgentTool(
		FetchToolName,
		"",
		func(ctx context.Context, params FetchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	downloadTool := fantasy.NewAgentTool(
		DownloadToolName,
		"",
		func(ctx context.Context, params DownloadParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		FetchToolName:    fetchTool,
		DownloadToolName: downloadTool,
	}

	fetchPrepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "fetch-1",
		Name:  FetchToolName,
		Input: `{"address":"https://example.com","output":"markdown","timeout_seconds":12}`,
	}, registry)
	require.NoError(t, err)

	var fetchInput map[string]any
	require.NoError(t, json.Unmarshal([]byte(fetchPrepared.Input), &fetchInput))
	require.Equal(t, "https://example.com", fetchInput["url"])
	require.Equal(t, "markdown", fetchInput["format"])
	require.Equal(t, float64(12), fetchInput["timeout"])

	downloadPrepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "download-1",
		Name:  DownloadToolName,
		Input: `{"source":"https://example.com/file.txt","output":"artifact.txt"}`,
	}, registry)
	require.NoError(t, err)

	var downloadInput map[string]any
	require.NoError(t, json.Unmarshal([]byte(downloadPrepared.Input), &downloadInput))
	require.Equal(t, "https://example.com/file.txt", downloadInput["url"])
	require.Equal(t, "artifact.txt", downloadInput["file_path"])
}

func TestPrepareToolCallNormalizesAgenticFetchAndWebSearchAliases(t *testing.T) {
	t.Parallel()

	agenticFetchTool := fantasy.NewAgentTool(
		AgenticFetchToolName,
		"",
		func(ctx context.Context, params AgenticFetchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	webSearchTool := fantasy.NewAgentTool(
		WebSearchToolName,
		"",
		func(ctx context.Context, params WebSearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		AgenticFetchToolName: agenticFetchTool,
		WebSearchToolName:    webSearchTool,
	}

	agenticPrepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "agentic-fetch-1",
		Name:  AgenticFetchToolName,
		Input: `{"links":"https://example.com","query":"summarize the page"}`,
	}, registry)
	require.NoError(t, err)

	var agenticInput map[string]any
	require.NoError(t, json.Unmarshal([]byte(agenticPrepared.Input), &agenticInput))
	require.Equal(t, "https://example.com", agenticInput["url"])
	require.Equal(t, "summarize the page", agenticInput["prompt"])

	webPrepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "web-search-1",
		Name:  WebSearchToolName,
		Input: `{"search":"runtime control","limit":7}`,
	}, registry)
	require.NoError(t, err)

	var webInput map[string]any
	require.NoError(t, json.Unmarshal([]byte(webPrepared.Input), &webInput))
	require.Equal(t, "runtime control", webInput["query"])
	require.Equal(t, float64(7), webInput["max_results"])
}

func TestPrepareToolCallParsesStringifiedMCPArguments(t *testing.T) {
	t.Parallel()

	mcpTool := fantasy.NewAgentTool(
		CallMCPToolName,
		"",
		func(ctx context.Context, params CallMCPToolParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		CallMCPToolName: mcpTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "mcp-1",
		Name:  CallMCPToolName,
		Input: `{"server":"neon","mcp_tool":"query","args":"{\"sql\":\"select 1\"}"}`,
	}, registry)
	require.NoError(t, err)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "neon", input["mcp_name"])
	require.Equal(t, "query", input["tool_name"])
	require.IsType(t, map[string]any{}, input["arguments"])
}

func TestPrepareToolCallInfersConnectMCPNameFromDescription(t *testing.T) {
	t.Parallel()

	connectTool := fantasy.NewAgentTool(
		ConnectMCPToolName,
		"",
		func(ctx context.Context, params ConnectMCPParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		ConnectMCPToolName: connectTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:   "mcp-connect-1",
		Name: ConnectMCPToolName,
		Input: `{
			"description":"A managed MCP server enabling AI agents to access AWS using docs and API calls. Use io.github.aws/aws-mcp."
		}`,
	}, registry)
	require.NoError(t, err)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "io.github.aws/aws-mcp", input["mcp_name"])
}

func TestPrepareToolCallNormalizesCollectResultAliases(t *testing.T) {
	t.Parallel()

	collectTool := fantasy.NewAgentTool(
		"collect_result",
		"",
		func(ctx context.Context, params struct {
			IDs []string `json:"ids"`
		}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		"collect_result": collectTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "collect-1",
		Name:  "collect_result",
		Input: `{"agent_ids":["agent-1","agent-2"]}`,
	}, registry)
	require.NoError(t, err)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	gotIDs, ok := input["ids"].([]any)
	require.True(t, ok)
	require.Len(t, gotIDs, 2)
}
