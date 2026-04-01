package agent

import (
	"fmt"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

type singularityCognitiveProfile struct {
	TaskClass                 string
	Complexity                int
	Broad                     bool
	RequireObjectiveLock      bool
	RequireConstraintLock     bool
	RequireRepositoryReading  bool
	RequirePlanning           bool
	RequireTradeoffReasoning  bool
	RequireValidation         bool
	RequireRecoveryDiscipline bool
	RequireLongHorizon        bool
}

type singularityCognitiveAssessment struct {
	TaskClass               string `json:"task_class"`
	ContextDiscipline       string `json:"context_discipline"`
	PlanningDiscipline      string `json:"planning_discipline"`
	DecompositionDiscipline string `json:"decomposition_discipline"`
	PlanQualityDiscipline   string `json:"plan_quality_discipline"`
	ArchitectureDiscipline  string `json:"architecture_discipline"`
	ValidationDiscipline    string `json:"validation_discipline"`
	RecoveryDiscipline      string `json:"recovery_discipline"`
	TradeoffDiscipline      string `json:"tradeoff_discipline"`
	ExecutionRisk           string `json:"execution_risk"`
}

func buildSingularityCognitiveProfile(prompt string, family learnedTaskFamily) singularityCognitiveProfile {
	decision := evaluateSubAgentLaunch(prompt)
	profile := singularityCognitiveProfile{
		TaskClass:                 family.GoalType,
		Complexity:                decision.Complexity,
		Broad:                     family.Breadth == "broad",
		RequireObjectiveLock:      true,
		RequireConstraintLock:     true,
		RequireRepositoryReading:  family.Breadth == "broad" || decision.Complexity >= 2,
		RequirePlanning:           decision.Complexity >= 2 || family.Breadth == "broad" || family.GoalType == "initialize" || family.GoalType == "migration" || family.GoalType == "research" || family.GoalType == "design" || family.GoalType == "review",
		RequireValidation:         decision.Complexity >= 2 || family.GoalType == "debug" || family.GoalType == "implementation" || family.GoalType == "initialize" || family.GoalType == "design" || family.GoalType == "review" || family.GoalType == "migration" || family.GoalType == "research",
		RequireRecoveryDiscipline: decision.Complexity >= 2 || family.Breadth == "broad",
		RequireLongHorizon:        family.Breadth == "broad" || decision.Complexity >= 3,
	}

	switch family.GoalType {
	case "design", "review", "migration", "research":
		profile.RequireTradeoffReasoning = true
	case "implementation":
		profile.RequireTradeoffReasoning = family.Breadth == "broad" || decision.Complexity >= 3
	}
	return profile
}

func shouldInjectSingularityCognitiveContract(prompt string, family learnedTaskFamily) bool {
	if family.GoalType == "initialize" || family.Breadth == "broad" {
		return true
	}
	return evaluateSubAgentLaunch(prompt).Complexity >= 2
}

func renderSingularityCognitiveContract(profile singularityCognitiveProfile) string {
	lines := []string{
		fmt.Sprintf("<singularity_cognitive_contract task_class=%q complexity=%d>", profile.TaskClass, profile.Complexity),
		"- Lock the exact user objective before acting. Do not widen scope, soften constraints, or optimize for a different outcome.",
		"- Classify the task correctly and keep behavior aligned with that class for the whole turn.",
	}
	if profile.RequireConstraintLock {
		lines = append(lines, "- Extract hard constraints from the prompt early and keep them live while planning, coding, and validating.")
	}
	if profile.RequireRepositoryReading {
		lines = append(lines, "- On non-trivial repo work, read enough real code before committing to a design, edit, or conclusion. Shallow reads are treated as failure.")
	}
	if profile.RequirePlanning {
		lines = append(lines, "- Build the smallest correct plan that reduces uncertainty first. Do not confuse long plans with good plans.")
	}
	if profile.RequireTradeoffReasoning {
		lines = append(lines, "- Compare viable options and commit only after considering trade-offs, migration cost, and repo fit.")
	}
	if profile.RequireValidation {
		lines = append(lines, "- Decide how you will validate the work before execution or before making a confident claim.")
	}
	if profile.RequireRecoveryDiscipline {
		lines = append(lines, "- Treat contradictions, tool guidance errors, and missing evidence as a signal to change course early instead of pushing through.")
	}
	if profile.RequireLongHorizon {
		lines = append(lines, "- Preserve consistency with prior decisions, accepted constraints, and rejected paths unless new evidence forces a change.")
	}
	lines = append(lines, "</singularity_cognitive_contract>")
	return strings.Join(lines, "\n")
}

func coldStartRoutePolicy(prompt string) (learnedRoutePolicy, learnedTaskFamily, bool) {
	family := classifyLearnedTaskFamily(prompt)
	decision := evaluateSubAgentLaunch(prompt)
	if family.GoalType == "initialize" && family.Breadth == "broad" {
		return learnedRoutePolicy{
			TaskFamily:                   family.ID,
			TaskFamilySlug:               sanitizeLearnedSlug(family.ID),
			GoalType:                     family.GoalType,
			Breadth:                      family.Breadth,
			Domains:                      append([]string{}, family.Domains...),
			PreferredDiscovery:           []string{tools.ToolSearchToolName, tools.RGFilesToolName, tools.AgenticViewToolName, tools.RGToolName, tools.LSToolName},
			PreferredVerification:        []string{tools.SingleViewToolName, tools.ViewToolName, tools.AgenticViewToolName, tools.DiagnosticsToolName},
			RequireHarness:               true,
			PreferParallel:               true,
			PreferIndexCodebase:          false,
			ForbidBashDiscovery:          true,
			RequireContextRead:           true,
			RequirePostWriteVerification: promptMentionsAgentsArtifact(prompt),
			Confidence:                   72,
			PromotionState:               learnedPolicyStateCandidate,
		}, family, true
	}
	if family.Breadth == "broad" {
		requireHarness := family.GoalType == "design" || family.GoalType == "research" || family.GoalType == "review" || family.GoalType == "migration" || decision.Parallelizable || decision.Complexity >= 4
		requirePlan := family.GoalType == "design" || family.GoalType == "research" || family.GoalType == "review" || family.GoalType == "migration" || (family.GoalType == "implementation" && (decision.Parallelizable || decision.Complexity >= 3)) || decision.Complexity >= 4
		confidence := 62
		if family.GoalType == "design" || family.GoalType == "research" {
			confidence = 68
		}
		return learnedRoutePolicy{
			TaskFamily:                   family.ID,
			TaskFamilySlug:               sanitizeLearnedSlug(family.ID),
			GoalType:                     family.GoalType,
			Breadth:                      family.Breadth,
			Domains:                      append([]string{}, family.Domains...),
			PreferredDiscovery:           []string{tools.ToolSearchToolName, tools.RGFilesToolName, tools.AgenticViewToolName, tools.RGToolName, tools.LSToolName},
			PreferredVerification:        []string{tools.DiagnosticsToolName, tools.AgenticViewToolName, tools.ViewToolName, tools.SingleViewToolName},
			RequireHarness:               requireHarness,
			PreferParallel:               decision.Parallelizable || family.GoalType == "design" || family.GoalType == "research",
			ForbidBashDiscovery:          true,
			RequireContextRead:           true,
			RequireExplicitPlan:          requirePlan,
			RequirePostWriteVerification: promptMentionsAgentsArtifact(prompt),
			Confidence:                   confidence,
			PromotionState:               learnedPolicyStateCandidate,
		}, family, true
	}
	return learnedRoutePolicy{}, family, false
}

func promptMentionsAgentsArtifact(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	return strings.Contains(lower, "agents.md") || strings.Contains(lower, "agent.md")
}

func assessSingularityCognition(trace *completedTurnTrace) singularityCognitiveAssessment {
	profile := buildSingularityCognitiveProfile(trace.Prompt, trace.Family)
	experience := compileSingularityExperience(trace)
	contextDiscipline := experience.Context.Discipline

	planningDiscipline := "adequate"
	switch {
	case containsTool(uniqueLearnedToolNames(trace.OrderedTools), tools.RunHarnessToolName):
		planningDiscipline = "strong"
	case trace.ToolCalls[tools.UpdatePlanToolName] > 0:
		planningDiscipline = "adequate"
	case profile.RequirePlanning:
		planningDiscipline = "weak"
	}

	validationDiscipline := experience.Verification.Discipline
	planQuality := assessTracePlanQuality(trace)
	architecture := assessTraceArchitectureQuality(trace)

	recoveryDiscipline := "clean"
	switch {
	case len(trace.ToolErrorCodes) > 0 && strings.EqualFold(trace.Status, "completed"):
		recoveryDiscipline = "strong"
	case len(trace.ToolErrorCodes) > 0:
		recoveryDiscipline = "weak"
	case profile.RequireRecoveryDiscipline:
		recoveryDiscipline = "clean"
	}

	tradeoffDiscipline := "not_required"
	if profile.RequireTradeoffReasoning {
		if resultShowsTradeoffReasoning(effectiveSingularityResultText(trace)) {
			tradeoffDiscipline = "strong"
		} else {
			tradeoffDiscipline = "weak"
		}
	}

	executionRisk := "low"
	weakCount := 0
	for _, value := range []string{
		contextDiscipline,
		planningDiscipline,
		experience.Decomposition.Discipline,
		planQuality.Discipline,
		architecture.Discipline,
		validationDiscipline,
		recoveryDiscipline,
		tradeoffDiscipline,
	} {
		if value == "weak" {
			weakCount++
		}
	}
	switch {
	case weakCount >= 2:
		executionRisk = "high"
	case weakCount == 1:
		executionRisk = "medium"
	}

	return singularityCognitiveAssessment{
		TaskClass:               trace.Family.GoalType,
		ContextDiscipline:       contextDiscipline,
		PlanningDiscipline:      planningDiscipline,
		DecompositionDiscipline: experience.Decomposition.Discipline,
		PlanQualityDiscipline:   planQuality.Discipline,
		ArchitectureDiscipline:  architecture.Discipline,
		ValidationDiscipline:    validationDiscipline,
		RecoveryDiscipline:      recoveryDiscipline,
		TradeoffDiscipline:      tradeoffDiscipline,
		ExecutionRisk:           executionRisk,
	}
}

func structuredDiscoveryBeforeProtectedExecution(trace *completedTurnTrace) bool {
	if trace == nil {
		return false
	}
	structuredIndex := -1
	protectedIndex := -1
	for idx, toolName := range trace.OrderedTools {
		canonical := strings.TrimSpace(toolName)
		switch canonical {
		case tools.ToolSearchToolName, tools.RGFilesToolName, tools.RGToolName, tools.GlobToolName, tools.GrepToolName:
			if structuredIndex == -1 {
				structuredIndex = idx
			}
		}
		if protectedIndex == -1 && isSingularityProtectedExecutionTool(canonical) {
			protectedIndex = idx
		}
	}
	if structuredIndex == -1 {
		return false
	}
	if protectedIndex == -1 {
		return true
	}
	return structuredIndex < protectedIndex
}

func isSingularityProtectedExecutionTool(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case tools.BashToolName, tools.WriteToolName, tools.EditToolName, tools.SingleEditToolName, tools.AgenticEditToolName, tools.ApplyPatchToolName, SpawnAgentToolName, WaitAgentsToolName, CollectResultToolName:
		return true
	default:
		return false
	}
}

func resultShowsTradeoffReasoning(resultSummary string) bool {
	lower := strings.ToLower(strings.TrimSpace(resultSummary))
	if lower == "" {
		return false
	}
	signals := []string{"trade-off", "trade-offs", "tradeoff", "versus", "vs ", "option", "risk", "cost", "constraint", "migration", "pros", "cons", "advantages", "disadvantages"}
	return hasAnySignal(lower, signals)
}

func resultShowsValidationReasoning(resultSummary string) bool {
	lower := strings.ToLower(strings.TrimSpace(resultSummary))
	if lower == "" {
		return false
	}
	signals := []string{
		"validate", "validated", "validation", "grounded",
		"against the current package structure", "against the current repository",
		"based on a review of the repository", "based on the repository",
		"current package structure", "current repository structure",
	}
	return hasAnySignal(lower, signals)
}

func looksLikeValidationShellCommand(command string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	if command == "" {
		return false
	}
	prefixes := []string{
		"go test", "go build", "go vet", "cargo test", "cargo check",
		"npm test", "npm run test", "npm run build", "pnpm test", "pnpm build",
		"yarn test", "yarn build", "bun test", "bun run test", "pytest",
		"vitest", "jest", "ruff check", "golangci-lint", "make test", "make build",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}
