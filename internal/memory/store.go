// Package memory provides a persistent structured memory system for Sapphire sessions.
// It runs entirely in the background, never blocking the main agent.
package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store is the project-shared SQLite persistent memory store.
// All writes go through a single serialized writer. Reads are concurrent.
type Store struct {
	db        *sql.DB
	sessionID string
	project   string
	writeMu   sync.Mutex
}

const schema = `
CREATE TABLE IF NOT EXISTS memory_records (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	project_scope TEXT NOT NULL,
	event_type TEXT NOT NULL,
	timestamp INTEGER NOT NULL,
	turn_index INTEGER NOT NULL DEFAULT 0,
	salience REAL NOT NULL DEFAULT 0.5,
	content_json TEXT NOT NULL,
	raw_source TEXT NOT NULL DEFAULT '',
	is_negative_constraint INTEGER NOT NULL DEFAULT 0,
	is_architectural_decision INTEGER NOT NULL DEFAULT 0,
	is_failure_mode INTEGER NOT NULL DEFAULT 0,
	dedup_hash TEXT UNIQUE
);

CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(content_json, content='memory_records', content_rowid='id');

CREATE TRIGGER IF NOT EXISTS memory_records_ai AFTER INSERT ON memory_records BEGIN
	INSERT INTO memory_fts(rowid, content_json) VALUES (new.id, new.content_json);
END;

CREATE TRIGGER IF NOT EXISTS memory_records_ad AFTER DELETE ON memory_records BEGIN
	INSERT INTO memory_fts(memory_fts, rowid, content_json) VALUES('delete', old.id, old.content_json);
END;

CREATE TRIGGER IF NOT EXISTS memory_records_au AFTER UPDATE ON memory_records BEGIN
	INSERT INTO memory_fts(memory_fts, rowid, content_json) VALUES('delete', old.id, old.content_json);
	INSERT INTO memory_fts(rowid, content_json) VALUES (new.id, new.content_json);
END;

CREATE TABLE IF NOT EXISTS project_constitution (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	project_scope TEXT NOT NULL,
	content TEXT NOT NULL DEFAULT '',
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS compaction_checkpoints (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	project_scope TEXT NOT NULL,
	checkpoint_json TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS memory_embeddings (
	record_id INTEGER PRIMARY KEY,
	session_id TEXT NOT NULL,
	project_scope TEXT NOT NULL,
	vector BLOB NOT NULL,
	dim INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS memory_dead_letter (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	project_scope TEXT NOT NULL,
	event_type TEXT NOT NULL,
	timestamp INTEGER NOT NULL,
	turn_index INTEGER NOT NULL DEFAULT 0,
	reason TEXT NOT NULL,
	raw_source TEXT NOT NULL
);
`

// NewStore opens (or creates) a project-shared SQLite memory database.
// Session separation happens at the row level via session_id, not by creating
// one database file per agent or session.
func NewStore(dataDir, sessionID, projectRoot string) (*Store, error) {
	projectScope := projectScopeHash(projectRoot)
	dir := filepath.Join(dataDir, "memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("memory: create dir: %w", err)
	}

	dbPath := filepath.Join(dir, fmt.Sprintf("%s.db", projectScope[:12]))
	legacyPath := filepath.Join(dir, fmt.Sprintf("%s_.db", projectScope[:12]))
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		if _, legacyErr := os.Stat(legacyPath); legacyErr == nil {
			_ = os.Rename(legacyPath, dbPath)
			_ = os.Rename(legacyPath+"-wal", dbPath+"-wal")
			_ = os.Rename(legacyPath+"-shm", dbPath+"-shm")
		}
	}
	db, err := openMemoryDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("memory: open db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		// If schema fails (corruption), try a fresh database.
		slog.Warn("memory: schema init failed, recreating", "error", err)
		os.Remove(dbPath)
		db2, err2 := openMemoryDB(dbPath)
		if err2 != nil {
			return nil, fmt.Errorf("memory: reopen db: %w", err2)
		}
		if _, err3 := db2.Exec(schema); err3 != nil {
			db2.Close()
			return nil, fmt.Errorf("memory: schema reinit: %w", err3)
		}
		db = db2
	}

	return &Store{
		db:        db,
		sessionID: sessionID,
		project:   projectScope,
	}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func projectScopeHash(root string) string {
	h := sha256.Sum256([]byte(root))
	return hex.EncodeToString(h[:])
}

// dedupHash returns a deterministic hash for event deduplication.
func dedupHash(sessionID string, turnIndex int, eventType string) string {
	raw := fmt.Sprintf("%s:%d:%s", sessionID, turnIndex, eventType)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:16])
}

