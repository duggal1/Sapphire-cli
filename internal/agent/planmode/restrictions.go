// Codex Plan Mode tool restrictions
// Based on Codex CLI plan mode behavior where editing/execution tools are FORBIDDEN
//
// Reference: Codex CLI v0.88.0+ - when in plan mode:
// - Background terminal is forbidden
// - Agentic editor calls are forbidden
// - Any type of edit tool is forbidden
// - This mode is for "think hard and plan" only
// - Another LM can implement/monitor this mode

package planmode

import (
	"fmt"
	"strings"
	"unicode"
)

// ToolRestrictions defines which tools are allowed in each mode (Codex-inspired)
// In PlanMode: editing, execution, and terminal tools are FORBIDDEN
type ToolRestrictions struct {
	// AllowedTools - List of tool names allowed in the current mode
	AllowedTools []string

	// ForbiddenTools - List of tool names forbidden in the current mode
	ForbiddenTools []string

	// StrictAllowlist rejects every tool that is not explicitly listed in AllowedTools.
	StrictAllowlist bool
}

// Forbidden tools in PlanMode (Codex plan mode restrictions)
// These tools are FORBIDDEN because plan mode is for thinking and planning only
// Reference: codex-rs/core/templates/collaboration_mode/plan.md
// "Plan Mode is not changed by user intent, tone, or imperative language"
// "If a user asks for execution while in Plan Mode, treat it as a request to plan the execution, not perform it"
var planModeForbiddenTools = []string{
	// File editing tools - FORBIDDEN (mutating operations)
	"edit",
	"single_edit",
	"agentic_edit",
	"multiedit",
	"apply_patch",

	// File writing tools - FORBIDDEN (mutating operations)
	"write",

	// Shell/terminal tools - FORBIDDEN (side-effectful commands)
	"bash",
	"job_output",
	"job_kill",
	"python",
	"download",

	// Background execution tools - FORBIDDEN (doing the work)
	"agent",
	"orchestrate_worktrees",
	"report_agent_job_result",
	"call_mcp_tool",
	"connect_mcp",
	"install_mcp",
	"install_skill",

	// Planning tool - FORBIDDEN in Plan Mode (Codex behavior)
	// Plan Mode uses conversation-based planning with <proposed_plan> blocks
	// update_plan is a checklist/progress tool, separate from Plan Mode
	"update_plan",
}

// Allowed tools in PlanMode (Codex plan mode - read/exploration only)
// Reference: codex-rs/core/templates/collaboration_mode/plan.md
// "Allowed: Reading/searching files, configs, schemas, types, manifests, docs"
// "Static analysis, inspection, repo exploration"
// "Dry-run commands (when they don't edit repo-tracked files)"
var planModeAllowedTools = []string{
	// Research/view tools - ALLOWED (non-mutating exploration)
	"view",
	"single_view",
	"agentic_view",
	"glob",
	"ls",
	"grep",
	"search_tools",

	// Reference tools - ALLOWED (gathering information)
	"references",
	"sourcegraph",
	"fetch",
	"agentic_fetch",
	"web_fetch",
	"web_search",
	"google_search",

	// Memory tools - ALLOWED (retrieving context)
	"view_memory",
	"recall_memory",

	// Tool management - ALLOWED
	"list_tools",
	"tool_suggest",

	// MCP tools - ALLOWED (for research)
	"list_available_mcps",
	"list_mcp_tools",
	"list_mcp_resources",
	"read_mcp_resource",

	// Skill tools - ALLOWED
	"list_skills",
	"search_skills",
	"load_skill",

	// LSP tools - ALLOWED (static analysis, non-mutating)
	"lsp_diagnostics",

	// Mode switching - ALLOWED (to exit plan mode)
	"set_mode",

	// Structured questions - ALLOWED (Codex request_user_input tool)
	"request_user_input",
}

var specialistModeForbiddenTools = []string{
	"edit",
	"single_edit",
	"agentic_edit",
	"multiedit",
	"apply_patch",
	"write",
}

// GetToolRestrictions returns the tool restrictions for the given mode (Codex-inspired)
func GetToolRestrictions(mode SessionMode) *ToolRestrictions {
	switch NormalizeMode(mode) {
	case PlanMode:
		return &ToolRestrictions{
			AllowedTools:    planModeAllowedTools,
			ForbiddenTools:  planModeForbiddenTools,
			StrictAllowlist: true,
		}
	case ArchitectureMode, SecurityMode, DebugMode, ReviewMode, OrchestratorMode:
		return &ToolRestrictions{
			ForbiddenTools: specialistModeForbiddenTools,
		}
	default:
		// PairProgrammingMode and ExecuteMode have no restrictions
		return nil
	}
}

// IsToolAllowed checks if a tool is allowed in the given mode (Codex plan mode enforcement)
func IsToolAllowed(mode SessionMode, toolName string) bool {
	restrictions := GetToolRestrictions(mode)
	if restrictions == nil {
		return true
	}
	canonical := canonicalToolName(toolName)
	if restrictions.StrictAllowlist {
		return canonical != "" && containsToolName(restrictions.AllowedTools, canonical)
	}
	return canonical == "" || !containsToolName(restrictions.ForbiddenTools, canonical)
}

func canonicalToolName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	lastUnderscore := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
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

func containsToolName(items []string, toolName string) bool {
	for _, item := range items {
		if canonicalToolName(item) == toolName {
			return true
		}
	}
	return false
}

// ToolRestrictionError represents an error when a tool is blocked by mode policy.
type ToolRestrictionError struct {
	Mode     SessionMode
	ToolName string
	Message  string
}

// Error implements the error interface for ToolRestrictionError.
func (e *ToolRestrictionError) Error() string {
	mode := NormalizeMode(e.Mode)
	title := mode.Title()
	if strings.TrimSpace(title) == "" {
		title = mode.String()
	}
	return fmt.Sprintf("%s mode restriction: tool %q is forbidden - %s", strings.ToLower(title), e.ToolName, e.Message)
}

// NewToolRestrictionError creates a new mode-aware restriction error.
func NewToolRestrictionError(mode SessionMode, toolName string) *ToolRestrictionError {
	mode = NormalizeMode(mode)
	message := "switch to default mode to use this tool"
	switch mode {
	case PlanMode:
		message = "plan mode is analyze-only and read-only - inspect, clarify, and return a <proposed_plan> instead of executing changes"
	case ArchitectureMode, SecurityMode, DebugMode, ReviewMode, OrchestratorMode:
		message = "this mode may inspect and run supporting tools, but direct file-mutation tools are blocked - switch to default mode to edit code"
	}
	return &ToolRestrictionError{
		Mode:     mode,
		ToolName: toolName,
		Message:  message,
	}
}

func ValidateModeToolCall(mode SessionMode, toolName string) error {
	if !IsToolAllowed(mode, toolName) {
		return NewToolRestrictionError(mode, toolName)
	}
	return nil
}

// NewPlanModeError creates a new plan mode error (compatibility wrapper).
func NewPlanModeError(toolName string) *ToolRestrictionError {
	return NewToolRestrictionError(PlanMode, toolName)
}

// ValidatePlanModeToolCall validates a tool call against the current mode policy.
func ValidatePlanModeToolCall(mode SessionMode, toolName string) error {
	return ValidateModeToolCall(mode, toolName)
}
