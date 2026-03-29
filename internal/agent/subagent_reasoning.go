package agent

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/duggal1/Sapphire-cli/internal/config"
)

const gemini25MaxThinkingBudget int64 = 32768

var subAgentReasoningHeavySignals = []string{
	"analyze",
	"analysis",
	"architecture",
	"architectural",
	"audit",
	"compare",
	"cross-check",
	"cross check",
	"debug",
	"diagnose",
	"diagnostic",
	"diagnostics",
	"design",
	"evaluate",
	"explain",
	"investigate",
	"overview",
	"plan",
	"reason",
	"reasoning",
	"review",
	"risk",
	"risks",
	"root cause",
	"survey",
	"synthesize",
	"threat model",
	"triage",
	"validate",
	"verification",
}

var subAgentExecutionSignals = []string{
	"add",
	"build",
	"change",
	"compile",
	"create",
	"edit",
	"execute",
	"fix",
	"format",
	"implement",
	"implementation",
	"modify",
	"patch",
	"refactor",
	"remove",
	"rename",
	"run",
	"test",
	"update",
	"wire",
	"write",
}

func (c *coordinator) resolveSubAgentModelOverride(rawModel, requestedReasoning, prompt string, decision subAgentLaunchDecision) (*agentModelOverride, error) {
	largeModelCfg, ok := c.cfg.Models[config.SelectedModelTypeLarge]
	if !ok {
		return nil, fmt.Errorf("large model not selected")
	}

	selected := largeModelCfg
	rawModel = strings.TrimSpace(rawModel)
	if rawModel != "" {
		parts := strings.SplitN(rawModel, ":", 2)
		if len(parts) == 2 {
			selected.Provider = strings.TrimSpace(parts[0])
			selected.Model = strings.TrimSpace(parts[1])
		} else {
			selected.Model = strings.TrimSpace(rawModel)
		}
	}

	if selected.Provider == "" || selected.Model == "" {
		return nil, fmt.Errorf("invalid model override")
	}
	if _, ok := c.cfg.Providers.Get(selected.Provider); !ok {
		return nil, fmt.Errorf("model provider %q not configured", selected.Provider)
	}

	model := c.cfg.GetModel(selected.Provider, selected.Model)
	if model == nil {
		return nil, fmt.Errorf("model %q not configured for provider %q", selected.Model, selected.Provider)
	}

	override := &agentModelOverride{
		Provider: selected.Provider,
		Model:    selected.Model,
	}
	hasModelOverride := selected.Provider != largeModelCfg.Provider || selected.Model != largeModelCfg.Model

	targetReasoning := strings.ToLower(strings.TrimSpace(requestedReasoning))
	if targetReasoning == "" {
		targetReasoning = subAgentReasoningEffortForTask(prompt, decision)
	}

	switch {
	case !model.CanReason:
		if hasModelOverride {
			return override, nil
		}
		return nil, nil
	case isGemini25ReasoningModel(model.ID):
		override.ProviderOptions = map[string]any{
			"thinking_config": map[string]any{
				"thinking_budget":  gemini25MaxThinkingBudget,
				"include_thoughts": true,
			},
		}
	case targetReasoning != "":
		override.ReasoningEffort = normalizeSubAgentReasoningEffort(model, targetReasoning)
	}

	if !hasModelOverride && override.ReasoningEffort == "" && len(override.ProviderOptions) == 0 {
		return nil, nil
	}
	return override, nil
}

func applyAgentModelOverride(base config.SelectedModel, override *agentModelOverride) config.SelectedModel {
	if override == nil {
		return base
	}

	modelChanged := (override.Provider != "" && override.Provider != base.Provider) || (override.Model != "" && override.Model != base.Model)
	if modelChanged {
		base.Think = false
		base.ReasoningEffort = ""
		base.ProviderOptions = nil
	}

	if override.Provider != "" {
		base.Provider = override.Provider
	}
	if override.Model != "" {
		base.Model = override.Model
	}
	if override.ReasoningEffort != "" {
		base.ReasoningEffort = override.ReasoningEffort
	}
	if len(override.ProviderOptions) > 0 {
		if base.ProviderOptions == nil {
			base.ProviderOptions = make(map[string]any, len(override.ProviderOptions))
		} else {
			base.ProviderOptions = maps.Clone(base.ProviderOptions)
		}
		for key, value := range override.ProviderOptions {
			base.ProviderOptions[key] = value
		}
	}

	return base
}

