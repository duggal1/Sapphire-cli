package activity

import (
	"context"
	"fmt"
	"time"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

type Entry = orchestrationdb.AgentActivity

type Service struct {
	store *orchestrationdb.Store
}

func NewService(store *orchestrationdb.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Log(ctx context.Context, agentID string, eventType EventType, detailsJSON string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("agent activity service is not initialized")
	}
	return s.store.RecordActivity(ctx, orchestrationdb.AgentActivity{
		AgentID:     agentID,
		EventType:   string(eventType),
		DetailsJSON: detailsJSON,
		CreatedAt:   time.Now().UTC(),
	})
}

func (s *Service) Feed(ctx context.Context, agentIDs []string, limit int) ([]Entry, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("agent activity service is not initialized")
	}
	return s.store.ListActivityFeed(ctx, agentIDs, limit)
}

func (s *Service) Recent(ctx context.Context, agentID string, limit int) ([]Entry, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("agent activity service is not initialized")
	}
	return s.store.ListRecentActivity(ctx, agentID, limit)
}
