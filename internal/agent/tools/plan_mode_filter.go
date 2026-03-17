// Codex Plan Mode tool filter
// Based on Codex CLI plan mode tool restrictions architecture
//
// Reference: Codex CLI v0.88.0+ - when in plan mode, editing/execution tools are FORBIDDEN

package tools

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/agent/planmode"
	"github.com/charmbracelet/sapphire/internal/session"
)

// PlanModeToolFilter wraps tools to enforce plan mode restrictions
// Returns filtered tool list based on session mode
func PlanModeToolFilter(ctx context.Context, sessions session.Service, sessionID string, tools []fantasy.AgentTool) ([]fantasy.AgentTool, error) {
	// Get session mode
	mode, err := sessions.GetMode(ctx, sessionID)
	if err != nil {
		// If we can't get the mode, default to pair programming (no restrictions)
		mode = planmode.DefaultMode()
	}

	// If not in plan mode, return all tools
	if mode != planmode.PlanMode {
		return tools, nil
	}

	// In plan mode, filter out forbidden tools
	filtered := make([]fantasy.AgentTool, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}

		toolName := tool.Info().Name
		if planmode.IsToolAllowed(mode, toolName) {
			filtered = append(filtered, tool)
		}
	}

	return filtered, nil
}

// GetPlanModeToolRestrictionsMessage returns a user-friendly message about plan mode restrictions
func GetPlanModeToolRestrictionsMessage() string {
	return `**Plan Mode Restrictions**

In Plan Mode, the following tools are FORBIDDEN:
- File editing: edit, single_edit, agentic_edit, multiedit, write
- Shell commands: bash, python, job_output, job_kill
- Background execution: orchestrate_worktrees, report_agent_job_result

Plan Mode is for THINKING AND PLANNING ONLY.

To use these tools, switch to pair_programming or execute mode using the set_mode tool.`
}

// ValidatePlanModeToolCall validates a tool call against plan mode restrictions
func ValidatePlanModeToolCall(ctx context.Context, sessions session.Service, sessionID, toolName string) error {
	if sessions == nil || sessionID == "" {
		return nil
	}

	mode, err := sessions.GetMode(ctx, sessionID)
	if err != nil {
		return nil // If we can't get mode, allow the call
	}

	if !planmode.IsToolAllowed(mode, toolName) {
		return planmode.NewPlanModeError(toolName)
	}

	return nil
}

// CreatePlanModeViolationError creates a descriptive error for plan mode violations
func CreatePlanModeViolationError(toolName string) error {
	return fmt.Errorf("%w: tool %q is forbidden in plan mode - switch to pair_programming or execute mode to use this tool",
		planmode.NewPlanModeError(toolName), toolName)
}
