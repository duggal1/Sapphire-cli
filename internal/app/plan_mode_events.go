package app

import (
	"context"

	"github.com/charmbracelet/sapphire/internal/planmode"
	"github.com/charmbracelet/sapphire/internal/pubsub"
)

func SubscribePlanModeRequests(ctx context.Context) <-chan pubsub.Event[planmode.Request] {
	return planmode.SubscribeRequests(ctx)
}
