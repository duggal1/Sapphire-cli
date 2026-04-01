package agent

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

type singularityContextAssessment struct {
	Discipline              string
	StructuredEvidenceCount int
	ReadEvidenceCount       int
}

type singularityVerificationAssessment struct {
	Discipline     string
	RepoGrounded   bool
	EvidenceCount  int
	PreferredTools []string
}

type singularityDecompositionAssessment struct {
	Discipline string
}

type singularityExperienceReport struct {
	Context       singularityContextAssessment
	Verification  singularityVerificationAssessment
	Decomposition singularityDecompositionAssessment
}

func compileSingularityExperience(trace *completedTurnTrace) singularityExperienceReport {
	profile := buildSingularityCognitiveProfile(trace.Prompt, trace.Family)
	return singularityExperienceReport{
		Context:       assessTraceContextSufficiency(profile, trace),
		Verification:  assessTraceVerificationGraph(profile, trace),
		Decomposition: assessTraceDecomposition(profile, trace),
	}
}

func assessTraceContextSufficiency(profile singularityCognitiveProfile, trace *completedTurnTrace) singularityContextAssessment {
	structuredEvidence := countPositiveTraceEvidence(trace.StructuredEvidence)
	if structuredEvidence == 0 && hasStructuredDiscovery(trace) {
		structuredEvidence = 1
	}
	readEvidence := countPositiveTraceEvidence(trace.ReadEvidence)
	if readEvidence == 0 && trace.ToolCalls[tools.AgenticViewToolName]+trace.ToolCalls[tools.ViewToolName]+trace.ToolCalls[tools.SingleViewToolName] > 0 {
		readEvidence = 1
	}

	discipline := "adequate"
	switch {
	case structuredEvidence > 0 && readEvidence > 0 && structuredDiscoveryBeforeProtectedExecution(trace):
		discipline = "strong"
	case structuredEvidence > 0 && readEvidence > 0:
		discipline = "adequate"
	case profile.RequireRepositoryReading:
		discipline = "weak"
	}

	return singularityContextAssessment{
		Discipline:              discipline,
		StructuredEvidenceCount: structuredEvidence,
		ReadEvidenceCount:       readEvidence,
	}
}

func assessTraceVerificationGraph(profile singularityCognitiveProfile, trace *completedTurnTrace) singularityVerificationAssessment {
	evidenceCount := countPositiveTraceEvidence(trace.VerificationEvidence)
	resultText := effectiveSingularityResultText(trace)
	repoGrounded := resultShowsValidationReasoning(resultText) ||
		countPositiveTraceEvidence(trace.ReadEvidence) > 0 ||
		hasStructuredDiscovery(trace)

	discipline := "adequate"
	switch {
	case trace.ValidationChecks > 0 && repoGrounded:
		discipline = "strong"
	case profile.RequireValidation && resultShowsValidationReasoning(resultText) && repoGrounded:
		discipline = "strong"
	case trace.ValidationChecks > 0:
		discipline = "adequate"
	case repoGrounded && evidenceCount > 0:
		discipline = "adequate"
	case profile.RequireValidation:
		discipline = "weak"
	}

	return singularityVerificationAssessment{
		Discipline:     discipline,
		RepoGrounded:   repoGrounded,
		EvidenceCount:  evidenceCount,
		PreferredTools: deriveVerificationPreference(trace),
	}
}

func assessTraceDecomposition(profile singularityCognitiveProfile, trace *completedTurnTrace) singularityDecompositionAssessment {
	if trace == nil {
		return singularityDecompositionAssessment{Discipline: "adequate"}
	}
	decision := evaluateSubAgentLaunch(trace.Prompt)
	seenTools := uniqueLearnedToolNames(trace.OrderedTools)
	hasHarness := containsTool(seenTools, tools.RunHarnessToolName)
	hasPlan := trace.ToolCalls[tools.UpdatePlanToolName] > 0
	hasParallel := containsAnyTool(seenTools, SpawnAgentToolName, WaitAgentsToolName, CollectResultToolName)

	discipline := "adequate"
	switch {
	case profile.RequirePlanning && hasHarness && (hasPlan || !profile.RequireLongHorizon) && (!decision.Parallelizable || hasParallel || decision.Complexity < 3):
		discipline = "strong"
	case profile.RequirePlanning && (hasHarness || hasPlan):
		discipline = "adequate"
	case profile.RequirePlanning:
		discipline = "weak"
	}

	return singularityDecompositionAssessment{Discipline: discipline}
}

func deriveVerificationPreference(trace *completedTurnTrace) []string {
	if trace == nil {
		return nil
	}
	candidates := []string{
		tools.DiagnosticsToolName,
		tools.SingleViewToolName,
		tools.ViewToolName,
		tools.AgenticViewToolName,
		tools.BashToolName,
	}
	type scored struct {
		name  string
		score int
	}
	scoredTools := make([]scored, 0, len(candidates))
	for _, candidate := range candidates {
		score := trace.ToolCalls[candidate]
		if score == 0 {
			continue
		}
		switch candidate {
		case tools.BashToolName:
			if countPositiveTraceEvidence(trace.VerificationEvidence) == 0 {
				continue
			}
		case tools.SingleViewToolName, tools.ViewToolName, tools.AgenticViewToolName:
			if countPositiveTraceEvidence(trace.VerificationEvidence) == 0 && trace.ValidationChecks == 0 {
				continue
			}
		}
		scoredTools = append(scoredTools, scored{name: candidate, score: score})
	}
	sort.Slice(scoredTools, func(i, j int) bool {
		if scoredTools[i].score == scoredTools[j].score {
			return scoredTools[i].name < scoredTools[j].name
		}
		return scoredTools[i].score > scoredTools[j].score
	})
	out := make([]string, 0, len(scoredTools))
	for _, item := range scoredTools {
		out = append(out, item.name)
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func countPositiveTraceEvidence(values map[string]int) int {
	count := 0
	for _, value := range values {
		if value > 0 {
			count++
		}
	}
	return count
}

func effectiveSingularityResultText(trace *completedTurnTrace) string {
	if trace == nil {
		return ""
	}
	if text := strings.TrimSpace(trace.ResultText); text != "" {
		return text
	}
	return strings.TrimSpace(trace.ResultSummary)
}

func parseObservedToolInput(rawInput string) map[string]any {
	rawInput = strings.TrimSpace(rawInput)
	if rawInput == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(rawInput), &payload); err != nil {
		return nil
	}
	return payload
}
