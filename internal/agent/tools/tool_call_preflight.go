package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"charm.land/fantasy"
	"charm.land/fantasy/schema"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/filetracker"
	mvdanShell "mvdan.cc/sh/v3/shell"
)

var (
	validationTrackerMu sync.RWMutex
	validationTracker   filetracker.Service
)

type skipPreparedToolUsageKey string

const SkipPreparedToolUsageContextKey skipPreparedToolUsageKey = "skip_prepared_tool_usage"

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
	if err := validateModeAwareToolCall(ctx, call.Name); err != nil {
		return call, nil, err
	}

	normalized, ok := NormalizeToolCall(call, tools)
	if !ok {
		suggestions := FindSimilarToolNames(call.Name, tools)
		if len(suggestions) > 0 {
			return call, nil, NewToolGuidanceError(
				call.Name,
				"tool_not_found",
				"Unknown tool call.",
				fmt.Sprintf(
					"tool not found: %s. Stop inventing tool names. Use one exact tool name from the current registry only. Suggested matches: %s. If you need repo search use tool_search/rg_files/rg/ls; file reads use single_view or agentic_view; edits use edit or agentic_edit; delegation use spawn_agent/send_input/wait.",
					call.Name,
					strings.Join(suggestions, ", "),
				),
			)
		}
		available := make([]string, 0, len(tools))
		for name := range tools {
			available = append(available, name)
		}
		return call, nil, NewToolGuidanceError(
			call.Name,
			"tool_not_found",
			"Unknown tool call.",
			fmt.Sprintf(
				"tool not found: %s. Stop inventing tool names. Use one exact tool name from the current registry only. Available tools: %s.",
				call.Name,
				strings.Join(available, ", "),
			),
		)
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
	call, tool, input = rewriteToHarnessWhenRequired(ctx, call, tool, input, tools)
	call, tool, input = rewriteToStructuredDiscoveryWhenPreferred(ctx, call, tool, input, tools)
	call, tool, input = rewriteToContextReadWhenRequired(ctx, call, tool, input, tools)
	call, tool, input = rewriteBroadReadDetourToStructuredDiscovery(ctx, call, tool, input, tools)
	call, tool, input = rewriteBroadSkillDetourToDiscovery(ctx, call, tool, input, tools)
	call, tool, input = rewriteInitializationSkillDetourToDiscovery(ctx, call, tool, input, tools)
	call, tool, input = rewriteRepeatedInitializationSkillDetour(ctx, call, tool, input, tools)
	call, tool, input = rewriteInitializationArtifactWriteDetour(ctx, call, tool, input, tools)
	call, tool, input = rewriteInitializationMemoryArtifactDetour(ctx, call, tool, input, tools)
	call, tool, input = rewriteLateTurnImplementationExecutionFocus(ctx, call, tool, input, tools)
	call, tool, input = rewriteRedundantInitializationDiscovery(ctx, call, tool, input, tools)
	call, tool, input = rewriteForbiddenDiscoveryBash(ctx, call, tool, input, tools)
	call, tool, input = rewriteToExplicitPlanWhenRequired(ctx, call, tool, input, tools)
	call, tool, input = rewriteDeepPlanningTransition(ctx, call, tool, input, tools)
	input = repairEmptyGuardrailedUpdatePlan(ctx, call.Name, input)

	if err := enforceTurnPolicy(ctx, call.Name, input); err != nil {
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

	recordPreparedToolUsage(ctx, call.Name, input)

	return call, tool, nil
}

func validateModeAwareToolCall(ctx context.Context, toolName string) error {
	canonical := canonicalToolNameForModePolicy(toolName)
	if canonical == "" {
		return nil
	}
	return planmode.ValidateModeToolCall(GetSessionModeFromContext(ctx), canonical)
}

func canonicalToolNameForModePolicy(name string) string {
	name = stripToolNamespace(name)
	if name == "" {
		return ""
	}
	if alias := toolNameAlias(name); alias != "" {
		return alias
	}
	return normalizeToolName(name)
}

func parseToolInput(raw string, toolName string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return map[string]any{}, nil
	}
	obj, err := parseFlexibleToolInputValue(raw)
	if err != nil {
		if coerced, ok := coerceInputFromString(toolName, stripToolInputCodeFence(raw)); ok {
			return coerced, nil
		}
		return nil, fmt.Errorf("tool input must be valid JSON: %w", err)
	}
	return normalizeParsedToolInput(toolName, obj)
}

func parseFlexibleToolInputValue(raw string) (any, error) {
	candidates := []string{
		strings.TrimSpace(raw),
		strings.TrimSpace(stripToolInputCodeFence(raw)),
	}
	if extracted, ok := extractBalancedJSON(raw); ok {
		candidates = append(candidates, strings.TrimSpace(extracted))
	}

	var lastErr error
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		obj, state, err := schema.ParsePartialJSON(candidate)
		if state != schema.ParseStateFailed {
			return obj, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("unable to parse tool input")
	}
	return nil, lastErr
}

func normalizeParsedToolInput(toolName string, obj any) (map[string]any, error) {
	for depth := 0; depth < 6; depth++ {
		switch value := obj.(type) {
		case map[string]any:
			if next, ok := unwrapToolInputMap(toolName, value); ok {
				obj = next
				continue
			}
			return value, nil
		case string:
			if parsed, err := parseFlexibleToolInputValue(value); err == nil {
				obj = parsed
				continue
			}
			if coerced, ok := coerceInputFromString(toolName, value); ok {
				return coerced, nil
			}
			return nil, errors.New("tool input must be a JSON object")
		case []any:
			if len(value) == 1 {
				obj = value[0]
				continue
			}
			return nil, errors.New("tool input must be a JSON object")
		default:
			return nil, errors.New("tool input must be a JSON object")
		}
	}
	return nil, errors.New("tool input nesting is too deep")
}

func unwrapToolInputMap(toolName string, input map[string]any) (any, bool) {
	if len(input) == 0 {
		return nil, false
	}
	for _, key := range []string{"arguments", "args", "input", "params", "parameters", "payload", "data"} {
		raw, ok := input[key]
		if !ok {
			continue
		}
		if len(outerInputExtras(input, key)) > 0 {
			continue
		}
		if next, ok := normalizeWrappedToolInputValue(toolName, raw); ok {
			return next, true
		}
	}
	if len(input) == 1 {
		for key, raw := range input {
			if !toolEnvelopeMatches(toolName, key) {
				continue
			}
			if next, ok := normalizeWrappedToolInputValue(toolName, raw); ok {
				return next, true
			}
		}
	}
	return nil, false
}

func normalizeWrappedToolInputValue(toolName string, raw any) (any, bool) {
	switch value := raw.(type) {
	case map[string]any:
		return value, true
	case string:
		if parsed, err := parseFlexibleToolInputValue(value); err == nil {
			return parsed, true
		}
		if coerced, ok := coerceInputFromString(toolName, value); ok {
			return coerced, true
		}
	case []any:
		if len(value) == 1 {
			return normalizeWrappedToolInputValue(toolName, value[0])
		}
	}
	return nil, false
}

func outerInputExtras(input map[string]any, wrappedKey string) map[string]any {
	hasExplicitEnvelopeName := false
	for _, key := range []string{"name", "tool", "tool_name", "toolName"} {
		if _, ok := input[key]; ok {
			hasExplicitEnvelopeName = true
			break
		}
	}

	extras := make(map[string]any)
	for key, value := range input {
		if key == wrappedKey {
			continue
		}
		switch key {
		case "name", "tool", "tool_name", "toolName":
			continue
		case "id", "call_id", "type":
			if hasExplicitEnvelopeName {
				continue
			}
			extras[key] = value
		default:
			extras[key] = value
		}
	}
	if len(extras) == 0 {
		return nil
	}
	return extras
}

func toolEnvelopeMatches(toolName, key string) bool {
	if normalizeToolName(key) == normalizeToolName(toolName) {
		return true
	}
	if alias := toolNameAlias(key); alias != "" && normalizeToolName(alias) == normalizeToolName(toolName) {
		return true
	}
	return false
}

func stripToolInputCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return trimmed
	}
	lines = lines[1:]
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractBalancedJSON(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	start := strings.IndexAny(trimmed, "{[")
	if start == -1 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	var opener rune
	for idx, r := range trimmed[start:] {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString:
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
		case r == '{' || r == '[':
			if depth == 0 {
				opener = r
			}
			depth++
		case r == '}' || r == ']':
			if depth == 0 {
				continue
			}
			if (opener == '{' && r != '}') || (opener == '[' && r != ']') {
				continue
			}
			depth--
			if depth == 0 {
				return trimmed[start : start+idx+1], true
			}
		}
	}
	return "", false
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
	case UpdatePlanToolName:
		return call, tool, repairUpdatePlanInput(input), nil
	case EditToolName, SingleEditToolName, AgenticEditToolName:
		return repairEditCall(ctx, call, tool, input, tools)
	case WriteToolName:
		normalizeKey(input, "file_path", "path", "file", "filename", "filepath")
		normalizeKey(input, "content", "text", "body", "data", "file_content")
	case RunHarnessToolName:
		normalizeKey(input, "task", "prompt", "request", "instruction", "work")
		normalizeKey(input, "working_dir", "working_directory", "cwd", "dir", "directory")
		normalizeKey(input, "goal_type", "goal", "task_type")
		normalizeKey(input, "force", "required")
		normalizeKey(input, "mode", "execution_mode")
	case ApplyPatchToolName:
		normalizeKey(input, "file_path", "path", "file", "filename", "filepath")
		normalizeKey(input, "unified_diff", "patch", "diff")
		normalizeKey(input, "execution_mode", "mode")
		normalizeKey(input, "justification", "reason", "description")

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
				if toolName == SingleViewToolName {
					if _, exists := tools[toolName]; !exists {
						toolName = ViewToolName
					}
				}
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
		normalizeKey(input, "queries", "searches")
		normalizeKey(input, "max_results", "num_results", "count", "limit", "results")
	case GoogleSearchToolName:
		normalizeKey(input, "query", "q", "search", "search_query", "term", "prompt")
		normalizeKey(input, "url", "uri", "link", "href")
		normalizeKey(input, "urls", "links", "resources")
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
	case "view_memory":
		normalizeKey(input, "mode", "type", "scope")
		normalizeKey(input, "session_id", "session", "id")
		normalizeKey(input, "limit", "count", "max_results")
		normalizeKey(input, "query", "q", "search", "term")
		normalizeKey(input, "since", "timestamp", "after")
		if limit, ok := coerceInt(input["limit"]); ok && limit != 0 {
			input["limit"] = limit
		}
	case "refresh_memory":
		normalizeKey(input, "session_id", "session", "id")
		normalizeKey(input, "reason", "note", "why")
	case "recall_memory":
		normalizeKey(input, "query", "q", "search", "term")
		normalizeKey(input, "filter", "type", "scope")
		normalizeKey(input, "limit", "count", "max_results")
		if limit, ok := coerceInt(input["limit"]); ok && limit != 0 {
			input["limit"] = limit
		}
	case "save_memory":
		normalizeKey(input, "event_type", "type", "event", "kind", "category")
		normalizeKey(input, "content", "data", "value", "payload")

	// ── Skill tools ──────────────────────────────────────────────────
	case LoadSkillToolName:
		normalizeKey(input, "name", "skill", "skill_name", "skillName")
	case InstallSkillToolName:
		normalizeKey(input, "query", "q", "search", "term", "skill", "skill_name", "name")
	case IndexCodebaseToolName:
		normalizeKey(input, "force", "refresh", "rebuild")
	case "list_skills":
		// No parameters needed
	case "search_skills":
		normalizeKey(input, "query", "q", "search", "term")
		normalizeKey(input, "limit", "count", "max_results")
		if limit, ok := coerceInt(input["limit"]); ok && limit != 0 {
			input["limit"] = limit
		}

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
	case ListMCPResourcesToolName, ReadMCPResourceToolName, ConnectMCPToolName, InstallMCPToolName:
		normalizeKey(input, "mcp_name", "server", "server_name", "name")
		inferMCPName(input)
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
	case "collect_result":
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

