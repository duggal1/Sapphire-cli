package tools

import (
	"encoding/json"
	"errors"
	"strings"

	"charm.land/fantasy"
)

const toolErrorMetadataKey = "tool_error"

type ToolErrorMetadata struct {
	ToolName  string `json:"tool_name,omitempty"`
	Code      string `json:"code,omitempty"`
	UIMessage string `json:"ui_message,omitempty"`
}

type ToolGuidanceError struct {
	ToolName     string
	Code         string
	UIMessage    string
	ModelMessage string
}

func (e *ToolGuidanceError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.ModelMessage)
}

func NewToolGuidanceError(toolName, code, uiMessage, modelMessage string) error {
	return &ToolGuidanceError{
		ToolName:     strings.TrimSpace(toolName),
		Code:         strings.TrimSpace(code),
		UIMessage:    strings.TrimSpace(uiMessage),
		ModelMessage: strings.TrimSpace(modelMessage),
	}
}

func NewGuidanceErrorResponse(toolName, code, uiMessage, modelMessage string) fantasy.ToolResponse {
	resp := fantasy.NewTextErrorResponse(strings.TrimSpace(modelMessage))
	return fantasy.WithResponseMetadata(resp, map[string]any{
		toolErrorMetadataKey: ToolErrorMetadata{
			ToolName:  strings.TrimSpace(toolName),
			Code:      strings.TrimSpace(code),
			UIMessage: strings.TrimSpace(uiMessage),
		},
	})
}

func ParseToolErrorMetadata(metadata string) (ToolErrorMetadata, bool) {
	metadata = strings.TrimSpace(metadata)
	if metadata == "" {
		return ToolErrorMetadata{}, false
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(metadata), &root); err != nil {
		return ToolErrorMetadata{}, false
	}

	if raw, ok := root[toolErrorMetadataKey]; ok {
		var parsed ToolErrorMetadata
		if err := json.Unmarshal(raw, &parsed); err == nil && strings.TrimSpace(parsed.UIMessage) != "" {
			return parsed, true
		}
	}

	var parsed ToolErrorMetadata
	if err := json.Unmarshal([]byte(metadata), &parsed); err == nil && strings.TrimSpace(parsed.UIMessage) != "" {
		return parsed, true
	}
	return ToolErrorMetadata{}, false
}

func AnnotateToolErrorMetadata(toolName string, err error, existing string) string {
	if meta, ok := ParseToolErrorMetadata(existing); ok && strings.TrimSpace(meta.UIMessage) != "" {
		return existing
	}

	var meta ToolErrorMetadata
	var guidance *ToolGuidanceError
	if errors.As(err, &guidance) {
		meta = ToolErrorMetadata{
			ToolName:  firstFetchString(guidance.ToolName, toolName),
			Code:      guidance.Code,
			UIMessage: guidance.UIMessage,
		}
	} else {
		derived, ok := DeriveToolErrorMetadata(toolName, err.Error())
		if !ok {
			return existing
		}
		meta = derived
	}

	merged, ok := mergeToolErrorMetadata(existing, meta)
	if !ok {
		return existing
	}
	return merged
}

func UserVisibleToolError(toolName, content, metadata string) string {
	if parsed, ok := ParseToolErrorMetadata(metadata); ok && strings.TrimSpace(parsed.UIMessage) != "" {
		return parsed.UIMessage
	}
	if derived, ok := DeriveToolErrorMetadata(toolName, content); ok && strings.TrimSpace(derived.UIMessage) != "" {
		return derived.UIMessage
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "Tool failed."
	}
	return content
}

func DeriveToolErrorMetadata(toolName, content string) (ToolErrorMetadata, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ToolErrorMetadata{}, false
	}

	normalized := strings.ToLower(trimmed)
	toolName = strings.TrimSpace(toolName)

	switch toolName {
	case BashToolName:
		if strings.Contains(normalized, "command is required") {
			return ToolErrorMetadata{ToolName: toolName, Code: "missing_command", UIMessage: "Missing command."}, true
		}
		if strings.Contains(normalized, "do not use bash for repository discovery") {
			return ToolErrorMetadata{ToolName: toolName, Code: "use_structured_tools", UIMessage: "Use structured tools instead of bash."}, true
		}
	case ViewToolName, SingleViewToolName, AgenticViewToolName:
		if strings.Contains(normalized, "file_path is required") || strings.Contains(normalized, "no file paths provided") || strings.Contains(normalized, "no valid file paths provided") {
			return ToolErrorMetadata{ToolName: toolName, Code: "missing_file_path", UIMessage: "Missing file path."}, true
		}
	case EditToolName, SingleEditToolName:
		if strings.Contains(normalized, "file_path is required") {
			return ToolErrorMetadata{ToolName: toolName, Code: "missing_file_path", UIMessage: "Missing file path."}, true
		}
		if strings.Contains(normalized, "you must read the file before editing") {
			return ToolErrorMetadata{ToolName: toolName, Code: "read_before_edit", UIMessage: "Read the file before editing."}, true
		}
	case AgenticEditToolName:
		if strings.Contains(normalized, "at least one file edit operation is required") || strings.Contains(normalized, "file_path is required") {
			return ToolErrorMetadata{ToolName: toolName, Code: "missing_edit_payload", UIMessage: "Missing edit target or edits."}, true
		}
		if strings.Contains(normalized, "you must read the file before editing") {
			return ToolErrorMetadata{ToolName: toolName, Code: "read_before_edit", UIMessage: "Read files before editing."}, true
		}
	case WebSearchToolName, GoogleSearchToolName:
		if strings.Contains(normalized, "query is required") || strings.Contains(normalized, "no query provided") {
			return ToolErrorMetadata{ToolName: toolName, Code: "missing_query", UIMessage: "Missing search query."}, true
		}
	case ConnectMCPToolName, "read_mcp_resource", "list_mcp_resources", "call_mcp_tool":
		if strings.Contains(normalized, "mcp_name parameter is required") {
			return ToolErrorMetadata{ToolName: toolName, Code: "missing_mcp_name", UIMessage: "Missing MCP name."}, true
		}
	}

	switch {
	case strings.HasPrefix(normalized, "tool not found:"):
		return ToolErrorMetadata{ToolName: toolName, Code: "tool_not_found", UIMessage: "Unknown tool call."}, true
	case strings.HasPrefix(normalized, "invalid parameters:"):
		return ToolErrorMetadata{ToolName: toolName, Code: "invalid_parameters", UIMessage: "Invalid tool parameters."}, true
	case strings.Contains(normalized, "missing required parameter"):
		return ToolErrorMetadata{ToolName: toolName, Code: "missing_required_parameter", UIMessage: "Missing required parameter."}, true
	case strings.HasSuffix(normalized, " is required") || strings.HasSuffix(normalized, " parameter is required"):
		return ToolErrorMetadata{ToolName: toolName, Code: "missing_required_parameter", UIMessage: "Missing required parameter."}, true
	default:
		return ToolErrorMetadata{}, false
	}
}

func mergeToolErrorMetadata(existing string, meta ToolErrorMetadata) (string, bool) {
	payload := map[string]any{}
	existing = strings.TrimSpace(existing)
	if existing != "" {
		if err := json.Unmarshal([]byte(existing), &payload); err != nil {
			payload = map[string]any{}
		}
	}
	payload[toolErrorMetadataKey] = meta
	out, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(out), true
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
