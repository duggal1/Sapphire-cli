package codeindex

import (
	"context"

	"github.com/duggal1/Sapphire-cli/internal/pubsub"
)

var progressBroker = pubsub.NewBroker[Progress]()

func SubscribeEvents(ctx context.Context) <-chan pubsub.Event[Progress] {
	return progressBroker.Subscribe(ctx)
}

func publishProgress(eventType pubsub.EventType, progress Progress) {
	progressBroker.Publish(eventType, progress)
}

