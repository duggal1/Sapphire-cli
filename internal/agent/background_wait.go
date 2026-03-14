package agent

import (
	"context"

	"github.com/charmbracelet/sapphire/internal/agent/tools"
)

func (c *coordinator) waitForBackgroundWork(ctx context.Context, sessionID string) error {
	c.pollBackgroundSubAgents(sessionID)
	tools.PollBackgroundShells(sessionID)
	return nil
}
