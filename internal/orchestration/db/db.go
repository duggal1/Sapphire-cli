package orchestrationdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Store struct {
	conn *sql.DB
}

var sqlitePragmas = map[string]string{
	"foreign_keys":  "ON",
	"journal_mode":  "WAL",
	"page_size":     "4096",
	"cache_size":    "-8000",
	"synchronous":   "NORMAL",
	"secure_delete": "ON",
	"busy_timeout":  "5000",
}

func Open(ctx context.Context, dataDir string) (*Store, error) {
	if stringsTrim(dataDir) == "" {
		return nil, fmt.Errorf("orchestration data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create orchestration data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "orchestration.db")
	params := url.Values{}
	for name, value := range sqlitePragmas {
		params.Add("_pragma", fmt.Sprintf("%s(%s)", name, value))
	}

	conn, err := sql.Open("sqlite", fmt.Sprintf("file:%s?%s", dbPath, params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("open orchestration database: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping orchestration database: %w", err)
	}
	if err := ensureSchema(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Store{conn: conn}, nil
}

func (s *Store) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *Store) SendMail(ctx context.Context, mail AgentMail) (AgentMail, error) {
	if s == nil || s.conn == nil {
		return AgentMail{}, fmt.Errorf("orchestration store is not initialized")
	}
	if stringsTrim(mail.ToAgent) == "" {
		return AgentMail{}, fmt.Errorf("to_agent is required")
	}
	if stringsTrim(mail.FromAgent) == "" {
		return AgentMail{}, fmt.Errorf("from_agent is required")
	}
	if stringsTrim(mail.Subject) == "" {
		mail.Subject = "(no subject)"
	}
	if stringsTrim(mail.ID) == "" {
		mail.ID = uuid.NewString()
	}
	if stringsTrim(mail.ThreadID) == "" {
		mail.ThreadID = "thread-" + uuid.NewString()
	}
	if mail.CreatedAt.IsZero() {
		mail.CreatedAt = time.Now().UTC()
	}
	if mail.Read {
		mail.ReadAt = mail.CreatedAt
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO agent_mail (id, to_agent, from_agent, subject, body, priority, thread_id, read, created_at, read_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mail.ID,
		mail.ToAgent,
		mail.FromAgent,
		mail.Subject,
		mail.Body,
		mail.Priority,
		mail.ThreadID,
		boolToInt(mail.Read),
		mail.CreatedAt.Unix(),
		timeToUnix(mail.ReadAt),
	)
	if err != nil {
		return AgentMail{}, fmt.Errorf("insert agent mail: %w", err)
	}
	return mail, nil
}

func (s *Store) ListInbox(ctx context.Context, agentID string, unreadOnly bool, limit int) ([]AgentMail, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	agentID = stringsTrim(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `SELECT id, to_agent, from_agent, subject, body, priority, thread_id, read, created_at, read_at
		FROM agent_mail
		WHERE to_agent = ?`
	args := []any{agentID}
	if unreadOnly {
		query += ` AND read = 0`
	}
	query += ` ORDER BY created_at ASC LIMIT ?`
	args = append(args, limit)

	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list inbox: %w", err)
	}
	defer rows.Close()

	var items []AgentMail
	for rows.Next() {
		var (
			item        AgentMail
			readInt     int
			createdUnix int64
			readUnix    int64
		)
		if err := rows.Scan(&item.ID, &item.ToAgent, &item.FromAgent, &item.Subject, &item.Body, &item.Priority, &item.ThreadID, &readInt, &createdUnix, &readUnix); err != nil {
			return nil, fmt.Errorf("scan inbox item: %w", err)
		}
		item.Read = readInt != 0
		item.CreatedAt = time.Unix(createdUnix, 0).UTC()
		if readUnix > 0 {
			item.ReadAt = time.Unix(readUnix, 0).UTC()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inbox: %w", err)
	}
	return items, nil
}

func (s *Store) MarkRead(ctx context.Context, agentID, messageID string) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	_, err := s.conn.ExecContext(
		ctx,
		`UPDATE agent_mail SET read = 1, read_at = ? WHERE id = ? AND to_agent = ?`,
		time.Now().UTC().Unix(),
		stringsTrim(messageID),
		stringsTrim(agentID),
	)
	if err != nil {
		return fmt.Errorf("mark mail read: %w", err)
	}
	return nil
}

func (s *Store) Thread(ctx context.Context, agentID, threadID string, limit int) ([]AgentMail, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT id, to_agent, from_agent, subject, body, priority, thread_id, read, created_at, read_at
		 FROM agent_mail
		 WHERE thread_id = ? AND (to_agent = ? OR from_agent = ?)
		 ORDER BY created_at ASC
		 LIMIT ?`,
		stringsTrim(threadID),
		stringsTrim(agentID),
		stringsTrim(agentID),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list thread: %w", err)
	}
	defer rows.Close()

	var items []AgentMail
	for rows.Next() {
		var (
			item        AgentMail
			readInt     int
			createdUnix int64
			readUnix    int64
		)
		if err := rows.Scan(&item.ID, &item.ToAgent, &item.FromAgent, &item.Subject, &item.Body, &item.Priority, &item.ThreadID, &readInt, &createdUnix, &readUnix); err != nil {
			return nil, fmt.Errorf("scan thread item: %w", err)
		}
		item.Read = readInt != 0
		item.CreatedAt = time.Unix(createdUnix, 0).UTC()
		if readUnix > 0 {
			item.ReadAt = time.Unix(readUnix, 0).UTC()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread: %w", err)
	}
	return items, nil
}

func (s *Store) UpsertAgentState(ctx context.Context, state AgentState) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	now := time.Now().UTC()
	if state.LastHeartbeat.IsZero() {
		state.LastHeartbeat = now
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = now
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO agent_state (agent_id, role, status, session_id, worktree_path, branch, parent_agent_id, last_heartbeat, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(agent_id) DO UPDATE SET
			role = excluded.role,
			status = excluded.status,
			session_id = excluded.session_id,
			worktree_path = excluded.worktree_path,
			branch = excluded.branch,
			parent_agent_id = excluded.parent_agent_id,
			last_heartbeat = excluded.last_heartbeat,
			updated_at = excluded.updated_at`,
		stringsTrim(state.AgentID),
		stringsTrim(state.Role),
		stringsTrim(state.Status),
		stringsTrim(state.SessionID),
		stringsTrim(state.WorktreePath),
		stringsTrim(state.Branch),
		stringsTrim(state.ParentAgentID),
		state.LastHeartbeat.Unix(),
		state.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("upsert agent state: %w", err)
	}
	return nil
}

func (s *Store) RecordActivity(ctx context.Context, activity AgentActivity) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	if stringsTrim(activity.ID) == "" {
		activity.ID = uuid.NewString()
	}
	if activity.CreatedAt.IsZero() {
		activity.CreatedAt = time.Now().UTC()
	}
	if stringsTrim(activity.DetailsJSON) == "" {
		activity.DetailsJSON = "{}"
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO agent_activity (id, agent_id, event_type, details_json, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		activity.ID,
		stringsTrim(activity.AgentID),
		stringsTrim(activity.EventType),
		activity.DetailsJSON,
		activity.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (s *Store) MarshalDetails(details any) string {
	if details == nil {
		return "{}"
	}
	data, err := json.Marshal(details)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func timeToUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().Unix()
}

func stringsTrim(v string) string {
	return strings.TrimSpace(v)
}
