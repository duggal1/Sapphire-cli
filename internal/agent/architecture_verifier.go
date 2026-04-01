package agent

import "strings"

type singularityArchitectureAssessment struct {
	Discipline string
	Blockers   []string
}

func assessTraceArchitectureQuality(trace *completedTurnTrace) singularityArchitectureAssessment {
	if trace == nil {
		return singularityArchitectureAssessment{Discipline: "adequate"}
	}
	if !requiresArchitectureVerification(trace) {
		return singularityArchitectureAssessment{Discipline: "not_required"}
	}
	text := strings.TrimSpace(trace.ResultText)
	if text == "" {
		text = strings.TrimSpace(trace.ResultSummary)
	}
	lower := strings.ToLower(text)

	optionComparison := hasAnySignal(lower, []string{
		"design a", "design b", "option a", "option b", "approach a", "approach b",
		"compare", "compared", "versus", " vs ", "trade-off", "tradeoffs", "trade-offs",
		"pros", "cons", "alternative", "alternatives", "recommended over",
	})
	repoFit := resultShowsValidationReasoning(text) || hasAnySignal(lower, []string{
		"repo fit", "repository fit", "current package structure", "current repository structure",
		"existing package", "existing packages", "existing boundaries", "current boundaries",
		"reuses existing", "fits the current repo", "matches go conventions", "blast radius: low",
		"cmd/api", "internal/platform", "internal/auth", "internal/billing",
	})
	migrationFit := hasAnySignal(lower, []string{
		"migration", "incremental", "blast radius", "compatibility", "rollout", "operational cost",
		"rewrite cost", "change surface", "dependency", "dependencies", "test matrix",
		"backward compatible", "gradual migration", "structural complexity",
	})

	blockers := make([]string, 0, 3)
	if !optionComparison {
		blockers = append(blockers, "option_comparison")
	}
	if !repoFit {
		blockers = append(blockers, "repo_fit")
	}
	if !migrationFit {
		blockers = append(blockers, "migration_cost")
	}

	switch {
	case optionComparison && repoFit && migrationFit:
		return singularityArchitectureAssessment{Discipline: "strong"}
	case optionComparison && repoFit:
		return singularityArchitectureAssessment{Discipline: "adequate", Blockers: blockers}
	default:
		return singularityArchitectureAssessment{Discipline: "weak", Blockers: blockers}
	}
}

func requiresArchitectureVerification(trace *completedTurnTrace) bool {
	if trace == nil {
		return false
	}
	switch strings.TrimSpace(trace.Family.GoalType) {
	case "design", "research", "review", "migration":
		return true
	case "implementation":
		return trace.Family.Breadth == "broad"
	default:
		return false
	}
}