func rewriteToHarnessWhenRequired(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	requirement := GetHarnessRequirementFromContext(ctx)
	if !requirement.Required {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if canonical == "" || canonical == RunHarnessToolName {
		return call, tool, input
	}
	if _, ok := GetHarnessDecision(ctx); ok {
		return call, tool, input
	}
	shouldRewrite := isHarnessProtectedTool(canonical)
	if requirement.RequireBeforeDiscovery && isHarnessDiscoveryTool(canonical) {
		shouldRewrite = true
	}
	if !shouldRewrite {
		return call, tool, input
	}
	runHarnessTool, ok := tools[RunHarnessToolName]
	if !ok {
		return call, tool, input
	}
	call.Name = RunHarnessToolName
	tool = runHarnessTool
	input = map[string]any{
		"task": requirement.Task,
	}
	if workingDir := strings.TrimSpace(GetWorkingDirFromContext(ctx)); workingDir != "" {
		input["working_dir"] = workingDir
	}
	return call, tool, input
}

func rewriteToStructuredDiscoveryWhenPreferred(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if !policy.PreferStructuredDiscovery {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if canonical != LSToolName {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	if usage != nil && usage.Total(ToolSearchToolName, RGFilesToolName, RGToolName, GlobToolName, GrepToolName) > 0 {
		return call, tool, input
	}
	if next, ok := tools[ToolSearchToolName]; ok {
		call.Name = ToolSearchToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryQuery(ctx, policy),
		}
		return call, tool, input
	}
	if next, ok := tools[RGFilesToolName]; ok {
		call.Name = RGFilesToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryFileQuery(ctx, policy),
			"limit": 40,
		}
		if path, ok := input["path"].(string); ok && strings.TrimSpace(path) != "" {
			input["path"] = strings.TrimSpace(path)
		}
		return call, tool, input
	}
	return call, tool, input
}

func rewriteToContextReadWhenRequired(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if !policy.RequireContextRead {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if !isLearnedContextProtectedTool(canonical) {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	if hasBroadInitializationContext(usage) {
		return call, tool, input
	}
	if usage != nil && usage.StructuredEvidenceCount() > 0 && usage.ReadEvidenceCount() == 0 {
		if path := firstGuardedContextReadPathFromInput(ctx, canonical, input); path != "" {
			if next, ok := tools[SingleViewToolName]; ok {
				call.Name = SingleViewToolName
				tool = next
				input = map[string]any{"file_path": path}
				return call, tool, input
			}
			if next, ok := tools[ViewToolName]; ok {
				call.Name = ViewToolName
				tool = next
				input = map[string]any{"file_path": path}
				return call, tool, input
			}
		}
	}
	if next, ok := tools[ToolSearchToolName]; ok {
		call.Name = ToolSearchToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryQuery(ctx, policy),
		}
		return call, tool, input
	}
	if next, ok := tools[RGFilesToolName]; ok {
		call.Name = RGFilesToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryFileQuery(ctx, policy),
			"limit": 40,
		}
		return call, tool, input
	}
	return call, tool, input
}

func rewriteToExplicitPlanWhenRequired(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if !policy.RequireExplicitPlan {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if !shouldBlockForExplicitPlan(canonical, usage) {
		return call, tool, input
	}
	next, ok := tools[UpdatePlanToolName]
	if !ok {
		return call, tool, input
	}
	call.Name = UpdatePlanToolName
	tool = next
	input = buildExplicitPlanCheckpointInput(ctx, policy, canonical)
	return call, tool, input
}

func rewriteDeepPlanningTransition(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	if !GetDeepPlanningActiveFromContext(ctx) {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	if usage != nil && usage.HasPublishedPlan() {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if canonical == "" || isDeepPlanningAllowedTool(canonical) {
		return call, tool, input
	}
	if hasStructuredDiscoveryOrReadContext(usage) {
		if next, ok := tools[UpdatePlanToolName]; ok {
			call.Name = UpdatePlanToolName
			tool = next
			input = buildDeepPlanningCheckpointPlanInput(ctx, canonical)
			return call, tool, input
		}
	}
	if next, ok := tools[ToolSearchToolName]; ok {
		call.Name = ToolSearchToolName
		tool = next
		input = map[string]any{
			"query": deepPlanningStructuredDiscoveryQuery(ctx),
		}
		return call, tool, input
	}
	if next, ok := tools[RGFilesToolName]; ok {
		call.Name = RGFilesToolName
		tool = next
		input = map[string]any{
			"query": deepPlanningStructuredDiscoveryFileQuery(ctx),
			"limit": 40,
		}
		return call, tool, input
	}
	return call, tool, input
}

func rewriteInitializationSkillDetourToDiscovery(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if strings.TrimSpace(policy.TaskFamily) != "initialize/broad/codebase" {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if !isInitializationSkillDetourTool(canonical) {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	if hasStructuredDiscoveryOrReadContext(usage) {
		return call, tool, input
	}
	if next, ok := tools[ToolSearchToolName]; ok {
		call.Name = ToolSearchToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryQuery(ctx, policy),
		}
		return call, tool, input
	}
	if next, ok := tools[RGFilesToolName]; ok {
		call.Name = RGFilesToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryFileQuery(ctx, policy),
			"limit": 40,
		}
		return call, tool, input
	}
	return call, tool, input
}

func rewriteBroadSkillDetourToDiscovery(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	taskFamily := strings.TrimSpace(policy.TaskFamily)
	if taskFamily == "" || !policy.RequireContextRead {
		return call, tool, input
	}
	if !strings.HasPrefix(taskFamily, "design/") &&
		!strings.HasPrefix(taskFamily, "research/") &&
		!strings.HasPrefix(taskFamily, "review/") &&
		!strings.HasPrefix(taskFamily, "migration/") &&
		!strings.HasPrefix(taskFamily, "implementation/") {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if !isInitializationSkillDetourTool(canonical) {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	if HasRequiredContextReadEvidence(usage) {
		return call, tool, input
	}
	if next, ok := tools[ToolSearchToolName]; ok {
		call.Name = ToolSearchToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryQuery(ctx, policy),
		}
		return call, tool, input
	}
	if next, ok := tools[RGFilesToolName]; ok {
		call.Name = RGFilesToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryFileQuery(ctx, policy),
			"limit": 40,
		}
		return call, tool, input
	}
	return call, tool, input
}

func rewriteBroadReadDetourToStructuredDiscovery(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	taskFamily := strings.TrimSpace(policy.TaskFamily)
	if taskFamily == "" || !policy.RequireContextRead {
		return call, tool, input
	}
	if !strings.HasPrefix(taskFamily, "design/") &&
		!strings.HasPrefix(taskFamily, "research/") &&
		!strings.HasPrefix(taskFamily, "review/") &&
		!strings.HasPrefix(taskFamily, "migration/") &&
		!strings.HasPrefix(taskFamily, "implementation/") {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	if usage == nil || usage.ReadEvidenceCount() < 1 || usage.StructuredEvidenceCount() > 0 {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if !isBroadReadDetourTool(canonical, input) {
		return call, tool, input
	}
	if next, ok := tools[ToolSearchToolName]; ok {
		call.Name = ToolSearchToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryQuery(ctx, policy),
		}
		return call, tool, input
	}
	if next, ok := tools[RGFilesToolName]; ok {
		call.Name = RGFilesToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryFileQuery(ctx, policy),
			"limit": 40,
		}
		return call, tool, input
	}
	return call, tool, input
}

func rewriteRepeatedInitializationSkillDetour(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if strings.TrimSpace(policy.TaskFamily) != "initialize/broad/codebase" {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if !isInitializationSkillDetourTool(canonical) {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	if usage == nil || usage.Total("list_skills", "search_skills", LoadSkillToolName) < 1 {
		return call, tool, input
	}
	if next, ok := tools[RGFilesToolName]; ok {
		call.Name = RGFilesToolName
		tool = next
		input = buildRepeatedInitializationRouteCorrectionInput(ctx, policy, usage)
		return call, tool, input
	}
	if next, ok := tools[ToolSearchToolName]; ok {
		call.Name = ToolSearchToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryQuery(ctx, policy),
		}
		return call, tool, input
	}
	return call, tool, input
}

func rewriteInitializationArtifactWriteDetour(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if !shouldRestrictInitializationArtifactWrite(policy, canonical) {
		return call, tool, input
	}
	blockedPath := firstNonInitializationArtifactWritePath(ctx, canonical, input)
	if blockedPath == "" {
		return call, tool, input
	}
	next, ok := tools[UpdatePlanToolName]
	if !ok {
		return call, tool, input
	}
	call.Name = UpdatePlanToolName
	tool = next
	input = buildInitializationArtifactCorrectionPlanInput(ctx, blockedPath)
	return call, tool, input
}

func rewriteInitializationMemoryArtifactDetour(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if strings.TrimSpace(policy.TaskFamily) != "initialize/broad/codebase" {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	paths := extractPotentialMemoryPaths(canonical, input)
	if len(paths) == 0 {
		return call, tool, input
	}
	for _, rawPath := range paths {
		if _, isMemoryPath, _ := classifyMemoryArtifactPath(ctx, rawPath); !isMemoryPath {
			continue
		}
		if next, ok := tools[ToolSearchToolName]; ok {
			call.Name = ToolSearchToolName
			tool = next
			input = map[string]any{
				"query": preferredStructuredDiscoveryQuery(ctx, policy),
			}
			return call, tool, input
		}
		if next, ok := tools[RGFilesToolName]; ok {
			call.Name = RGFilesToolName
			tool = next
			input = map[string]any{
				"query": preferredStructuredDiscoveryFileQuery(ctx, policy),
				"limit": 40,
			}
			return call, tool, input
		}
		return call, tool, input
	}
	return call, tool, input
}

func rewriteLateTurnImplementationExecutionFocus(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if !strings.HasPrefix(strings.TrimSpace(policy.TaskFamily), "implementation/") {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	if !shouldForceLateImplementationExecutionFocus(ctx, usage) {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if !isLateImplementationExecutionFocusTool(canonical, input) {
		return call, tool, input
	}
	if verificationTool, verificationInput, ok := buildGenericArtifactVerificationRewrite(tools, usage); ok {
		call.Name = verificationTool.Info().Name
		tool = verificationTool
		input = verificationInput
		return call, tool, input
	}
	if mostWrittenTool, mostWrittenInput, ok := buildMostWrittenFileReadRewrite(tools, usage); ok {
		call.Name = mostWrittenTool.Info().Name
		tool = mostWrittenTool
		input = mostWrittenInput
		return call, tool, input
	}
	next, ok := tools[UpdatePlanToolName]
	if !ok {
		return call, tool, input
	}
	call.Name = UpdatePlanToolName
	tool = next
	input = buildImplementationExecutionFocusPlanInput(ctx, usage, canonical)
	return call, tool, input
}

func rewriteRedundantInitializationDiscovery(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if strings.TrimSpace(policy.TaskFamily) != "initialize/broad/codebase" {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if !isInitializationDiscoveryTool(canonical) {
		return call, tool, input
	}
	usage := GetToolUsageStateFromContext(ctx)
	if !shouldRewriteRedundantInitializationDiscovery(canonical, usage) {
		return call, tool, input
	}
	if verificationTool, verificationInput, ok := buildInitializationVerificationRewrite(tools, usage); ok {
		call.Name = verificationTool.Info().Name
		tool = verificationTool
		input = verificationInput
		return call, tool, input
	}
	next, ok := tools[UpdatePlanToolName]
	if !ok {
		return call, tool, input
	}
	call.Name = UpdatePlanToolName
	tool = next
	input = buildInitializationExecutionFocusPlanInput(canonical, usage)
	return call, tool, input
}

func rewriteForbiddenDiscoveryBash(
	ctx context.Context,
	call fantasy.ToolCall,
	tool fantasy.AgentTool,
	input map[string]any,
	tools map[string]fantasy.AgentTool,
) (fantasy.ToolCall, fantasy.AgentTool, map[string]any) {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if !policy.ForbidBashDiscovery {
		return call, tool, input
	}
	canonical := canonicalToolNameForModePolicy(call.Name)
	if canonical == "" {
		canonical = normalizeToolName(call.Name)
	}
	if canonical != BashToolName {
		return call, tool, input
	}
	command, _ := input["command"].(string)
	command = strings.TrimSpace(command)
	if !looksLikeDiscoveryShellCommand(command) {
		return call, tool, input
	}
	if strings.TrimSpace(policy.TaskFamily) == "initialize/broad/codebase" {
		usage := GetToolUsageStateFromContext(ctx)
		if shouldForceLateInitializationExecutionFocus(ctx, usage) {
			if verificationTool, verificationInput, ok := buildGenericArtifactVerificationRewrite(tools, usage); ok {
				call.Name = verificationTool.Info().Name
				tool = verificationTool
				input = verificationInput
				return call, tool, input
			}
			if next, ok := tools[UpdatePlanToolName]; ok {
				call.Name = UpdatePlanToolName
				tool = next
				input = buildInitializationExecutionFocusPlanInput(BashToolName, usage)
				return call, tool, input
			}
		}
	}
	if nextName, nextInput, ok := simpleBashToTool(command); ok {
		if next, exists := tools[nextName]; exists {
			call.Name = nextName
			tool = next
			input = nextInput
			return call, tool, input
		}
	}
	if next, ok := tools[ToolSearchToolName]; ok {
		call.Name = ToolSearchToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryQuery(ctx, policy),
		}
		return call, tool, input
	}
	if next, ok := tools[RGFilesToolName]; ok {
		call.Name = RGFilesToolName
		tool = next
		input = map[string]any{
			"query": preferredStructuredDiscoveryFileQuery(ctx, policy),
			"limit": 40,
		}
		return call, tool, input
	}
	return call, tool, input
}

func shouldForceLateImplementationExecutionFocus(ctx context.Context, usage *ToolUsageState) bool {
	if usage == nil || !HasRequiredContextReadEvidence(usage) || !usage.HasPublishedPlan() {
		return false
	}
	metrics := usage.SnapshotDeterministicLoopMetrics()
	if totalDeterministicWrites(metrics) == 0 {
		return false
	}
	remaining := GetRemainingTurnStepsFromContext(ctx)
	if remaining > 0 && remaining <= 3 {
		return true
	}
	if metrics.TotalCalls >= 9 {
		return true
	}
	return maxDeterministicWriteCount(metrics) >= 3
}

func shouldForceLateInitializationExecutionFocus(ctx context.Context, usage *ToolUsageState) bool {
	if usage == nil || !hasBroadInitializationContext(usage) {
		return false
	}
	if len(usage.PendingArtifactVerificationPaths()) > 0 {
		return true
	}
	remaining := GetRemainingTurnStepsFromContext(ctx)
	if remaining > 0 && remaining <= 3 && usage.HasPublishedPlan() {
		return true
	}
	return totalInitializationDiscoveryCalls(usage) >= 5 && usage.HasPublishedPlan()
}

func isLateImplementationExecutionFocusTool(toolName string, input map[string]any) bool {
	switch toolName {
	case ToolSearchToolName, RGFilesToolName, RGToolName, GlobToolName, GrepToolName, LSToolName:
		return true
	case BashToolName:
		command, _ := input["command"].(string)
		return looksLikeDiscoveryShellCommand(command)
	case EditToolName, SingleEditToolName, AgenticEditToolName, ApplyPatchToolName, WriteToolName:
		return true
	default:
		return false
	}
}

func isBroadReadDetourTool(toolName string, input map[string]any) bool {
	switch toolName {
	case AgenticViewToolName, ViewToolName, SingleViewToolName, LSToolName:
		return true
	case BashToolName:
		command, _ := input["command"].(string)
		return looksLikeDiscoveryShellCommand(command)
	case RGToolName:
		pattern, _ := input["pattern"].(string)
		query, _ := input["query"].(string)
		return strings.TrimSpace(pattern) == "" && strings.TrimSpace(query) == ""
	default:
		return false
	}
}

func firstGuardedContextReadPathFromInput(ctx context.Context, toolName string, input map[string]any) string {
	for _, path := range extractArtifactWritePathsFromInput(toolName, input) {
		if resolved := resolveArtifactVerificationPath(ctx, path); resolved != "" {
			return resolved
		}
	}
	for _, path := range extractArtifactVerificationPathsFromInput(toolName, input) {
		if resolved := resolveArtifactVerificationPath(ctx, path); resolved != "" {
			return resolved
		}
	}
	return ""
}

func preferredStructuredDiscoveryQuery(ctx context.Context, policy LearnedToolPolicy) string {
	if requirement := GetHarnessRequirementFromContext(ctx); strings.TrimSpace(requirement.Task) != "" {
		return strings.TrimSpace(requirement.Task)
	}
	if taskFamily := strings.TrimSpace(policy.TaskFamily); taskFamily != "" {
		return taskFamily
	}
	if reason := strings.TrimSpace(policy.Reason); reason != "" {
		return reason
	}
	return "repo architecture and initialization"
}

func preferredStructuredDiscoveryFileQuery(ctx context.Context, policy LearnedToolPolicy) string {
	query := strings.ToLower(strings.TrimSpace(preferredStructuredDiscoveryQuery(ctx, policy)))
	switch {
	case strings.Contains(query, "initialize"), strings.Contains(query, "codebase"), strings.Contains(query, "repository"):
		return "readme agents package go mod cmd internal app src config main"
	default:
		return "readme agents main app src internal cmd"
	}
}

func buildRepeatedInitializationRouteCorrectionInput(ctx context.Context, policy LearnedToolPolicy, usage *ToolUsageState) map[string]any {
	query := preferredStructuredDiscoveryFileQuery(ctx, policy)
	limit := 40
	if usage != nil && usage.Total(ToolSearchToolName, RGFilesToolName, RGToolName, GlobToolName, GrepToolName) > 0 {
		query = "readme agents config package go mod main cmd internal app src"
		limit = 24
	}
	if usage != nil && usage.Total(AgenticViewToolName, ViewToolName, SingleViewToolName) > 0 {
		query = "config package go mod tsconfig main cmd internal app src server api"
		limit = 20
	}
	return map[string]any{
		"query": query,
		"limit": limit,
	}
}

func buildInitializationVerificationRewrite(tools map[string]fantasy.AgentTool, usage *ToolUsageState) (fantasy.AgentTool, map[string]any, bool) {
	if usage == nil {
		return nil, nil, false
	}
	pending := usage.PendingArtifactVerificationPaths()
	if len(pending) == 0 {
		return nil, nil, false
	}
	if len(pending) == 1 {
		if next, ok := tools[SingleViewToolName]; ok {
			return next, map[string]any{"file_path": pending[0]}, true
		}
		if next, ok := tools[ViewToolName]; ok {
			return next, map[string]any{"file_path": pending[0]}, true
		}
	}
	if next, ok := tools[AgenticViewToolName]; ok {
		paths := make([]any, 0, len(pending))
		for _, path := range pending {
			paths = append(paths, path)
		}
		return next, map[string]any{"file_paths": paths}, true
	}
	return nil, nil, false
}

func buildGenericArtifactVerificationRewrite(tools map[string]fantasy.AgentTool, usage *ToolUsageState) (fantasy.AgentTool, map[string]any, bool) {
	return buildInitializationVerificationRewrite(tools, usage)
}

func buildMostWrittenFileReadRewrite(tools map[string]fantasy.AgentTool, usage *ToolUsageState) (fantasy.AgentTool, map[string]any, bool) {
	if usage == nil {
		return nil, nil, false
	}
	metrics := usage.SnapshotDeterministicLoopMetrics()
	path := mostWrittenDeterministicPath(metrics)
	if path == "" {
		return nil, nil, false
	}
	if next, ok := tools[SingleViewToolName]; ok {
		return next, map[string]any{"file_path": path}, true
	}
	if next, ok := tools[ViewToolName]; ok {
		return next, map[string]any{"file_path": path}, true
	}
	return nil, nil, false
}

func buildInitializationExecutionFocusPlanInput(blockedToolName string, usage *ToolUsageState) map[string]any {
	explanation := fmt.Sprintf(
		"Broad initialization already has enough repository evidence. Stop another %s sweep and move directly toward the AGENTS.md artifact.",
		blockedToolName,
	)
	writeStatus := StepStatusInProgress
	verifyStatus := StepStatusPending
	if usage != nil && len(usage.PendingArtifactVerificationPaths()) > 0 {
		writeStatus = StepStatusCompleted
		verifyStatus = StepStatusInProgress
	}
	args := NormalizeUpdatePlanArgs(UpdatePlanArgs{
		Explanation: &explanation,
		Plan: []PlanItem{
			{Step: "Collect repository evidence for AGENTS.md", Status: StepStatusCompleted},
			{Step: "Write or refine AGENTS.md only", Status: writeStatus},
			{Step: "Verify AGENTS.md after writing", Status: verifyStatus},
			{Step: "Deliver final initialization summary", Status: StepStatusPending},
		},
	})
	plan := make([]any, 0, len(args.Plan))
	for _, item := range args.Plan {
		plan = append(plan, map[string]any{
			"step":   item.Step,
			"status": item.Status,
		})
	}
	return map[string]any{
		"explanation": explanation,
		"plan":        plan,
	}
}

func buildImplementationExecutionFocusPlanInput(ctx context.Context, usage *ToolUsageState, blockedToolName string) map[string]any {
	explanation := fmt.Sprintf(
		"Broad implementation is late in the turn. Stop low-yield %s churn, finish the smallest correct change, and move directly into verification.",
		blockedToolName,
	)
	finishStatus := StepStatusInProgress
	verifyStatus := StepStatusPending
	if usage != nil && len(usage.PendingArtifactVerificationPaths()) > 0 {
		finishStatus = StepStatusCompleted
		verifyStatus = StepStatusInProgress
	}
	args := NormalizeUpdatePlanArgs(UpdatePlanArgs{
		Explanation: &explanation,
		Plan: []PlanItem{
			{Step: "Lock the exact implementation files and invariants", Status: StepStatusCompleted},
			{Step: "Finish the smallest correct code change only", Status: finishStatus},
			{Step: "Verify the changed code path before any more edits", Status: verifyStatus},
			{Step: "Deliver the final implementation summary", Status: StepStatusPending},
		},
	})
	if task := strings.TrimSpace(GetHarnessRequirementFromContext(ctx).Task); task != "" {
		args.Plan[1].Step = "Finish the smallest correct code change for " + task
	}
	plan := make([]any, 0, len(args.Plan))
	for _, item := range args.Plan {
		plan = append(plan, map[string]any{
			"step":   item.Step,
			"status": item.Status,
		})
	}
	return map[string]any{
		"explanation": explanation,
		"plan":        plan,
	}
}

func buildInitializationCheckpointPlanInput(ctx context.Context) map[string]any {
	explanation := "Lock the broad initialization plan before continuing. Stay scoped to AGENTS.md, then verify that artifact before concluding."
	firstStepStatus := StepStatusInProgress
	if usage := GetToolUsageStateFromContext(ctx); hasStructuredDiscoveryOrReadContext(usage) {
		firstStepStatus = StepStatusCompleted
	}
	args := NormalizeUpdatePlanArgs(UpdatePlanArgs{
		Explanation: &explanation,
		Plan: []PlanItem{
			{Step: "Collect repository evidence for AGENTS.md", Status: firstStepStatus},
			{Step: "Identify core entrypoints, domains, and constraints", Status: StepStatusInProgress},
			{Step: "Write or refine AGENTS.md only", Status: StepStatusPending},
			{Step: "Verify AGENTS.md after writing", Status: StepStatusPending},
		},
	})
	plan := make([]any, 0, len(args.Plan))
	for _, item := range args.Plan {
		plan = append(plan, map[string]any{
			"step":   item.Step,
			"status": item.Status,
		})
	}
	return map[string]any{
		"explanation": explanation,
		"plan":        plan,
	}
}

func buildDeepPlanningCheckpointPlanInput(ctx context.Context, blockedToolName string) map[string]any {
	explanation := fmt.Sprintf(
		"Deep planning evidence is sufficient. Lock the concrete execution checklist now before using %s or leaving the planning phase.",
		blockedToolName,
	)
	firstStepStatus := StepStatusInProgress
	if usage := GetToolUsageStateFromContext(ctx); hasStructuredDiscoveryOrReadContext(usage) {
		firstStepStatus = StepStatusCompleted
	}
	args := NormalizeUpdatePlanArgs(UpdatePlanArgs{
		Explanation: &explanation,
		Plan: []PlanItem{
			{Step: "Read AGENTS.md and the highest-signal files for the task", Status: firstStepStatus},
			{Step: "Map the relevant architecture, constraints, and edge cases", Status: StepStatusInProgress},
			{Step: "Execute the task against the published plan in small verified steps", Status: StepStatusPending},
			{Step: "Verify the final result and report the outcome concisely", Status: StepStatusPending},
		},
	})
	plan := make([]any, 0, len(args.Plan))
	for _, item := range args.Plan {
		plan = append(plan, map[string]any{
			"step":   item.Step,
			"status": item.Status,
		})
	}
	return map[string]any{
		"explanation": explanation,
		"plan":        plan,
	}
}

func isInitializationDiscoveryTool(toolName string) bool {
	switch toolName {
	case ToolSearchToolName, RGFilesToolName, RGToolName, GlobToolName, GrepToolName, LSToolName:
		return true
	default:
		return false
	}
}

func deepPlanningStructuredDiscoveryQuery(ctx context.Context) string {
	if task := strings.TrimSpace(GetHarnessRequirementFromContext(ctx).Task); task != "" {
		return task
	}
	if family := strings.TrimSpace(GetLearnedToolPolicyFromContext(ctx).TaskFamily); family != "" {
		return family
	}
	return "main relevant files, entrypoints, and architecture for the current task"
}

func deepPlanningStructuredDiscoveryFileQuery(ctx context.Context) string {
	if task := strings.TrimSpace(GetHarnessRequirementFromContext(ctx).Task); task != "" {
		return task
	}
	return "main relevant files for the current task"
}

func shouldRewriteRedundantInitializationDiscovery(toolName string, usage *ToolUsageState) bool {
	if usage == nil || !hasBroadInitializationContext(usage) {
		return false
	}
	if len(usage.PendingArtifactVerificationPaths()) > 0 {
		return true
	}
	if !usage.HasPublishedPlan() {
		return true
	}
	switch toolName {
	case LSToolName:
		return usage.Count(LSToolName) >= 1
	case ToolSearchToolName:
		return usage.Count(ToolSearchToolName) >= 1 && totalInitializationDiscoveryCalls(usage) >= 4
	case RGFilesToolName, RGToolName, GlobToolName, GrepToolName:
		return totalInitializationDiscoveryCalls(usage) >= 4
	default:
		return false
	}
}

func totalInitializationDiscoveryCalls(usage *ToolUsageState) int {
	if usage == nil {
		return 0
	}
	return usage.Total(ToolSearchToolName, RGFilesToolName, RGToolName, GlobToolName, GrepToolName, LSToolName)
}

func totalDeterministicWrites(metrics DeterministicLoopMetrics) int {
	total := 0
	for _, count := range metrics.WriteCounts {
		total += count
	}
	return total
}

func maxDeterministicWriteCount(metrics DeterministicLoopMetrics) int {
	maxCount := 0
	for _, count := range metrics.WriteCounts {
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}

func mostWrittenDeterministicPath(metrics DeterministicLoopMetrics) string {
	bestPath := ""
	bestCount := 0
	for path, count := range metrics.WriteCounts {
		if count > bestCount || (count == bestCount && bestPath != "" && path < bestPath) {
			bestPath = path
			bestCount = count
		}
	}
	return bestPath
}

func buildInitializationArtifactCorrectionPlanInput(ctx context.Context, blockedPath string) map[string]any {
	explanation := fmt.Sprintf(
		"Initialization is scoped to AGENTS.md only. Do not write %s. Refresh AGENTS.md instead, then verify that artifact before concluding.",
		blockedPath,
	)
	firstStepStatus := StepStatusInProgress
	if hasBroadInitializationContext(GetToolUsageStateFromContext(ctx)) {
		firstStepStatus = StepStatusCompleted
	}
	args := NormalizeUpdatePlanArgs(UpdatePlanArgs{
		Explanation: &explanation,
		Plan: []PlanItem{
			{Step: "Collect repository evidence for AGENTS.md", Status: firstStepStatus},
			{Step: "Write or refine AGENTS.md only", Status: StepStatusInProgress},
			{Step: "Verify AGENTS.md after writing", Status: StepStatusPending},
			{Step: "Deliver final initialization summary", Status: StepStatusPending},
		},
	})
	plan := make([]any, 0, len(args.Plan))
	for _, item := range args.Plan {
		plan = append(plan, map[string]any{
			"step":   item.Step,
			"status": item.Status,
		})
	}
	return map[string]any{
		"explanation": explanation,
		"plan":        plan,
	}
}

func isInitializationSkillDetourTool(toolName string) bool {
	switch toolName {
	case LoadSkillToolName, "search_skills", "list_skills":
		return true
	default:
		return false
	}
}

func hasStructuredDiscoveryOrReadContext(usage *ToolUsageState) bool {
	if usage == nil {
		return false
	}
	return usage.Total(ToolSearchToolName, RGFilesToolName, RGToolName, GlobToolName, GrepToolName, AgenticViewToolName, ViewToolName, SingleViewToolName) > 0
}

func repairUpdatePlanInput(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}

	rawPlan, ok := input["plan"]
	if !ok {
		return input
	}

	items, err := coerceObjectSlice(rawPlan)
	if err != nil {
		return input
	}

	plan := make([]PlanItem, 0, len(items))
	for _, item := range items {
		step, _ := item["step"].(string)
		status, _ := item["status"].(string)
		plan = append(plan, PlanItem{Step: step, Status: StepStatus(status)})
	}

	args := NormalizeUpdatePlanArgs(UpdatePlanArgs{
		Plan: plan,
	})
	if explanation, ok := input["explanation"].(string); ok {
		args.Explanation = &explanation
		args = NormalizeUpdatePlanArgs(args)
	}

	normalized := make([]any, 0, len(args.Plan))
	for _, item := range args.Plan {
		normalized = append(normalized, map[string]any{
			"step":   item.Step,
			"status": item.Status,
		})
	}
	if args.Explanation != nil {
		input["explanation"] = *args.Explanation
	} else {
		delete(input, "explanation")
	}
	input["plan"] = normalized
	return input
}

func repairEmptyGuardrailedUpdatePlan(ctx context.Context, toolName string, input map[string]any) map[string]any {
	if canonicalToolNameForModePolicy(toolName) != UpdatePlanToolName {
		return input
	}
	if !updatePlanInputHasNoSteps(input) {
		return input
	}

	policy := GetLearnedToolPolicyFromContext(ctx)
	switch {
	case strings.TrimSpace(policy.TaskFamily) == "initialize/broad/codebase":
		return buildInitializationCheckpointPlanInput(ctx)
	case policy.RequireExplicitPlan:
		return buildExplicitPlanCheckpointInput(ctx, policy, "analysis")
	default:
		return input
	}
}

func updatePlanInputHasNoSteps(input map[string]any) bool {
	if input == nil {
		return true
	}
	rawPlan, ok := input["plan"]
	if !ok {
		return true
	}
	items, err := coerceObjectSlice(rawPlan)
	if err != nil {
		return true
	}
	plan := make([]PlanItem, 0, len(items))
	for _, item := range items {
		step, _ := item["step"].(string)
		status, _ := item["status"].(string)
		plan = append(plan, PlanItem{Step: step, Status: StepStatus(status)})
	}
	return len(NormalizePlanItems(plan)) == 0
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
		return call, tool, input, NewToolGuidanceError(
			call.Name,
			"missing_file_path",
			"Missing file path.",
			"single_view/agentic_view require explicit file path arguments. Do not pass empty input or natural language. Use file_path for one file or file_paths for multiple files, then retry with real repo-relative paths.",
		)
	}

	input = map[string]any{}
	if call.Name == AgenticViewToolName {
		input["file_paths"] = paths
		return call, tool, input, nil
	}
	if len(paths) > 2 {
		if next, ok := tools[AgenticViewToolName]; ok {
			call.Name = AgenticViewToolName
			tool = next
			input["file_paths"] = paths
			return call, tool, input, nil
		}
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

func editPayloadRequiresAgenticEdit(_ map[string]any, multi MultiEditParams) bool {
	// Only rewrite when the payload is already a concrete multi-target edit batch.
	if len(multi.FileEdits) > 1 {
		return true
	}
	if len(multi.FileEdits) == 1 {
		return len(multi.FileEdits[0].Edits) > 1
	}
	if len(multi.Edits) > 1 {
		return strings.TrimSpace(multi.FilePath) != ""
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
	case ApplyPatchToolName:
		if raw, ok := input["file_path"]; ok {
			if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
				return []string{value}
			}
		}
	}
	return nil
}

func enforceTurnPolicy(ctx context.Context, toolName string, input map[string]any) error {
	policy := GetTurnPolicyFromContext(ctx)
	if policy.DirectResponseOnly {
		return NewToolGuidanceError(
			toolName,
			"direct_response_only",
			"This turn is casual conversation only.",
			"This turn is casual conversation only. Reply naturally without tool calls, repository reads, planning, or background work.",
		)
	}
	if err := enforceDeepPlanningTransition(ctx, toolName); err != nil {
		return err
	}
	if err := enforceHarnessRequirement(ctx, toolName); err != nil {
		return err
	}
	if err := enforceLearnedRoutePolicy(ctx, toolName, input); err != nil {
		return err
	}
	return enforceMemoryAccessPolicy(ctx, policy, toolName, input)
}

func enforceDeepPlanningTransition(ctx context.Context, toolName string) error {
	if !GetDeepPlanningActiveFromContext(ctx) {
		return nil
	}
	usage := GetToolUsageStateFromContext(ctx)
	if usage != nil && usage.HasPublishedPlan() {
		return nil
	}
	canonical := canonicalToolNameForModePolicy(toolName)
	if canonical == "" {
		canonical = normalizeToolName(toolName)
	}
	if canonical == "" || isDeepPlanningAllowedTool(canonical) {
		return nil
	}
	return NewToolGuidanceError(
		toolName,
		"deep_planning_active",
		"Complete deep planning before execution.",
		fmt.Sprintf(
			"Deep planning is active for this turn. Stay in planning until you publish the first `update_plan`: use fast repository discovery (`tool_search`, `rg_files`, `rg`, `ls`, `wc`, `wc_l`), inspect real code (`agentic_view`, `view`, `single_view`), optionally use `run_harness` or `request_user_input`, then call `update_plan`. Editing, execution, delegation, and MCP mutation are blocked before that planning transition. Blocked tool: %s.",
			canonical,
		),
	)
}

func isDeepPlanningAllowedTool(toolName string) bool {
	switch toolName {
	case UpdatePlanToolName, RunHarnessToolName, RequestUserInputToolName,
		ToolSearchToolName, RGFilesToolName, RGToolName, LSToolName, GlobToolName, GrepToolName,
		WCToolName, WCLToolName, IndexCodebaseToolName,
		AgenticViewToolName, ViewToolName, SingleViewToolName,
		ListAvailableMCPsToolName, ListMCPResourcesToolName, ReadMCPResourceToolName,
		"spawn_agent", "resume_agent", "send_input", "wait", "collect_result", "close_agent":
		return true
	default:
		return false
	}
}

func enforceHarnessRequirement(ctx context.Context, toolName string) error {
	requirement := GetHarnessRequirementFromContext(ctx)
	if !requirement.Required {
		return nil
	}
	canonical := canonicalToolNameForModePolicy(toolName)
	if canonical == "" {
		canonical = normalizeToolName(toolName)
	}
	if canonical == "" || canonical == RunHarnessToolName || !isHarnessProtectedTool(canonical) {
		if !(requirement.RequireBeforeDiscovery && isHarnessDiscoveryTool(canonical)) {
			return nil
		}
	}
	if requirement.RequireBeforeDiscovery && isHarnessDiscoveryTool(canonical) {
		if _, ok := GetHarnessDecision(ctx); ok {
			return nil
		}
		reason := strings.TrimSpace(requirement.Reason)
		if reason == "" {
			reason = "broad discovery turn"
		}
		return NewToolGuidanceError(
			toolName,
			"harness_required",
			"Run harness first.",
			fmt.Sprintf("This turn is classified as complex (%s). Start with `run_harness` before repo-wide discovery so the route, skills, and validation plan are locked first. Discovery tool blocked: %s.", reason, canonical),
		)
	}
	if canonical == "" || canonical == RunHarnessToolName || !isHarnessProtectedTool(canonical) {
		return nil
	}
	if _, ok := GetHarnessDecision(ctx); ok {
		return nil
	}
	reason := strings.TrimSpace(requirement.Reason)
	if reason == "" {
		reason = "complex turn"
	}
	return NewToolGuidanceError(
		toolName,
		"harness_required",
		"Run harness first.",
		fmt.Sprintf("This turn is classified as complex (%s). Before editing, executing, or delegating, call `run_harness` with the current task. Read and search tools remain allowed before harness. Protected tool blocked: %s.", reason, canonical),
	)
}

func isHarnessProtectedTool(toolName string) bool {
	switch toolName {
	case "agent", "spawn_agent", "resume_agent", "send_input", "spawn_agents_on_csv",
		"report_agent_job_result", "orchestrate_worktrees", BashToolName, "python",
		EditToolName, SingleEditToolName, AgenticEditToolName, ApplyPatchToolName,
		WriteToolName, DownloadToolName:
		return true
	default:
		return false
	}
}

func isHarnessDiscoveryTool(toolName string) bool {
	switch toolName {
	case ToolSearchToolName, RGFilesToolName, RGToolName, AgenticViewToolName, LSToolName, GlobToolName, GrepToolName:
		return true
	default:
		return false
	}
}

func enforceLearnedRoutePolicy(ctx context.Context, toolName string, input map[string]any) error {
	policy := GetLearnedToolPolicyFromContext(ctx)
	if !policy.HasGuardrails() {
		return nil
	}
	canonical := canonicalToolNameForModePolicy(toolName)
	if canonical == "" {
		canonical = normalizeToolName(toolName)
	}
	if canonical != BashToolName || !policy.ForbidBashDiscovery {
		if canonical == LSToolName && policy.PreferStructuredDiscovery {
			usage := GetToolUsageStateFromContext(ctx)
			if usage != nil && usage.Count(LSToolName) >= 1 && usage.Total(ToolSearchToolName, RGFilesToolName, RGToolName, GlobToolName, GrepToolName, AgenticViewToolName) == 0 {
				return NewToolGuidanceError(
					toolName,
					"learned_route_policy",
					"Use structured discovery tools.",
					fmt.Sprintf(
						"%s: repeated `ls` browsing is blocked before structured discovery. After one quick layout check, switch to `tool_search`, `rg_files`, `rg`, or `agentic_view` instead of another `ls` sweep.",
						policy.GuidanceReason(),
					),
				)
			}
		}
		if policy.RequireExplicitPlan {
			usage := GetToolUsageStateFromContext(ctx)
			if shouldBlockForExplicitPlan(canonical, usage) {
				return NewToolGuidanceError(
					toolName,
					"learned_route_policy",
					"Publish update_plan before continuing.",
					fmt.Sprintf(
						"%s: broad non-trivial turns must lock the working plan after the first real evidence pass. After `run_harness` and the initial structured read/search context, call `update_plan` before continuing with more discovery, delegation, or execution. Blocked tool: %s.",
						policy.GuidanceReason(),
						canonical,
					),
				)
			}
		}
		if policy.RequireContextRead && isLearnedContextProtectedTool(canonical) && !hasBroadInitializationContext(GetToolUsageStateFromContext(ctx)) {
			return NewToolGuidanceError(
				toolName,
				"learned_route_policy",
				"Read more code before mutating or executing.",
				fmt.Sprintf(
					"%s: this broad task must gather repository evidence before writing, delegating, executing, or making a confident conclusion. Complete structured discovery with `tool_search`, `rg_files`, or `rg`, then inspect real code with `agentic_view`, `view`, or `single_view` before using %s.",
					policy.GuidanceReason(),
					canonical,
				),
			)
		}
		if shouldRestrictInitializationArtifactWrite(policy, canonical) {
			if blockedPath := firstNonInitializationArtifactWritePath(ctx, canonical, input); blockedPath != "" {
				return NewToolGuidanceError(
					toolName,
					"learned_route_policy",
					"Write only the initialization artifact.",
					fmt.Sprintf(
						"%s: broad initialization turns that generate `AGENTS.md` must not mutate unrelated files. Write the initialization artifact only, then verify it before completing. Blocked path: %s.",
						policy.GuidanceReason(),
						blockedPath,
					),
				)
			}
		}
		if blockedSkill := shouldBlockRepeatedInitializationSkillTool(policy, canonical, GetToolUsageStateFromContext(ctx)); blockedSkill {
			return NewToolGuidanceError(
				toolName,
				"learned_route_policy",
				"Stop loading more skills; continue repo inspection.",
				fmt.Sprintf(
					"%s: broad initialization turns may use at most one skill inventory step. Continue with repository discovery and code reads instead of more `list_skills`, `search_skills`, or `load_skill` calls. Sapphire could not auto-reroute this call because the structured discovery tools needed for route correction were unavailable. Blocked tool: %s.",
					policy.GuidanceReason(),
					canonical,
				),
			)
		}
		return nil
	}
	command, _ := input["command"].(string)
	command = strings.TrimSpace(command)
	if !looksLikeDiscoveryShellCommand(command) {
		return nil
	}
	return NewToolGuidanceError(
		toolName,
		"learned_route_policy",
		"Use structured discovery tools.",
		fmt.Sprintf(
			"%s: discovery-oriented bash is blocked. Use `tool_search` when the file or symbol is unknown, `rg_files` for path-shape search, `rg` for exact content search, `agentic_view` for broad reads, and `ls` only for layout checks. Blocked command: %s",
			policy.GuidanceReason(),
			command,
		),
	)
}

func isLearnedContextProtectedTool(toolName string) bool {
	switch toolName {
	case BashToolName, "python", EditToolName, SingleEditToolName, AgenticEditToolName, ApplyPatchToolName, WriteToolName, DownloadToolName,
		"spawn_agent", "resume_agent", "send_input", "collect_result", "agent":
		return true
	default:
		return false
	}
}

func shouldRestrictInitializationArtifactWrite(policy LearnedToolPolicy, toolName string) bool {
	return strings.TrimSpace(policy.TaskFamily) == "initialize/broad/codebase" &&
		policy.RequirePostWriteVerification &&
		isArtifactWriteTool(toolName)
}

func firstNonInitializationArtifactWritePath(ctx context.Context, toolName string, input map[string]any) string {
	for _, path := range extractArtifactWritePathsFromInput(toolName, input) {
		if !isInitializationArtifactPath(ctx, path) {
			return path
		}
	}
	return ""
}

func isInitializationArtifactPath(ctx context.Context, path string) bool {
	resolved := resolveArtifactVerificationPath(ctx, path)
	if resolved == "" {
		return false
	}
	return strings.EqualFold(filepath.Base(resolved), "AGENTS.md")
}

func shouldBlockRepeatedInitializationSkillTool(policy LearnedToolPolicy, toolName string, usage *ToolUsageState) bool {
	if strings.TrimSpace(policy.TaskFamily) != "initialize/broad/codebase" || !isInitializationSkillDetourTool(toolName) {
		return false
	}
	if usage == nil {
		return false
	}
	return usage.Total("list_skills", "search_skills", LoadSkillToolName) >= 1
}

func hasBroadInitializationContext(usage *ToolUsageState) bool {
	return HasRequiredContextReadEvidence(usage)
}

func shouldBlockForExplicitPlan(toolName string, usage *ToolUsageState) bool {
	if usage == nil || usage.HasPublishedPlan() || !hasPlanningSeedContext(usage) {
		return false
	}
	return isLearnedPlanProtectedTool(toolName)
}

func hasPlanningSeedContext(usage *ToolUsageState) bool {
	return HasPlanningSeedEvidence(usage)
}

func isLearnedPlanProtectedTool(toolName string) bool {
	switch toolName {
	case ToolSearchToolName, RGFilesToolName, RGToolName, AgenticViewToolName, ViewToolName, SingleViewToolName, LSToolName, GlobToolName, GrepToolName,
		"agent", "spawn_agent", "resume_agent", "send_input", "wait", "collect_result", "spawn_agents_on_csv",
		BashToolName, "python", EditToolName, SingleEditToolName, AgenticEditToolName, ApplyPatchToolName, WriteToolName, DownloadToolName:
		return true
	default:
		return false
	}
}

func recordPreparedToolUsage(ctx context.Context, toolName string, input map[string]any) {
	if skip, ok := ctx.Value(SkipPreparedToolUsageContextKey).(bool); ok && skip {
		return
	}
	usage := GetToolUsageStateFromContext(ctx)
	if usage == nil {
		return
	}
	canonical := canonicalToolNameForModePolicy(toolName)
	if canonical == "" {
		canonical = normalizeToolName(toolName)
	}
	usage.Increment(canonical)
	recordContextEvidence(usage, canonical, input)
	recordDeterministicToolUsage(ctx, usage, canonical, input)
	if canonical == UpdatePlanToolName {
		usage.MarkPlanPublished()
	}
}

func looksLikeDiscoveryShellCommand(command string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	if command == "" {
		return false
	}
	if _, _, ok := simpleBashToTool(command); ok {
		return true
	}
	if strings.Contains(command, "cat <<") {
		return false
	}
	for _, token := range []string{
		"ls",
		"find ",
		"fd ",
		"rg ",
		"grep ",
		"cat ",
		"bat ",
		"sed -n ",
		"tree",
		"ack ",
		"ag ",
		"wc ",
		"head ",
		"tail ",
		"git status",
		"git log",
		"git diff",
		"git show",
		"git ls-files",
	} {
		if hasShellCommandToken(command, token) {
			return true
		}
	}
	return false
}

func hasShellCommandToken(command string, token string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	token = strings.ToLower(strings.TrimSpace(token))
	if command == "" || token == "" {
		return false
	}
	if command == token || strings.HasPrefix(command, token+" ") {
		return true
	}
	for _, prefix := range []string{"&& ", "|| ", "; ", "| ", "\n", "\t", "("} {
		if strings.Contains(command, prefix+token) {
			return true
		}
	}
	return false
}

func buildExplicitPlanCheckpointInput(ctx context.Context, policy LearnedToolPolicy, blockedToolName string) map[string]any {
	task := strings.TrimSpace(GetHarnessRequirementFromContext(ctx).Task)
	if task == "" {
		task = strings.TrimSpace(policy.TaskFamily)
	}
	explanation := "Lock the broad analysis plan after the first real repository evidence pass before continuing with " + blockedToolName + "."

	focusStep := "Compare candidate designs and trade-offs"
	validateStep := "Validate recommendation against repo constraints"
	deliverStep := "Deliver final repo-grounded recommendation"
	taskFamily := strings.ToLower(strings.TrimSpace(policy.TaskFamily))
	switch {
	case strings.Contains(taskFamily, "research/") || strings.HasPrefix(taskFamily, "research"):
		focusStep = "Compare research directions and trade-offs"
		validateStep = "Validate conclusions against repo evidence"
		deliverStep = "Deliver final research recommendation"
	case strings.Contains(taskFamily, "review/") || strings.HasPrefix(taskFamily, "review"):
		focusStep = "Collect review findings and trade-offs"
		validateStep = "Validate findings against repo evidence"
		deliverStep = "Deliver final review recommendation"
	case strings.Contains(taskFamily, "migration/") || strings.HasPrefix(taskFamily, "migration"):
		focusStep = "Compare migration options and trade-offs"
		validateStep = "Validate migration plan against repo constraints"
		deliverStep = "Deliver final migration recommendation"
	case strings.Contains(taskFamily, "implementation/") || strings.HasPrefix(taskFamily, "implementation"):
		focusStep = "Sequence implementation phases and trade-offs"
		validateStep = "Validate execution plan against repo constraints"
		deliverStep = "Deliver final implementation plan"
	}

	args := NormalizeUpdatePlanArgs(UpdatePlanArgs{
		Explanation: &explanation,
		Plan: []PlanItem{
			{Step: "Collect initial repository evidence", Status: StepStatusCompleted},
			{Step: focusStep, Status: StepStatusInProgress},
			{Step: validateStep, Status: StepStatusPending},
			{Step: deliverStep, Status: StepStatusPending},
		},
	})
	if trimmed := strings.TrimSpace(task); trimmed != "" {
		args.Plan[1].Step = focusStep + " for " + trimmed
	}

	plan := make([]any, 0, len(args.Plan))
	for _, item := range args.Plan {
		plan = append(plan, map[string]any{
			"step":   item.Step,
			"status": item.Status,
		})
	}
	return map[string]any{
		"explanation": explanation,
		"plan":        plan,
	}
}

func enforceMemoryAccessPolicy(ctx context.Context, policy TurnPolicy, toolName string, input map[string]any) error {
	if isDurableMemoryReadTool(toolName) && !policy.AllowMemoryRead {
		return errors.New("durable memory reads are blocked for this turn. Use memory only after compaction or resume, when the user explicitly asks for prior context, during active long-horizon work, or when session context load is about 50%+.")
	}
	if isDurableMemoryWriteTool(toolName) && !policy.AllowMemoryWrite {
		return errors.New("durable memory writes are blocked for this turn. Do not refresh, save, or rewrite memory during normal short-horizon work.")
	}

	paths := extractPotentialMemoryPaths(toolName, input)
	if len(paths) == 0 {
		return nil
	}

	isWriteTool := isMemoryArtifactWriteTool(toolName)
	for _, rawPath := range paths {
		canonicalPath, isMemoryPath, requiresCanonicalPath := classifyMemoryArtifactPath(ctx, rawPath)
		if !isMemoryPath {
			continue
		}
		if requiresCanonicalPath {
			return fmt.Errorf("durable memory files are not at repo root. Use %q instead of %q", canonicalPath, rawPath)
		}
		if isWriteTool {
			if !policy.AllowMemoryWrite {
				return errors.New("durable memory writes are blocked for this turn. Do not edit .sapphire-memory/* during normal short-horizon work.")
			}
			continue
		}
		if !policy.AllowMemoryRead {
			return errors.New("durable memory file access is blocked for this turn. Do not inspect .sapphire-memory/* unless the turn is post-compaction or resume, explicitly needs prior context, is long-horizon, or the session is about 50%+ full.")
		}
	}
	return nil
}

func isDurableMemoryReadTool(toolName string) bool {
	switch toolName {
	case MemoryQueryToolName, "view_memory", "recall_memory", "memory_health":
		return true
	default:
		return false
	}
}

func isDurableMemoryWriteTool(toolName string) bool {
	switch toolName {
	case "refresh_memory", "save_memory":
		return true
	default:
		return false
	}
}

func isMemoryArtifactWriteTool(toolName string) bool {
	switch toolName {
	case EditToolName, SingleEditToolName, AgenticEditToolName, WriteToolName, DownloadToolName, ApplyPatchToolName:
		return true
	default:
		return false
	}
}

func extractPotentialMemoryPaths(toolName string, input map[string]any) []string {
	switch toolName {
	case ViewToolName, SingleViewToolName, AgenticViewToolName:
		return extractViewPaths(input)
	case EditToolName, SingleEditToolName, AgenticEditToolName, WriteToolName, DownloadToolName, ApplyPatchToolName:
		return extractWritePaths(toolName, input)
	case LSToolName, GlobToolName, GrepToolName:
		return extractGenericPathFields(input)
	default:
		return nil
	}
}

func extractGenericPathFields(input map[string]any) []string {
	var out []string
	appendString := func(raw any) {
		if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	appendStrings := func(raw any) {
		switch typed := raw.(type) {
		case []string:
			for _, value := range typed {
				if strings.TrimSpace(value) != "" {
					out = append(out, value)
				}
			}
		case []any:
			for _, item := range typed {
				appendString(item)
			}
		}
	}
	appendString(input["path"])
	appendStrings(input["paths"])
	return out
}

func classifyMemoryArtifactPath(ctx context.Context, rawPath string) (canonicalPath string, isMemoryPath bool, requiresCanonicalPath bool) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", false, false
	}

	cleaned := filepath.ToSlash(filepath.Clean(rawPath))
	if canonical, ok := canonicalMemoryAlias(cleaned); ok {
		return canonical, true, true
	}
	if cleaned == ".sapphire-memory" || strings.HasPrefix(cleaned, ".sapphire-memory/") {
		return cleaned, true, false
	}

	if !filepath.IsAbs(rawPath) {
		return "", false, false
	}

	workingDir := GetWorkingDirFromContext(ctx)
	if strings.TrimSpace(workingDir) == "" {
		return "", false, false
	}

	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", false, false
	}
	absRawPath, err := filepath.Abs(rawPath)
	if err != nil {
		return "", false, false
	}
	relPath, err := filepath.Rel(absWorkingDir, absRawPath)
	if err != nil {
		return "", false, false
	}
	relPath = filepath.ToSlash(relPath)
	if canonical, ok := canonicalMemoryAlias(relPath); ok {
		return canonical, true, true
	}
	if relPath == ".sapphire-memory" || strings.HasPrefix(relPath, ".sapphire-memory/") {
		return relPath, true, false
	}
	return "", false, false
}

func canonicalMemoryAlias(cleaned string) (string, bool) {
	switch cleaned {
	case "memory_summary.md":
		return ".sapphire-memory/memory_summary.md", true
	case "MEMORY.md", "memory.md", "memory.md-by-sapphire-agent":
		return ".sapphire-memory/MEMORY.md", true
	case "raw_memories.md":
		return ".sapphire-memory/raw_memories.md", true
	default:
		return "", false
	}
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

func inferMCPName(input map[string]any) {
	if input == nil {
		return
	}
	if name, ok := input["mcp_name"].(string); ok && strings.TrimSpace(name) != "" {
		return
	}
	for _, key := range []string{"description", "value", "text", "query"} {
		raw, ok := input[key].(string)
		if !ok {
			continue
		}
		if name := extractMCPNameCandidate(raw); name != "" {
			input["mcp_name"] = name
			return
		}
	}
}

var mcpNameCandidatePattern = regexp.MustCompile(`[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)+`)

func extractMCPNameCandidate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	matches := mcpNameCandidatePattern.FindAllString(raw, -1)
	for _, match := range matches {
		match = strings.TrimRight(match, ".,:;)")
		if strings.Count(match, "/") == 0 {
			continue
		}
		return match
	}
	return ""
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
	if toolName, toolInput, ok := rewriteSimpleBashReadCommand(cmd); ok {
		return toolName, toolInput, true
	}
	if strings.ContainsAny(cmd, "|&;><") || strings.Contains(cmd, "&&") || strings.Contains(cmd, "||") || strings.Contains(cmd, "$(") {
		return "", nil, false
	}
	parts, err := mvdanShell.Fields(cmd, nil)
	if err != nil || len(parts) == 0 {
		return "", nil, false
	}
	switch parts[0] {
	case "ls":
		return rewriteSimpleLSCommand(parts)
	case "tree":
		return rewriteSimpleTreeCommand(parts)
	case "cat", "bat":
		return rewriteSimpleCatCommand(parts)
	case "eza":
		return rewriteSimpleLSCommand(append([]string{"ls"}, parts[1:]...))
	case "grep", "rg":
		return rewriteSimpleSearchCommand(parts)
	case "sed":
		return rewriteSimpleSedSliceCommand(parts)
	}
	return "", nil, false
}

var (
	headCommandPattern     = regexp.MustCompile(`^\s*head\s+-n\s+(\d+)\s+(.+?)\s*$`)
	catHeadCommandPattern  = regexp.MustCompile(`^\s*cat\s+(.+?)\s*\|\s*head\s+-n\s+(\d+)\s*$`)
	findNameCommandPattern = regexp.MustCompile(`^\s*find\s+(\S+)\s+-name\s+['"]?([^'"]+)['"]?\s*$`)
	findDirsCommandPattern = regexp.MustCompile(`^\s*find\s+(\S+)\s+-maxdepth\s+(\d+)\s+-type\s+d\s*$`)
	sedSlicePattern        = regexp.MustCompile(`^(\d+)(?:,(\d+))?p$`)
)

func rewriteSimpleBashReadCommand(command string) (string, map[string]any, bool) {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return "", nil, false
	}

	if matches := catHeadCommandPattern.FindStringSubmatch(cmd); len(matches) == 3 {
		filePath := strings.TrimSpace(trimShellQuotes(matches[1]))
		limit, err := strconv.Atoi(matches[2])
		if filePath == "" || err != nil || limit <= 0 {
			return "", nil, false
		}
		return SingleViewToolName, map[string]any{
			"file_path": filePath,
			"limit":     limit,
		}, true
	}

	if matches := headCommandPattern.FindStringSubmatch(cmd); len(matches) == 3 {
		limit, err := strconv.Atoi(matches[1])
		filePath := strings.TrimSpace(trimShellQuotes(matches[2]))
		if filePath == "" || err != nil || limit <= 0 {
			return "", nil, false
		}
		return SingleViewToolName, map[string]any{
			"file_path": filePath,
			"limit":     limit,
		}, true
	}

	if matches := findNameCommandPattern.FindStringSubmatch(cmd); len(matches) == 3 {
		searchPath := strings.TrimSpace(trimShellQuotes(matches[1]))
		query := strings.TrimSpace(matches[2])
		if searchPath == "" || query == "" {
			return "", nil, false
		}
		return RGFilesToolName, map[string]any{
			"path":  searchPath,
			"query": query,
		}, true
	}

	if matches := findDirsCommandPattern.FindStringSubmatch(cmd); len(matches) == 3 {
		searchPath := strings.TrimSpace(trimShellQuotes(matches[1]))
		depth, err := strconv.Atoi(matches[2])
		if searchPath == "" || err != nil || depth < 0 {
			return "", nil, false
		}
		return LSToolName, map[string]any{
			"path":  searchPath,
			"depth": depth,
		}, true
	}

	return "", nil, false
}

func rewriteSimpleCatCommand(parts []string) (string, map[string]any, bool) {
	if len(parts) < 2 {
		return "", nil, false
	}
	files := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		if part == "--" {
			continue
		}
		if strings.HasPrefix(part, "-") {
			return "", nil, false
		}
		files = append(files, trimShellQuotes(part))
	}
	files = uniqueStrings(files)
	switch len(files) {
	case 0:
		return "", nil, false
	case 1:
		return SingleViewToolName, map[string]any{"file_path": files[0]}, true
	default:
		return AgenticViewToolName, map[string]any{"file_paths": files}, true
	}
}

func rewriteSimpleLSCommand(parts []string) (string, map[string]any, bool) {
	if len(parts) == 0 || parts[0] != "ls" {
		return "", nil, false
	}
	paths := make([]string, 0, 2)
	for _, part := range parts[1:] {
		if part == "--" || strings.HasPrefix(part, "-") {
			continue
		}
		paths = append(paths, trimShellQuotes(part))
	}
	input := map[string]any{}
	switch len(paths) {
	case 0:
		return LSToolName, input, true
	case 1:
		input["path"] = paths[0]
	default:
		input["paths"] = uniqueStrings(paths)
	}
	return LSToolName, input, true
}

func rewriteSimpleTreeCommand(parts []string) (string, map[string]any, bool) {
	if len(parts) == 0 || parts[0] != "tree" {
		return "", nil, false
	}
	input := map[string]any{}
	paths := make([]string, 0, 2)
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		switch {
		case part == "--":
			continue
		case part == "-L" && i+1 < len(parts):
			if depth, err := strconv.Atoi(parts[i+1]); err == nil && depth >= 0 {
				input["depth"] = depth
				i++
				continue
			}
			return "", nil, false
		case strings.HasPrefix(part, "-"):
			continue
		default:
			paths = append(paths, trimShellQuotes(part))
		}
	}
	switch len(paths) {
	case 0:
		return LSToolName, input, true
	case 1:
		input["path"] = paths[0]
	default:
		input["paths"] = uniqueStrings(paths)
	}
	return LSToolName, input, true
}

func rewriteSimpleSedSliceCommand(parts []string) (string, map[string]any, bool) {
	if len(parts) != 4 || parts[0] != "sed" || parts[1] != "-n" {
		return "", nil, false
	}
	matches := sedSlicePattern.FindStringSubmatch(parts[2])
	if len(matches) != 3 {
		return "", nil, false
	}
	start, err := strconv.Atoi(matches[1])
	if err != nil || start <= 0 {
		return "", nil, false
	}
	end := start
	if strings.TrimSpace(matches[2]) != "" {
		end, err = strconv.Atoi(matches[2])
		if err != nil || end < start {
			return "", nil, false
		}
	}
	return SingleViewToolName, map[string]any{
		"file_path": parts[3],
		"offset":    start,
		"limit":     end - start + 1,
	}, true
}

func rewriteSimpleSearchCommand(parts []string) (string, map[string]any, bool) {
	if len(parts) == 0 || (parts[0] != "grep" && parts[0] != "rg") {
		return "", nil, false
	}
	commandName := parts[0]

	var (
		pattern         string
		include         string
		literalText     bool
		caseInsensitive bool
		targets         []string
	)

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		switch {
		case part == "--":
			continue
		case part == "-n" || part == "-l" || part == "-H" || part == "-S" || part == "-s" || part == "-R" || part == "-r" || part == "--recursive" || part == "--line-number" || part == "--files-with-matches" || part == "--with-filename":
			continue
		case part == "-F" || part == "--fixed-strings":
			literalText = true
			continue
		case part == "-i" || part == "--ignore-case":
			caseInsensitive = true
			continue
		case part == "-e" || part == "--regexp":
			if i+1 >= len(parts) {
				return "", nil, false
			}
			pattern = parts[i+1]
			i++
			continue
		case strings.HasPrefix(part, "-e") && len(part) > 2:
			pattern = part[2:]
			continue
		case part == "-g" || part == "--glob":
			if i+1 >= len(parts) {
				return "", nil, false
			}
			include = trimShellQuotes(parts[i+1])
			i++
			continue
		case strings.HasPrefix(part, "-g") && len(part) > 2:
			include = trimShellQuotes(part[2:])
			continue
		case strings.HasPrefix(part, "--glob="):
			include = trimShellQuotes(strings.TrimPrefix(part, "--glob="))
			continue
		case part == "-t" || part == "--type":
			if i+1 >= len(parts) {
				return "", nil, false
			}
			mapped := languageTypeToInclude(parts[i+1])
			if mapped == "" {
				return "", nil, false
			}
			include = mapped
			i++
			continue
		case strings.HasPrefix(part, "-t") && len(part) > 2:
			mapped := languageTypeToInclude(part[2:])
			if mapped == "" {
				return "", nil, false
			}
			include = mapped
			continue
		case strings.HasPrefix(part, "--type="):
			mapped := languageTypeToInclude(strings.TrimPrefix(part, "--type="))
			if mapped == "" {
				return "", nil, false
			}
			include = mapped
			continue
		case strings.HasPrefix(part, "-"):
			return "", nil, false
		case pattern == "":
			pattern = trimShellQuotes(part)
		default:
			targets = append(targets, trimShellQuotes(part))
		}
	}

	if strings.TrimSpace(pattern) == "" {
		return "", nil, false
	}

	input := map[string]any{}
	if include != "" {
		input["include"] = include
	}

	searchRoots, searchInclude, ok := normalizeSearchTargetsForStructuredTool(targets)
	if !ok {
		return "", nil, false
	}
	if searchInclude != "" {
		if existing, _ := input["include"].(string); existing != "" && existing != searchInclude {
			return "", nil, false
		}
		input["include"] = searchInclude
	}
	switch len(searchRoots) {
	case 0:
	case 1:
		input["path"] = searchRoots[0]
	default:
		input["paths"] = searchRoots
	}

	if commandName == "rg" {
		input["pattern"] = pattern
		input["case_sensitive"] = !caseInsensitive
		if literalText {
			input["literal_text"] = true
		}
		return RGToolName, input, true
	}

	if caseInsensitive {
		if literalText || !patternLooksRegex(pattern) {
			input["pattern"] = "(?i)" + regexp.QuoteMeta(pattern)
		} else {
			input["pattern"] = "(?i)" + pattern
		}
	} else {
		input["pattern"] = pattern
		if literalText {
			input["literal_text"] = true
		}
	}

	return GrepToolName, input, true
}

func normalizeSearchTargetsForStructuredTool(targets []string) ([]string, string, bool) {
	roots := make([]string, 0, len(targets))
	var include string
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		root := target
		if hasShellGlob(target) {
			root = filepath.Dir(target)
			nextInclude := filepath.Base(target)
			if nextInclude == "." || nextInclude == string(filepath.Separator) || nextInclude == "" {
				return nil, "", false
			}
			if include != "" && include != nextInclude {
				return nil, "", false
			}
			include = nextInclude
		}
		roots = append(roots, root)
	}
	return uniqueStrings(roots), include, true
}

func hasShellGlob(value string) bool {
	return strings.ContainsAny(value, "*?[{")
}

func patternLooksRegex(value string) bool {
	return strings.ContainsAny(value, `.+*?()[]{}^$|\`)
}

func languageTypeToInclude(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "go":
		return "*.go"
	case "js", "javascript":
		return "*.js"
	case "jsx":
		return "*.jsx"
	case "ts", "typescript":
		return "*.{ts,tsx}"
	case "tsx":
		return "*.tsx"
	case "py", "python":
		return "*.py"
	case "rs", "rust":
		return "*.rs"
	case "json":
		return "*.json"
	case "yaml", "yml":
		return "*.{yaml,yml}"
	case "toml":
		return "*.toml"
	case "md", "markdown":
		return "*.md"
	case "sql":
		return "*.sql"
	default:
		return ""
	}
}

func trimShellQuotes(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func shouldRejectBashForStructuredRepoOps(command string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return false
	}
	if _, _, ok := simpleBashToTool(command); ok {
		return false
	}
	if strings.Contains(cmd, "cat <<") && (strings.Contains(cmd, ".csv") || strings.Contains(cmd, ".txt")) {
		return true
	}
	if strings.Contains(cmd, "&&") || strings.Contains(cmd, "||") || strings.ContainsAny(cmd, "|;") {
		for _, token := range []string{"find ", "cat ", "head ", "tail ", "ls ", "tree", "grep ", "rg ", "sed -n", "wc ", "fd ", "bat ", "eza"} {
			if strings.Contains(cmd, token) {
				return true
			}
		}
	}
	for _, prefix := range []string{"find ", "cat ", "head ", "tail ", "tree", "grep ", "rg ", "sed -n", "awk ", "wc ", "fd ", "bat ", "eza"} {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	if strings.HasPrefix(cmd, "ls ") && !strings.Contains(cmd, "git ") {
		return true
	}
	return false
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
	case string:
		value = strings.TrimSpace(value)
		if value == "" {
			return 0, false
		}
		if i, err := strconv.Atoi(value); err == nil {
			return i, true
		}
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
	case RunHarnessToolName:
		return map[string]any{"task": value}, true
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
	case WebSearchToolName, GoogleSearchToolName:
		return map[string]any{"query": value}, true
	case FetchToolName, WebFetchToolName:
		return map[string]any{"url": value}, true
	case AgenticFetchToolName:
		if looksLikeURL(value) {
			return map[string]any{"url": value}, true
		}
		return map[string]any{"prompt": value}, true
	default:
		return nil, false
	}
}

func looksLikeURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
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
