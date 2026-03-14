package memory

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"google.golang.org/genai"
)

const (
	// DefaultEmbeddingModel is a lightweight Gemini embedding model.
	DefaultEmbeddingModel = "gemini-embedding-001"
	// DefaultEmbeddingDimensions balances quality and performance.
	DefaultEmbeddingDimensions = 768
)

// Embedder provides semantic embeddings for memory retrieval.
type Embedder interface {
	Name() string
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

// GeminiEmbedder uses the Gemini embedding API.
type GeminiEmbedder struct {
	client     *genai.Client
	model      string
	dimensions int
}

// NewGeminiEmbedder creates a Gemini embedding client.
func NewGeminiEmbedder(apiKey, model string, dimensions int) (*GeminiEmbedder, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("memory: embedding model requires API key")
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
		return nil, fmt.Errorf("memory: create embedding client: %w", err)
	}
	return &GeminiEmbedder{
		client:     client,
		model:      model,
		dimensions: dimensions,
	}, nil
}

func (g *GeminiEmbedder) Name() string { return "gemini-embedder" }

func (g *GeminiEmbedder) Dimensions() int { return g.dimensions }

func (g *GeminiEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("memory: empty query for embedding")
	}
	contents := []*genai.Content{
		{Parts: []*genai.Part{genai.NewPartFromText(text)}},
	}
	result, err := g.client.Models.EmbedContent(ctx, g.model, contents, &genai.EmbedContentConfig{
		TaskType:             "RETRIEVAL_QUERY",
		OutputDimensionality: int32Ptr(g.dimensions),
	})
	if err != nil {
		return nil, fmt.Errorf("memory: embed query failed: %w", err)
	}
	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("memory: empty embedding response")
	}
	vec := normalizeVector(result.Embeddings[0].Values)
	return vec, nil
}

func (g *GeminiEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = genai.NewContentFromText(text, genai.RoleUser)
	}
	result, err := g.client.Models.EmbedContent(ctx, g.model, contents, &genai.EmbedContentConfig{
		TaskType:             "RETRIEVAL_DOCUMENT",
		OutputDimensionality: int32Ptr(g.dimensions),
	})
	if err != nil {
		return nil, fmt.Errorf("memory: embed documents failed: %w", err)
	}
	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("memory: expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}
	vectors := make([][]float32, len(texts))
	for i, emb := range result.Embeddings {
		vectors[i] = normalizeVector(emb.Values)
	}
	return vectors, nil
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
	for i, v := range a {
		dot += float64(v * b[i])
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
		return nil, fmt.Errorf("memory: invalid embedding blob size")
	}
	n := len(blob) / 4
	vec := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(blob[i*4:])
		vec[i] = math.Float32frombits(bits)
	}
	return vec, nil
}

func int32Ptr(v int) *int32 {
	val := int32(v)
	return &val
}
