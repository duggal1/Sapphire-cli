package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

func validateToolCallInput(ctx context.Context, tool fantasy.AgentTool, call fantasy.ToolCall, input map[string]any) error {
	if tool == nil {
		return errors.New("tool not found")
	}

	switch call.Name {
	case ViewToolName, SingleViewToolName, AgenticViewToolName:
		if len(extractViewPaths(input)) == 0 {
			return NewToolGuidanceError(
				call.Name,
				"missing_file_path",
				"Missing file path.",
				"single_view/agentic_view require explicit file path arguments. Do not pass empty input or natural language. Use file_path for one file or file_paths for multiple files, then retry with real repo-relative paths.",
			)
		}
	case BashToolName:
		return validateBashInputMap(input)
	case UpdatePlanToolName:
		return validateUpdatePlanInputMap(input)
	case EditToolName, SingleEditToolName:
		// Validate using the normalized input map to honor parameter aliasing
		// (e.g. "path" → "file_path") already applied by the middleware.
		return validateEditInputMap(input)
	case AgenticEditToolName:
		return validateAgenticEditInputMap(input)
	case WebSearchToolName:
		if len(normalizeBatchTargets(firstStringValueFromMap(input, "query"), coerceStringSlice(input["queries"]), "")) == 0 {
			return NewToolGuidanceError(
				call.Name,
				"missing_query",
				"Missing search query.",
				"web_search requires query or queries. Do not call it with empty input. Provide one concrete search query string or a non-empty queries array, then retry.",
			)
		}
	case GoogleSearchToolName:
		query := strings.TrimSpace(firstStringValueFromMap(input, "query"))
		urls := coerceStringSlice(input["urls"])
		urls = append(urls, coerceStringSlice(input["url"])...)
		if query == "" && len(urls) == 0 {
			return NewToolGuidanceError(
				call.Name,
				"missing_query",
				"Missing search query.",
				"google_search requires a grounded query or at least one URL context target. Do not call it empty. Provide query, url, or urls, then retry.",
			)
		}
	case ConnectMCPToolName:
		if strings.TrimSpace(firstStringValueFromMap(input, "mcp_name")) == "" {
			return NewToolGuidanceError(
				call.Name,
				"missing_mcp_name",
				"Missing MCP name.",
				"connect_mcp requires mcp_name. Use an exact installed MCP server name from list_available_mcps or install_mcp output. Do not call connect_mcp with empty input.",
			)
		}
	default:
		return nil
	}

	return nil
}

func validateBashInputMap(input map[string]any) error {
	if input == nil {
		return errors.New("bash input must be a JSON object")
	}
	command, _ := input["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		return NewToolGuidanceError(
			BashToolName,
			"missing_command",
			"Missing command.",
			"bash requires a non-empty command string in command. Do not omit it. If the task is repository discovery, file reading, code search, web search, or delegation setup, use structured tools instead of bash.",
		)
	}
	if shouldRejectBashForStructuredRepoOps(command) {
		return NewToolGuidanceError(
			BashToolName,
			"use_structured_tools",
			"Use structured tools instead of bash.",
			"do not use bash for repository discovery, code search, or file reads when structured tools exist. Route strictly: unknown location -> tool_search; filename/path discovery -> rg_files; exact text or symbol search -> rg; layout inspection -> ls; broad file reads -> agentic_view; trivial one-file reads -> single_view. Use web_search/google_search for web lookup, and pass spawn_agent/send_input messages directly instead of writing temporary payload files.",
		)
	}
	return nil
}

func validateUpdatePlanInputMap(input map[string]any) error {
	if input == nil {
		return errors.New("update_plan input must be a JSON object")
	}

	rawPlan, ok := input["plan"]
	if !ok {
		return errors.New("plan is required")
	}

	items, err := coerceObjectSlice(rawPlan)
	if err != nil {
		return errors.New("plan must be an array of step objects")
	}

	plan := make([]PlanItem, 0, len(items))
	for _, item := range items {
		step, _ := item["step"].(string)
		status, _ := item["status"].(string)
		plan = append(plan, PlanItem{
			Step:   step,
			Status: StepStatus(status),
		})
	}
	plan = NormalizePlanItems(plan)
	if len(plan) == 0 {
		return errors.New("plan must contain at least one non-empty step")
	}

	return ValidatePlanItems(plan)
}

func extractViewPaths(input map[string]any) []string {
	if input == nil {
		return nil
	}
	paths := make([]string, 0, 2)
	if raw, ok := input["file_paths"]; ok {
		paths = append(paths, coerceStringSlice(raw)...)
	}
	if raw, ok := input["file_path"]; ok {
		paths = append(paths, coerceStringSlice(raw)...)
	}
	if raw, ok := input["paths"]; ok {
		paths = append(paths, coerceStringSlice(raw)...)
	}
	if raw, ok := input["files"]; ok {
		paths = append(paths, coerceStringSlice(raw)...)
	}
	if raw, ok := input["path"]; ok {
		paths = append(paths, coerceStringSlice(raw)...)
	}
	return uniqueStrings(paths)
}

// validateEditInputMap validates edit parameters from the already-normalized input map.
func validateEditInputMap(input map[string]any) error {
	if input == nil {
		return errors.New("edit input must be a JSON object")
	}
	filePath, _ := input["file_path"].(string)
	if strings.TrimSpace(filePath) == "" {
		return NewToolGuidanceError(
			EditToolName,
			"missing_file_path",
			"Missing file path.",
			"edit requires file_path. Do not call edit without an explicit target file. Retry with a real repo-relative file_path after reading the file first.",
		)
	}
	if _, hasOld := input["old_string"]; !hasOld {
		if _, hasNew := input["new_string"]; !hasNew {
			return NewToolGuidanceError(
				EditToolName,
				"missing_edit_payload",
				"Missing edit payload.",
				"edit requires old_string or new_string. Do not call edit with only a path. Provide a precise edit payload after reading the latest file contents.",
			)
		}
	}
	return nil
}

