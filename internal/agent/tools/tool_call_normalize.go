package tools

import (
	"strings"

	"charm.land/fantasy"
)

func NormalizeToolCall(call fantasy.ToolCall, registry map[string]fantasy.AgentTool) (fantasy.ToolCall, bool) {
	name := stripToolNamespace(call.Name)
	if name == "" {
		return call, false
	}
	if alias := toolNameAlias(name); alias != "" {
		name = alias
	}
	if _, ok := registry[name]; ok {
		call.Name = name
		return call, true
	}
	if idx := strings.LastIndex(name, ":"); idx != -1 && idx < len(name)-1 {
		trimmed := strings.TrimSpace(name[idx+1:])
		if trimmed != "" {
			if _, ok := registry[trimmed]; ok {
				call.Name = trimmed
				return call, true
			}
			if match, ok := findNormalizedTool(trimmed, registry); ok {
				call.Name = match
				return call, true
			}
		}
	}
	if match, ok := findNormalizedTool(name, registry); ok {
		call.Name = match
		return call, true
	}
	return call, false
}

func findNormalizedTool(name string, registry map[string]fantasy.AgentTool) (string, bool) {
	normalized := normalizeToolName(stripToolNamespace(name))
	if normalized == "" {
		return "", false
	}
	for toolName := range registry {
		if normalizeToolName(toolName) == normalized {
			return toolName, true
		}
	}
	return "", false
}

func normalizeToolName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	lastUnderscore := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func toolNameAlias(name string) string {
	switch normalizeToolName(name) {
	// Bash / shell aliases
	case "run_command", "run_cmd", "runcommand", "command", "execute",
		"shell", "terminal", "exec", "run_shell", "run_bash",
		"execute_command", "shell_command":
		return BashToolName

	// Background job aliases
	case "job_list", "jobs", "list_jobs", "background_jobs", "job_status":
		return JobListToolName
	case "job_output", "job_logs", "job_log", "logs", "tail_job":
		return JobOutputToolName
	case "job_kill", "kill_job", "stop_job", "cancel_job", "terminate_job":
		return JobKillToolName

	// Agent mail aliases
	case "agent_mail_send", "mail_send", "send_mail", "send_agent_mail", "agent_send_mail":
		return "agent_mail_send"
	case "agent_mail_inbox", "mail_inbox", "check_mail", "read_mail", "inbox":
		return "agent_mail_inbox"

	// Todo aliases - COMMENTED OUT, replaced with update_plan
	// case "todo", "to_do", "todo_list", "to_do_list",
	// 	"task", "tasks", "task_list", "checklist":
	// 	return UpdatePlanToolName  // Replaced with Codex-style update_plan

	// File listing aliases — fixes "tool not found: list_files"
	case "list_files", "listfiles", "list_dir", "listdir",
		"list_directory", "dir", "directory":
		return LSToolName

	// File reading aliases — fixes "tool not found: read_file"
	case "read_file", "readfile", "read", "open_file", "openfile",
		"show_file", "showfile", "cat", "get_file", "getfile",
		"file_content", "get_file_content", "read_file_content":
		return ViewToolName

	// File search aliases — fixes "tool not found: search_files"
	case "search_files", "searchfiles", "search", "search_code",
		"code_search", "find_in_files", "ripgrep", "rg",
		"search_text", "text_search":
		return GrepToolName

	// File glob/find aliases — fixes "tool not found: find_files"
	case "find_files", "findfiles", "find", "find_file",
		"file_search", "locate", "glob_files":
		return GlobToolName

	// File writing aliases — fixes "tool not found: write_file"
	case "write_file", "writefile", "create_file", "createfile",
		"new_file", "newfile", "save_file", "savefile":
		return WriteToolName

	// File editing aliases — fixes "tool not found: modify_file"
	case "update_file", "updatefile", "modify_file", "modifyfile",
		"patch_file", "patchfile", "replace", "replace_in_file":
		return EditToolName

	// Multi-view aliases
	case "read_files", "readfiles", "view_files", "viewfiles",
		"open_files", "openfiles", "batch_view", "multi_view":
		return AgenticViewToolName

	// Multi-edit aliases
	case "edit_files", "editfiles", "update_files", "updatefiles",
		"modify_files", "modifyfiles", "batch_edit", "multi_edit":
		return AgenticEditToolName

	default:
		return ""
	}
}

func stripToolNamespace(name string) string {
	name = strings.TrimSpace(name)
	for {
		sep := strings.IndexAny(name, ":/")
		if sep == -1 {
			break
		}
		prefix := strings.TrimSpace(name[:sep])
		if prefix == "" {
			break
		}
		lower := strings.ToLower(prefix)
		if lower != "default" && lower != "tool" {
			break
		}
		name = strings.TrimSpace(name[sep+1:])
	}
	return name
}

// findSimilarToolNames returns tool names from the registry that share
// a substring of length >= 3 with the requested name. This helps the
// model self-correct when it hallucinates a tool name that is close
// but not exact.
func FindSimilarToolNames(invalidName string, registry map[string]fantasy.AgentTool) []string {
	norm := normalizeToolName(invalidName)
	if norm == "" {
		return nil
	}
	var matches []string
	for toolName := range registry {
		toolNorm := normalizeToolName(toolName)
		if toolNorm == "" {
			continue
		}
		// Check if either contains the other, or they share a 3+ char substring
		if strings.Contains(norm, toolNorm) || strings.Contains(toolNorm, norm) {
			matches = append(matches, toolName)
			continue
		}
		// Check for shared word stems
		for _, part := range strings.Split(norm, "_") {
			if len(part) >= 3 && strings.Contains(toolNorm, part) {
				matches = append(matches, toolName)
				break
			}
		}
	}
	if len(matches) > 5 {
		matches = matches[:5]
	}
	return matches
}
