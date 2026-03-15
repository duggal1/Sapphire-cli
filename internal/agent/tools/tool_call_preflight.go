package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"charm.land/fantasy"
	"charm.land/fantasy/schema"
	"github.com/charmbracelet/sapphire/internal/filetracker"
)

var (
	validationTrackerMu sync.RWMutex
	validationTracker   filetracker.Service
)

func SetValidationFileTracker(tracker filetracker.Service) {
	validationTrackerMu.Lock()
	defer validationTrackerMu.Unlock()
	validationTracker = tracker
}

func getValidationFileTracker() filetracker.Service {
	validationTrackerMu.RLock()
	defer validationTrackerMu.RUnlock()
	return validationTracker
}

func PrepareToolCall(ctx context.Context, call fantasy.ToolCall, tools map[string]fantasy.AgentTool) (fantasy.ToolCall, fantasy.AgentTool, error) {
	normalized, ok := NormalizeToolCall(call, tools)
	if !ok {
		// Build suggestion list for the model to self-correct
		suggestions := FindSimilarToolNames(call.Name, tools)
		if len(suggestions) > 0 {
			return call, nil, fmt.Errorf("tool not found: %s. Did you mean one of: %s? Use exact tool names from the registry", call.Name, strings.Join(suggestions, ", "))
		}
		available := make([]string, 0, len(tools))
		for name := range tools {
			available = append(available, name)
		}
		return call, nil, fmt.Errorf("tool not found: %s. Available tools: %s", call.Name, strings.Join(available, ", "))
	}
	call = normalized
	tool := tools[call.Name]

	input, err := parseToolInput(call.Input, call.Name)
	if err != nil {
		return call, tool, err
	}

	call, tool, input, err = repairToolCall(ctx, call, tool, input, tools)
	if err != nil {
		return call, tool, err
	}

	if err := enforceWriteScope(ctx, call.Name, input); err != nil {
		return call, tool, err
	}

	encoded, err := json.Marshal(input)
	if err != nil {
		return call, tool, fmt.Errorf("failed to serialize tool input: %w", err)
	}
	call.Input = string(encoded)

	if err := validateToolCallInput(ctx, tool, call, input); err != nil {
		return call, tool, err
	}

	return call, tool, nil
}

func parseToolInput(raw string, toolName string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return map[string]any{}, nil
	}
	obj, state, err := schema.ParsePartialJSON(raw)
	if state == schema.ParseStateFailed {
		return nil, fmt.Errorf("tool input must be valid JSON: %w", err)
	}
	m, ok := obj.(map[string]any)
	if !ok {
		if s, ok := obj.(string); ok {
			coerced, ok := coerceInputFromString(toolName, s)
			if ok {
				return coerced, nil
			}
		}
		return nil, errors.New("tool input must be a JSON object")
	}
	return m, nil
}

