package codeindex

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"time"
)

type embedder struct {
	client     *http.Client
	runtime    *ollamaRuntime
	model      string
	dimensions int
}

func newEmbedder(apiKey, model string, dimensions int, dataDir, ollamaURL string) (*embedder, error) {
	_ = apiKey
	if model == "" {
		model = DefaultEmbeddingModel
	}
	if dimensions <= 0 {
		dimensions = DefaultEmbeddingDimensions
	}
	baseURL := normalizeOllamaURL(ollamaURL)
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 32,
		MaxConnsPerHost:     32,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}
	return &embedder{
		client:     &http.Client{Timeout: 2 * 60 * time.Second, Transport: transport},
		runtime:    newOllamaRuntime(baseURL, filepath.Join(dataDir, "vectordb")),
		model:      model,
		dimensions: dimensions,
	}, nil
}

func (e *embedder) Close() error {
	if e == nil || e.runtime == nil {
		return nil
	}
	return e.runtime.Close()
}

func (e *embedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("code index: empty query")
	}
	vectors, err := e.embed(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("code index: embed query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("code index: empty query embedding response")
	}
	return vectors[0], nil
}

func (e *embedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	vectors, err := e.embed(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("code index: embed documents: %w", err)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("code index: expected %d embeddings, got %d", len(texts), len(vectors))
	}
	return vectors, nil
}

func (e *embedder) embed(ctx context.Context, texts []string) ([][]float32, error) {
	if err := e.runtime.EnsureReady(ctx); err != nil {
		return nil, err
	}
	if err := e.runtime.EnsureModel(ctx, e.model); err != nil {
		return nil, err
	}
	body := map[string]any{
		"model":    e.model,
		"input":    texts,
		"truncate": true,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.runtime.baseURL+"/api/embed", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("code index: ollama embed failed with status %d: %s", resp.StatusCode, string(respBody))
	}
	var payload struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("code index: decode ollama embed response: %w", err)
	}
	if len(payload.Embeddings) == 0 {
		return nil, fmt.Errorf("code index: empty ollama embedding response")
	}
	out := make([][]float32, len(payload.Embeddings))
	for i, embedding := range payload.Embeddings {
		if e.dimensions > 0 {
			if len(embedding) < e.dimensions {
				return nil, fmt.Errorf("code index: embedding dimension %d is smaller than configured %d", len(embedding), e.dimensions)
			}
			embedding = embedding[:e.dimensions]
		}
		out[i] = normalizeVector(embedding)
	}
	return out, nil
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
