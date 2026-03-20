// Codex Plan Mode architecture implementation
// Based on Codex CLI collaboration modes (Plan, Pair Programming, Execute)
//
// Reference: https://github.com/openai/codex/blob/main/codex-rs/core/src/features.rs
// Reference: https://github.com/openai/codex/blob/main/codex-rs/core/src/collab_mode.rs

package planmode

type ModeDescriptor struct {
	Mode          SessionMode
	Title         string
	Description   string
	FooterSummary string
	Mock          bool
}

// SessionMode represents the current collaboration mode (Codex-inspired)
// Based on Codex CLI v0.88.0+ collaboration modes
type SessionMode string

const (
	// DefaultSessionMode - standard coding mode.
	DefaultSessionMode SessionMode = "default"

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

	// ArchitectureMode - Static architecture planning mockup mode.
	ArchitectureMode SessionMode = "architecture"

	// SecurityMode - Static security planning mockup mode.
	SecurityMode SessionMode = "security"

	// DebugMode - Static debugging mockup mode.
	DebugMode SessionMode = "debug"

	// OrchestratorMode - Static multi-agent orchestration mockup mode.
	OrchestratorMode SessionMode = "orchestrator"
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
	return DefaultSessionMode
}

func NormalizeMode(mode SessionMode) SessionMode {
	switch mode {
	case "", DefaultSessionMode, PairProgrammingMode, ExecuteMode:
		return DefaultSessionMode
	case PlanMode:
		return PlanMode
	default:
		return mode
	}
}

// String returns the string representation of SessionMode
func (m SessionMode) String() string {
	return string(m)
}

// IsValid returns true if the session mode is valid
func (m SessionMode) IsValid() bool {
	switch m {
	case DefaultSessionMode, PlanMode, PairProgrammingMode, ExecuteMode, ArchitectureMode, SecurityMode, DebugMode, OrchestratorMode:
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
	mode := NormalizeMode(m)
	return mode == DefaultSessionMode || mode == ExecuteMode || mode == DebugMode || mode == OrchestratorMode
}

func AvailableModes() []ModeDescriptor {
	return []ModeDescriptor{
		{
			Mode:        DefaultSessionMode,
			Title:       "Default",
			Description: "",
		},
		{
			Mode:          PlanMode,
			Title:         "Plan",
			Description:   "",
			FooterSummary: "",
		},
	}
}

func LookupMode(mode SessionMode) ModeDescriptor {
	mode = NormalizeMode(mode)
	for _, item := range AvailableModes() {
		if item.Mode == mode {
			return item
		}
	}
	return ModeDescriptor{
		Mode:          mode,
		Title:         mode.String(),
		Description:   "",
		FooterSummary: "",
	}
}

func (m SessionMode) Title() string {
	return LookupMode(m).Title
}

func (m SessionMode) Description() string {
	return LookupMode(m).Description
}

func (m SessionMode) FooterSummary() string {
	return LookupMode(m).FooterSummary
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
