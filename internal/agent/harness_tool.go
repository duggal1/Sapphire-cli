package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/skills"
)

//go:embed templates/run_harness.md
var runHarnessToolDescription []byte

const bundledHarnessSkillPath = "internal/skills/bundled/harness/SKILL.md"

type RunHarnessParams struct {
	Task       string `json:"task" description:"The exact task to classify and route through the harness"`
	WorkingDir string `json:"working_dir,omitempty" description:"Optional working directory for scope-sensitive planning"`
	GoalType   string `json:"goal_type,omitempty" description:"Optional explicit goal type such as implementation, debug, review, design, or migration"`
	Force      bool   `json:"force,omitempty" description:"Force harness planning even if the task is simple"`
	Mode       string `json:"mode,omitempty" description:"execute or plan_only"`
}

type HarnessAgentRole struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type HarnessSkillPolicy struct {
	Mode             string   `json:"mode"`
	LoadImmediately  []string `json:"load_immediately"`
	ExtendedAllowed  bool     `json:"extended_allowed"`
	ExtendedOnlyWhen string   `json:"extended_only_when,omitempty"`
}

type HarnessExecutionContract struct {
	Required         bool               `json:"required"`
	Reason           string             `json:"reason"`
	ComplexityScore  int                `json:"complexity_score"`
	Mode             string             `json:"mode"`
	GoalType         string             `json:"goal_type"`
	WorkingDir       string             `json:"working_dir,omitempty"`
	ExecutionMode    string             `json:"execution_mode"`
	Pattern          string             `json:"pattern"`
	Agents           []HarnessAgentRole `json:"agents"`
	RequiredSkills   []string           `json:"required_skills"`
	SkillPolicy      HarnessSkillPolicy `json:"skill_policy"`
	Phases           []string           `json:"phases"`
	Artifacts        []string           `json:"artifacts"`
	VerificationPlan []string           `json:"verification_plan"`
	NextAction       string             `json:"next_action"`
	SourceSkill      string             `json:"source_skill"`
	Domains          []string           `json:"domains,omitempty"`
}

func buildHarnessRequirement(task string) tools.HarnessRequirement {
	normalized := strings.TrimSpace(task)
	if normalized == "" {
		return tools.HarnessRequirement{}
	}
	goalType := inferHarnessGoalType(normalized, "")
	decision := evaluateSubAgentLaunch(normalized)

	if isInitializationStylePrompt(normalized) {
		return tools.HarnessRequirement{
			Required:               true,
			Reason:                 "broad codebase initialization",
			ComplexityScore:        max(decision.Complexity, 3),
			Task:                   normalized,
			RequireBeforeDiscovery: true,
		}
	}
	if isBroadHarnessAnalysisTask(normalized, goalType, decision) {
		reason := "broad design analysis"
		if goalType == "research" {
			reason = "broad research task"
		}
		return tools.HarnessRequirement{
			Required:               true,
			Reason:                 reason,
			ComplexityScore:        max(decision.Complexity, 3),
			Task:                   normalized,
			RequireBeforeDiscovery: true,
		}
	}

	required := false
	reason := "simple task"
	switch {
	case decision.Parallelizable && decision.Complexity >= 3:
		required = true
		reason = "parallel or multi-domain task"
	case len(decision.Domains) >= 2:
		required = true
		reason = "multi-domain task"
	case decision.Complexity >= 4:
		required = true
		reason = "multi-phase non-trivial task"
	case decision.Complexity >= 3 && hasHarnessOperationalSignal(normalized):
		required = true
		reason = "non-trivial implementation task"
	}

	return tools.HarnessRequirement{
		Required:        required,
		Reason:          reason,
		ComplexityScore: decision.Complexity,
		Task:            normalized,
	}
}

