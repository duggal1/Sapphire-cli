package codeindex

import (
	"errors"
	"time"
)

const (
	DefaultEmbeddingModel      = "jina-code-embeddings-1.5b"
	DefaultEmbeddingDimensions = 1024
)

var ErrMissingAPIKey = errors.New("code index: Jina API key is required for embeddings")

type Config struct {
	WorkspaceRoot string
	DataDir       string
	APIKey        string
	Model         string
	Dimensions    int
	QdrantURL     string
}

type Stats struct {
	FileCount       int
	ChunkCount      int
	EmbeddedCount   int
	EstimatedTokens int
	LastIndexedAt   time.Time
}

type SemanticAgentProgress struct {
	ID        string
	Label     string
	Status    string
	Task      string
	Scope     string
	FileCount int
}

type Progress struct {
	Workspace       string
	Phase           string
	Message         string
	Active          bool
	Finished        bool
	FilesDiscovered int
	FilesProcessed  int
	FilesIndexed    int
	ChunksTotal     int
	ChunksEmbedded  int
	Percent         float64
	StartedAt       time.Time
	UpdatedAt       time.Time
	Error           string
	Stats           Stats
	SemanticAgents  []SemanticAgentProgress
}

type SearchResult struct {
	Path      string
	Language  string
	Kind      string
	StartLine int
	EndLine   int
	Score     float64
	Snippet   string
}