func repairToolCall(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any, error) {
	switch call.Name {
	// ── File operations ──────────────────────────────────────────────
	case ViewToolName, SingleViewToolName, AgenticViewToolName:
		return repairViewCall(call, tool, input, tools)
	case EditToolName, SingleEditToolName, AgenticEditToolName:
		return repairEditCall(ctx, call, tool, input, tools)
	case WriteToolName:
		normalizeKey(input, "file_path", "path", "file", "filename", "filepath")
		normalizeKey(input, "content", "text", "body", "data", "file_content")

	// ── Todos ────────────────────────────────────────────────────────
	case TodosToolName:
		normalizeKey(input, "text", "todo")
		normalizeKey(input, "tasks", "todos", "items")
		normalizeKey(input, "action", "type", "operation", "op")
		normalizeKey(input, "task_id", "id", "taskId", "task_identifier")
		normalizeKey(input, "task_key", "key", "taskKey", "task_name")
		normalizeKey(input, "task_content", "content", "title", "name")
		normalizeTodosInput(input)
		ensureTodosPayload(input)

	// ── Bash ─────────────────────────────────────────────────────────
	case BashToolName:
		normalizeKey(input, "command", "cmd", "bash_command", "script", "shell_command", "run")
		normalizeKey(input, "description", "desc", "reason", "explanation")
		normalizeKey(input, "working_dir", "working_directory", "cwd", "dir", "directory")
		normalizeKey(input, "run_in_background", "background", "bg", "async")
		if cmd, ok := input["command"].(string); ok {
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				if desc, ok := input["description"].(string); ok {
					desc = strings.TrimSpace(desc)
					if looksLikeShellCommand(desc) {
						input["command"] = desc
						input["description"] = "run command"
						cmd = desc
					}
				}
			}
			if cmd != "" {
				if desc, ok := input["description"].(string); !ok || strings.TrimSpace(desc) == "" {
					input["description"] = "run command"
				}
			}
		}
		if cmd, ok := input["command"].(string); ok {
			if toolName, toolInput, ok := simpleBashToTool(cmd); ok {
				if next, ok := tools[toolName]; ok {
					call.Name = toolName
					tool = next
					input = toolInput
				}
			}
		}

	// ── Search / Discovery tools ─────────────────────────────────────
	case GrepToolName:
		normalizeKey(input, "pattern", "regex", "search", "query", "text", "term")
		normalizeKey(input, "include", "glob", "file_pattern", "file_glob", "includes")
		normalizeKey(input, "exclude", "ignore", "excludes", "skip")
		normalizeKey(input, "path", "dir", "directory", "search_path", "folder")
	case GlobToolName:
		normalizeKey(input, "pattern", "glob", "search", "query", "file_pattern")
		normalizeKey(input, "path", "dir", "directory", "folder", "root")
	case LSToolName:
		normalizeKey(input, "path", "dir", "directory", "folder", "root")
		normalizeKey(input, "depth", "max_depth", "maxDepth", "level")
		normalizeKey(input, "max_items", "limit", "maxItems", "max_results")
	case "codebase_search":
		normalizeKey(input, "query", "search", "q", "term", "pattern")
		normalizeKey(input, "path", "dir", "directory", "folder", "scope")

	// ── Fetch / Web tools ────────────────────────────────────────────
	case FetchToolName:
		normalizeKey(input, "url", "uri", "link", "href", "address")
		normalizeKey(input, "format", "output", "output_format", "type")
		normalizeKey(input, "timeout", "timeout_seconds", "max_timeout")
	case WebSearchToolName:
		normalizeKey(input, "query", "q", "search", "search_query", "term")
		normalizeKey(input, "max_results", "num_results", "count", "limit", "results")
	case AgenticFetchToolName:
		normalizeKey(input, "url", "urls", "links")
		normalizeKey(input, "prompt", "query", "search", "q")
	case WebFetchToolName:
		normalizeKey(input, "url", "uri", "link", "href")

	// ── Download ─────────────────────────────────────────────────────
	case DownloadToolName:
		normalizeKey(input, "url", "uri", "link", "href", "source")
		normalizeKey(input, "file_path", "path", "destination", "dest", "file", "output")

	// ── LSP tools ────────────────────────────────────────────────────
	case DiagnosticsToolName:
		normalizeKey(input, "file_path", "path", "file", "filepath")
	case ReferencesToolName:
		normalizeKey(input, "symbol", "name", "identifier", "function", "method")
		normalizeKey(input, "path", "dir", "directory", "file_path", "folder")
	case LSPRestartToolName:
		// No parameters needed

	// ── Background job tools ─────────────────────────────────────────
	case JobKillToolName:
		normalizeKey(input, "job_id", "id", "shell_id", "shellId", "jobId")
	case JobOutputToolName:
		normalizeKey(input, "job_id", "id", "shell_id", "shellId", "jobId")

	// ── Memory tools ─────────────────────────────────────────────────
	case MemoryQueryToolName:
		normalizeKey(input, "query", "q", "search", "term")
	case "recall_memory":
		normalizeKey(input, "query", "q", "search", "term")
		normalizeKey(input, "filter", "type", "scope")
		normalizeKey(input, "limit", "count", "max_results")
	case "save_memory":
		normalizeKey(input, "event_type", "type", "event", "kind", "category")
		normalizeKey(input, "content", "data", "value", "payload")

	// ── Skill tools ──────────────────────────────────────────────────
	case LoadSkillToolName:
		normalizeKey(input, "name", "skill", "skill_name", "skillName")
	case "list_skills":
		// No parameters needed

	// ── Agent delegation tool ────────────────────────────────────────
	case "agent":
		normalizeKey(input, "prompt", "message", "task", "instruction")
		normalizeKey(input, "branch", "branch_name")
		normalizeKey(input, "worktree_path", "worktree", "worktree_dir", "worktree_directory")
		normalizeKey(input, "write_manifest", "manifest", "allowed_files", "allowed_paths", "owned_files")
		normalizeKey(input, "definition_of_done", "done", "acceptance_criteria")

	// ── Python tool ──────────────────────────────────────────────────
	case PythonToolName:
		normalizeKey(input, "code", "script", "python", "source")

	// ── Sourcegraph tool ─────────────────────────────────────────────
	case SourcegraphToolName:
		normalizeKey(input, "query", "search", "q", "pattern")

	// ── MCP tools ────────────────────────────────────────────────────
	case CallMCPToolName:
		normalizeKey(input, "mcp_name", "server", "server_name")
		normalizeKey(input, "tool_name", "mcp_tool")
		normalizeKey(input, "arguments", "args", "params", "parameters", "input")
		coerceJSONObjectField(input, "arguments")
	case ListMCPToolsToolName:
		normalizeKey(input, "mcp_name", "server", "server_name")
		normalizeKey(input, "query", "search", "q", "capability", "tool_query")
	case ListMCPResourcesToolName, ReadMCPResourceToolName, ConnectMCPToolName:
		normalizeKey(input, "mcp_name", "server", "server_name")
	case ListAvailableMCPsToolName:
		// No parameters needed

	// ── Sub-agent orchestration ──────────────────────────────────────
	case "spawn_agent":
		normalizeKey(input, "message", "prompt", "task", "instruction")
		normalizeKey(input, "agent", "agent_type", "agent_id", "agent_profile")
		normalizeKey(input, "model", "model_name")
		normalizeKey(input, "reasoning_effort", "reasoning")
		normalizeKey(input, "branch", "branch_name")
		normalizeKey(input, "worktree_path", "worktree", "worktree_dir", "worktree_directory")
		normalizeKey(input, "write_manifest", "manifest", "allowed_files", "allowed_paths", "owned_files")
		normalizeKey(input, "definition_of_done", "done", "acceptance_criteria")
	case "send_input":
		normalizeKey(input, "id", "agent_id")
		normalizeKey(input, "message", "prompt", "task")
	case "resume_agent":
		normalizeKey(input, "id", "agent_id")
		normalizeKey(input, "message", "prompt", "task")
	case "wait":
		normalizeKey(input, "ids", "agent_ids")
	case "close_agent":
		normalizeKey(input, "id", "agent_id")
	case "spawn_agents_on_csv":
		normalizeKey(input, "csv_path", "path", "csv", "input_csv", "file")
		normalizeKey(input, "instruction", "prompt", "message", "task")
		normalizeKey(input, "output_csv_path", "output_path", "output_file")
		normalizeKey(input, "id_column", "id_field")
		normalizeKey(input, "max_concurrency", "max_workers")
		coerceJSONObjectField(input, "output_schema")
	case "report_agent_job_result":
		normalizeKey(input, "job_id", "job")
		normalizeKey(input, "item_id", "item")
		coerceJSONObjectField(input, "result")

	// ── Meta tools ───────────────────────────────────────────────────
	case "search_tools":
		normalizeKey(input, "query", "q", "search", "term")
	case ToolSuggestToolName:
		normalizeKey(input, "query", "q", "search", "term", "capability")
	case ListToolsToolName:
		// No parameters needed
	case "orchestrate_worktrees":
		normalizeKey(input, "tasks", "plan", "worktrees", "worktree_tasks")
		normalizeKey(input, "test_command", "test_cmd", "tests")
		normalizeKey(input, "integration_prompt", "merge_prompt", "integration")
		normalizeKey(input, "integration_branch", "merge_branch", "integration_branch_name")

	// ── Generic fallback ─────────────────────────────────────────────
	default:
		// For ANY tool not explicitly listed above (including future tools
		// and dynamically registered MCP tools), attempt schema-based
		// parameter repair using the tool's declared parameter names.
		repairFromSchema(tool, input)
	}
	return call, tool, input, nil
}