func hasHarnessOperationalSignal(task string) bool {
	normalized := strings.ToLower(strings.TrimSpace(task))
	if normalized == "" {
		return false
	}
	return hasAnySignal(normalized, subAgentCodebaseSignals) ||
		hasAnySignal(normalized, subAgentDependencySignals) ||
		hasAnySignal(normalized, subAgentRiskSignals) ||
		hasAnySignal(normalized, []string{
			"frontend", "backend", "integration", "integrate", "auth", "database", "migration",
			"refactor", "architecture", "deploy", "deployment", "performance", "security",
			"debug", "research", "investigate", "analysis", "observability", "multi-file", "across the codebase", "end-to-end", "e2e",
		})
}

func inferHarnessGoalType(task string, explicit string) string {
	if trimmed := strings.TrimSpace(strings.ToLower(explicit)); trimmed != "" {
		return trimmed
	}
	normalized := strings.ToLower(task)
	switch {
	case isInitializationStylePrompt(normalized):
		return "initialize"
	case hasAnySignal(normalized, []string{"debug", "fix", "bug", "regression", "incident"}):
		return "debug"
	case hasAnySignal(normalized, []string{"research", "investigate", "survey", "deep dive", "explore the repo", "analyze the repo"}):
		return "research"
	case hasAnySignal(normalized, []string{"review", "audit", "inspect"}):
		return "review"
	case hasAnySignal(normalized, []string{"architecture", "architect", "design", "ui", "ux", "copy", "content"}):
		return "design"
	case hasAnySignal(normalized, []string{"migrate", "migration", "upgrade", "refactor"}):
		return "migration"
	default:
		return "implementation"
	}
}

func isBroadHarnessAnalysisTask(task, goalType string, decision subAgentLaunchDecision) bool {
	if goalType != "design" && goalType != "research" {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(task))
	if decision.Complexity >= 4 {
		return true
	}
	if hasAnySignal(normalized, subAgentCodebaseSignals) || hasAnySignal(normalized, subAgentDependencySignals) || hasAnySignal(normalized, subAgentRiskSignals) {
		return true
	}
	return hasAnySignal(normalized, []string{"repository", "repo", "codebase", "across", "compare", "trade-off", "tradeoff", "architecture", "design", "research"})
}

func normalizeHarnessMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "plan", "plan_only", "plan-only":
		return "plan_only"
	default:
		return "execute"
	}
}

func (c *coordinator) runHarnessTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		tools.RunHarnessToolName,
		string(runHarnessToolDescription),
		func(ctx context.Context, params RunHarnessParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			task := strings.TrimSpace(params.Task)
			if task == "" {
				return fantasy.NewTextErrorResponse("task is required"), nil
			}

			bundledHarness, err := skills.LoadBundledSkill("harness")
			if err != nil {
				return fantasy.NewTextErrorResponse("failed to load bundled harness skill"), nil
			}
			if strings.TrimSpace(bundledHarness.Instructions) == "" {
				return fantasy.NewTextErrorResponse("bundled harness skill is empty"), nil
			}

			requirement := buildHarnessRequirement(task)
			if params.Force {
				requirement.Required = true
				requirement.Reason = "forced harness routing"
			}

			mode := normalizeHarnessMode(params.Mode)
			goalType := inferHarnessGoalType(task, params.GoalType)
			workingDir := strings.TrimSpace(params.WorkingDir)
			if workingDir == "" {
				workingDir = strings.TrimSpace(tools.GetWorkingDirFromContext(ctx))
			}

			decision := evaluateSubAgentLaunch(task)
			requiredSkills, skillPolicy := c.selectHarnessSkills(task, decision.Domains)
			pattern, executionMode := selectHarnessPattern(requirement, decision, goalType, mode)
			contract := HarnessExecutionContract{
				Required:        requirement.Required,
				Reason:          firstNonEmptyHarnessString(requirement.Reason, "simple task"),
				ComplexityScore: requirement.ComplexityScore,
				Mode:            mode,
				GoalType:        goalType,
				WorkingDir:      workingDir,
				ExecutionMode:   executionMode,
				Pattern:         pattern,
				Agents:          selectHarnessAgents(pattern, goalType, mode, decision.Domains),
				RequiredSkills:  requiredSkills,
				SkillPolicy:     skillPolicy,
				Phases:          selectHarnessPhases(pattern, mode),
				Artifacts:       selectHarnessArtifacts(goalType, mode),
				VerificationPlan: selectHarnessVerificationPlan(
					goalType,
					mode,
				),
				NextAction:  selectHarnessNextAction(executionMode, len(requiredSkills) > 0, mode),
				SourceSkill: bundledHarnessSkillPath,
				Domains:     slices.Clone(decision.Domains),
			}

			tools.RecordHarnessDecision(ctx, tools.HarnessDecision{
				Required:        contract.Required,
				ComplexityScore: contract.ComplexityScore,
				Pattern:         contract.Pattern,
			})

			payload, err := json.Marshal(contract)
			if err != nil {
				return fantasy.NewTextErrorResponse("failed to serialize harness contract"), nil
			}
			return fantasy.ToolResponse{Content: string(payload)}, nil
		},
	), nil
}

