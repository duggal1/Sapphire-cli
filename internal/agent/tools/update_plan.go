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
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/session"
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

func normalizeStepStatus(status string) StepStatus {
	normalized := strings.TrimSpace(strings.ToLower(status))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return StepStatus(normalized)
}

// NormalizePlanItems coerces update_plan items into a safe canonical form.
// Invalid/blank steps are dropped, unknown statuses become pending, and only
// the first in_progress step remains in_progress.
func NormalizePlanItems(plan []PlanItem) []PlanItem {
	if len(plan) == 0 {
		return nil
	}
	normalized := make([]PlanItem, 0, len(plan))
	inProgressCount := 0
	for i := range plan {
		step := strings.TrimSpace(plan[i].Step)
		if step == "" {
			continue
		}
		status := normalizeStepStatus(string(plan[i].Status))
		switch status {
		case StepStatusPending, StepStatusInProgress, StepStatusCompleted:
			if status == StepStatusInProgress {
				inProgressCount++
				if inProgressCount > 1 {
					status = StepStatusPending
				}
			}
		default:
			status = StepStatusPending
		}
		normalized = append(normalized, PlanItem{
			Step:   step,
			Status: status,
		})
	}
	return normalized
}

func NormalizeUpdatePlanArgs(args UpdatePlanArgs) UpdatePlanArgs {
	args.Plan = NormalizePlanItems(args.Plan)
	if args.Explanation != nil {
		explanation := strings.TrimSpace(*args.Explanation)
		if explanation == "" {
			args.Explanation = nil
		} else {
			args.Explanation = &explanation
		}
	}
	return args
}

// ValidatePlanItems validates already-normalized update_plan items.
func ValidatePlanItems(plan []PlanItem) error {
	if len(plan) == 0 {
		return fmt.Errorf("plan cannot be empty")
	}
	inProgressCount := 0
	for i := range plan {
		if strings.TrimSpace(plan[i].Step) == "" {
			return fmt.Errorf("step %d is missing step text", i+1)
		}
		switch normalizeStepStatus(string(plan[i].Status)) {
		case StepStatusPending, StepStatusInProgress, StepStatusCompleted:
			if normalizeStepStatus(string(plan[i].Status)) == StepStatusInProgress {
				inProgressCount++
			}
		default:
			return fmt.Errorf("invalid status %q for step %q", plan[i].Status, plan[i].Step)
		}
	}
	if inProgressCount > 1 {
		return fmt.Errorf("at most one step can be in_progress at a time")
	}
	return nil
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
				return fantasy.ToolResponse{}, fmt.Errorf("update_plan is forbidden in Plan Mode - Plan Mode uses conversation-based planning with <plan> blocks, not the update_plan tool. Switch to default mode to use update_plan")
			}
			if err == nil {
				wasEmpty = len(currentSession.Todos) == 0
			}

			args = NormalizeUpdatePlanArgs(args)
			if len(args.Plan) == 0 {
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse("Plan unchanged"), PlanResponseMetadata{}), nil
			}
			if err := ValidatePlanItems(args.Plan); err != nil {
				return fantasy.ToolResponse{}, err
			}

			inProgressCount := 0
			for _, item := range args.Plan {
				if item.Status == StepStatusInProgress {
					inProgressCount++
				}
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
