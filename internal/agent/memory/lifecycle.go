package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/db"
)

const defaultContextTokenThreshold = 240000

// RolloverController manages long-horizon session rollover and resume logic.
// This is a thin compatibility wrapper over the durable memory runtime.
type RolloverController struct {
	q *db.Queries
}

func NewRolloverController(q *db.Queries) *RolloverController {
	return &RolloverController{q: q}
}

// DetectContextLimit checks if the active session is nearing the context threshold.
func (rc *RolloverController) DetectContextLimit(ctx context.Context, sessionID string) (bool, error) {
	if rc == nil || rc.q == nil || strings.TrimSpace(sessionID) == "" {
		return false, nil
	}
	item, err := rc.q.GetSessionByID(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return false, err
	}
	return item.PromptTokens+item.CompletionTokens >= defaultContextTokenThreshold, nil
}

// RolloverState captures the structured runtime state used when a caller wants
// to persist a handoff before creating a fresh model instance.
type RolloverState struct {
	CurrentTask      string
	CurrentPlan      []string
	Blockers         []string
	Uncertainties    []string
	TouchedFiles     []string
	TouchedSymbols   []string
	ValidationStatus map[string]any
}

// EmitHandoff exists for older call sites. The real durable handoff flow is
// implemented through Compiler.CreateResumePoint and persistHandoffPacket.
func (rc *RolloverController) EmitHandoff(ctx context.Context, sessionID string, state RolloverState) error {
	if rc == nil || rc.q == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	_, err := rc.q.GetSessionByID(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return err
	}
	_ = state
	return nil
}

// ResumeBootloader reconstructs the boot packet from durable memory.
func (rc *RolloverController) ResumeBootloader(ctx context.Context, newSessionID string, oldSessionID string) (*BootPacket, error) {
	_ = newSessionID
	_ = oldSessionID
	return nil, fmt.Errorf("resume bootloader has moved to Compiler.CreateResumePoint and RenderResumePointInjection")
}
