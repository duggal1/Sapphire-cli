package codeindex

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"google.golang.org/genai"
)

type embedder struct {
	client     *genai.Client
	model      string
	dimensions int
}

func newEmbedder(apiKey, model string, dimensions int) (*embedder, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("code index: Gemini API key is required for embeddings")
	}
	if model == "" {
		model = DefaultEmbeddingModel
	}
	if dimensions <= 0 {
		dimensions = DefaultEmbeddingDimensions
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("code index: create Gemini client: %w", err)
	}
	return &embedder{
		client:     client,
		model:      model,
		dimensions: dimensions,
	}, nil
}

func (e *embedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("code index: empty query")
	}
	resp, err := e.client.Models.EmbedContent(ctx, e.model, []*genai.Content{
		genai.NewContentFromText(text, genai.RoleUser),
	}, &genai.EmbedContentConfig{
		TaskType:             "CODE_RETRIEVAL_QUERY",
		OutputDimensionality: int32Ptr(e.dimensions),
	})
	if err != nil {
		return nil, fmt.Errorf("code index: embed query: %w", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("code index: empty query embedding response")
	}
	return normalizeVector(resp.Embeddings[0].Values), nil
}

func (e *embedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	contents := make([]*genai.Content, 0, len(texts))
	for _, text := range texts {
		contents = append(contents, genai.NewContentFromText(text, genai.RoleUser))
	}
	resp, err := e.client.Models.EmbedContent(ctx, e.model, contents, &genai.EmbedContentConfig{
		TaskType:             "RETRIEVAL_DOCUMENT",
		OutputDimensionality: int32Ptr(e.dimensions),
	})
	if err != nil {
		return nil, fmt.Errorf("code index: embed documents: %w", err)
	}
	if len(resp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("code index: expected %d embeddings, got %d", len(texts), len(resp.Embeddings))
	}
	out := make([][]float32, len(resp.Embeddings))
	for i, embedding := range resp.Embeddings {
		out[i] = normalizeVector(embedding.Values)
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

func int32Ptr(value int) *int32 {
	v := int32(value)
	return &v
}

