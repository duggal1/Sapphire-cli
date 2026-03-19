package mailbox

import (
	"context"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

type Message = orchestrationdb.AgentMail

type SendOptions struct {
	Priority  int
	ThreadID  string
	SkipNudge bool
}

type NudgeFunc func(ctx context.Context, recipient string) error
