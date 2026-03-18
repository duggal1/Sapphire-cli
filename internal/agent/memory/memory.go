package memory

import (
	"fmt"
	"time"
	"context"
	"database/sql"
	"encoding/json"

	"github.com/charmbracelet/sapphire/internal/db"
	"github.com/google/uuid"
)

type MemoryService interface {
	GetProjectConstitution(ctx context.Context, id string) (string, error)
	UpsertProjectConstitution(ctx context.Context, id, content string) error

	GetStructuredSummary(ctx context.Context, sessionID string) (*StructuredSummaryData, error)
	CreateStructuredSummary(ctx context.Context, sessionID string, data StructuredSummaryData) error

	GetCodebaseKnowledge(ctx context.Context, filePath string) ([]db.CodebaseKnowledge, error)
	UpsertCodebaseKnowledge(ctx context.Context, knowledge db.UpsertCodebaseKnowledgeParams) error

	ListStructuredSummaries(ctx context.Context, limit int) ([]db.StructuredSummary, error)
	SearchCodebaseKnowledge(ctx context.Context, query string, limit int) ([]db.CodebaseKnowledge, error)
}

type StructuredSummaryData struct {
	Decisions       []Decision       `json:"decisions"`
	FileChanges     []FileChange     `json:"file_changes"`
	FailureModes    []FailureMode    `json:"failure_modes"`
	DependencyGraph []DependencyEdge `json:"dependency_graph"`
	TodoStates      []TodoState      `json:"todo_states"`
}

type Decision struct {
	Symbol    string `json:"symbol"`
	File      string `json:"file"`
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
}

type FileChange struct {
	File           string `json:"file"`
	SemanticChange string `json:"semantic_change"`
}

type FailureMode struct {
	Issue      string `json:"issue"`
	Resolution string `json:"resolution"`
}

type DependencyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type TodoState struct {
	Content      string   `json:"content"`
	Status       string   `json:"status"`
	Dependencies []string `json:"dependencies"`
}

type memoryService struct {
	db *sql.DB
	q  *db.Queries
}

func NewMemoryService(q *db.Queries, db *sql.DB) MemoryService {
	return &memoryService{q: q, db: db}
}

func (s *memoryService) GetProjectConstitution(ctx context.Context, id string) (string, error) {
	c, err := s.q.GetProjectConstitution(ctx, id)
	if err != nil {
		return "", err
	}
	return c.Content, nil
}

func (s *memoryService) UpsertProjectConstitution(ctx context.Context, id, content string) error {
	_, err := s.q.UpsertProjectConstitution(ctx, db.UpsertProjectConstitutionParams{
		ID:      id,
		Content: content,
	})
	return err
}

func (s *memoryService) GetStructuredSummary(ctx context.Context, sessionID string) (*StructuredSummaryData, error) {
	summary, err := s.q.GetStructuredSummaryBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	var data StructuredSummaryData
	if err := json.Unmarshal([]byte(summary.SummaryData), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *memoryService) CreateStructuredSummary(ctx context.Context, sessionID string, data StructuredSummaryData) error {
	rawData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = s.q.CreateStructuredSummary(ctx, db.CreateStructuredSummaryParams{
		ID:          uuid.New().String(),
		SessionID:   sessionID,
		SummaryData: string(rawData),
	})
	return err
}

func (s *memoryService) GetCodebaseKnowledge(ctx context.Context, filePath string) ([]db.CodebaseKnowledge, error) {
	return s.q.GetCodebaseKnowledgeByFilePath(ctx, filePath)
}

func (s *memoryService) UpsertCodebaseKnowledge(ctx context.Context, knowledge db.UpsertCodebaseKnowledgeParams) error {
	if knowledge.ID == "" {
		knowledge.ID = uuid.New().String()
	}
	_, err := s.q.UpsertCodebaseKnowledge(ctx, knowledge)
	return err
}

func (s *memoryService) ListStructuredSummaries(ctx context.Context, limit int) ([]db.StructuredSummary, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, session_id, summary_data, updated_at, created_at FROM structured_summaries ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var summaries []db.StructuredSummary
	for rows.Next() {
		var sm db.StructuredSummary
		if err := rows.Scan(&sm.ID, &sm.SessionID, &sm.SummaryData, &sm.UpdatedAt, &sm.CreatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, sm)
	}
	return summaries, nil
}

func (s *memoryService) SearchCodebaseKnowledge(ctx context.Context, query string, limit int) ([]db.CodebaseKnowledge, error) {
	q := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, "SELECT id, file_path, symbol_name, symbol_type, signature, documentation, location_range, updated_at, created_at FROM codebase_knowledge WHERE symbol_name LIKE ? OR documentation LIKE ? OR file_path LIKE ? ORDER BY updated_at DESC LIMIT ?", q, q, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var knowledge []db.CodebaseKnowledge
	for rows.Next() {
		var k db.CodebaseKnowledge
		if err := rows.Scan(&k.ID, &k.FilePath, &k.SymbolName, &k.SymbolType, &k.Signature, &k.Documentation, &k.LocationRange, &k.UpdatedAt, &k.CreatedAt); err != nil {
			return nil, err
		}
		knowledge = append(knowledge, k)
	}
	return knowledge, nil
}

// RolloutSummaryFileStemFromParts creates a filename stem from parts.
func RolloutSummaryFileStemFromParts(sessionID string, t time.Time, slug *string) string {
	if slug != nil && *slug != "" {
		return *slug
	}
	return sessionID
}

// FormatRolloutSummaryHeader formats a rollout summary header.
func FormatRolloutSummaryHeader(sessionID string, timestamp, summaryPath, cwd, extra string) string {
	return fmt.Sprintf("# Rollout Summary: %s\n\n", sessionID)
}

// FormatRawMemoryEntryHeader formats a raw memory entry header.
func FormatRawMemoryEntryHeader(sessionID string, timestamp, cwd, summaryPath, filename string) string {
	return fmt.Sprintf("## Memory Entry: %s\n\n", sessionID)
}