// MemoryRecord is a single persisted memory entry.
type MemoryRecord struct {
	ID                      int64   `json:"id"`
	SessionID               string  `json:"session_id"`
	EventType               string  `json:"event_type"`
	Timestamp               int64   `json:"timestamp"`
	TurnIndex               int     `json:"turn_index"`
	Salience                float64 `json:"salience"`
	ContentJSON             string  `json:"content_json"`
	RawSource               string  `json:"raw_source,omitempty"`
	IsNegativeConstraint    bool    `json:"is_negative_constraint"`
	IsArchitecturalDecision bool    `json:"is_architectural_decision"`
	IsFailureMode           bool    `json:"is_failure_mode"`
}

// WriteRecord writes a memory record with deduplication and returns the record ID.
func (s *Store) WriteRecord(ctx context.Context, rec MemoryRecord) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	hash := dedupHash(rec.SessionID, rec.TurnIndex, rec.EventType)

	res, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO memory_records
			(session_id, project_scope, event_type, timestamp, turn_index, salience,
			 content_json, raw_source, is_negative_constraint, is_architectural_decision,
			 is_failure_mode, dedup_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.SessionID, s.project, rec.EventType, rec.Timestamp, rec.TurnIndex,
		rec.Salience, rec.ContentJSON, rec.RawSource,
		boolToInt(rec.IsNegativeConstraint), boolToInt(rec.IsArchitecturalDecision),
		boolToInt(rec.IsFailureMode), hash,
	)
	if err != nil {
		return 0, err
	}

	if id, err := res.LastInsertId(); err == nil && id > 0 {
		return id, nil
	}

	// If insert was ignored, fetch existing ID by dedup hash.
	var id int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM memory_records WHERE dedup_hash = ? LIMIT 1`, hash,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// QueryRecords retrieves top-K records by retrieval score with temporal decay.
func (s *Store) QueryRecords(ctx context.Context, filter string, limit int) ([]MemoryRecord, error) {
	return s.QueryRecordsBySession(ctx, s.sessionID, filter, limit)
}

// QueryRecordsBySession retrieves top-K records for a specific session ID.
func (s *Store) QueryRecordsBySession(ctx context.Context, sessionID, filter string, limit int) ([]MemoryRecord, error) {
	query := `SELECT id, session_id, event_type, timestamp, turn_index, salience,
		content_json, raw_source, is_negative_constraint, is_architectural_decision,
		is_failure_mode
		FROM memory_records WHERE session_id = ? AND project_scope = ?`
	args := []any{sessionID, s.project}

	switch filter {
	case "negative_constraints":
		query += " AND is_negative_constraint = 1"
	case "architectural":
		query += " AND is_architectural_decision = 1"
	case "failures":
		query += " AND is_failure_mode = 1"
	case "progress":
		query += " AND event_type = 'task_progress'"
	}

	query += " ORDER BY salience DESC, timestamp DESC"
	if limit > 0 {
		args = append(args, limit*3) // fetch extra for decay scoring
		query += " LIMIT ?"
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []MemoryRecord
	now := time.Now().Unix()
	for rows.Next() {
		var r MemoryRecord
		var neg, arch, fail int
		if err := rows.Scan(&r.ID, &r.SessionID, &r.EventType, &r.Timestamp,
			&r.TurnIndex, &r.Salience, &r.ContentJSON, &r.RawSource,
			&neg, &arch, &fail); err != nil {
			continue
		}
		r.IsNegativeConstraint = neg == 1
		r.IsArchitecturalDecision = arch == 1
		r.IsFailureMode = fail == 1

		// Apply temporal decay. Negative constraints and architectural decisions never decay.
		if r.IsNegativeConstraint || r.IsArchitecturalDecision {
			// Zero decay — keep original salience
		} else {
			hours := float64(now-r.Timestamp) / 3600.0
			r.Salience = r.Salience * math.Exp(-0.05*hours)
		}
		records = append(records, r)
	}

	// Re-sort by decayed salience
	sortRecordsBySalience(records)

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

// SearchFTS performs full-text search over memory content.
func (s *Store) SearchFTS(ctx context.Context, query string, limit int) ([]MemoryRecord, error) {
	return s.SearchFTSBySession(ctx, s.sessionID, query, limit)
}

// SearchFTSBySession performs full-text search over memory content for one session.
func (s *Store) SearchFTSBySession(ctx context.Context, sessionID, query string, limit int) ([]MemoryRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.id, m.session_id, m.event_type, m.timestamp, m.turn_index, m.salience,
			m.content_json, m.raw_source, m.is_negative_constraint,
			m.is_architectural_decision, m.is_failure_mode
		FROM memory_fts f
		JOIN memory_records m ON f.rowid = m.id
		WHERE memory_fts MATCH ? AND m.session_id = ? AND m.project_scope = ?
		ORDER BY rank LIMIT ?`,
		query, sessionID, s.project, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []MemoryRecord
	for rows.Next() {
		var r MemoryRecord
		var neg, arch, fail int
		if err := rows.Scan(&r.ID, &r.SessionID, &r.EventType, &r.Timestamp,
			&r.TurnIndex, &r.Salience, &r.ContentJSON, &r.RawSource,
			&neg, &arch, &fail); err != nil {
			continue
		}
		r.IsNegativeConstraint = neg == 1
		r.IsArchitecturalDecision = arch == 1
		r.IsFailureMode = fail == 1
		records = append(records, r)
	}
	return records, nil
}

