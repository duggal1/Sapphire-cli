package memories

import "context"

type Phase1Output struct {
	RawMemory      string `json:"raw_memory"`
	RolloutSummary string `json:"rollout_summary"`
	RolloutSlug    string `json:"rollout_slug,omitempty"`
}

type Phase1Invocation struct {
	SystemPrompt string
	UserPrompt   string
}

type Phase2Invocation struct {
	Prompt          string
	Root            string
	ParentSessionID string
}

type PhaseRunners struct {
	Phase1 func(ctx context.Context, invocation Phase1Invocation) (Phase1Output, error)
	Phase2 func(ctx context.Context, invocation Phase2Invocation) error
}

func (s *Service) SetRunners(runners PhaseRunners) {
	if s == nil {
		return
	}
	s.runnersMu.Lock()
	defer s.runnersMu.Unlock()
	s.runners = runners
}

func (s *Service) getPhase1Runner() func(ctx context.Context, invocation Phase1Invocation) (Phase1Output, error) {
	if s == nil {
		return nil
	}
	s.runnersMu.Lock()
	defer s.runnersMu.Unlock()
	return s.runners.Phase1
}

func (s *Service) getPhase2Runner() func(ctx context.Context, invocation Phase2Invocation) error {
	if s == nil {
		return nil
	}
	s.runnersMu.Lock()
	defer s.runnersMu.Unlock()
	return s.runners.Phase2
}
