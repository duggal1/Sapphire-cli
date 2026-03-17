// Codex Plan Mode architecture implementation
// Based on Codex CLI collaboration modes (Plan, Pair Programming, Execute)
//
// Reference: https://github.com/openai/codex/blob/main/codex-rs/core/src/features.rs
// Reference: https://github.com/openai/codex/blob/main/codex-rs/core/src/collab_mode.rs

package planmode

// SessionMode represents the current collaboration mode (Codex-inspired)
// Based on Codex CLI v0.88.0+ collaboration modes
type SessionMode string

const (
	// PlanMode - Design-focused mode
	// Organizes requirements, proposes policies, creates detailed plans
	// Tool calls for editing/execution are FORBIDDEN in this mode
	PlanMode SessionMode = "plan"

	// PairProgrammingMode - Default collaborative mode
	// Works in small steps, sharing reasons while proceeding with work
	PairProgrammingMode SessionMode = "pair_programming"

	// ExecuteMode - Autonomous execution mode
	// Makes reasonable assumptions, executes tasks end-to-end without asking questions
	ExecuteMode SessionMode = "execute"
)

// PlanModeRole represents the AI's role in plan mode (Codex-inspired)
// Reference: Codex CLI plan mode behavior
type PlanModeRole string

const (
	// PlanModeRoleOrganize - Organize requirements and surface dependencies
	PlanModeRoleOrganize PlanModeRole = "organize"

	// PlanModeRolePropose - Propose policies and architectural decisions
	PlanModeRolePropose PlanModeRole = "propose"

	// PlanModeRoleDesign - Create detailed design documents and plans
	PlanModeRoleDesign PlanModeRole = "design"
)

// DefaultMode returns the default session mode (Codex: pair_programming is default)
func DefaultMode() SessionMode {
	return PairProgrammingMode
}

// String returns the string representation of SessionMode
func (m SessionMode) String() string {
	return string(m)
}

// IsValid returns true if the session mode is valid
func (m SessionMode) IsValid() bool {
	switch m {
	case PlanMode, PairProgrammingMode, ExecuteMode:
		return true
	default:
		return false
	}
}

// IsPlanMode returns true if the current mode is plan mode
func (m SessionMode) IsPlanMode() bool {
	return m == PlanMode
}

// IsExecutionMode returns true if the mode allows execution (not plan mode)
func (m SessionMode) IsExecutionMode() bool {
	return m == PairProgrammingMode || m == ExecuteMode
}

// PlanModeConfig holds plan mode configuration (Codex-inspired)
type PlanModeConfig struct {
	// MaxPlanItems - Maximum number of plan items (Codex: 5-7 steps recommended)
	MaxPlanItems int `json:"max_plan_items"`

	// MaxStepWords - Maximum words per step (Codex: 5-7 words per step)
	MaxStepWords int `json:"max_step_words"`

	// AllowQuestions - Whether to ask clarifying questions (Codex: true in plan mode)
	AllowQuestions bool `json:"allow_questions"`

	// SkipPlanningForSmallTasks - Skip planning for trivial tasks (Codex behavior)
	SkipPlanningForSmallTasks bool `json:"skip_planning_for_small_tasks"`
}

// DefaultPlanModeConfig returns the default plan mode configuration (Codex defaults)
func DefaultPlanModeConfig() PlanModeConfig {
	return PlanModeConfig{
		MaxPlanItems:              7,
		MaxStepWords:              7,
		AllowQuestions:            true,
		SkipPlanningForSmallTasks: true,
	}
}
