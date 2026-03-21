package codeindex

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultJinaBaseURL        = "https://api.jina.ai"
	defaultJinaRequestTimeout = 90 * time.Second
	maxEmbeddingRetryAttempts = 7
)

type embedder struct {
	client     *http.Client
	baseURL    string
	apiKey     string
	model      string
	dimensions int
}

type jinaEmbeddingRequest struct {
	Model         string   `json:"model"`
	Input         []string `json:"input"`
	Task          string   `json:"task,omitempty"`
	Dimensions    int      `json:"dimensions,omitempty"`
	EmbeddingType string   `json:"embedding_type,omitempty"`
	Normalized    bool     `json:"normalized"`
	Truncate      bool     `json:"truncate"`
}

type jinaEmbeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

type jinaErrorEnvelope struct {
	Detail any    `json:"detail"`
	Error  string `json:"error"`
}

func newEmbedder(apiKey, model string, dimensions int) (*embedder, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	if model == "" {
		model = DefaultEmbeddingModel
	}
	if dimensions <= 0 {
		dimensions = DefaultEmbeddingDimensions
	}
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        128,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &embedder{
		client: &http.Client{
			Timeout:   defaultJinaRequestTimeout,
			Transport: transport,
		},
		baseURL:    defaultJinaBaseURL,
		apiKey:     apiKey,
		model:      model,
		dimensions: dimensions,
	}, nil
}

func (e *embedder) Close() error {
	if e == nil || e.client == nil {
		return nil
	}
	if transport, ok := e.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
}

func (e *embedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("code index: empty query")
	}
	vectors, err := e.embedTexts(ctx, []string{text}, "nl2code.query")
	if err != nil {
		return nil, fmt.Errorf("code index: embed query: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("code index: expected 1 query embedding, got %d", len(vectors))
	}
	return vectors[0], nil
}

func (e *embedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	vectors, err := e.embedTexts(ctx, texts, "nl2code.passage")
	if err != nil {
		return nil, fmt.Errorf("code index: embed documents: %w", err)
	}
	return vectors, nil
}

func (e *embedder) embedTexts(ctx context.Context, texts []string, task string) ([][]float32, error) {
	reqBody := jinaEmbeddingRequest{
		Model:         e.model,
		Input:         texts,
		Task:          task,
		Dimensions:    e.dimensions,
		EmbeddingType: "float",
		Normalized:    true,
		Truncate:      true,
	}
	resp, err := e.embedWithRetry(ctx, reqBody)
	if err != nil {
		return nil, err
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("code index: expected %d embeddings, got %d", len(texts), len(resp.Data))
	}
	out := make([][]float32, len(texts))
	for _, item := range resp.Data {
		if item.Index < 0 || item.Index >= len(texts) {
			return nil, fmt.Errorf("code index: invalid embedding index %d", item.Index)
		}
		out[item.Index] = normalizeVector(item.Embedding)
	}
	for i, vec := range out {
		if len(vec) == 0 {
			return nil, fmt.Errorf("code index: missing embedding at index %d", i)
		}
	}
	return out, nil
}

func (e *embedder) embedWithRetry(ctx context.Context, body jinaEmbeddingRequest) (*jinaEmbeddingResponse, error) {
	delay := 350 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < maxEmbeddingRetryAttempts; attempt++ {
		resp, retryAfter, err := e.embedOnce(ctx, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !shouldRetryEmbeddingError(err) || attempt == maxEmbeddingRetryAttempts-1 {
			break
		}
		wait := delay
		if retryAfter > wait {
			wait = retryAfter
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		if delay < 5*time.Second {
			delay *= 2
		}
	}
	return nil, lastErr
}

func (e *embedder) embedOnce(ctx context.Context, body jinaEmbeddingRequest) (*jinaEmbeddingResponse, time.Duration, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("code index: marshal Jina embeddings request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("code index: create Jina embeddings request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpResp, err := e.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer httpResp.Body.Close()

	retryAfter := parseRetryAfter(httpResp.Header.Get("Retry-After"))
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(httpResp.Body, 64*1024))
		return nil, retryAfter, buildJinaHTTPError(httpResp.StatusCode, bodyBytes)
	}

	var resp jinaEmbeddingResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, 0, fmt.Errorf("code index: decode Jina embeddings response: %w", err)
	}
	return &resp, 0, nil
}

func buildJinaHTTPError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if message != "" {
		var envelope jinaErrorEnvelope
		if err := json.Unmarshal(body, &envelope); err == nil {
			if detail := strings.TrimSpace(flattenJinaDetail(envelope.Detail)); detail != "" {
				message = detail
			} else if value := strings.TrimSpace(envelope.Error); value != "" {
				message = value
			}
		}
	}
	if message == "" {
		message = http.StatusText(status)
	}
	switch status {
	case http.StatusUnauthorized:
		return fmt.Errorf("code index: Jina API key was rejected: %s", message)
	case http.StatusForbidden:
		return fmt.Errorf("code index: Jina API access denied: %s", message)
	case http.StatusTooManyRequests:
		return fmt.Errorf("code index: Jina rate limit exceeded: %s", message)
	default:
		return fmt.Errorf("code index: Jina embeddings request failed with status %d: %s", status, message)
	}
}

func flattenJinaDetail(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(flattenJinaDetail(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		parts := make([]string, 0, len(typed))
		for _, key := range []string{"message", "msg", "detail", "type"} {
			if text := strings.TrimSpace(flattenJinaDetail(typed[key])); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ": ")
	default:
		return ""
	}
}

func shouldRetryEmbeddingError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrMissingAPIKey) {
		return false
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "429") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "concurrency limit") ||
		strings.Contains(lower, "service unavailable") ||
		strings.Contains(lower, "temporarily unavailable") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "503") ||
		strings.Contains(lower, "504") {
		return true
	}
	var netErr net.Error
	return strings.Contains(lower, "connection reset") || strings.Contains(lower, "eof") || (errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()))
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := time.Until(when)
	if delay < 0 {
		return 0
	}
	return delay
}

func normalizeVector(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return vec
	}
	norm := float32(1.0 / math.Sqrt(sum))
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = v * norm
	}
	return out
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	for i, value := range a {
		dot += float64(value * b[i])
	}
	return dot
}

func encodeVector(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func decodeVector(blob []byte) ([]float32, error) {
	if len(blob)%4 != 0 {
		return nil, fmt.Errorf("code index: invalid embedding blob size")
	}
	out := make([]float32, len(blob)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return out, nil
}
