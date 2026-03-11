package db

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
)

// CodebaseKnowledgeItem represents a symbol in the codebase.
type CodebaseKnowledgeItem struct {
	FilePath      string
	SymbolName    string
	SymbolType    string
	Signature     string
	Documentation string
	LocationRange string
}

// SaveCodebaseKnowledge inserts or updates symbol information.
func (q *Queries) SaveCodebaseKnowledge(ctx context.Context, item CodebaseKnowledgeItem) (CodebaseKnowledge, error) {
	return q.UpsertCodebaseKnowledge(ctx, UpsertCodebaseKnowledgeParams{
		ID:            uuid.New().String(),
		FilePath:      item.FilePath,
		SymbolName:    item.SymbolName,
		SymbolType:    item.SymbolType,
		Signature:     sql.NullString{String: item.Signature, Valid: item.Signature != ""},
		Documentation: sql.NullString{String: item.Documentation, Valid: item.Documentation != ""},
		LocationRange: sql.NullString{String: item.LocationRange, Valid: item.LocationRange != ""},
	})
}
