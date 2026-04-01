package agent

import (
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

type singularityPlanQualityAssessment struct {
	Discipline string
	Blockers   []string
}

func assessTracePlanQuality(trace *completedTurnTrace) singularityPlanQualityAssessment {
	if trace == nil {
		return singularityPlanQualityAssessment{Discipline: "adequate"}
	}
	profile := buildSingularityCognitiveProfile(trace.Prompt, trace.Family)
	mode := planmode.NormalizeMode(planmode.SessionMode(trace.Mode))
	text := strings.TrimSpace(trace.ResultText)
	if text == "" {
		text = strings.TrimSpace(trace.ResultSummary)
	}
	if !profile.RequirePlanning && mode != planmode.PlanMode {
		return singularityPlanQualityAssessment{Discipline: "not_required"}
	}
	if mode == planmode.PlanMode {
		return assessStructuredPlanQuality(text, trace)
	}
	return assessExecutionPlanQuality(text, trace)
}

func assessStructuredPlanQuality(text string, trace *completedTurnTrace) singularityPlanQualityAssessment {
	block, ok := planmode.ExtractStructuredBlockForMode(planmode.PlanMode, text)
	if !ok || !block.IsValid {
		return singularityPlanQualityAssessment{
			Discipline: "weak",
			Blockers:   []string{"structured_plan"},
		}
	}
	body := strings.TrimSpace(block.Content)
	lower := strings.ToLower(body)
	sectionCount := countPlanSections(body)
	itemCount := countPlanListItems(body)
	hasCurrentReality := hasAnySignal(lower, []string{"current reality", "current state", "current behavior", "existing behavior"})
	hasChanges := hasAnySignal(lower, []string{"key changes", "implementation changes", "changes", "approach", "summary"})
	hasValidation := hasAnySignal(lower, []string{"test plan", "tests", "validation", "acceptance", "verify", "verification"})
	hasRisks := hasAnySignal(lower, []string{"assumption", "assumptions", "risk", "risks", "open question", "open questions"})

	blockers := make([]string, 0, 4)
	if sectionCount < 3 {
		blockers = append(blockers, "plan_sections")
	}
	if itemCount < 4 {
		blockers = append(blockers, "actionable_steps")
	}
	if !hasValidation {
		blockers = append(blockers, "validation_plan")
	}
	if !hasRisks {
		blockers = append(blockers, "assumptions_or_risks")
	}

	switch {
	case len(blockers) == 0 && hasCurrentReality && hasChanges:
		return singularityPlanQualityAssessment{Discipline: "strong"}
	case len(blockers) <= 1 && (hasCurrentReality || hasChanges):
		return singularityPlanQualityAssessment{Discipline: "adequate", Blockers: blockers}
	default:
		return singularityPlanQualityAssessment{Discipline: "weak", Blockers: blockers}
	}
}

func assessExecutionPlanQuality(text string, trace *completedTurnTrace) singularityPlanQualityAssessment {
	lower := strings.ToLower(strings.TrimSpace(text))
	if trace == nil {
		return singularityPlanQualityAssessment{Discipline: "adequate"}
	}
	hasPublishedPlan := trace.ToolCalls[tools.UpdatePlanToolName] > 0
	hasSequence := hasAnySignal(lower, []string{"first", "then", "next", "phase", "step", "sequence", "rollout"}) || executionTraceShowsPlanSequence(trace)
	hasValidation := hasAnySignal(lower, []string{"test", "validate", "verification", "acceptance", "verify"})
	hasScopeLock := hasAnySignal(lower, []string{"constraint", "keep", "without changing", "do not", "scope"}) || promptShowsScopeLock(trace.Prompt)
	hasRisk := hasAnySignal(lower, []string{"risk", "blast radius", "migration", "compatibility", "assumption"}) || resultShowsTradeoffReasoning(text)

	blockers := make([]string, 0, 4)
	if !hasPublishedPlan {
		blockers = append(blockers, "published_plan")
	}
	if !hasSequence {
		blockers = append(blockers, "execution_sequence")
	}
	if !hasValidation {
		blockers = append(blockers, "validation_plan")
	}

	switch {
	case hasPublishedPlan && hasSequence && hasValidation && (hasScopeLock || hasRisk):
		return singularityPlanQualityAssessment{Discipline: "strong"}
	case hasPublishedPlan && hasSequence:
		return singularityPlanQualityAssessment{Discipline: "adequate", Blockers: blockers}
	default:
		return singularityPlanQualityAssessment{Discipline: "weak", Blockers: blockers}
	}
}

func executionTraceShowsPlanSequence(trace *completedTurnTrace) bool {
	if trace == nil {
		return false
	}
	if trace.ToolCalls[tools.UpdatePlanToolName] == 0 {
		return false
	}
	hasDiscovery := hasStructuredDiscovery(trace) || countPositiveTraceEvidence(trace.ReadEvidence) > 0
	if !hasDiscovery {
		return false
	}
	if len(trace.OrderedTools) == 0 {
		return true
	}
	discoveryIndex := -1
	planIndex := -1
	for idx, name := range trace.OrderedTools {
		switch strings.TrimSpace(name) {
		case tools.ToolSearchToolName, tools.RGFilesToolName, tools.RGToolName, tools.GlobToolName, tools.GrepToolName, tools.AgenticViewToolName, tools.ViewToolName, tools.SingleViewToolName:
			if discoveryIndex == -1 {
				discoveryIndex = idx
			}
		case tools.UpdatePlanToolName:
			if planIndex == -1 {
				planIndex = idx
			}
		}
	}
	if planIndex == -1 {
		return false
	}
	if discoveryIndex == -1 {
		return true
	}
	return planIndex > discoveryIndex
}

func promptShowsScopeLock(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if lower == "" {
		return false
	}
	return hasAnySignal(lower, []string{"without changing", "do not", "only", "minimal", "keep existing", "preserve", "constraint"})
}

func countPlanSections(body string) int {
	count := 0
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			count++
			continue
		}
		if hasAnySignal(strings.ToLower(trimmed), []string{"summary", "current reality", "current state", "key changes", "implementation changes", "risks", "test plan", "assumptions"}) {
			count++
		}
	}
	if count > 6 {
		return 6
	}
	return count
}

func countPlanListItems(body string) int {
	count := 0
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- "):
			count++
		case strings.HasPrefix(trimmed, "* "):
			count++
		case len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed, ". "):
			count++
		}
	}
	return count
}
