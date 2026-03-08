package skills

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"

	"google.golang.org/genai"
)

const (
	// EmbeddingModel is the Gemini model used for skill retrieval.
	EmbeddingModel = "gemini-embedding-001"

	// EmbeddingDimensions controls the output vector size.
	// 768 offers a strong quality/performance tradeoff per Google's MRL benchmarks.
	EmbeddingDimensions = 768

	// DefaultSimilarityThreshold is the minimum cosine similarity score
	// for a skill to be considered relevant to a user prompt.
	DefaultSimilarityThreshold = 0.45
)

// SkillEmbedding pairs a discovered skill with its embedding vector.
type SkillEmbedding struct {
	Skill  *Skill
	Vector []float32
}

// EmbeddingService manages skill embeddings and performs similarity-based retrieval.
// It is safe for concurrent use.
type EmbeddingService struct {
	apiKey    string
	threshold float64

	mu              sync.Mutex
	skillEmbeddings []SkillEmbedding
	initialized     bool
}

// NewEmbeddingService creates a new embedding service.
// apiKey is the Google/Gemini API key.
// If threshold <= 0, DefaultSimilarityThreshold is used.
func NewEmbeddingService(apiKey string, threshold float64) *EmbeddingService {
	if threshold <= 0 {
		threshold = DefaultSimilarityThreshold
	}
	return &EmbeddingService{
		apiKey:    apiKey,
		threshold: threshold,
	}
}

// EmbedSkills computes and caches embedding vectors for all provided skills.
// This is idempotent — it only embeds once. Subsequent calls are no-ops.
func (s *EmbeddingService) EmbedSkills(ctx context.Context, skills []*Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	if len(skills) == 0 {
		s.initialized = true
		return nil
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  s.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("skills/embedding: failed to create genai client: %w", err)
	}

	// Build embedding text for each skill: name + description + instructions summary.
	// Each text gets its own Content so we get one embedding per skill.
	contents := make([]*genai.Content, len(skills))
	for i, sk := range skills {
		contents[i] = genai.NewContentFromText(buildSkillEmbeddingText(sk), genai.RoleUser)
	}

	result, err := client.Models.EmbedContent(ctx, EmbeddingModel, contents, &genai.EmbedContentConfig{
		TaskType:            "RETRIEVAL_DOCUMENT",
		OutputDimensionality: int32Ptr(EmbeddingDimensions),
	})
	if err != nil {
		return fmt.Errorf("skills/embedding: failed to embed skills: %w", err)
	}

	if len(result.Embeddings) != len(skills) {
		return fmt.Errorf("skills/embedding: expected %d embeddings, got %d", len(skills), len(result.Embeddings))
	}

	s.skillEmbeddings = make([]SkillEmbedding, len(skills))
	for i, sk := range skills {
		vec := normalize(result.Embeddings[i].Values)
		s.skillEmbeddings[i] = SkillEmbedding{
			Skill:  sk,
			Vector: vec,
		}
	}

	s.initialized = true
	slog.Debug("Embedded all skills for retrieval", "count", len(skills))
	return nil
}

// FindRelevantSkills embeds the user prompt and returns skills that exceed
// the similarity threshold, sorted by relevance (highest first).
func (s *EmbeddingService) FindRelevantSkills(ctx context.Context, userPrompt string) ([]*Skill, error) {
	s.mu.Lock()
	embeddings := s.skillEmbeddings
	initialized := s.initialized
	s.mu.Unlock()

	if !initialized || len(embeddings) == 0 {
		return nil, nil
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  s.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("skills/embedding: failed to create genai client: %w", err)
	}

	contents := []*genai.Content{
		{Parts: []*genai.Part{genai.NewPartFromText(userPrompt)}},
	}

	result, err := client.Models.EmbedContent(ctx, EmbeddingModel, contents, &genai.EmbedContentConfig{
		TaskType:            "RETRIEVAL_QUERY",
		OutputDimensionality: int32Ptr(EmbeddingDimensions),
	})
	if err != nil {
		return nil, fmt.Errorf("skills/embedding: failed to embed user prompt: %w", err)
	}

	if len(result.Embeddings) == 0 {
		return nil, nil
	}

	queryVec := normalize(result.Embeddings[0].Values)

	type scoredSkill struct {
		skill *Skill
		score float64
	}

	var matched []scoredSkill
	for _, se := range embeddings {
		sim := cosineSimilarity(queryVec, se.Vector)
		slog.Debug("Skill similarity score", "skill", se.Skill.Name, "score", fmt.Sprintf("%.4f", sim))
		if sim >= s.threshold {
			matched = append(matched, scoredSkill{skill: se.Skill, score: sim})
		}
	}

	if len(matched) == 0 {
		return nil, nil
	}

	// Sort by score descending.
	for i := 1; i < len(matched); i++ {
		for j := i; j > 0 && matched[j].score > matched[j-1].score; j-- {
			matched[j], matched[j-1] = matched[j-1], matched[j]
		}
	}

	skills := make([]*Skill, len(matched))
	for i, m := range matched {
		skills[i] = m.skill
		slog.Debug("Selected relevant skill", "skill", m.skill.Name, "score", fmt.Sprintf("%.4f", m.score))
	}

	return skills, nil
}

// buildSkillEmbeddingText creates a rich text representation of a skill for embedding.
// Combines name, description, and a truncated version of instructions.
func buildSkillEmbeddingText(sk *Skill) string {
	var sb strings.Builder
	sb.WriteString("Skill: ")
	sb.WriteString(sk.Name)
	sb.WriteString("\nDescription: ")
	sb.WriteString(sk.Description)
	if sk.Instructions != "" {
		sb.WriteString("\nInstructions:\n")
		instr := sk.Instructions
		// Truncate to ~2000 chars to keep embedding focused.
		if len(instr) > 2000 {
			instr = instr[:2000]
		}
		sb.WriteString(instr)
	}
	return sb.String()
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 and 1.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, aMag, bMag float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		aMag += float64(a[i]) * float64(a[i])
		bMag += float64(b[i]) * float64(b[i])
	}

	if aMag == 0 || bMag == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(aMag) * math.Sqrt(bMag))
}

// normalize returns a unit-length copy of the vector.
// Required for sub-3072 dimension embeddings per Google's docs.
func normalize(v []float32) []float32 {
	var mag float64
	for _, val := range v {
		mag += float64(val) * float64(val)
	}
	mag = math.Sqrt(mag)
	if mag == 0 {
		return v
	}

	normalized := make([]float32, len(v))
	for i, val := range v {
		normalized[i] = float32(float64(val) / mag)
	}
	return normalized
}

// int32Ptr returns a pointer to the given int32 value.
func int32Ptr(v int32) *int32 {
	return &v
}

