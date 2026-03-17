// Codex Plan Mode switching tool
// Based on Codex CLI v0.88.0+ mode switching architecture
//
// Reference: Codex CLI collaboration modes (Plan, Pair Programming, Execute)
// Users can switch modes via /plan command or mode switching tool

package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/agent/planmode"
	"github.com/charmbracelet/sapphire/internal/session"
)

//go:embed set_mode.md
var setModeDescription []byte

const SetModeToolName = "set_mode"

// SetModeArgs represents the arguments for the set_mode tool
type SetModeArgs struct {
	// Mode - The session mode to switch to (plan, pair_programming, execute)
	Mode string `json:"mode" description:"The session mode to switch to: plan, pair_programming, or execute"`

	// Reason - Optional reason for the mode switch
	Reason *string `json:"reason,omitempty" description:"Optional brief explanation for the mode switch"`
}

// SetModeResponse contains the response from the set_mode tool
type SetModeResponse struct {
	PreviousMode string `json:"previous_mode"`
	NewMode      string `json:"new_mode"`
	Success      bool   `json:"success"`
	Message      string `json:"message"`
}

// NewSetModeTool creates the Codex-style mode switching tool
func NewSetModeTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SetModeToolName,
		string(setModeDescription),
		func(ctx context.Context, args SetModeArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Get session ID from context
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for set_mode")
			}

			// Normalize mode string
			modeStr := strings.ToLower(strings.TrimSpace(args.Mode))

			// Validate mode
			var newMode planmode.SessionMode
			switch modeStr {
			case "plan":
				newMode = planmode.PlanMode
			case "pair_programming":
				newMode = planmode.PairProgrammingMode
			case "execute":
				newMode = planmode.ExecuteMode
			default:
				return fantasy.ToolResponse{}, fmt.Errorf(
					"invalid mode %q - must be one of: plan, pair_programming, execute",
					args.Mode,
				)
			}

			// Get current mode
			currentSession, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			previousMode := currentSession.Mode

			// Check if mode is actually changing
			if previousMode == newMode {
				return fantasy.NewTextResponse(
					fmt.Sprintf("Already in %s mode - no change needed", newMode),
				), nil
			}

			// Set new mode
			if err := sessions.SetMode(ctx, sessionID, newMode); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to set mode: %w", err)
			}

			// Build response message
			var message strings.Builder
			message.WriteString(fmt.Sprintf("Switched from %s to %s mode", previousMode, newMode))

			if args.Reason != nil && *args.Reason != "" {
				message.WriteString(fmt.Sprintf("\n\nReason: %s", *args.Reason))
			}

			// Add mode-specific guidance
			switch newMode {
			case planmode.PlanMode:
				message.WriteString("\n\n**Plan Mode Active**\n- File editing and shell commands are FORBIDDEN\n- Use update_plan to create plans\n- Focus on thinking and planning only")
			case planmode.PairProgrammingMode:
				message.WriteString("\n\n**Pair Programming Mode Active**\n- All tools available\n- Work in small steps with user collaboration")
			case planmode.ExecuteMode:
				message.WriteString("\n\n**Execute Mode Active**\n- All tools available\n- Autonomous execution with minimal questions")
			}

			return fantasy.NewTextResponse(message.String()), nil
		},
	)
}
