package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/planmode"
)

//go:embed request_user_input.md
var requestUserInputDescription []byte

const RequestUserInputToolName = "request_user_input"

type RequestUserInputParams struct {
	Questions []planmode.Question `json:"questions" description:"Questions to show the user. Prefer 1 and do not exceed 3"`
}

func NewRequestUserInputTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RequestUserInputToolName,
		string(requestUserInputDescription),
		func(ctx context.Context, params RequestUserInputParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			mode := GetAgentModeFromContext(ctx)
			if mode != config.AgentModePlan {
				return fantasy.ToolResponse{}, fmt.Errorf("request_user_input is unavailable in %s mode", mode.Label())
			}
			if len(params.Questions) == 0 || len(params.Questions) > 3 {
				return fantasy.ToolResponse{}, fmt.Errorf("request_user_input requires 1 to 3 questions")
			}

			for _, q := range params.Questions {
				if strings.TrimSpace(q.ID) == "" || strings.TrimSpace(q.Header) == "" || strings.TrimSpace(q.Question) == "" {
					return fantasy.ToolResponse{}, fmt.Errorf("request_user_input requires id, header, and question for every question")
				}
				if len(q.Options) == 0 {
					return fantasy.ToolResponse{}, fmt.Errorf("request_user_input requires non-empty options for every question")
				}
				for _, option := range q.Options {
					if strings.TrimSpace(option.Label) == "" || strings.TrimSpace(option.Description) == "" {
						return fantasy.ToolResponse{}, fmt.Errorf("request_user_input requires label and description for every option")
					}
				}
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required")
			}

			resp, err := planmode.RequestInput(ctx, planmode.Request{
				ID:        call.ID,
				SessionID: sessionID,
				Questions: params.Questions,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if len(resp.Answers) == 0 {
				return fantasy.ToolResponse{}, planmode.ErrRequestCancelled
			}

			data, err := json.Marshal(resp)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("serialize request_user_input response: %w", err)
			}
			return fantasy.NewTextResponse(string(data)), nil
		},
	)
}
