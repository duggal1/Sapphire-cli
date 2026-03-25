package mailbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

type Service struct {
	store *orchestrationdb.Store
	nudge NudgeFunc

	mu            sync.RWMutex
	nudgeFailures map[string]int
	nudgeLastFail map[string]time.Time
}

func NewService(store *orchestrationdb.Store, nudge NudgeFunc) *Service {
	return &Service{
		store:         store,
		nudge:         nudge,
		nudgeFailures: make(map[string]int),
		nudgeLastFail: make(map[string]time.Time),
	}
}

func (s *Service) Send(ctx context.Context, to, from, subject, body string, opts SendOptions) (Message, error) {
	if s == nil || s.store == nil {
		return Message{}, fmt.Errorf("mailbox service is not initialized")
	}
	msg, err := s.store.SendMail(ctx, orchestrationdb.AgentMail{
		ToAgent:   to,
		FromAgent: from,
		Subject:   subject,
		Body:      body,
		Priority:  opts.Priority,
		ThreadID:  opts.ThreadID,
	})
	if err != nil {
		return Message{}, err
	}
	if !opts.SkipNudge && to != "" && to != from {
		_ = s.Nudge(ctx, to)
	}
	return msg, nil
}

func (s *Service) Inbox(ctx context.Context, agentID string, unreadOnly bool, limit int) ([]Message, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("mailbox service is not initialized")
	}
	return s.store.ListInbox(ctx, agentID, unreadOnly, limit)
}

func (s *Service) MarkRead(ctx context.Context, agentID, messageID string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("mailbox service is not initialized")
	}
	return s.store.MarkRead(ctx, agentID, messageID)
}

func (s *Service) Thread(ctx context.Context, agentID, threadID string, limit int) ([]Message, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("mailbox service is not initialized")
	}
	return s.store.Thread(ctx, agentID, threadID, limit)
}
