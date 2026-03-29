package mailbox

import (
	"context"
	"time"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

type Message = orchestrationdb.AgentMail

const (
	DefaultLeaseTTL         = 2 * time.Minute
	DefaultMaxLeaseAttempts = 3
)

type SendOptions struct {
	Address   string
	Priority  int
	ThreadID  string
	SkipNudge bool
}

type NudgeFunc func(ctx context.Context, recipient string) error
