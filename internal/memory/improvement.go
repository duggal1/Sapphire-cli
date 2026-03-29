package memory

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	MemoryEventArchitecturalDecision = "architectural_decision"
	MemoryEventNegativeConstraint    = "negative_constraint"
	MemoryEventFailureMode           = "failure_mode"
	MemoryEventTaskProgress          = "task_progress"
	MemoryEventImprovementEval       = "improvement_eval"
	MemoryEventStrategyPattern       = "strategy_pattern"
)

const (
	MemoryFilterAll                 = "all"
	MemoryFilterNegativeConstraints = "negative_constraints"
	MemoryFilterArchitectural       = "architectural"
	MemoryFilterFailures            = "failures"
	MemoryFilterProgress            = "progress"
	MemoryFilterEvals               = "evals"
	MemoryFilterStrategies          = "strategies"
)

type ImprovementEval struct {
	TaskShape        string   `json:"task_shape"`
	FailureSignature string   `json:"failure_signature"`
	Probe            string   `json:"probe"`
	SuccessCriteria  string   `json:"success_criteria"`
	PreventionRule   string   `json:"prevention_rule,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
}

type StrategyPattern struct {
	TaskShape       string   `json:"task_shape"`
	Strategy        string   `json:"strategy"`
	WhyItWorked     string   `json:"why_it_worked,omitempty"`
	TriggerSignals  []string `json:"trigger_signals,omitempty"`
	ValidationProbe string   `json:"validation_probe,omitempty"`
}

func isValidSavedMemoryEventType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case
		MemoryEventArchitecturalDecision,
		MemoryEventNegativeConstraint,
		MemoryEventFailureMode,
		MemoryEventTaskProgress,
		MemoryEventImprovementEval,
		MemoryEventStrategyPattern:
		return true
	default:
		return false
	}
}

func formatImprovementEvalRecord(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var payload ImprovementEval
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil && hasImprovementEvalContent(payload) {
		parts := make([]string, 0, 5)
		if value := strings.TrimSpace(payload.TaskShape); value != "" {
			parts = append(parts, value)
		}
		if value := strings.TrimSpace(payload.Probe); value != "" {
			parts = append(parts, "probe: "+value)
		}
		if value := strings.TrimSpace(payload.SuccessCriteria); value != "" {
			parts = append(parts, "success: "+value)
		}
		if value := strings.TrimSpace(payload.FailureSignature); value != "" {
			parts = append(parts, "failure: "+value)
		}
		if value := strings.TrimSpace(payload.PreventionRule); value != "" {
			parts = append(parts, "rule: "+value)
		}
		return strings.Join(parts, " | ")
	}
	return formatGenericImprovementRecord(trimmed, "probe", "success_criteria", "task_shape")
}

func formatStrategyPatternRecord(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var payload StrategyPattern
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil && hasStrategyPatternContent(payload) {
		parts := make([]string, 0, 5)
		if value := strings.TrimSpace(payload.TaskShape); value != "" {
			parts = append(parts, value)
		}
		if value := strings.TrimSpace(payload.Strategy); value != "" {
			parts = append(parts, value)
		}
		if value := strings.TrimSpace(payload.WhyItWorked); value != "" {
			parts = append(parts, "why: "+value)
		}
		if len(payload.TriggerSignals) > 0 {
			parts = append(parts, "signals: "+strings.Join(uniqueStrings(payload.TriggerSignals), ", "))
		}
		if value := strings.TrimSpace(payload.ValidationProbe); value != "" {
			parts = append(parts, "validation: "+value)
		}
		return strings.Join(parts, " | ")
	}
	return formatGenericImprovementRecord(trimmed, "strategy", "why_it_worked", "task_shape")
}

func hasImprovementEvalContent(payload ImprovementEval) bool {
	return strings.TrimSpace(payload.TaskShape) != "" ||
		strings.TrimSpace(payload.Probe) != "" ||
		strings.TrimSpace(payload.SuccessCriteria) != ""
}

func hasStrategyPatternContent(payload StrategyPattern) bool {
	return strings.TrimSpace(payload.TaskShape) != "" ||
		strings.TrimSpace(payload.Strategy) != "" ||
		strings.TrimSpace(payload.WhyItWorked) != ""
}

func formatGenericImprovementRecord(raw string, primaryKeys ...string) string {
	var generic map[string]any
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return raw
	}
	parts := make([]string, 0, len(primaryKeys))
	for _, key := range primaryKeys {
		value := strings.TrimSpace(fmt.Sprint(generic[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		if key == primaryKeys[0] {
			parts = append(parts, value)
			continue
		}
		parts = append(parts, key+": "+value)
	}
	if len(parts) == 0 {
		return raw
	}
	return strings.Join(parts, " | ")
}