// GetNegativeConstraints returns all negative constraint records (zero decay, always included).
func (s *Store) GetNegativeConstraints(ctx context.Context) ([]MemoryRecord, error) {
	return s.QueryRecords(ctx, "negative_constraints", 100)
}

// GetNegativeConstraintsBySession returns all negative constraint records for a session.
func (s *Store) GetNegativeConstraintsBySession(ctx context.Context, sessionID string) ([]MemoryRecord, error) {
	return s.QueryRecordsBySession(ctx, sessionID, "negative_constraints", 100)
}

// GetConstitution retrieves the project constitution.
func (s *Store) GetConstitution(ctx context.Context) (string, error) {
	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT content FROM project_constitution WHERE project_scope = ? LIMIT 1`,
		s.project,
	).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

// UpsertConstitution writes or updates the project constitution.
func (s *Store) UpsertConstitution(ctx context.Context, content string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO project_constitution (id, project_scope, content, updated_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
		s.project, content, time.Now().Unix(),
	)
	return err
}

// WriteCheckpoint writes a compaction checkpoint.
func (s *Store) WriteCheckpoint(ctx context.Context, checkpointJSON string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO compaction_checkpoints (session_id, project_scope, checkpoint_json, created_at)
		VALUES (?, ?, ?, ?)`,
		s.sessionID, s.project, checkpointJSON, time.Now().Unix(),
	)
	return err
}

// GetLatestCheckpoint returns the most recent compaction checkpoint.
func (s *Store) GetLatestCheckpoint(ctx context.Context) (string, error) {
	return s.GetLatestCheckpointBySession(ctx, s.sessionID)
}

// GetLatestCheckpointBySession returns the most recent compaction checkpoint for a session.
func (s *Store) GetLatestCheckpointBySession(ctx context.Context, sessionID string) (string, error) {
	var checkpoint string
	err := s.db.QueryRowContext(ctx,
		`SELECT checkpoint_json FROM compaction_checkpoints
		WHERE session_id = ? AND project_scope = ?
		ORDER BY created_at DESC LIMIT 1`,
		sessionID, s.project,
	).Scan(&checkpoint)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return checkpoint, err
}

// WriteDeadLetter persists an event that could not be enqueued.
func (s *Store) WriteDeadLetter(ctx context.Context, event ExtractionEvent, reason string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memory_dead_letter
		(session_id, project_scope, event_type, timestamp, turn_index, reason, raw_source)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.SessionID, s.project, event.EventType, time.Now().Unix(), event.TurnIndex, reason, event.RawSource,
	)
	return err
}

// DeadLetterCount returns the number of dead-lettered events for the session.
func (s *Store) DeadLetterCount(ctx context.Context) (int64, error) {
	return s.DeadLetterCountBySession(ctx, s.sessionID)
}

// DeadLetterCountBySession returns the number of dead-lettered events for a session.
func (s *Store) DeadLetterCountBySession(ctx context.Context, sessionID string) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memory_dead_letter WHERE session_id = ? AND project_scope = ?`,
		sessionID, s.project,
	).Scan(&count)
	return count, err
}

// CountRecords returns the total number of records for the session.
func (s *Store) CountRecords(ctx context.Context) (int64, error) {
	return s.CountRecordsBySession(ctx, s.sessionID)
}

// CountRecordsBySession returns the total number of records for a session.
func (s *Store) CountRecordsBySession(ctx context.Context, sessionID string) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memory_records WHERE session_id = ? AND project_scope = ?`,
		sessionID, s.project,
	).Scan(&count)
	return count, err
}

// TopSalience returns the top-K salience scores.
func (s *Store) TopSalience(ctx context.Context, limit int) ([]float64, error) {
	return s.TopSalienceBySession(ctx, s.sessionID, limit)
}

