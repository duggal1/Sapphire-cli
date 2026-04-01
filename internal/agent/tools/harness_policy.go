package tools

import (
	"context"
	"strings"
	"sync"
)

type harnessRequirementContextKey string

const (
	HarnessRequirementContextKey harnessRequirementContextKey = "harness_requirement"
	RunHarnessToolName                                        = "run_harness"
)

type HarnessRequirement struct {
	Required        bool   `json:"required"`
	Reason          string `json:"reason,omitempty"`
	ComplexityScore int    `json:"complexity_score,omitempty"`
	Task            string `json:"task,omitempty"`
}

type HarnessDecision struct {
	ToolName        string `json:"tool_name,omitempty"`
	Required        bool   `json:"required"`
	ComplexityScore int    `json:"complexity_score,omitempty"`
	Pattern         string `json:"pattern,omitempty"`
}

var (
	harnessDecisionMu        sync.RWMutex
	harnessDecisionByMessage = map[string]HarnessDecision{}
)

func GetHarnessRequirementFromContext(ctx context.Context) HarnessRequirement {
	return getContextValue(ctx, HarnessRequirementContextKey, HarnessRequirement{})
}

func RecordHarnessDecision(ctx context.Context, decision HarnessDecision) {
	messageID := strings.TrimSpace(GetMessageFromContext(ctx))
	if messageID == "" {
		return
	}
	if strings.TrimSpace(decision.ToolName) == "" {
		decision.ToolName = RunHarnessToolName
	}
	harnessDecisionMu.Lock()
	defer harnessDecisionMu.Unlock()
	harnessDecisionByMessage[messageID] = decision
}

func GetHarnessDecision(ctx context.Context) (HarnessDecision, bool) {
	messageID := strings.TrimSpace(GetMessageFromContext(ctx))
	if messageID == "" {
		return HarnessDecision{}, false
	}
	harnessDecisionMu.RLock()
	defer harnessDecisionMu.RUnlock()
	decision, ok := harnessDecisionByMessage[messageID]
	return decision, ok
}
