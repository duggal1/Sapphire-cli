package agent

import "errors"

var (
	ErrRequestCancelled                = errors.New("request canceled by user")
	ErrSessionBusy                     = errors.New("session is currently processing another request")
	ErrEmptyPrompt                     = errors.New("prompt is empty")
	ErrSessionMissing                  = errors.New("session id is missing")
	ErrToolFailureLimitReached         = errors.New("maximum tool failures per turn reached")
	ErrStepLimitReached                = errors.New("maximum steps per turn reached")
	ErrRepeatedToolLoopDetected        = errors.New("repeated tool-call loop detected")
	ErrDeterministicDoomLoop           = errors.New("deterministic doom loop detected")
	ErrHeadlessCompletionBudgetReached = errors.New("headless completion budget reached")
)