func subAgentReasoningEffortForTask(prompt string, decision subAgentLaunchDecision) string {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	if normalized == "" {
		return "medium"
	}
	if isReasoningHeavySubAgentTask(normalized, decision) {
		return "high"
	}
	return "medium"
}

func isReasoningHeavySubAgentTask(prompt string, decision subAgentLaunchDecision) bool {
	if hasAnySignal(prompt, subAgentReasoningHeavySignals) {
		return true
	}
	if hasAnySignal(prompt, subAgentDependencySignals) || hasAnySignal(prompt, subAgentMultiSourceSignals) || hasAnySignal(prompt, subAgentRiskSignals) {
		return true
	}
	if hasAnySignal(prompt, subAgentCodebaseSignals) && !hasAnySignal(prompt, subAgentExecutionSignals) {
		return true
	}
	if hasAnySignal(prompt, subAgentSourceSignals) && !hasAnySignal(prompt, subAgentExecutionSignals) {
		return true
	}
	return looksLikeQuestion(prompt) && decision.Complexity >= 3
}

func normalizeSubAgentReasoningEffort(model *catwalk.Model, desired string) string {
	desired = strings.ToLower(strings.TrimSpace(desired))
	if desired == "" {
		return ""
	}

	choices := subAgentReasoningChoices(model)
	if len(choices) == 0 {
		return desired
	}

	switch desired {
	case "xhigh":
		return firstAllowedReasoningChoice(choices, "xhigh", "high", "medium", "low", "minimal")
	case "high":
		return firstAllowedReasoningChoice(choices, "high", "xhigh", "medium", "low", "minimal")
	case "medium":
		return firstAllowedReasoningChoice(choices, "medium", "low", "minimal", "high", "xhigh")
	case "low":
		return firstAllowedReasoningChoice(choices, "low", "minimal", "medium", "high", "xhigh")
	case "minimal":
		return firstAllowedReasoningChoice(choices, "minimal", "low", "medium", "high", "xhigh")
	default:
		return firstAllowedReasoningChoice(choices, desired)
	}
}

func firstAllowedReasoningChoice(choices []string, candidates ...string) string {
	for _, candidate := range candidates {
		if slices.Contains(choices, candidate) {
			return candidate
		}
	}
	if len(choices) > 0 {
		return choices[0]
	}
	return ""
}

func subAgentReasoningChoices(model *catwalk.Model) []string {
	if model == nil || !model.CanReason {
		return nil
	}

	normalizedID := normalizeReasoningModelID(model.ID)
	switch {
	case config.IsGemini3Model(normalizedID):
		if strings.Contains(normalizedID, "flash") {
			return []string{"minimal", "low", "medium", "high"}
		}
		return []string{"low", "medium", "high"}
	case config.IsGemini25Model(normalizedID):
		return nil
	}

	choices := make([]string, 0, len(model.ReasoningLevels))
	for _, choice := range model.ReasoningLevels {
		trimmed := strings.ToLower(strings.TrimSpace(choice))
		if trimmed == "" || slices.Contains(choices, trimmed) {
			continue
		}
		choices = append(choices, trimmed)
	}
	if len(choices) == 0 {
		if fallback := strings.ToLower(strings.TrimSpace(model.DefaultReasoningEffort)); fallback != "" {
			return []string{fallback}
		}
	}
	return choices
}

func isGemini25ReasoningModel(modelID string) bool {
	return config.IsGemini25Model(normalizeReasoningModelID(modelID))
}

func normalizeReasoningModelID(modelID string) string {
	modelID = strings.ToLower(strings.TrimSpace(modelID))
	if slash := strings.LastIndex(modelID, "/"); slash >= 0 {
		modelID = modelID[slash+1:]
	}
	return modelID
}
