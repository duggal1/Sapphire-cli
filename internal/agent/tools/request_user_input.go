// Codex-style request_user_input tool implementation
// Reference: codex-rs/core/src/tools/handlers/request_user_input.rs
//
// CODEX LOGIC: Structured multiple-choice questions for Plan Mode
// - 1-3 questions only
// - Every question MUST have non-empty options
// - Only available in Plan Mode

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

//go:embed request_user_input.md
var requestUserInputDescription []byte

const RequestUserInputToolName = "request_user_input"

// Question represents a single question with multiple-choice options (Codex-compatible)
type Question struct {
	Question string   `json:"question" description:"The question to ask the user"`
	Options  []string `json:"options" description:"Multiple-choice options (2-4 options recommended)"`
	IsOther  bool     `json:"is_other" description:"If true, allows free-form other response"`
}

// RequestUserInputArgs represents the arguments for the request_user_input tool
type RequestUserInputArgs struct {
	Questions []Question `json:"questions" description:"1-3 questions to ask the user"`
}

// NewRequestUserInputTool creates the Codex-style request_user_input tool
func NewRequestUserInputTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RequestUserInputToolName,
		string(requestUserInputDescription),
		func(ctx context.Context, args RequestUserInputArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required")
			}

			// CODEX: Only available in Plan Mode
			currentSession, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			if currentSession.Mode != planmode.PlanMode {
				return fantasy.ToolResponse{}, fmt.Errorf("request_user_input is only available in Plan Mode")
			}

			// Validate 1-3 questions
			if len(args.Questions) == 0 {
				return fantasy.ToolResponse{}, fmt.Errorf("must provide at least 1 question")
			}
			if len(args.Questions) > 3 {
				return fantasy.ToolResponse{}, fmt.Errorf("maximum 3 questions allowed, got %d", len(args.Questions))
			}

			// CODEX: Every question MUST have non-empty options (min 2)
			for i, q := range args.Questions {
				if len(q.Options) < 2 {
					return fantasy.ToolResponse{}, fmt.Errorf("question %d must have at least 2 options, got %d", i+1, len(q.Options))
				}
				args.Questions[i].IsOther = true // Codex behavior
			}

			// Build question display
			var questionText strings.Builder
			questionText.WriteString("Please answer the following question(s):\n\n")
			for i, q := range args.Questions {
				questionText.WriteString(fmt.Sprintf("%d. %s\n", i+1, q.Question))
				for j, opt := range q.Options {
					questionText.WriteString(fmt.Sprintf("   %c) %s\n", 'a'+j, opt))
				}
				if q.IsOther {
					questionText.WriteString("   Or specify your own answer\n")
				}
				questionText.WriteString("\n")
			}

			return fantasy.NewTextResponse(questionText.String()), nil
		},
	)
}