func repairViewCall(
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any, error) {
	normalizeKey(input, "file_paths", "paths", "files")
	normalizeKey(input, "file_path", "path", "file")

	paths := extractViewPaths(input)
	offset, _ := coerceInt(input["offset"])
	limit, _ := coerceInt(input["limit"])
	if len(paths) == 0 {
		return call, tool, input, errors.New("file_path is required")
	}

	input = map[string]any{}
	if call.Name == AgenticViewToolName {
		input["file_paths"] = paths
		return call, tool, input, nil
	}

	input["file_path"] = paths[0]
	if offset != 0 {
		input["offset"] = offset
	}
	if limit != 0 {
		input["limit"] = limit
	}
	return call, tool, input, nil
}

func repairEditCall(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any, error) {
	normalizeKey(input, "file_path", "path", "file")
	normalizeKey(input, "old_string", "old", "target", "find", "search")
	normalizeKey(input, "new_string", "new", "replacement", "replace", "replace_with", "content")
	normalizeKey(input, "replace_all", "all", "global")
	promoteAgenticEditFileEditsShape(input)

	var multi MultiEditParams
	if err := decodeInto(input, &multi); err != nil {
		return call, tool, input, fmt.Errorf("edit input must be valid JSON: %w", err)
	}

	_ = ctx

	switch call.Name {
	case EditToolName, SingleEditToolName:
		if editPayloadRequiresAgenticEdit(input, multi) {
			if next, ok := tools[AgenticEditToolName]; ok {
				call.Name = AgenticEditToolName
				tool = next
			}
		}
	}

	return call, tool, input, nil
}

