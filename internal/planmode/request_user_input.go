package planmode

import (
	"context"
	"errors"
	"sync"

	"github.com/charmbracelet/sapphire/internal/pubsub"
)

type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type Question struct {
	ID       string           `json:"id"`
	Header   string           `json:"header"`
	Question string           `json:"question"`
	Options  []QuestionOption `json:"options"`
}

type Request struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	Questions []Question `json:"questions"`
}

type Answer struct {
	Answers []string `json:"answers"`
}

type Response struct {
	Answers map[string]Answer `json:"answers"`
}

var ErrRequestCancelled = errors.New("request_user_input was cancelled before receiving a response")

type pendingRequest struct {
	ch chan Response
}

var (
	requestBroker = pubsub.NewBroker[Request]()

	requestsMu sync.Mutex
	requests   = map[string]pendingRequest{}
)

func SubscribeRequests(ctx context.Context) <-chan pubsub.Event[Request] {
	return requestBroker.Subscribe(ctx)
}

func RequestInput(ctx context.Context, req Request) (Response, error) {
	ch := make(chan Response, 1)

	requestsMu.Lock()
	requests[req.ID] = pendingRequest{ch: ch}
	requestsMu.Unlock()

	defer func() {
		requestsMu.Lock()
		delete(requests, req.ID)
		requestsMu.Unlock()
	}()

	requestBroker.Publish(pubsub.CreatedEvent, req)

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}
}

func Respond(id string, resp Response) bool {
	requestsMu.Lock()
	req, ok := requests[id]
	requestsMu.Unlock()
	if !ok {
		return false
	}

	select {
	case req.ch <- resp:
		return true
	default:
		return false
	}
}

func Cancel(id string) bool {
	return Respond(id, Response{})
}
