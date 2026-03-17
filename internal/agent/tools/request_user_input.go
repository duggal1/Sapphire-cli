// Codex-style request_user_input tool implementation
// Reference: codex-rs/core/src/tools/handlers/request_user_input.rs
//
// CODEX LOGIC: Structured multiple-choice questions for Plan Mode
// - 1-3 questions only
// - Every question MUST have non-empty options
// - Only available in Plan Mode
// - Returns JSON-serialized user responses

package tools

import (
	"context"
	_ "embed"
	"encoding/json"
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
	IsOther  bool     `json:"is_other" description:"If true, allows free-form 'other' response"`
}

// RequestUserInputArgs represents the arguments for the request_user_input tool (Codex-compatible)
type RequestUserInputArgs struct {
	Questions []Question `json:"questions" description:"1-3 questions to ask the user"`
}

// UserInputResponse represents the user's responses to the questions
type UserInputResponse struct {
	Answers []string `json:"answers"`
}

// NewRequestUserInputTool creates the Codex-style request_user_input tool
func NewRequestUserInputTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RequestUserInputToolName,
		string(requestUserInputDescription),
		func(ctx context.Context, args RequestUserInputArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Get session ID from context
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for request_user_input")
			}

			// CODEX LOGIC: request_user_input is ONLY available in Plan Mode
			// Reference: codex-rs/core/src/tools/handlers/request_user_input.rs
			currentSession, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			if currentSession.Mode != planmode.PlanMode {
				return fantasy.ToolResponse{}, fmt.Errorf(
					"request_user_input is only available in Plan Mode - current mode is %s",
					currentSession.Mode,
				)
			}

			// Validate number of questions (Codex: 1-3 questions only)
			if len(args.Questions) == 0 {
				return fantasy.ToolResponse{}, fmt.Errorf("must provide at least 1 question")
			}
			if len(args.Questions) > 3 {
				return fantasy.ToolResponse{}, fmt.Errorf("maximum 3 questions allowed, got %d", len(args.Questions))
			}

			// CODEX LOGIC: Every question MUST have non-empty options
			// Reference: codex-rs/core/src/tools/handlers/request_user_input.rs
			for i, q := range args.Questions {
				if len(q.Options) == 0 {
					return fantasy.ToolResponse{}, fmt.Errorf(
						"question %d (%q) must have non-empty options - request_user_input requires multiple-choice options for every question",
						i+1,
						q.Question,
					)
				}
				if len(q.Options) < 2 {
					return fantasy.ToolResponse{}, fmt.Errorf(
						"question %d (%q) must have at least 2 options, got %d",
						i+1,
						q.Question,
						len(q.Options),
					)
				}
			}

			// Mark all questions as is_other=true (Codex behavior)
			for i := range args.Questions {
				args.Questions[i].IsOther = true
			}

			// Build the question display for the user
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

			// CODEX LOGIC: Return response → UI renders question → User responds
			// The response will be captured and sent back to the model
			response := questionText.String()

			metadata := map[string]any{
				"question_count": len(args.Questions),
				"total_options":  countTotalOptions(args.Questions),
			}

			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
		},
	)
}

func countTotalOptions(questions []Question) int {
	total := 0
	for _, q := range questions {
		total += len(q.Options)
	}
	return total
}

// ParseUserInputResponse parses a user's response to request_user_input
func ParseUserInputResponse(response string) (*UserInputResponse, error) {
	var answers UserInputResponse
	if err := json.Unmarshal([]byte(response), &answers); err != nil {
		// If not JSON, treat as a single free-form answer
		answers = UserInputResponse{
			Answers: []string{strings.TrimSpace(response)},
		}
	}
	return &answers, nil
}