func promoteAgenticEditFileEditsShape(input map[string]any) {
	if input == nil {
		return
	}
	if _, ok := input["file_edits"]; ok {
		return
	}
	rawEdits, ok := input["edits"]
	if !ok {
		return
	}
	items, err := coerceObjectSlice(rawEdits)
	if err != nil || len(items) == 0 {
		return
	}
	for _, item := range items {
		if _, hasFilePath := item["file_path"]; hasFilePath {
			input["file_edits"] = rawEdits
			delete(input, "edits")
			return
		}
		if _, hasPath := item["path"]; hasPath {
			input["file_edits"] = rawEdits
			delete(input, "edits")
			return
		}
	}
}

func editPayloadRequiresAgenticEdit(input map[string]any, multi MultiEditParams) bool {
	if _, ok := input["file_edits"]; ok {
		return true
	}
	if _, ok := input["edits"]; ok {
		if len(multi.Edits) > 1 {
			return true
		}
		if len(multi.FileEdits) == 1 && len(multi.FileEdits[0].Edits) > 1 {
			return true
		}
	}
	if len(multi.FileEdits) > 1 {
		return true
	}
	return false
}

func extractEditPaths(multi MultiEditParams, input map[string]any) []string {
	if len(multi.FileEdits) > 0 {
		paths := make([]string, 0, len(multi.FileEdits))
		for _, fe := range multi.FileEdits {
			if fe.FilePath != "" {
				paths = append(paths, fe.FilePath)
			}
		}
		return paths
	}
	if multi.FilePath != "" {
		return []string{multi.FilePath}
	}
	if raw, ok := input["file_path"]; ok {
		if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
			return []string{value}
		}
	}
	return nil
}

