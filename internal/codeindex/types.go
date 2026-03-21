package codeindex

import "time"

const (
	DefaultEmbeddingModel      = "gemini-embedding-001"
	DefaultEmbeddingDimensions = 3072
)

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