// TopSalienceBySession returns the top-K salience scores for a session.
func (s *Store) TopSalienceBySession(ctx context.Context, sessionID string, limit int) ([]float64, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT salience FROM memory_records
		WHERE session_id = ? AND project_scope = ?
		ORDER BY salience DESC LIMIT ?`,
		sessionID, s.project, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []float64
	for rows.Next() {
		var svalue float64
		if err := rows.Scan(&svalue); err != nil {
			continue
		}
		scores = append(scores, svalue)
	}
	return scores, nil
}

// LatestCheckpointAgeSeconds returns age seconds and timestamp of the latest checkpoint.
func (s *Store) LatestCheckpointAgeSeconds(ctx context.Context) (int64, int64, error) {
	return s.LatestCheckpointAgeSecondsBySession(ctx, s.sessionID)
}

// LatestCheckpointAgeSecondsBySession returns age seconds and timestamp of the latest checkpoint for a session.
func (s *Store) LatestCheckpointAgeSecondsBySession(ctx context.Context, sessionID string) (int64, int64, error) {
	var createdAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT created_at FROM compaction_checkpoints
		WHERE session_id = ? AND project_scope = ?
		ORDER BY created_at DESC LIMIT 1`,
		sessionID, s.project,
	).Scan(&createdAt)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	age := time.Now().Unix() - createdAt
	return age, createdAt, nil
}

