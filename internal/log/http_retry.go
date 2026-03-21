package log

import (
	"bytes"
	"io"
	"net/http"
)

// RetryRoundTripper retries one transport-level failure for retriable request classes.
type RetryRoundTripper struct {
	Transport   http.RoundTripper
	ShouldRetry func(*http.Request, *http.Response, error) bool
	MaxRetries  int
}

func (r *RetryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, nil
	}

	transport := r.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	maxRetries := r.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	currentReq, err := cloneRequestForRetry(req)
	if err != nil {
		return nil, err
	}

	resp, err := transport.RoundTrip(currentReq)
	for attempt := 0; attempt < maxRetries && r.shouldRetry(currentReq, resp, err); attempt++ {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		currentReq, err = cloneRequestForRetry(req)
		if err != nil {
			return nil, err
		}
		resp, err = transport.RoundTrip(currentReq)
	}
	return resp, err
}

func (r *RetryRoundTripper) shouldRetry(req *http.Request, resp *http.Response, err error) bool {
	if r == nil || r.ShouldRetry == nil {
		return false
	}
	return r.ShouldRetry(req, resp, err)
}

func cloneRequestForRetry(req *http.Request) (*http.Request, error) {
	cloned := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		cloned.Body = http.NoBody
		return cloned, nil
	}
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		cloned.Body = body
		return cloned, nil
	}
	body1, body2, err := cloneBody(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = body1
	cloned.Body = body2
	return cloned, nil
}

func cloneBody(body io.ReadCloser) (io.ReadCloser, io.ReadCloser, error) {
	if body == nil || body == http.NoBody {
		return http.NoBody, http.NoBody, nil
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, nil, err
	}
	if err := body.Close(); err != nil {
		return nil, nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), io.NopCloser(bytes.NewReader(data)), nil
}
