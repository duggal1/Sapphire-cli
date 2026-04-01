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
	Required               bool   `json:"required"`
	Reason                 string `json:"reason,omitempty"`
	ComplexityScore        int    `json:"complexity_score,omitempty"`
	Task                   string `json:"task,omitempty"`
	RequireBeforeDiscovery bool   `json:"require_before_discovery,omitempty"`
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

func harnessDecisionKeys(ctx context.Context) []string {
	keys := make([]string, 0, 2)
	if messageID := strings.TrimSpace(GetMessageFromContext(ctx)); messageID != "" {
		keys = append(keys, "message:"+messageID)
	}
	if sessionID := strings.TrimSpace(GetSessionFromContext(ctx)); sessionID != "" {
		keys = append(keys, "session:"+sessionID)
	}
	return keys
}

func GetHarnessRequirementFromContext(ctx context.Context) HarnessRequirement {
	return getContextValue(ctx, HarnessRequirementContextKey, HarnessRequirement{})
}

func RecordHarnessDecision(ctx context.Context, decision HarnessDecision) {
	keys := harnessDecisionKeys(ctx)
	if len(keys) == 0 {
		return
	}
	if strings.TrimSpace(decision.ToolName) == "" {
		decision.ToolName = RunHarnessToolName
	}
	harnessDecisionMu.Lock()
	defer harnessDecisionMu.Unlock()
	for _, key := range keys {
		harnessDecisionByMessage[key] = decision
	}
}

func GetHarnessDecision(ctx context.Context) (HarnessDecision, bool) {
	keys := harnessDecisionKeys(ctx)
	if len(keys) == 0 {
		return HarnessDecision{}, false
	}
	harnessDecisionMu.RLock()
	defer harnessDecisionMu.RUnlock()
	for _, key := range keys {
		if decision, ok := harnessDecisionByMessage[key]; ok {
			return decision, true
		}
	}
	return HarnessDecision{}, false
}
