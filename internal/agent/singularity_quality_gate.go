package agent

import (
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
)

const (
	singularityLearningVerdictAccepted    = "accepted"
	singularityLearningVerdictRejected    = "rejected"
	singularityLearningVerdictQuarantined = "quarantined"
)

type singularityConfidenceVector struct {
	Context       int `json:"context"`
	Planning      int `json:"planning"`
	Decomposition int `json:"decomposition"`
	PlanQuality   int `json:"plan_quality"`
	Architecture  int `json:"architecture"`
	Validation    int `json:"validation"`
	Tradeoff      int `json:"tradeoff"`
	Recovery      int `json:"recovery"`
}

type singularityLearningVerdict struct {
	Decision string                      `json:"decision"`
	Blockers []string                    `json:"blockers,omitempty"`
	Vector   singularityConfidenceVector `json:"vector"`
}

func evaluateSingularityLearningVerdict(trace *completedTurnTrace, assessment singularityCognitiveAssessment) singularityLearningVerdict {
	verdict := singularityLearningVerdict{
		Decision: singularityLearningVerdictRejected,
		Vector: singularityConfidenceVector{
			Context:       disciplineConfidence(assessment.ContextDiscipline),
			Planning:      disciplineConfidence(assessment.PlanningDiscipline),
			Decomposition: disciplineConfidence(assessment.DecompositionDiscipline),
			PlanQuality:   disciplineConfidence(assessment.PlanQualityDiscipline),
			Architecture:  disciplineConfidence(assessment.ArchitectureDiscipline),
			Validation:    disciplineConfidence(assessment.ValidationDiscipline),
			Tradeoff:      disciplineConfidence(assessment.TradeoffDiscipline),
			Recovery:      disciplineConfidence(assessment.RecoveryDiscipline),
		},
	}
	if trace == nil || !strings.EqualFold(strings.TrimSpace(trace.Status), "completed") {
		return verdict
	}
	verdict.Decision = singularityLearningVerdictAccepted

	profile := buildSingularityCognitiveProfile(trace.Prompt, trace.Family)
	blockers := make([]string, 0, 6)
	addBlocker := func(name string) {
		for _, existing := range blockers {
			if existing == name {
				return
			}
		}
		blockers = append(blockers, name)
	}

	if profile.RequireRepositoryReading && assessment.ContextDiscipline == "weak" {
		addBlocker("context")
	}
	if profile.RequirePlanning && assessment.PlanningDiscipline == "weak" {
		addBlocker("planning")
	}
	if profile.RequirePlanning && assessment.DecompositionDiscipline == "weak" {
		addBlocker("decomposition")
	}
	if planmode.NormalizeMode(planmode.SessionMode(trace.Mode)) == planmode.PlanMode {
		if assessment.PlanQualityDiscipline != "strong" {
			addBlocker("plan_quality")
		}
	} else if requiresPlanQualityLearningGate(trace, profile) && assessment.PlanQualityDiscipline == "weak" {
		addBlocker("plan_quality")
	}

	switch trace.Family.GoalType {
	case "design", "research", "review", "migration":
		if assessment.ArchitectureDiscipline != "strong" {
			addBlocker("architecture")
		}
		if assessment.ValidationDiscipline != "strong" {
			addBlocker("validation")
		}
		if assessment.TradeoffDiscipline != "strong" {
			addBlocker("tradeoff")
		}
	case "implementation":
		if trace.Family.Breadth == "broad" && assessment.ArchitectureDiscipline == "weak" {
			addBlocker("architecture")
		}
		if profile.RequireValidation && assessment.ValidationDiscipline == "weak" {
			addBlocker("validation")
		}
	case "initialize", "debug":
		if profile.RequireValidation && assessment.ValidationDiscipline == "weak" {
			addBlocker("validation")
		}
	default:
		if profile.RequireValidation && assessment.ValidationDiscipline == "weak" {
			addBlocker("validation")
		}
	}

	if trace.ClosureMode != "" && trace.ClosureMode != headlessClosureModeNormal {
		switch trace.Family.GoalType {
		case "design", "research", "review", "migration":
			if assessment.ArchitectureDiscipline != "strong" || assessment.ValidationDiscipline != "strong" || assessment.TradeoffDiscipline != "strong" {
				addBlocker("recovery")
			}
		default:
			addBlocker("recovery")
		}
	}

	if assessment.RecoveryDiscipline == "weak" {
		addBlocker("recovery")
	}

	if len(blockers) == 0 {
		return verdict
	}
	verdict.Decision = singularityLearningVerdictQuarantined
	verdict.Blockers = blockers
	return verdict
}

func requiresPlanQualityLearningGate(trace *completedTurnTrace, profile singularityCognitiveProfile) bool {
	if trace == nil {
		return false
	}
	if planmode.NormalizeMode(planmode.SessionMode(trace.Mode)) == planmode.PlanMode {
		return true
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

func disciplineConfidence(value string) int {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "strong":
		return 95
	case "adequate":
		return 70
	case "clean":
		return 88
	case "not_required":
		return 100
	case "weak":
		return 20
	default:
		return 50
	}
}
