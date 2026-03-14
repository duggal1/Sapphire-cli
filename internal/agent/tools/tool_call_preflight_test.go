package tools

import (
	"context"
	"encoding/json"
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