func enforceWriteScope(ctx context.Context, toolName string, input map[string]any) error {
	scope := GetWriteScopeFromContext(ctx)
	if scope == nil {
		return nil
	}
	paths := extractWritePaths(toolName, input)
	if len(paths) == 0 {
		return nil
	}
	workingDir := GetWorkingDirFromContext(ctx)
	var blocked []string
	for _, pth := range paths {
		if strings.TrimSpace(pth) == "" {
			continue
		}
		normalized := pth
		if !filepath.IsAbs(normalized) && workingDir != "" {
			normalized = filepath.Join(workingDir, normalized)
		}
		if !scope.Allows(normalized) {
			blocked = append(blocked, normalized)
		}
	}
	if len(blocked) == 0 {
		return nil
	}
	return fmt.Errorf("write blocked: %s is outside the allowed manifest", strings.Join(blocked, ", "))
}

func extractWritePaths(toolName string, input map[string]any) []string {
	switch toolName {
	case EditToolName, SingleEditToolName, AgenticEditToolName:
		var multi MultiEditParams
		if err := decodeInto(input, &multi); err != nil {
			return nil
		}
		return extractEditPaths(multi, input)
	case WriteToolName:
		if raw, ok := input["file_path"]; ok {
			if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
				return []string{value}
			}
		}
	case DownloadToolName:
		if raw, ok := input["file_path"]; ok {
			if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
				return []string{value}
			}
		}
	}
	return nil
}

func unreadFilePaths(ctx context.Context, paths []string) []string {
	tracker := getValidationFileTracker()
	sessionID := GetSessionFromContext(ctx)
	paths = uniqueStrings(paths)
	if len(paths) == 0 {
		return nil
	}
	// If sessionID is empty or tracker is nil, treat ALL paths as unread.
	// This ensures read-before-edit enforcement is unconditional and cannot
	// be bypassed by tool calls that lack a session context.
	if sessionID == "" || tracker == nil {
		return paths
	}
	workingDir := GetWorkingDirFromContext(ctx)
	var unread []string
	for _, pth := range paths {
		if pth == "" {
			continue
		}
		normalized := pth
		if !filepath.IsAbs(normalized) && workingDir != "" {
			normalized = filepath.Join(workingDir, normalized)
		}
		if tracker.LastReadTime(ctx, sessionID, normalized).IsZero() {
			unread = append(unread, pth)
		}
	}
	return unread
}

func normalizeKey(input map[string]any, target string, aliases ...string) {
	if input == nil {
		return
	}
	if _, ok := input[target]; ok {
		return
	}
	for _, alias := range aliases {
		if val, ok := input[alias]; ok {
			input[target] = val
			return
		}
	}
}

// repairFromSchema is a generic fallback parameter repair function.
// It reads the tool's declared parameter schema and attempts to map
// any unrecognized parameter names from the model to the closest
// matching schema-declared parameter. This allows even future tools
// and dynamically registered MCP tools to benefit from automatic
// parameter correction without explicit switch cases.
func repairFromSchema(tool fantasy.AgentTool, input map[string]any) {
	if tool == nil || input == nil {
		return
	}
	info := tool.Info()
	if len(info.Parameters) == 0 {
		return
	}

	// Extract declared parameter names from the JSON schema.
	// The schema follows JSON Schema format: {"type":"object","properties":{...}}
	propsRaw, ok := info.Parameters["properties"]
	if !ok {
		return
	}
	props, ok := propsRaw.(map[string]any)
	if !ok {
		return
	}
	if len(props) == 0 {
		return
	}

	// Build a set of known parameter names from the tool's schema
	knownParams := make(map[string]bool, len(props))
	knownNames := make([]string, 0, len(props))
	for paramName := range props {
		knownParams[paramName] = true
		knownNames = append(knownNames, paramName)
	}

	// Find parameters in input that don't match any known schema param
	unknownKeys := make([]string, 0)
	for key := range input {
		if !knownParams[key] {
			unknownKeys = append(unknownKeys, key)
		}
	}

	if len(unknownKeys) == 0 {
		return
	}

	// For each unknown parameter, try to find the closest matching
	// schema parameter using normalized name comparison
	for _, unknownKey := range unknownKeys {
		normUnknown := normalizeToolName(unknownKey)
		if normUnknown == "" {
			continue
		}

		bestMatch := ""
		bestScore := 0

		for _, paramName := range knownNames {
			// Skip if the target already has a value in the input
			if _, exists := input[paramName]; exists {
				continue
			}

			normProp := normalizeToolName(paramName)
			if normProp == "" {
				continue
			}

			// Exact normalized match (e.g. "file_path" == "filePath" after normalization)
			if normUnknown == normProp {
				bestMatch = paramName
				bestScore = 100
				break
			}

			// Substring containment (e.g. "path" in "file_path")
			score := 0
			if strings.Contains(normProp, normUnknown) || strings.Contains(normUnknown, normProp) {
				score = len(normUnknown)
			}

			// Shared word stems
			if score == 0 {
				for _, part := range strings.Split(normUnknown, "_") {
					if len(part) >= 3 && strings.Contains(normProp, part) {
						score = len(part)
						break
					}
				}
			}

			if score > bestScore {
				bestScore = score
				bestMatch = paramName
			}
		}

		// Only apply the mapping if we found a reasonably confident match
		// (at least 3 characters of shared content)
		if bestMatch != "" && bestScore >= 3 {
			input[bestMatch] = input[unknownKey]
			delete(input, unknownKey)
		}
	}
}

