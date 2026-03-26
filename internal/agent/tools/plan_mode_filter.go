// Codex Plan Mode tool filter
// Based on Codex CLI plan mode tool restrictions architecture
//
// Reference: Codex CLI v0.88.0+ - when in plan mode, editing/execution tools are FORBIDDEN

package tools

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/session"
)

type modeRestrictedTool struct {
	mode planmode.SessionMode
	base fantasy.AgentTool
}

func (t modeRestrictedTool) Info() fantasy.ToolInfo {
	return t.base.Info()
}

func (t modeRestrictedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	if err := planmode.ValidateModeToolCall(t.mode, t.base.Info().Name); err != nil {
		return fantasy.ToolResponse{}, err
	}
	return t.base.Run(ctx, call)
}

func (t modeRestrictedTool) ProviderOptions() fantasy.ProviderOptions {
	return t.base.ProviderOptions()
}

func (t modeRestrictedTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.base.SetProviderOptions(opts)
}

// PlanModeToolFilter wraps tools to enforce plan mode restrictions
// Returns filtered tool list based on session mode
func PlanModeToolFilter(ctx context.Context, sessions session.Service, sessionID string, tools []fantasy.AgentTool) ([]fantasy.AgentTool, error) {
	// Get session mode
	mode, err := sessions.GetMode(ctx, sessionID)
	if err != nil {
		// If we can't get the mode, default to standard coding mode (no restrictions)
		mode = planmode.DefaultMode()
	}

	restrictions := planmode.GetToolRestrictions(mode)
	if restrictions == nil || (!restrictions.StrictAllowlist && len(restrictions.ForbiddenTools) == 0) {
		return tools, nil
	}

	filtered := make([]fantasy.AgentTool, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}

		toolName := tool.Info().Name
		if planmode.IsToolAllowed(mode, toolName) {
			filtered = append(filtered, modeRestrictedTool{mode: mode, base: tool})
		}
	}

	return filtered, nil
}

// GetPlanModeToolRestrictionsMessage returns a user-friendly message about plan mode restrictions
func GetPlanModeToolRestrictionsMessage() string {
	return `**Mode Restrictions**

In Plan mode, the following tools are FORBIDDEN:
- File editing: edit, single_edit, agentic_edit, multiedit, write, apply_patch
- Shell and execution: bash, python, tests, builds, run commands, background jobs
- Task execution: sub-agent dispatch, worktree orchestration, update_plan

Plan mode is analyze-only. Inspect the repo, ask targeted questions when needed, and return a <proposed_plan>.

To use these tools, switch to default mode using the set_mode tool.`
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

	return planmode.ValidateModeToolCall(mode, toolName)
}

// CreatePlanModeViolationError creates a descriptive error for plan mode violations
func CreatePlanModeViolationError(toolName string) error {
	return fmt.Errorf("%w: tool %q is forbidden in plan mode - switch to default mode to use this tool",
		planmode.NewPlanModeError(toolName), toolName)
}
