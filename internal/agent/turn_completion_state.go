package agent

import (
	"strings"
	"sync"
)

const (
	headlessClosureModeNormal                   = "normal"
	headlessClosureModeForcedFinalize           = "forced_finalize"
	headlessClosureModeSalvagedGroundedAnalysis = "salvaged_grounded_analysis"
	headlessClosureModeWatchdogRejected         = "watchdog_rejected"
)

type turnCompletionMetadata struct {
	ClosureMode          string
	PhaseAtInterrupt     string
	ProviderFallbackUsed bool
}

var (
	turnCompletionMetadataMu        sync.Mutex
	turnCompletionMetadataBySession = map[string]turnCompletionMetadata{}
)

func recordTurnCompletionMetadata(sessionID string, meta turnCompletionMetadata) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	meta.ClosureMode = strings.TrimSpace(meta.ClosureMode)
	meta.PhaseAtInterrupt = strings.TrimSpace(meta.PhaseAtInterrupt)
	turnCompletionMetadataMu.Lock()
	defer turnCompletionMetadataMu.Unlock()
	turnCompletionMetadataBySession[sessionID] = meta
}

func takeTurnCompletionMetadata(sessionID string) turnCompletionMetadata {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return turnCompletionMetadata{}
	}
	turnCompletionMetadataMu.Lock()
	defer turnCompletionMetadataMu.Unlock()
	meta := turnCompletionMetadataBySession[sessionID]
	delete(turnCompletionMetadataBySession, sessionID)
	return meta
}
