package agent

import (
	"fmt"
	"sync"
	"time"
)

type runtimePhase string

const (
	runtimePhaseObserve runtimePhase = "observe"
	runtimePhaseReason  runtimePhase = "reason"
	runtimePhaseAct     runtimePhase = "act"
	runtimePhaseWait    runtimePhase = "wait"
)

type runtimeControl struct {
	mu                 sync.Mutex
	phase              runtimePhase
	toolCallsInStep    int
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
	if r.phase == runtimePhaseWait {
		return
	}
	r.phase = runtimePhaseObserve
	r.toolCallsInStep = 0
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
	if r.phase == runtimePhaseWait {
		return fmt.Errorf("execution loop violation: tool call attempted while waiting on a prior tool")
	}
	if r.toolCallsInStep >= 1 {
		return fmt.Errorf("execution loop violation: only one tool call is allowed per step")
	}
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
	if r.phase == runtimePhaseAct {
		r.phase = runtimePhaseWait
		r.lastTool = toolName
		r.lastChange = time.Now()
	}
}

func (r *runtimeControl) FinishToolExecution(_ string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phase == runtimePhaseWait || r.phase == runtimePhaseAct {
		r.phase = runtimePhaseObserve
		r.toolCallsInStep = 0
		r.lastChange = time.Now()
	}
}