func decodeInto(input map[string]any, target any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func coerceJSONObjectField(input map[string]any, key string) {
	if input == nil {
		return
	}
	raw, ok := input[key]
	if !ok {
		return
	}
	text, ok := raw.(string)
	if !ok {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err == nil && decoded != nil {
		input[key] = decoded
	}
}

func simpleBashToTool(command string) (string, map[string]any, bool) {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return "", nil, false
	}
	if strings.ContainsAny(cmd, "|&;><") || strings.Contains(cmd, "&&") || strings.Contains(cmd, "||") || strings.Contains(cmd, "$(") {
		return "", nil, false
	}
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", nil, false
	}
	switch parts[0] {
	case "ls":
		if len(parts) > 1 && strings.HasPrefix(parts[1], "-") {
			return "", nil, false
		}
		input := map[string]any{}
		if len(parts) > 1 {
			input["path"] = parts[1]
		}
		return LSToolName, input, true
	case "cat":
		if len(parts) < 2 || strings.HasPrefix(parts[1], "-") {
			return "", nil, false
		}
		return ViewToolName, map[string]any{"file_path": parts[1]}, true
	default:
		return "", nil, false
	}
}

func coerceInt(v any) (int, bool) {
	switch value := v.(type) {
	case int:
		return value, true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	case float32:
		return int(value), true
	case json.Number:
		if i, err := value.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

func coerceInputFromString(toolName, value string) (map[string]any, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	switch toolName {
	case BashToolName:
		return map[string]any{
			"command":     value,
			"description": "run command",
		}, true
	case ViewToolName, SingleViewToolName, AgenticViewToolName:
		return map[string]any{"file_path": value}, true
	case LSToolName:
		return map[string]any{"path": value}, true
	case GlobToolName:
		return map[string]any{"pattern": value}, true
	case GrepToolName:
		return map[string]any{"pattern": value}, true
	case TodosToolName:
		return map[string]any{
			"action": "create",
			"tasks": []any{
				map[string]any{
					"content":     value,
					"status":      "pending",
					"active_form": "",
				},
			},
		}, true
	default:
		return nil, false
	}
}

func looksLikeShellCommand(input string) bool {
	if input == "" {
		return false
	}
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}
	cmd := parts[0]
	switch cmd {
	case "ls", "cat", "rg", "grep", "find", "git", "go", "npm", "pnpm", "yarn", "make", "pwd", "which", "sed", "awk", "python", "node":
		return true
	default:
		return false
	}
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func normalizeTodosInput(input map[string]any) {
	if input == nil {
		return
	}
	action, _ := input["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))

	items := extractTodosPayload(input)
	if len(items) > 0 {
		normalized := make([]any, 0, len(items))
		for _, item := range items {
			if entry, ok := normalizeTodoEntry(item); ok {
				normalized = append(normalized, entry)
			}
		}
		if len(normalized) > 0 {
			input["tasks"] = normalized
			if action == "" {
				input["action"] = "create"
			} else {
				input["action"] = action
			}
		}
		delete(input, "todos")
		delete(input, "items")
		delete(input, "text")
		return
	}

	if action != "" {
		input["action"] = action
		return
	}

	if _, ok := input["task"]; ok {
		input["action"] = "update"
	}
}

func ensureTodosPayload(input map[string]any) {
	if input == nil {
		return
	}
	if hasTodosPayload(input) {
		return
	}
	input["action"] = "create"
	input["tasks"] = []any{
		map[string]any{
			"content":     "Proceed with the requested task",
			"status":      "in_progress",
			"active_form": "Working on the requested task",
		},
	}
}

func hasTodosPayload(input map[string]any) bool {
	if raw, ok := input["action"].(string); ok {
		if strings.TrimSpace(raw) != "" {
			return true
		}
	}
	for _, key := range []string{"tasks", "todos", "items"} {
		if raw, ok := input[key]; ok {
			if list, ok := raw.([]any); ok && len(list) > 0 {
				return true
			}
		}
	}
	if raw, ok := input["text"].(string); ok {
		if strings.TrimSpace(raw) != "" {
			return true
		}
	}
	return false
}

func extractTodosPayload(input map[string]any) []any {
	if input == nil {
		return nil
	}
	for _, key := range []string{"tasks", "todos", "items"} {
		if raw, ok := input[key]; ok {
			if list, ok := raw.([]any); ok {
				return list
			}
			if parsed := parseTodoStringPayload(raw); len(parsed) > 0 {
				return parsed
			}
		}
	}
	if raw, ok := input["text"].(string); ok {
		lines := strings.Split(raw, "\n")
		items := make([]any, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			items = append(items, line)
		}
		return items
	}
	return nil
}

func parseTodoStringPayload(raw any) []any {
	text, ok := raw.(string)
	if !ok {
		return nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var decoded any
	if json.Unmarshal([]byte(text), &decoded) == nil {
		switch value := decoded.(type) {
		case []any:
			return value
		case map[string]any:
			return []any{value}
		case string:
			text = value
		}
	}
	lines := strings.Split(text, "\n")
	items := make([]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if line == "" {
			continue
		}
		items = append(items, line)
	}
	return items
}

func normalizeTodoEntry(item any) (map[string]any, bool) {
	switch v := item.(type) {
	case string:
		content := strings.TrimSpace(v)
		if content == "" {
			return nil, false
		}
		return map[string]any{
			"content":     content,
			"status":      "pending",
			"active_form": "",
		}, true
	case map[string]any:
		content := ""
		id := ""
		for _, key := range []string{"id", "task_id", "taskId"} {
			if val, ok := v[key].(string); ok && strings.TrimSpace(val) != "" {
				id = strings.TrimSpace(val)
				break
			}
		}
		for _, key := range []string{"content", "title", "name", "task", "text"} {
			if val, ok := v[key].(string); ok && strings.TrimSpace(val) != "" {
				content = strings.TrimSpace(val)
				break
			}
		}
		if content == "" {
			return nil, false
		}
		status := ""
		if val, ok := v["status"].(string); ok {
			status = strings.TrimSpace(val)
		}
		if status == "" {
			if completed, ok := v["completed"].(bool); ok {
				if completed {
					status = "completed"
				} else {
					status = "pending"
				}
			}
		}
		if status == "" {
			status = "pending"
		}
		activeForm := ""
		if val, ok := v["active_form"].(string); ok {
			activeForm = strings.TrimSpace(val)
		} else if val, ok := v["activeForm"].(string); ok {
			activeForm = strings.TrimSpace(val)
		}
		return map[string]any{
			"id":          id,
			"content":     content,
			"status":      status,
			"active_form": activeForm,
		}, true
	default:
		return nil, false
	}
}
