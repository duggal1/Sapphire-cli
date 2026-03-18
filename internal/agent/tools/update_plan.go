// Codex-style update_plan tool implementation
// Based on /codex/codex-rs/protocol/src/plan_tool.rs architecture
//
// CODEX LOGIC: Event-driven plan updates (like Codex EventMsg::PlanUpdate)
// Tool response becomes message → pubsub → UI renders immediately
// session.Todos is OPTIONAL for backwards compatibility only
//
// CRITICAL: This tool is FORBIDDEN in Plan Mode (Codex behavior)
// Plan Mode uses conversation-based planning with <plan> blocks, not update_plan tool

package tools

import (
	"context"
	_ "embed"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/agent/planmode"
	"github.com/charmbracelet/sapphire/internal/session"
)

//go:embed update_plan.md
var updatePlanDescription []byte

const UpdatePlanToolName = "update_plan"

// StepStatus represents the status of a plan step (Codex-compatible)
type StepStatus string

const (
	StepStatusPending    StepStatus = "pending"
	StepStatusInProgress StepStatus = "in_progress"
	StepStatusCompleted  StepStatus = "completed"
)

// PlanItem represents a single step in the plan (Codex PlanItemArg equivalent)
type PlanItem struct {
	Step   string     `json:"step" description:"One-sentence description of the step (max 5-7 words)"`
	Status StepStatus `json:"status" description:"Step status: pending, in_progress, or completed"`
}

// UpdatePlanArgs represents the arguments for the update_plan tool (Codex UpdatePlanArgs equivalent)
type UpdatePlanArgs struct {
	Explanation *string    `json:"explanation,omitempty" description:"Optional brief explanation for the plan update"`
	Plan        []PlanItem `json:"plan" description:"List of plan items with steps and statuses"`
}

// PlanResponseMetadata contains metadata about the plan update
type PlanResponseMetadata struct {
	TotalSteps      int  `json:"total_steps"`
	CompletedSteps  int  `json:"completed_steps"`
	PendingSteps    int  `json:"pending_steps"`
	InProgressSteps int  `json:"in_progress_steps"`
	IsNew           bool `json:"is_new"`
}

// NewUpdatePlanTool creates the Codex-style update_plan tool
func NewUpdatePlanTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		UpdatePlanToolName,
		string(updatePlanDescription),
		func(ctx context.Context, args UpdatePlanArgs, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Get session ID from context
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for update_plan")
			}

			// CODEX LOGIC: update_plan is FORBIDDEN in Plan Mode
			// Plan Mode uses conversation-based planning with <plan> blocks
			// Reference: codex-rs/core/src/tools/handlers/plan.rs
			currentSession, err := sessions.Get(ctx, sessionID)
			wasEmpty := false
			if err == nil && currentSession.Mode == planmode.PlanMode {
				return fantasy.ToolResponse{}, fmt.Errorf("update_plan is forbidden in Plan Mode - Plan Mode uses conversation-based planning with <plan> blocks, not the update_plan tool. Switch to pair_programming or execute mode to use update_plan")
			}
			if err == nil {
				wasEmpty = len(currentSession.Todos) == 0
			}

			// Validate plan is not empty (Codex constraint)
			if len(args.Plan) == 0 {
				return fantasy.ToolResponse{}, fmt.Errorf("plan cannot be empty")
			}

			// Validate plan has at most one in_progress step (Codex constraint)
			inProgressCount := 0
			for _, item := range args.Plan {
				switch item.Status {
				case StepStatusPending, StepStatusInProgress, StepStatusCompleted:
					if item.Status == StepStatusInProgress {
						inProgressCount++
					}
				default:
					return fantasy.ToolResponse{}, fmt.Errorf("invalid status %q for step %q", item.Status, item.Step)
				}
			}

			if inProgressCount > 1 {
				return fantasy.ToolResponse{}, fmt.Errorf("at most one step can be in_progress at a time")
			}

			// CODEX LOGIC: Tool response becomes the event
			// The fantasy agent framework will:
			// 1. Add tool response to current assistant message
			// 2. Publish message via pubsub
			// 3. UI receives and renders immediately (event-driven)

			// OPTIONAL: Also save to session.Todos for backwards compatibility
			// This does NOT affect Codex event-driven rendering
			// It's only for legacy agent.go code that reads session.Todos
			// Reuse currentSession from Plan Mode check above
			if err == nil {
				// Convert Codex PlanItem to Sapphire Todo (for backwards compat only)
				todos := make([]session.Todo, len(args.Plan))
				for i, item := range args.Plan {
					todos[i] = session.Todo{
						ID:         fmt.Sprintf("step_%d", i),
						Content:    item.Step,
						Status:     session.TodoStatus(item.Status),
						ActiveForm: item.Step,
					}
				}
				currentSession.Todos = todos
				_, _ = sessions.Save(ctx, currentSession) // Ignore errors - this is optional
			}

			// Count steps by status
			completedCount := 0
			pendingCount := 0
			for _, item := range args.Plan {
				switch item.Status {
				case StepStatusCompleted:
					completedCount++
				case StepStatusPending:
					pendingCount++
				}
			}

			response := "Plan updated"
			metadata := PlanResponseMetadata{
				TotalSteps:      len(args.Plan),
				CompletedSteps:  completedCount,
				PendingSteps:    pendingCount,
				InProgressSteps: inProgressCount,
				IsNew:           err == nil && wasEmpty,
			}

			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
		},
	)
}
