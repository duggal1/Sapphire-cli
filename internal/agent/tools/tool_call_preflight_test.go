package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"charm.land/fantasy"
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

func TestPrepareToolCallRewritesFindNameBashToGlob(t *testing.T) {
	t.Parallel()

	bashTool := fantasy.NewAgentTool(
		BashToolName,
		"",
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	globTool := fantasy.NewAgentTool(
		GlobToolName,
		"",
		func(ctx context.Context, params GlobParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, nil
		},
	)
	registry := map[string]fantasy.AgentTool{
		BashToolName: bashTool,
		GlobToolName: globTool,
	}

	prepared, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "bash-find-1",
		Name:  BashToolName,
		Input: `{"command":"find internal -name \"*mcp*\"","description":"discover files"}`,
	}, registry)
	require.NoError(t, err)
	require.Equal(t, GlobToolName, prepared.Name)

	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(prepared.Input), &input))
	require.Equal(t, "internal", input["path"])
	require.Equal(t, "**/*mcp*", input["pattern"])
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
