package tools

import (
	"context"

	"charm.land/fantasy"
)

type runtimePreflightTool struct {
	base    fantasy.AgentTool
	toolMap map[string]fantasy.AgentTool
}

func (t runtimePreflightTool) Info() fantasy.ToolInfo {
	return t.base.Info()
}

func (t runtimePreflightTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	prepared, preparedTool, err := PrepareToolCall(ctx, call, t.toolMap)
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if preparedTool == nil {
		preparedTool = t.base
	}
	result, err := preparedTool.Run(ctx, prepared)
	if err != nil {
		return fantasy.ToolResponse{}, WrapRuntimeExecutionError(err, call, prepared)
	}
	result.Metadata = AnnotateRuntimeExecutionMetadata(result.Metadata, call, prepared)
	return result, nil
}

func (t runtimePreflightTool) ProviderOptions() fantasy.ProviderOptions {
	return t.base.ProviderOptions()
}

func (t runtimePreflightTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.base.SetProviderOptions(opts)
}

func WrapRuntimePreflightTools(items []fantasy.AgentTool) []fantasy.AgentTool {
	if len(items) == 0 {
		return nil
	}
	toolMap := make(map[string]fantasy.AgentTool, len(items))
	for _, tool := range items {
		if tool == nil {
			continue
		}
		toolMap[tool.Info().Name] = tool
	}
	wrapped := make([]fantasy.AgentTool, 0, len(items))
	for _, tool := range items {
		if tool == nil {
			continue
		}
		wrapped = append(wrapped, runtimePreflightTool{
			base:    tool,
			toolMap: toolMap,
		})
	}
	return wrapped
}
