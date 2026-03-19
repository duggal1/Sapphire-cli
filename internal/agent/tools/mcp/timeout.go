package mcp

import (
	"context"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/config"
)

func withMCPTimeout(ctx context.Context, cfg *config.Config, name string) (context.Context, context.CancelFunc) {
	if cfg != nil {
		if m, ok := cfg.MCP[name]; ok {
			return context.WithTimeout(ctx, mcpTimeout(m))
		}
	}
	return context.WithTimeout(ctx, 15*time.Second)
}
