package agent

import (
	"sync"
	"time"
)

type runtimePhase string

const (
	runtimePhaseObserve runtimePhase = "observe"
	runtimePhaseReason  runtimePhase = "reason"
	runtimePhaseAct     runtimePhase = "act"
)

type runtimeControl struct {
	mu                 sync.Mutex
	phase              runtimePhase
	toolCallsInStep    int
	activeExecutions   int
	lastTool           string
	lastChange         time.Time
	mistakeSelfHealing *mistakeSelfHealingMonitor
}

func newRuntimeControl(selfHealingMode bool) *runtimeControl {
	return &runtimeControl{
		phase:              runtimePhaseObserve,
		lastChange:         time.Now(),
		mistakeSelfHealing: newMistakeSelfHealingMonitor(selfHealingMode),
	}
}

func (r *runtimeControl) Observe() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phase = runtimePhaseObserve
	r.toolCallsInStep = 0
	r.lastChange = time.Now()
}

func (r *runtimeControl) ObserveAfterStep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phase = runtimePhaseObserve
	r.toolCallsInStep = 0
	r.activeExecutions = 0
	r.lastChange = time.Now()
}

func (r *runtimeControl) NoteReasoning() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phase == runtimePhaseObserve {
		r.phase = runtimePhaseReason
		r.lastChange = time.Now()
	}
}

func (r *runtimeControl) AllowToolCall(toolName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phase == runtimePhaseObserve {
		r.phase = runtimePhaseReason
	}
	r.phase = runtimePhaseAct
	r.toolCallsInStep++
	r.lastTool = toolName
	r.lastChange = time.Now()
	return nil
}

func (r *runtimeControl) BeginToolExecution(toolName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phase = runtimePhaseAct
	r.activeExecutions++
	r.lastTool = toolName
	r.lastChange = time.Now()
}

func (r *runtimeControl) FinishToolExecution(_ string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activeExecutions > 0 {
		r.activeExecutions--
	}
	if r.phase == runtimePhaseAct && r.activeExecutions == 0 {
		r.phase = runtimePhaseObserve
		r.toolCallsInStep = 0
		r.lastChange = time.Now()
	}
}
