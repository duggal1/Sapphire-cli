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
		Address:         firstNonEmptyAddress(opts.Address, to),
		ToAgent:         to,
		ResolvedToAgent: to,
		FromAgent:       from,
		Subject:         subject,
		Body:            body,
		Priority:        opts.Priority,
		ThreadID:        opts.ThreadID,
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

func (s *Service) Actionable(ctx context.Context, agentID string, limit int) ([]Message, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("mailbox service is not initialized")
	}
	return s.store.ListActionableMail(ctx, agentID, limit)
}

func (s *Service) LeaseInbox(ctx context.Context, agentID, leaseOwner string, limit int, leaseTTL time.Duration) ([]Message, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("mailbox service is not initialized")
	}
	if leaseTTL <= 0 {
		leaseTTL = DefaultLeaseTTL
	}
	return s.store.LeaseInbox(ctx, agentID, leaseOwner, limit, leaseTTL)
}

func (s *Service) Ack(ctx context.Context, agentID, messageID string) (Message, error) {
	if s == nil || s.store == nil {
		return Message{}, fmt.Errorf("mailbox service is not initialized")
	}
	return s.store.AckMail(ctx, agentID, messageID)
}

func (s *Service) RequeueExpiredLeases(ctx context.Context, maxAttempts int) ([]Message, []Message, error) {
	if s == nil || s.store == nil {
		return nil, nil, fmt.Errorf("mailbox service is not initialized")
	}
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxLeaseAttempts
	}
	return s.store.RequeueExpiredMailLeases(ctx, maxAttempts)
}

func (s *Service) Thread(ctx context.Context, agentID, threadID string, limit int) ([]Message, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("mailbox service is not initialized")
	}
	return s.store.Thread(ctx, agentID, threadID, limit)
}

func firstNonEmptyAddress(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