func (c *coordinator) selectHarnessSkills(task string, domains []string) ([]string, HarnessSkillPolicy) {
	c.ensureSkillsDiscovered()

	available := make(map[string]struct{}, len(c.discoveredSkills))
	for _, skill := range c.discoveredSkills {
		if skill == nil {
			continue
		}
		info := skillInfoFromSkill(skill)
		available[strings.ToLower(info.Name)] = struct{}{}
	}

	selected := make([]string, 0, 6)
	seen := map[string]struct{}{}
	add := func(name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return
		}
		lower := strings.ToLower(trimmed)
		if _, ok := available[lower]; !ok {
			return
		}
		if _, ok := seen[lower]; ok {
			return
		}
		seen[lower] = struct{}{}
		selected = append(selected, trimmed)
	}

	add("harness")

	domainSkillMap := map[string][]string{
		"frontend":      {"frontend", "frontend-design"},
		"backend":       {"backend"},
		"database":      {"backend"},
		"infra":         {"devops", "cloudflare-deploy", "render-deploy", "netlify-deploy", "vercel-deploy"},
		"testing":       {"audit"},
		"security":      {"security", "security-best-practices", "security-threat-model"},
		"observability": {"optimize"},
	}
	for _, domain := range domains {
		for _, candidate := range domainSkillMap[domain] {
			add(candidate)
		}
	}

	for _, match := range c.matchSkillsByKeyword(task) {
		add(skillInfoFromSkill(match).Name)
	}

	for _, match := range rankSkills(c.discoveredSkills, task, 8) {
		if match.score < 120 {
			continue
		}
		add(match.info.Name)
		if len(selected) >= 6 {
			break
		}
	}

	extendedAllowed := len(selected) <= 1 && (len(domains) > 0 || looksLikeVendorIntegration(task))
	policy := HarnessSkillPolicy{
		Mode:            "local_required_then_extended_if_missing",
		LoadImmediately: slices.Clone(selected),
		ExtendedAllowed: extendedAllowed,
	}
	if extendedAllowed {
		policy.ExtendedOnlyWhen = "allow extended skills only if local search returns no direct domain match or only weak generic matches"
	}
	return selected, policy
}

func looksLikeVendorIntegration(task string) bool {
	normalized := strings.ToLower(task)
	return hasAnySignal(normalized, []string{
		"stripe", "supabase", "clerk", "firebase", "sentry", "linear", "notion",
		"slack", "github", "gitlab", "vercel", "netlify", "cloudflare", "render",
		"aws", "gcp", "azure", "twilio", "shopify", "postgres", "mysql", "redis",
		"mongodb", "kafka", "mcp",
	})
}

