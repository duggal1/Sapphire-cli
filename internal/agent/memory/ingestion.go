package memory

import (
	"context"

	"github.com/duggal1/Sapphire-cli/internal/db"
)

// SubagentIngester handles durable normalized memory ingestion from subagents.
type SubagentIngester struct {
	q *db.Queries
}

func NewSubagentIngester(q *db.Queries) *SubagentIngester {
	return &SubagentIngester{q: q}
}

// IngestFindings saves findings durably with provenance.
func (si *SubagentIngester) IngestFindings(ctx context.Context, agentID string, findings []string, provenance string) error {
	// TODO: persist to database
	return nil
}

// IngestDecisions saves subagent decisions.
func (si *SubagentIngester) IngestDecisions(ctx context.Context, agentID string, decisions []string, provenance string) error {
	// TODO: persist to database
	return nil
}
