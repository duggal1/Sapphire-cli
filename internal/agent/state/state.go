package state

import (
	"context"
	"fmt"
	"time"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

type Snapshot = orchestrationdb.AgentState

type Service struct {
	store *orchestrationdb.Store
}

func NewService(store *orchestrationdb.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Register(ctx context.Context, snapshot Snapshot) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("agent state service is not initialized")
	}
	return s.store.UpsertAgentState(ctx, snapshot)
}

func (s *Service) Heartbeat(ctx context.Context, agentID string, updated Snapshot) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("agent state service is not initialized")
	}
	if updated.AgentID == "" {
		updated.AgentID = agentID
	}
	updated.LastHeartbeat = time.Now().UTC()
	if updated.UpdatedAt.IsZero() {
		updated.UpdatedAt = updated.LastHeartbeat
	}
	return s.store.UpsertAgentState(ctx, updated)
}

func (s *Service) Status(ctx context.Context, agentID string) (Snapshot, error) {
	if s == nil || s.store == nil {
		return Snapshot{}, fmt.Errorf("agent state service is not initialized")
	}
	return s.store.GetAgentState(ctx, agentID)
}

func (s *Service) List(ctx context.Context, limit int) ([]Snapshot, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("agent state service is not initialized")
	}
	return s.store.ListAgentStates(ctx, limit)
}

func (s *Service) ListByParent(ctx context.Context, parentAgentID string, limit int) ([]Snapshot, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("agent state service is not initialized")
	}
	return s.store.ListAgentStatesByParent(ctx, parentAgentID, limit)
}

func (s *Service) ListBySession(ctx context.Context, sessionID string, limit int) ([]Snapshot, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("agent state service is not initialized")
	}
	return s.store.ListAgentStatesBySession(ctx, sessionID, limit)
}

func (s *Service) ListStale(ctx context.Context, staleBefore time.Time, limit int) ([]Snapshot, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("agent state service is not initialized")
	}
	return s.store.ListStaleAgentStates(ctx, staleBefore, limit)
}
