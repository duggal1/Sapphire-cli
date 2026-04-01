package tools

import (
	"encoding/json"
	"errors"
	"strings"

	"charm.land/fantasy"
)

const runtimeExecutionMetadataKey = "runtime_execution"

type RuntimeExecutionMetadata struct {
	RequestedToolName string `json:"requested_tool_name,omitempty"`
	RequestedInput    string `json:"requested_input,omitempty"`
	ExecutedToolName  string `json:"executed_tool_name,omitempty"`
	ExecutedInput     string `json:"executed_input,omitempty"`
	Rewritten         bool   `json:"rewritten,omitempty"`
}

type runtimeExecutionError struct {
	cause error
	meta  RuntimeExecutionMetadata
}

func (e *runtimeExecutionError) Error() string {
	if e == nil || e.cause == nil {
		return ""
	}
	return e.cause.Error()
}

func (e *runtimeExecutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func buildRuntimeExecutionMetadata(requested, executed fantasy.ToolCall) RuntimeExecutionMetadata {
	requestedName := strings.TrimSpace(requested.Name)
	executedName := strings.TrimSpace(executed.Name)
	if requestedName == "" {
		requestedName = executedName
	}
	if executedName == "" {
		executedName = requestedName
	}
	if requestedName == "" && executedName == "" {
		return RuntimeExecutionMetadata{}
	}
	return RuntimeExecutionMetadata{
		RequestedToolName: requestedName,
		RequestedInput:    strings.TrimSpace(requested.Input),
		ExecutedToolName:  executedName,
		ExecutedInput:     strings.TrimSpace(executed.Input),
		Rewritten:         requestedName != executedName,
	}
}

func hasRuntimeExecutionRewrite(meta RuntimeExecutionMetadata) bool {
	return strings.TrimSpace(meta.RequestedToolName) != "" &&
		strings.TrimSpace(meta.ExecutedToolName) != "" &&
		strings.TrimSpace(meta.RequestedToolName) != strings.TrimSpace(meta.ExecutedToolName)
}

func mergeResponseMetadata(existing string, key string, value any) (string, bool) {
	payload := map[string]any{}
	existing = strings.TrimSpace(existing)
	if existing != "" {
		if err := json.Unmarshal([]byte(existing), &payload); err != nil {
			payload = map[string]any{}
		}
	}
	payload[strings.TrimSpace(key)] = value
	out, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(out), true
}

func AnnotateRuntimeExecutionMetadata(existing string, requested, executed fantasy.ToolCall) string {
	meta := buildRuntimeExecutionMetadata(requested, executed)
	if !hasRuntimeExecutionRewrite(meta) {
		return existing
	}
	merged, ok := mergeResponseMetadata(existing, runtimeExecutionMetadataKey, meta)
	if !ok {
		return existing
	}
	return merged
}

func WrapRuntimeExecutionError(err error, requested, executed fantasy.ToolCall) error {
	if err == nil {
		return nil
	}
	meta := buildRuntimeExecutionMetadata(requested, executed)
	if !hasRuntimeExecutionRewrite(meta) {
		return err
	}
	return &runtimeExecutionError{cause: err, meta: meta}
}

func AnnotateRuntimeExecutionErrorMetadata(existing string, err error) string {
	var wrapped *runtimeExecutionError
	if !errors.As(err, &wrapped) || wrapped == nil {
		return existing
	}
	if !hasRuntimeExecutionRewrite(wrapped.meta) {
		return existing
	}
	merged, ok := mergeResponseMetadata(existing, runtimeExecutionMetadataKey, wrapped.meta)
	if !ok {
		return existing
	}
	return merged
}

func ParseRuntimeExecutionMetadata(metadata string) (RuntimeExecutionMetadata, bool) {
	metadata = strings.TrimSpace(metadata)
	if metadata == "" {
		return RuntimeExecutionMetadata{}, false
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(metadata), &root); err != nil {
		return RuntimeExecutionMetadata{}, false
	}
	if raw, ok := root[runtimeExecutionMetadataKey]; ok {
		var parsed RuntimeExecutionMetadata
		if err := json.Unmarshal(raw, &parsed); err == nil && hasRuntimeExecutionRewrite(parsed) {
			return parsed, true
		}
	}

	var parsed RuntimeExecutionMetadata
	if err := json.Unmarshal([]byte(metadata), &parsed); err == nil && hasRuntimeExecutionRewrite(parsed) {
		return parsed, true
	}
	return RuntimeExecutionMetadata{}, false
}

func ResolveObservedToolExecution(defaultToolName, defaultInput, metadata string) (string, string) {
	if parsed, ok := ParseRuntimeExecutionMetadata(metadata); ok {
		return firstNonEmptyString(parsed.ExecutedToolName, defaultToolName), firstNonEmptyString(parsed.ExecutedInput, defaultInput)
	}
	return strings.TrimSpace(defaultToolName), strings.TrimSpace(defaultInput)
}
