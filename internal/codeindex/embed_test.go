package codeindex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"testing"
)

func TestNewEmbedderRequiresAPIKey(t *testing.T) {
	_, err := newEmbedder("", DefaultEmbeddingModel, DefaultEmbeddingDimensions)
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("expected ErrMissingAPIKey, got %v", err)
	}
}

func TestEmbedDocumentsUsesJinaPassageTask(t *testing.T) {
	type requestBody struct {
		Model         string   `json:"model"`
		Input         []string `json:"input"`
		Task          string   `json:"task"`
		Dimensions    int      `json:"dimensions"`
		EmbeddingType string   `json:"embedding_type"`
		Normalized    bool     `json:"normalized"`
		Truncate      bool     `json:"truncate"`
	}

	var captured requestBody
	embedder, err := newEmbedder("jina_test_key", DefaultEmbeddingModel, 1024)
	if err != nil {
		t.Fatalf("newEmbedder: %v", err)
	}
	embedder.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/embeddings" {
				t.Fatalf("unexpected path %q", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer jina_test_key" {
				t.Fatalf("unexpected auth header %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"data": []map[string]any{
					{"index": 1, "embedding": []float32{0, 6}},
					{"index": 0, "embedding": []float32{3, 4}},
				},
			}), nil
		}),
	}

	vectors, err := embedder.EmbedDocuments(context.Background(), []string{"alpha", "beta"})
	if err != nil {
		t.Fatalf("EmbedDocuments: %v", err)
	}

	if captured.Model != DefaultEmbeddingModel {
		t.Fatalf("unexpected model %q", captured.Model)
	}
	if captured.Task != "nl2code.passage" {
		t.Fatalf("unexpected task %q", captured.Task)
	}
	if captured.Dimensions != 1024 {
		t.Fatalf("unexpected dimensions %d", captured.Dimensions)
	}
	if captured.EmbeddingType != "float" || !captured.Normalized || !captured.Truncate {
		t.Fatalf("unexpected request flags: %+v", captured)
	}
	if len(vectors) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vectors))
	}
	assertApproxVector(t, vectors[0], []float32{0.6, 0.8})
	assertApproxVector(t, vectors[1], []float32{0, 1})
}

func TestEmbedQueryUsesJinaQueryTask(t *testing.T) {
	type requestBody struct {
		Task string `json:"task"`
	}
	var captured requestBody
	embedder, err := newEmbedder("jina_test_key", DefaultEmbeddingModel, DefaultEmbeddingDimensions)
	if err != nil {
		t.Fatalf("newEmbedder: %v", err)
	}
	embedder.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"data": []map[string]any{
					{"index": 0, "embedding": []float32{5, 0}},
				},
			}), nil
		}),
	}

	vector, err := embedder.EmbedQuery(context.Background(), "find retry logic")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if captured.Task != "nl2code.query" {
		t.Fatalf("unexpected task %q", captured.Task)
	}
	assertApproxVector(t, vector, []float32{1, 0})
}

func assertApproxVector(t *testing.T, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("vector length mismatch: got %d want %d", len(got), len(want))
	}
	for i := range got {
		if math.Abs(float64(got[i]-want[i])) > 0.0001 {
			t.Fatalf("vector[%d] mismatch: got %.6f want %.6f", i, got[i], want[i])
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, payload any) *http.Response {
	body, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}
