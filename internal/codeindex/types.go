package codeindex

import "time"

const (
	DefaultEmbeddingModel      = "unclemusclez/jina-embeddings-v2-base-code:latest"
	DefaultEmbeddingDimensions = 768
	DefaultOllamaURL           = "http://127.0.0.1:11434"
)

type Config struct {
	WorkspaceRoot string
	DataDir       string
	APIKey        string
	Model         string
	Dimensions    int
	QdrantURL     string
	OllamaURL     string
}

type Stats struct {
	FileCount       int
	ChunkCount      int
	EmbeddedCount   int
	EstimatedTokens int
	LastIndexedAt   time.Time
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
	SetupRequired   bool
	Stats           Stats
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
