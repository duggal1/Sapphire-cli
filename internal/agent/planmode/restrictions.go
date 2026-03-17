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
	"slices"
)

// ToolRestrictions defines which tools are allowed in each mode (Codex-inspired)
// In PlanMode: editing, execution, and terminal tools are FORBIDDEN
type ToolRestrictions struct {
	// AllowedTools - List of tool names allowed in the current mode
	AllowedTools []string

	// ForbiddenTools - List of tool names forbidden in the current mode
	ForbiddenTools []string
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
	
	// File writing tools - FORBIDDEN (mutating operations)
	"write",
	
	// Shell/terminal tools - FORBIDDEN (side-effectful commands)
	"bash",
	"job_output",
	"job_kill",
	"python",
	
	// Background execution tools - FORBIDDEN (doing the work)
	"orchestrate_worktrees",
	"report_agent_job_result",
	
	// Planning tool - FORBIDDEN in Plan Mode (Codex behavior)
	// Plan Mode uses conversation-based planning with <plan> blocks
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
	"rg",
	"search_tools",
	
	// Reference tools - ALLOWED (gathering information)
	"references",
	"sourcegraph",
	"fetch",
	"agentic_fetch",
	"download",
	
	// Memory tools - ALLOWED (retrieving context)
	"recall_memory",
	"save_memory",
	
	// Tool management - ALLOWED
	"list_tools",
	"tool_suggest",
	
	// MCP tools - ALLOWED (for research)
	"list_available_mcps",
	"connect_mcp",
	"list_mcp_tools",
	"list_mcp_resources",
	"read_mcp_resource",
	
	// Skill tools - ALLOWED
	"list_skills",
	"load_skill",
	
	// LSP tools - ALLOWED (static analysis, non-mutating)
	"lsp_diagnostics",
	"lsp_references",
	"lsp_restart",
	
	// Mode switching - ALLOWED (to exit plan mode)
	"set_mode",
	
	// Structured questions - ALLOWED (Codex request_user_input tool)
	"request_user_input",
}

// GetToolRestrictions returns the tool restrictions for the given mode (Codex-inspired)
func GetToolRestrictions(mode SessionMode) *ToolRestrictions {
	switch mode {
	case PlanMode:
		return &ToolRestrictions{
			AllowedTools:   planModeAllowedTools,
			ForbiddenTools: planModeForbiddenTools,
		}
	default:
		// PairProgrammingMode and ExecuteMode have no restrictions
		return &ToolRestrictions{
			AllowedTools:   nil,
			ForbiddenTools: nil,
		}
	}
}

// IsToolAllowed checks if a tool is allowed in the given mode (Codex plan mode enforcement)
func IsToolAllowed(mode SessionMode, toolName string) bool {
	if mode != PlanMode {
		// All tools allowed in non-plan modes
		return true
	}

	// In plan mode, check if tool is explicitly forbidden
	if slices.Contains(planModeForbiddenTools, toolName) {
		return false
	}

	// Tool is allowed (either in allowed list or not restricted)
	return true
}

// PlanModeError represents an error when a tool is blocked in plan mode
type PlanModeError struct {
	ToolName string
	Message  string
}

// Error implements the error interface for PlanModeError
func (e *PlanModeError) Error() string {
	return fmt.Sprintf("plan mode restriction: tool %q is forbidden in plan mode - %s", e.ToolName, e.Message)
}

// NewPlanModeError creates a new plan mode error (Codex-style error message)
func NewPlanModeError(toolName string) *PlanModeError {
	message := "plan mode is for thinking and planning only - switch to pair_programming or execute mode to use this tool"
	return &PlanModeError{
		ToolName: toolName,
		Message:  message,
	}
}

// ValidatePlanModeToolCall validates a tool call in plan mode (Codex enforcement)
// Returns nil if the tool is allowed, or a PlanModeError if forbidden
func ValidatePlanModeToolCall(mode SessionMode, toolName string) error {
	if mode != PlanMode {
		return nil
	}

	if !IsToolAllowed(mode, toolName) {
		return NewPlanModeError(toolName)
	}

	return nil
}