// validateAgenticEditInputMap validates agentic_edit parameters from the already-normalized input map.
func validateAgenticEditInputMap(input map[string]any) error {
	if input == nil {
		return errors.New("agentic_edit input must be a JSON object")
	}

	if fileEdits, ok := input["file_edits"]; ok {
		editItems, err := coerceObjectSlice(fileEdits)
		if err != nil {
			return NewToolGuidanceError(
				AgenticEditToolName,
				"invalid_file_edits",
				"Invalid edit payload.",
				"agentic_edit file_edits must be an object or array of file edit specs. Do not send free-form text. Retry with structured file_edits JSON.",
			)
		}
		if len(editItems) == 0 {
			return NewToolGuidanceError(
				AgenticEditToolName,
				"missing_edit_payload",
				"Missing edit target or edits.",
				"agentic_edit requires at least one file edit operation. Provide file_edits with file_path and edits, or use the single-file shape with file_path plus edits.",
			)
		}
		validItems := 0
		for _, editMap := range editItems {
			filePath, _ := editMap["file_path"].(string)
			if strings.TrimSpace(filePath) == "" && !hasAgenticEditOperations(editMap) {
				continue
			}
			if err := validateAgenticEditOperationsMap(editMap); err != nil {
				if strings.TrimSpace(filePath) == "" {
					continue
				}
				return fmt.Errorf("%s: %w", filePath, err)
			}
			if strings.TrimSpace(filePath) == "" {
				return NewToolGuidanceError(
					AgenticEditToolName,
					"missing_file_path",
					"Missing edit target or edits.",
					"each agentic_edit file_edits item requires file_path. Do not submit edit operations without explicit target files.",
				)
			}
			validItems++
		}
		if validItems == 0 {
			return NewToolGuidanceError(
				AgenticEditToolName,
				"missing_edit_payload",
				"Missing edit target or edits.",
				"agentic_edit requires at least one valid file edit operation. Empty or pathless edit items are invalid.",
			)
		}
		return nil
	}

	// Fallback: single-file edit shape
	if edits, ok := input["edits"]; ok {
		if editItems, err := coerceObjectSlice(edits); err == nil && len(editItems) > 0 {
			for _, editMap := range editItems {
				if _, hasFilePath := editMap["file_path"]; hasFilePath {
					return validateAgenticEditInputMap(map[string]any{"file_edits": edits})
				}
				if _, hasPath := editMap["path"]; hasPath {
					return validateAgenticEditInputMap(map[string]any{"file_edits": edits})
				}
			}
		}
	}

	filePath, _ := input["file_path"].(string)
	if strings.TrimSpace(filePath) == "" {
		path, _ := input["path"].(string)
		if strings.TrimSpace(path) == "" {
			return NewToolGuidanceError(
				AgenticEditToolName,
				"missing_edit_payload",
				"Missing edit target or edits.",
				"agentic_edit requires either file_edits or a single-file shape with file_path/path plus edit operations. Do not call it with empty input.",
			)
		}
	}

	return validateAgenticEditOperationsMap(input)
}

func validateAgenticEditOperationsMap(editMap map[string]any) error {
	if edits, ok := editMap["edits"]; ok {
		editList, err := coerceObjectSlice(edits)
		if err != nil {
			return NewToolGuidanceError(
				AgenticEditToolName,
				"invalid_edits",
				"Invalid edit payload.",
				"agentic_edit edits must be an object or array of edit operations. Do not pass free-form text or malformed JSON.",
			)
		}
		if len(editList) == 0 {
			return NewToolGuidanceError(
				AgenticEditToolName,
				"missing_edit_payload",
				"Missing edit target or edits.",
				"agentic_edit edits cannot be empty. Provide at least one edit operation with old_string or new_string.",
			)
		}
		for _, op := range editList {
			opMap := op
			if _, hasOld := opMap["old_string"]; !hasOld {
				if _, hasNew := opMap["new_string"]; !hasNew {
					return NewToolGuidanceError(
						AgenticEditToolName,
						"missing_edit_payload",
						"Invalid edit payload.",
						"each agentic_edit operation must include old_string or new_string. Do not submit empty edit operations.",
					)
				}
			}
		}
		return nil
	}

	if _, ok := editMap["old_string"]; ok {
		return nil
	}
	if _, ok := editMap["new_string"]; ok {
		return nil
	}
	return NewToolGuidanceError(
		AgenticEditToolName,
		"missing_edit_payload",
		"Missing edit target or edits.",
		"agentic_edit requires at least one concrete edit operation. Provide edits or old_string/new_string instead of empty input.",
	)
}

func hasAgenticEditOperations(editMap map[string]any) bool {
	if editMap == nil {
		return false
	}
	if _, ok := editMap["edits"]; ok {
		return true
	}
	if _, ok := editMap["old_string"]; ok {
		return true
	}
	if _, ok := editMap["new_string"]; ok {
		return true
	}
	return false
}

func firstStringValueFromMap(input map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := input[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func coerceObjectSlice(v any) ([]map[string]any, error) {
	switch value := v.(type) {
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return nil, errors.New("value must be an object or array")
		}
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			return nil, errors.New("value must be an object or array")
		}
		return coerceObjectSlice(decoded)
	case map[string]any:
		return []map[string]any{value}, nil
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, errors.New("value must contain JSON objects")
			}
			out = append(out, obj)
		}
		return out, nil
	default:
		return nil, errors.New("value must be an object or array")
	}
}

func coerceStringSlice(v any) []string {
	switch value := v.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []string{value}
	case []string:
		return value
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
