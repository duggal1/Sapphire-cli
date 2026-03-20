package agent

import (
	"context"
	_ "embed"
	"errors"
	"strings"
	"time"

	"charm.land/fantasy"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

//go:embed tools/check_hook.md
var checkHookDescription []byte

const CheckHookToolName = "check_hook"

type CheckHookParams struct {
	AgentID string `json:"agent_id,omitempty" description:"Optional agent id. Defaults to the current running agent for this session."`
}

func (c *coordinator) checkHookTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		CheckHookToolName,
		string(checkHookDescription),
		func(ctx context.Context, params CheckHookParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if c.hookService == nil {
				return fantasy.NewTextErrorResponse("hook service not initialized"), nil
			}
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" && strings.TrimSpace(params.AgentID) == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			agentID := strings.TrimSpace(params.AgentID)
			if agentID == "" {
				agentID = c.mailboxIdentityForSession(sessionID)
			}
			if agentID == "" {
				return fantasy.NewTextResponse("hook empty"), nil
			}
			hookSnapshot, err := c.hookService.GetHook(ctx, agentID)
			if err != nil {
				return fantasy.NewTextResponse("hook empty"), nil
			}
			if strings.TrimSpace(hookSnapshot.Hook.HookBeadID) == "" || hookSnapshot.Hook.Status == "idle" {
				return fantasy.NewTextResponse("hook empty"), nil
			}
			payload := map[string]any{
				"agent_id":     hookSnapshot.Hook.AgentID,
				"hook_bead_id": hookSnapshot.Hook.HookBeadID,
				"status":       hookSnapshot.Hook.Status,
				"hooked_at":    "",
				"work_item": map[string]any{
					"id":           hookSnapshot.WorkItem.ID,
					"title":        hookSnapshot.WorkItem.Title,
					"description":  hookSnapshot.WorkItem.Description,
					"status":       hookSnapshot.WorkItem.Status,
					"assignee":     hookSnapshot.WorkItem.Assignee,
					"parent_id":    hookSnapshot.WorkItem.ParentID,
					"convoy_id":    hookSnapshot.WorkItem.ConvoyID,
					"dependencies": hookSnapshot.WorkItem.Dependencies,
				},
			}
			if !hookSnapshot.Hook.HookedAt.IsZero() {
				payload["hooked_at"] = hookSnapshot.Hook.HookedAt.UTC().Format(time.RFC3339)
			}
			return fantasy.NewTextResponse(marshalPrettyJSON(payload)), nil
		},
	), nil
}