func selectHarnessPattern(requirement tools.HarnessRequirement, decision subAgentLaunchDecision, goalType, mode string) (string, string) {
	if mode == "plan_only" {
		return "planner_reviewer", "planning_only"
	}
	if requirement.Required && (decision.Parallelizable || len(decision.Domains) >= 2) {
		return "parallel_specialists", "agent_team"
	}
	if requirement.Required && goalType == "migration" {
		return "planner_executor_reviewer", "agent_team"
	}
	if requirement.Required && hasAnySignal(strings.ToLower(goalType), []string{"design", "review", "debug", "research"}) {
		return "producer_reviewer", "agent_team"
	}
	if requirement.Required {
		return "planner_executor_reviewer", "agent_team"
	}
	return "single_track", "single_agent"
}

func selectHarnessAgents(pattern, goalType, mode string, domains []string) []HarnessAgentRole {
	if mode == "plan_only" {
		return []HarnessAgentRole{
			{Name: "planner", Role: "task decomposition"},
			{Name: "reviewer", Role: "plan verification"},
		}
	}
	switch pattern {
	case "parallel_specialists":
		agents := []HarnessAgentRole{{Name: "planner", Role: "task decomposition"}}
		for _, domain := range domains {
			agents = append(agents, HarnessAgentRole{
				Name: domain + "_specialist",
				Role: domain + " execution",
			})
		}
		agents = append(agents,
			HarnessAgentRole{Name: "integrator", Role: "result integration"},
			HarnessAgentRole{Name: "reviewer", Role: "verification"},
		)
		return agents
	case "producer_reviewer":
		return []HarnessAgentRole{
			{Name: "implementer", Role: goalType + " execution"},
			{Name: "reviewer", Role: "verification"},
		}
	default:
		return []HarnessAgentRole{
			{Name: "planner", Role: "task decomposition"},
			{Name: "implementer", Role: goalType + " execution"},
			{Name: "reviewer", Role: "verification"},
		}
	}
}

func selectHarnessPhases(pattern, mode string) []string {
	if mode == "plan_only" {
		return []string{"classify", "load_skills", "decompose", "define_verification"}
	}
	if pattern == "single_track" {
		return []string{"classify", "load_skills", "execute", "verify"}
	}
	return []string{"classify", "load_skills", "plan", "execute", "verify", "integrate"}
}

func selectHarnessArtifacts(goalType, mode string) []string {
	if mode == "plan_only" {
		return []string{"execution_contract", "task_breakdown", "verification_checklist"}
	}
	base := []string{"execution_contract", "working_notes", "verification_report"}
	switch goalType {
	case "debug":
		return append(base, "root_cause_summary")
	case "migration":
		return append(base, "migration_plan")
	case "review":
		return append(base, "review_findings")
	default:
		return append(base, "change_summary")
	}
}

func selectHarnessVerificationPlan(goalType, mode string) []string {
	if mode == "plan_only" {
		return []string{
			"define acceptance criteria",
			"define narrowest relevant checks",
			"define rollback or recovery conditions",
		}
	}
	switch goalType {
	case "debug":
		return []string{
			"reproduce the issue before changing behavior",
			"run the narrowest fix validation first",
			"check diagnostics after edits",
			"confirm the regression is removed",
		}
	case "research":
		return []string{
			"lock the research question and acceptance criteria first",
			"verify findings against repository evidence before concluding",
			"compare viable options with explicit trade-offs",
			"state uncertainty and missing evidence before final recommendations",
		}
	default:
		return []string{
			"load required local skills before implementation",
			"run the narrowest relevant verification first",
			"check diagnostics after each edit batch",
			"confirm the result matches the requested scope only",
		}
	}
}

func selectHarnessNextAction(executionMode string, hasSkills bool, mode string) string {
	if mode == "plan_only" {
		if hasSkills {
			return "load_skills_then_plan"
		}
		return "plan"
	}
	if executionMode == "agent_team" {
		if hasSkills {
			return "load_skills_then_spawn_agents"
		}
		return "spawn_agents"
	}
	if hasSkills {
		return "load_skills_then_execute"
	}
	return "execute"
}

func firstNonEmptyHarnessString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
