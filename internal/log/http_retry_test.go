package log

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRetryRoundTripperRetriesConnectionReset(t *testing.T) {
	attempts := 0
	rt := &RetryRoundTripper{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("read: connection reset by peer")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
		MaxRetries: 1,
		ShouldRetry: func(_ *http.Request, _ *http.Response, err error) bool {
			return err != nil && strings.Contains(strings.ToLower(err.Error()), "connection reset by peer")
		},
	}

	req, err := http.NewRequest(http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryRoundTripperDoesNotRetryWithoutMatch(t *testing.T) {
	attempts := 0
	rt := &RetryRoundTripper{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return nil, errors.New("boom")
		}),
		MaxRetries: 1,
		ShouldRetry: func(_ *http.Request, _ *http.Response, err error) bool {
			return err != nil && strings.Contains(strings.ToLower(err.Error()), "connection reset by peer")
		},
	}

	req, err := http.NewRequest(http.MethodGet, "https://openrouter.ai/api/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}
