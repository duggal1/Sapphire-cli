package codeindex

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEmbedChunkRefBatchSplitsAroundRejectedChunk(t *testing.T) {
	var requests atomic.Int32

	embedder, err := newEmbedder("jina_test_key", DefaultEmbeddingModel, DefaultEmbeddingDimensions)
	if err != nil {
		t.Fatalf("newEmbedder: %v", err)
	}
	embedder.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests.Add(1)

			var body jinaEmbeddingRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			hasRejected := false
			for _, text := range body.Input {
				if strings.Contains(text, "bad chunk") {
					hasRejected = true
					break
				}
			}
			if hasRejected {
				return jsonResponse(http.StatusBadRequest, map[string]any{
					"detail": "failed to encode text",
				}), nil
			}

			data := make([]map[string]any, 0, len(body.Input))
			for i := range body.Input {
				data = append(data, map[string]any{
					"index":     i,
					"embedding": []float32{1, 0},
				})
			}
			return jsonResponse(http.StatusOK, map[string]any{"data": data}), nil
		}),
	}

	service := &Service{embedder: embedder}
	files := []indexedFile{
		{
			Path: "a.go",
			Chunks: []indexedChunk{
				{Path: "a.go", SearchText: "good chunk 1"},
				{Path: "a.go", SearchText: "good chunk 2"},
				{Path: "a.go", SearchText: "bad chunk"},
				{Path: "a.go", SearchText: "good chunk 3"},
			},
		},
	}
	batch := flattenChunkRefs(files)
	progress := &Progress{ChunksTotal: len(batch)}
	var embedded atomic.Int64

	if err := service.embedChunkRefBatch(context.Background(), files, batch, progress, &embedded); err != nil {
		t.Fatalf("embedChunkRefBatch: %v", err)
	}

	if got := embedded.Load(); got != 3 {
		t.Fatalf("embedded count = %d, want 3", got)
	}
	if files[0].Chunks[2].Embedding != nil {
		t.Fatalf("rejected chunk should not have an embedding")
	}
	for _, idx := range []int{0, 1, 3} {
		if len(files[0].Chunks[idx].Embedding) == 0 {
			t.Fatalf("chunk %d missing embedding", idx)
		}
	}
	if got := requests.Load(); got != 5 {
		t.Fatalf("request count = %d, want 5", got)
	}
}
