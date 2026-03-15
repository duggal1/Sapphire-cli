package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/session"
)

func validateToolCallInput(ctx context.Context, tool fantasy.AgentTool, call fantasy.ToolCall, input map[string]any) error {
	if tool == nil {
		return errors.New("tool not found")
	}

	switch call.Name {
	case TodosToolName:
		// Validate using the normalized input map, NOT the original call.Input.
		// The middleware has already repaired aliases and structure; validating
		// the raw string would undo all normalization work.
		return validateTodosInputMap(input)
	case ViewToolName, SingleViewToolName, AgenticViewToolName:
		if len(extractViewPaths(input)) == 0 {
			return errors.New("file_path is required")
		}
	case EditToolName, SingleEditToolName:
		// Validate using the normalized input map to honor parameter aliasing
		// (e.g. "path" → "file_path") already applied by the middleware.
		return validateEditInputMap(input)
	case AgenticEditToolName:
		return validateAgenticEditInputMap(input)
	default:
		return nil
	}

	return nil
}

// validateTodosInputMap validates the normalized full-list todos contract.
func validateTodosInputMap(input map[string]any) error {
	if input == nil {
		return errors.New("todos input must be a JSON object")
	}

	var params TodosParams
	if err := decodeInto(input, &params); err != nil {
		return fmt.Errorf("invalid parameters: %w", err)
	}

	for _, todo := range params.Todos {
		content := strings.TrimSpace(todo.Content)
		activeForm := strings.TrimSpace(todo.ActiveForm)
		if content == "" && activeForm == "" {
			continue
		}
		switch strings.TrimSpace(todo.Status) {
		case string(session.TodoStatusPending), string(session.TodoStatusInProgress), string(session.TodoStatusCompleted):
		default:
			return fmt.Errorf("invalid status %q for todo %q", todo.Status, todo.Content)
		}
	}
	return nil
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
		return errors.New("file_path is required")
	}
	if _, hasOld := input["old_string"]; !hasOld {
		if _, hasNew := input["new_string"]; !hasNew {
			return errors.New("at least one of old_string or new_string is required")
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
			return errors.New("agentic_edit file_edits must be an object or array")
		}
		if len(editItems) == 0 {
			return errors.New("at least one file edit operation is required")
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
				return errors.New("file_path is required in each file_edit")
			}
			validItems++
		}
		if validItems == 0 {
			return errors.New("at least one file edit operation is required")
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
			return errors.New("at least one file edit operation is required")
		}
	}

	return validateAgenticEditOperationsMap(input)
}

func validateAgenticEditOperationsMap(editMap map[string]any) error {
	if edits, ok := editMap["edits"]; ok {
		editList, err := coerceObjectSlice(edits)
		if err != nil {
			return errors.New("agentic_edit edits must be an object or array")
		}
		if len(editList) == 0 {
			return errors.New("at least one file edit operation is required")
		}
		for _, op := range editList {
			opMap := op
			if _, hasOld := opMap["old_string"]; !hasOld {
				if _, hasNew := opMap["new_string"]; !hasNew {
					return errors.New("each edit must include old_string or new_string")
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
	return errors.New("at least one file edit operation is required")
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

func coerceObjectSlice(v any) ([]map[string]any, error) {
	switch value := v.(type) {
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