// UpsertEmbedding stores or updates an embedding vector for a record.
func (s *Store) UpsertEmbedding(ctx context.Context, recordID int64, vector []float32, dimensions int) error {
	if recordID == 0 || len(vector) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	blob := encodeVector(vector)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memory_embeddings (record_id, session_id, project_scope, vector, dim, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(record_id) DO UPDATE SET vector = excluded.vector, dim = excluded.dim, updated_at = excluded.updated_at`,
		recordID, s.sessionID, s.project, blob, dimensions, time.Now().Unix(),
	)
	return err
}

type embeddedRecord struct {
	record MemoryRecord
	vector []float32
}

// LoadEmbeddingCandidates fetches recent/high-salience records with embeddings.
func (s *Store) LoadEmbeddingCandidates(ctx context.Context, limit int) ([]embeddedRecord, error) {
	return s.LoadEmbeddingCandidatesBySession(ctx, s.sessionID, limit)
}

// LoadEmbeddingCandidatesBySession fetches recent/high-salience records with embeddings for a session.
func (s *Store) LoadEmbeddingCandidatesBySession(ctx context.Context, sessionID string, limit int) ([]embeddedRecord, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.id, m.session_id, m.event_type, m.timestamp, m.turn_index, m.salience,
			m.content_json, m.raw_source, m.is_negative_constraint,
			m.is_architectural_decision, m.is_failure_mode, e.vector
		FROM memory_embeddings e
		JOIN memory_records m ON e.record_id = m.id
		WHERE m.session_id = ? AND m.project_scope = ?
		ORDER BY m.salience DESC, m.timestamp DESC LIMIT ?`,
		sessionID, s.project, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []embeddedRecord
	for rows.Next() {
		var r MemoryRecord
		var neg, arch, fail int
		var blob []byte
		if err := rows.Scan(&r.ID, &r.SessionID, &r.EventType, &r.Timestamp,
			&r.TurnIndex, &r.Salience, &r.ContentJSON, &r.RawSource,
			&neg, &arch, &fail, &blob); err != nil {
			continue
		}
		r.IsNegativeConstraint = neg == 1
		r.IsArchitecturalDecision = arch == 1
		r.IsFailureMode = fail == 1

		vec, err := decodeVector(blob)
		if err != nil {
			continue
		}
		results = append(results, embeddedRecord{record: r, vector: vec})
	}
	return results, nil
}

// SearchHybrid merges FTS and semantic retrieval results.
func (s *Store) SearchHybrid(ctx context.Context, query string, limit int, embedder Embedder) ([]MemoryRecord, error) {
	return s.SearchHybridBySession(ctx, s.sessionID, query, limit, embedder)
}

// SearchHybridBySession merges FTS and semantic retrieval results for one session.
func (s *Store) SearchHybridBySession(ctx context.Context, sessionID, query string, limit int, embedder Embedder) ([]MemoryRecord, error) {
	if query == "" {
		return s.QueryRecordsBySession(ctx, sessionID, "all", limit)
	}

	ftsRecords, ftsErr := s.SearchFTSBySession(ctx, sessionID, query, limit)
	if embedder == nil {
		if ftsErr == nil && len(ftsRecords) > 0 {
			return ftsRecords, nil
		}
		return s.QueryRecordsBySession(ctx, sessionID, "all", limit)
	}

	queryVec, err := embedder.EmbedQuery(ctx, query)
	if err != nil {
		slog.Debug("memory: semantic embed failed, falling back to FTS", "error", err)
		if ftsErr == nil && len(ftsRecords) > 0 {
			return ftsRecords, nil
		}
		return s.QueryRecordsBySession(ctx, sessionID, "all", limit)
	}

	candidateLimit := limit * 20
	if candidateLimit < 50 {
		candidateLimit = 50
	}
	if candidateLimit > 300 {
		candidateLimit = 300
	}
	embedded, err := s.LoadEmbeddingCandidatesBySession(ctx, sessionID, candidateLimit)
	if err != nil {
		slog.Debug("memory: semantic candidates failed, falling back to FTS", "error", err)
		if ftsErr == nil && len(ftsRecords) > 0 {
			return ftsRecords, nil
		}
		return s.QueryRecordsBySession(ctx, sessionID, "all", limit)
	}

	var semanticScored []scoredRecord
	for _, item := range embedded {
		score := cosineSimilarity(queryVec, item.vector)
		if score <= 0 {
			continue
		}
		semanticScored = append(semanticScored, scoredRecord{
			record: item.record,
			score:  score,
		})
	}

	// Sort semantic results by similarity.
	for i := 1; i < len(semanticScored); i++ {
		for j := i; j > 0 && semanticScored[j].score > semanticScored[j-1].score; j-- {
			semanticScored[j], semanticScored[j-1] = semanticScored[j-1], semanticScored[j]
		}
	}

	if len(semanticScored) > limit {
		semanticScored = semanticScored[:limit]
	}

	return mergeHybridResults(ftsRecords, semanticScored, limit), nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func sortRecordsBySalience(records []MemoryRecord) {
	for i := 1; i < len(records); i++ {
		for j := i; j > 0 && records[j].Salience > records[j-1].Salience; j-- {
			records[j], records[j-1] = records[j-1], records[j]
		}
	}
}

type scoredRecord struct {
	record MemoryRecord
	score  float64
}

func mergeHybridResults(fts []MemoryRecord, semantic []scoredRecord, limit int) []MemoryRecord {
	if limit <= 0 {
		return nil
	}
	if len(fts) == 0 && len(semantic) == 0 {
		return nil
	}

	scores := make(map[int64]scoredRecord)

	// Seed with FTS results.
	for i, rec := range fts {
		score := 0.6
		if len(fts) > 1 {
			score = 0.6 + 0.4*(1.0-float64(i)/float64(len(fts)))
		}
		scores[rec.ID] = scoredRecord{record: rec, score: score}
	}

	// Merge semantic results.
	for _, srec := range semantic {
		existing, ok := scores[srec.record.ID]
		if ok {
			// Boost when both methods agree.
			combined := srec.score
			if existing.score > combined {
				combined = existing.score + 0.1
			} else {
				combined = srec.score + 0.1
			}
			existing.score = combined
			scores[srec.record.ID] = existing
			continue
		}
		scores[srec.record.ID] = srec
	}

	merged := make([]scoredRecord, 0, len(scores))
	for _, srec := range scores {
		merged = append(merged, srec)
	}

	// Sort by score, then salience.
	for i := 1; i < len(merged); i++ {
		for j := i; j > 0; j-- {
			if merged[j].score > merged[j-1].score {
				merged[j], merged[j-1] = merged[j-1], merged[j]
				continue
			}
			if merged[j].score == merged[j-1].score &&
				merged[j].record.Salience > merged[j-1].record.Salience {
				merged[j], merged[j-1] = merged[j-1], merged[j]
				continue
			}
			break
		}
	}

	if len(merged) > limit {
		merged = merged[:limit]
	}

	out := make([]MemoryRecord, len(merged))
	for i, srec := range merged {
		out[i] = srec.record
	}
	return out
}

// MarshalRecordsJSON returns a JSON string of the given records for injection.
func MarshalRecordsJSON(records []MemoryRecord) string {
	if len(records) == 0 {
		return "[]"
	}
	type compactRecord struct {
		EventType string          `json:"event_type"`
		Salience  float64         `json:"salience"`
		Content   json.RawMessage `json:"content"`
	}
	compact := make([]compactRecord, 0, len(records))
	for _, r := range records {
		compact = append(compact, compactRecord{
			EventType: r.EventType,
			Salience:  math.Round(r.Salience*100) / 100,
			Content:   json.RawMessage(r.ContentJSON),
		})
	}
	data, err := json.MarshalIndent(compact, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(data)
}
