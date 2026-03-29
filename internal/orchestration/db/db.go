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
	if stringsTrim(mail.Address) == "" {
		mail.Address = mail.ToAgent
	}
	if stringsTrim(mail.ResolvedToAgent) == "" {
		mail.ResolvedToAgent = mail.ToAgent
	}
	if stringsTrim(mail.DeliveryState) == "" {
		mail.DeliveryState = MailDeliveryStatePending
	}
	if mail.CreatedAt.IsZero() {
		mail.CreatedAt = time.Now().UTC()
	}
	if mail.Read {
		mail.ReadAt = mail.CreatedAt
	}
	if mail.DeliveryState == MailDeliveryStateAcked && mail.AckedAt.IsZero() {
		mail.AckedAt = mail.CreatedAt
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO agent_mail (
			id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
			delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mail.ID,
		mail.Address,
		mail.ToAgent,
		mail.ResolvedToAgent,
		mail.FromAgent,
		mail.Subject,
		mail.Body,
		mail.Priority,
		mail.ThreadID,
		mail.DeliveryState,
		mail.DeliveryAttempts,
		mail.LeaseOwner,
		timeToUnix(mail.LeaseExpiresAt),
		boolToInt(mail.Read),
		mail.CreatedAt.Unix(),
		timeToUnix(mail.ReadAt),
		timeToUnix(mail.AckedAt),
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
	query := `SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		FROM agent_mail
		WHERE resolved_to_agent = ?`
	args := []any{agentID}
	if unreadOnly {
		query += ` AND read = 0`
	}
	query += ` ORDER BY priority DESC, created_at ASC, rowid ASC LIMIT ?`
	args = append(args, limit)

	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list inbox: %w", err)
	}
	defer rows.Close()
	return scanMailRows(rows)
}

func (s *Store) MarkRead(ctx context.Context, agentID, messageID string) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	_, err := s.conn.ExecContext(
		ctx,
		`UPDATE agent_mail SET read = 1, read_at = ? WHERE id = ? AND resolved_to_agent = ?`,
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
		`SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		        delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		 FROM agent_mail
		 WHERE thread_id = ? AND (resolved_to_agent = ? OR from_agent = ?)
		 ORDER BY created_at ASC, rowid ASC
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
	return scanMailRows(rows)
}

func (s *Store) ListActionableMail(ctx context.Context, agentID string, limit int) ([]AgentMail, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if agentID = stringsTrim(agentID); agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		        delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		   FROM agent_mail
		  WHERE resolved_to_agent = ?
		    AND delivery_state IN (?, ?)
		  ORDER BY priority DESC, created_at ASC, rowid ASC
		  LIMIT ?`,
		agentID,
		MailDeliveryStatePending,
		MailDeliveryStateLeased,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list actionable mail: %w", err)
	}
	defer rows.Close()
	return scanMailRows(rows)
}

func (s *Store) LeaseInbox(ctx context.Context, agentID, leaseOwner string, limit int, leaseTTL time.Duration) ([]AgentMail, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if agentID = stringsTrim(agentID); agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if leaseOwner = stringsTrim(leaseOwner); leaseOwner == "" {
		return nil, fmt.Errorf("lease owner is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if leaseTTL <= 0 {
		leaseTTL = 2 * time.Minute
	}

	now := time.Now().UTC()
	expiry := now.Add(leaseTTL)
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin mail lease transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	existingRows, err := tx.QueryContext(
		ctx,
		`SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		        delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		   FROM agent_mail
		  WHERE resolved_to_agent = ?
		    AND delivery_state = ?
		    AND lease_owner = ?
		    AND lease_expires_at > ?
		  ORDER BY priority DESC, created_at ASC, rowid ASC
		  LIMIT ?`,
		agentID,
		MailDeliveryStateLeased,
		leaseOwner,
		now.Unix(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query active mail leases: %w", err)
	}
	existing, err := scanMailRows(existingRows)
	existingRows.Close()
	if err != nil {
		return nil, err
	}
	if len(existing) >= limit {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit mail lease transaction: %w", err)
		}
		tx = nil
		return existing, nil
	}

	rows, err := tx.QueryContext(
		ctx,
		`SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		        delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		   FROM agent_mail
		  WHERE resolved_to_agent = ?
		    AND delivery_state = ?
		  ORDER BY priority DESC, created_at ASC, rowid ASC
		  LIMIT ?`,
		agentID,
		MailDeliveryStatePending,
		limit-len(existing),
	)
	if err != nil {
		return nil, fmt.Errorf("query pending mail leases: %w", err)
	}
	pending, err := scanMailRows(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}

	leased := append([]AgentMail{}, existing...)
	for _, item := range pending {
		result, execErr := tx.ExecContext(
			ctx,
			`UPDATE agent_mail
			    SET delivery_state = ?, delivery_attempts = delivery_attempts + 1, lease_owner = ?, lease_expires_at = ?
			  WHERE rowid = ? AND delivery_state = ?`,
			MailDeliveryStateLeased,
			leaseOwner,
			expiry.Unix(),
			item.RowID,
			MailDeliveryStatePending,
		)
		if execErr != nil {
			return nil, fmt.Errorf("lease mail %s: %w", item.ID, execErr)
		}
		affected, affErr := result.RowsAffected()
		if affErr != nil {
			return nil, fmt.Errorf("lease mail rows affected %s: %w", item.ID, affErr)
		}
		if affected == 0 {
			continue
		}
		item.DeliveryState = MailDeliveryStateLeased
		item.DeliveryAttempts++
		item.LeaseOwner = leaseOwner
		item.LeaseExpiresAt = expiry
		leased = append(leased, item)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit mail lease transaction: %w", err)
	}
	tx = nil
	return leased, nil
}

func (s *Store) AckMail(ctx context.Context, agentID, messageID string) (AgentMail, error) {
	if s == nil || s.conn == nil {
		return AgentMail{}, fmt.Errorf("orchestration store is not initialized")
	}
	agentID = stringsTrim(agentID)
	messageID = stringsTrim(messageID)
	if agentID == "" {
		return AgentMail{}, fmt.Errorf("agent id is required")
	}
	if messageID == "" {
		return AgentMail{}, fmt.Errorf("message id is required")
	}

	ackedAt := time.Now().UTC()
	result, err := s.conn.ExecContext(
		ctx,
		`UPDATE agent_mail
		    SET delivery_state = ?, acked_at = ?, lease_owner = '', lease_expires_at = 0, read = 1, read_at = CASE WHEN read_at = 0 THEN ? ELSE read_at END
		  WHERE id = ? AND resolved_to_agent = ? AND delivery_state IN (?, ?)`,
		MailDeliveryStateAcked,
		ackedAt.Unix(),
		ackedAt.Unix(),
		messageID,
		agentID,
		MailDeliveryStatePending,
		MailDeliveryStateLeased,
	)
	if err != nil {
		return AgentMail{}, fmt.Errorf("ack mail: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AgentMail{}, fmt.Errorf("ack mail rows affected: %w", err)
	}
	if affected == 0 {
		return AgentMail{}, sql.ErrNoRows
	}

	row := s.conn.QueryRowContext(
		ctx,
		`SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		        delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		   FROM agent_mail
		  WHERE id = ?`,
		messageID,
	)
	item, err := scanMail(row)
	if err != nil {
		return AgentMail{}, fmt.Errorf("load acked mail: %w", err)
	}
	return item, nil
}

func (s *Store) DeadLetterMail(ctx context.Context, messageID string) (AgentMail, error) {
	if s == nil || s.conn == nil {
		return AgentMail{}, fmt.Errorf("orchestration store is not initialized")
	}
	messageID = stringsTrim(messageID)
	if messageID == "" {
		return AgentMail{}, fmt.Errorf("message id is required")
	}

	result, err := s.conn.ExecContext(
		ctx,
		`UPDATE agent_mail
		    SET delivery_state = ?, lease_owner = '', lease_expires_at = 0
		  WHERE id = ? AND delivery_state IN (?, ?)`,
		MailDeliveryStateDeadLetter,
		messageID,
		MailDeliveryStatePending,
		MailDeliveryStateLeased,
	)
	if err != nil {
		return AgentMail{}, fmt.Errorf("dead-letter mail: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AgentMail{}, fmt.Errorf("dead-letter mail rows affected: %w", err)
	}
	if affected == 0 {
		return AgentMail{}, sql.ErrNoRows
	}

	row := s.conn.QueryRowContext(
		ctx,
		`SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		        delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		   FROM agent_mail
		  WHERE id = ?`,
		messageID,
	)
	item, err := scanMail(row)
	if err != nil {
		return AgentMail{}, fmt.Errorf("load dead-letter mail: %w", err)
	}
	return item, nil
}

func (s *Store) RequeueExpiredMailLeases(ctx context.Context, maxAttempts int) ([]AgentMail, []AgentMail, error) {
	if s == nil || s.conn == nil {
		return nil, nil, fmt.Errorf("orchestration store is not initialized")
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	now := time.Now().UTC()
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin expired mail requeue transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(
		ctx,
		`SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		        delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		   FROM agent_mail
		  WHERE delivery_state = ?
		    AND lease_expires_at > 0
		    AND lease_expires_at <= ?
		  ORDER BY lease_expires_at ASC, rowid ASC`,
		MailDeliveryStateLeased,
		now.Unix(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("query expired mail leases: %w", err)
	}
	items, err := scanMailRows(rows)
	rows.Close()
	if err != nil {
		return nil, nil, err
	}

	requeued := make([]AgentMail, 0, len(items))
	deadLetters := make([]AgentMail, 0, len(items))
	for _, item := range items {
		nextState := MailDeliveryStatePending
		target := &requeued
		if item.DeliveryAttempts >= maxAttempts {
			nextState = MailDeliveryStateDeadLetter
			target = &deadLetters
		}
		if _, execErr := tx.ExecContext(
			ctx,
			`UPDATE agent_mail
			    SET delivery_state = ?, lease_owner = '', lease_expires_at = 0
			  WHERE rowid = ? AND delivery_state = ?`,
			nextState,
			item.RowID,
			MailDeliveryStateLeased,
		); execErr != nil {
			return nil, nil, fmt.Errorf("requeue expired mail %s: %w", item.ID, execErr)
		}
		item.DeliveryState = nextState
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		*target = append(*target, item)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit expired mail requeue transaction: %w", err)
	}
	tx = nil
	return requeued, deadLetters, nil
}

func (s *Store) ListStalePendingMail(ctx context.Context, olderThan time.Time, limit int) ([]AgentMail, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT rowid, id, address, to_agent, resolved_to_agent, from_agent, subject, body, priority, thread_id,
		        delivery_state, delivery_attempts, lease_owner, lease_expires_at, read, created_at, read_at, acked_at
		   FROM agent_mail
		  WHERE delivery_state = ?
		    AND created_at > 0
		    AND created_at <= ?
		  ORDER BY created_at ASC, rowid ASC
		  LIMIT ?`,
		MailDeliveryStatePending,
		olderThan.UTC().Unix(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list stale pending mail: %w", err)
	}
	defer rows.Close()
	return scanMailRows(rows)
}

func (s *Store) LatestMailRowID(ctx context.Context, agentID string) (int64, error) {
	if s == nil || s.conn == nil {
		return 0, fmt.Errorf("orchestration store is not initialized")
	}
	row := s.conn.QueryRowContext(
		ctx,
		`SELECT COALESCE(MAX(rowid), 0)
		   FROM agent_mail
		  WHERE resolved_to_agent = ?`,
		stringsTrim(agentID),
	)
	var latest int64
	if err := row.Scan(&latest); err != nil {
		return 0, fmt.Errorf("latest mail row id: %w", err)
	}
	return latest, nil
}

func (s *Store) LatestActivityRowID(ctx context.Context, agentIDs []string) (int64, error) {
	if s == nil || s.conn == nil {
		return 0, fmt.Errorf("orchestration store is not initialized")
	}
	agentIDs = normalizeStringArgs(agentIDs)
	if len(agentIDs) == 0 {
		return 0, nil
	}
	placeholders := make([]string, 0, len(agentIDs))
	args := make([]any, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		placeholders = append(placeholders, "?")
		args = append(args, agentID)
	}
	query := fmt.Sprintf(
		`SELECT COALESCE(MAX(rowid), 0)
		   FROM agent_activity
		  WHERE agent_id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	row := s.conn.QueryRowContext(ctx, query, args...)
	var latest int64
	if err := row.Scan(&latest); err != nil {
		return 0, fmt.Errorf("latest activity row id: %w", err)
	}
	return latest, nil
}

func (s *Store) UpsertAgentState(ctx context.Context, state AgentState) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	now := time.Now().UTC()
	if state.LastHeartbeat.IsZero() {
		state.LastHeartbeat = now
	}
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = now
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO agent_state (agent_id, role, status, session_id, worktree_path, branch, hook_bead_id, parent_agent_id, last_heartbeat, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(agent_id) DO UPDATE SET
			role = excluded.role,
			status = excluded.status,
			session_id = excluded.session_id,
			worktree_path = excluded.worktree_path,
			branch = excluded.branch,
			hook_bead_id = excluded.hook_bead_id,
			parent_agent_id = excluded.parent_agent_id,
			last_heartbeat = excluded.last_heartbeat,
			updated_at = excluded.updated_at`,
		stringsTrim(state.AgentID),
		stringsTrim(state.Role),
		stringsTrim(state.Status),
		stringsTrim(state.SessionID),
		stringsTrim(state.WorktreePath),
		stringsTrim(state.Branch),
		stringsTrim(state.HookBeadID),
		stringsTrim(state.ParentAgentID),
		state.LastHeartbeat.Unix(),
		state.CreatedAt.Unix(),
		state.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("upsert agent state: %w", err)
	}
	return nil
}

func (s *Store) GetAgentState(ctx context.Context, agentID string) (AgentState, error) {
	if s == nil || s.conn == nil {
		return AgentState{}, fmt.Errorf("orchestration store is not initialized")
	}
	row := s.conn.QueryRowContext(
		ctx,
		`SELECT agent_id, role, status, session_id, worktree_path, branch, hook_bead_id, parent_agent_id, last_heartbeat, created_at, updated_at
		 FROM agent_state
		 WHERE agent_id = ?`,
		stringsTrim(agentID),
	)
	state, err := scanAgentState(row)
	if err != nil {
		return AgentState{}, fmt.Errorf("get agent state: %w", err)
	}
	return state, nil
}

func (s *Store) ListAgentStates(ctx context.Context, limit int) ([]AgentState, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT agent_id, role, status, session_id, worktree_path, branch, hook_bead_id, parent_agent_id, last_heartbeat, created_at, updated_at
		 FROM agent_state
		 ORDER BY updated_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent states: %w", err)
	}
	defer rows.Close()
	return scanAgentStateRows(rows)
}

func (s *Store) ListAgentStatesByParent(ctx context.Context, parentAgentID string, limit int) ([]AgentState, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT agent_id, role, status, session_id, worktree_path, branch, hook_bead_id, parent_agent_id, last_heartbeat, created_at, updated_at
		 FROM agent_state
		 WHERE parent_agent_id = ?
		 ORDER BY updated_at DESC
		 LIMIT ?`,
		stringsTrim(parentAgentID),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent states by parent: %w", err)
	}
	defer rows.Close()
	return scanAgentStateRows(rows)
}

func (s *Store) ListAgentStatesBySession(ctx context.Context, sessionID string, limit int) ([]AgentState, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT agent_id, role, status, session_id, worktree_path, branch, hook_bead_id, parent_agent_id, last_heartbeat, created_at, updated_at
		 FROM agent_state
		 WHERE session_id = ?
		 ORDER BY updated_at DESC
		 LIMIT ?`,
		stringsTrim(sessionID),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent states by session: %w", err)
	}
	defer rows.Close()
	return scanAgentStateRows(rows)
}

func (s *Store) ListStaleAgentStates(ctx context.Context, staleBefore time.Time, limit int) ([]AgentState, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT agent_id, role, status, session_id, worktree_path, branch, hook_bead_id, parent_agent_id, last_heartbeat, created_at, updated_at
		 FROM agent_state
		 WHERE last_heartbeat > 0 AND last_heartbeat < ?
		 ORDER BY last_heartbeat ASC
		 LIMIT ?`,
		staleBefore.UTC().Unix(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list stale agent states: %w", err)
	}
	defer rows.Close()
	return scanAgentStateRows(rows)
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

func (s *Store) ListRecentActivity(ctx context.Context, agentID string, limit int) ([]AgentActivity, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT rowid, id, agent_id, event_type, details_json, created_at
		 FROM agent_activity
		 WHERE agent_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		stringsTrim(agentID),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent activity: %w", err)
	}
	defer rows.Close()
	return scanActivityRows(rows)
}

func (s *Store) ListActivityFeed(ctx context.Context, agentIDs []string, limit int) ([]AgentActivity, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if len(agentIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	placeholders := make([]string, 0, len(agentIDs))
	args := make([]any, 0, len(agentIDs)+1)
	for _, agentID := range agentIDs {
		agentID = stringsTrim(agentID)
		if agentID == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, agentID)
	}
	if len(placeholders) == 0 {
		return nil, nil
	}
	args = append(args, limit)
	query := fmt.Sprintf(
		`SELECT rowid, id, agent_id, event_type, details_json, created_at
		 FROM agent_activity
		 WHERE agent_id IN (%s)
		 ORDER BY created_at DESC
		 LIMIT ?`,
		strings.Join(placeholders, ","),
	)
	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list activity feed: %w", err)
	}
	defer rows.Close()
	return scanActivityRows(rows)
}

func (s *Store) UpsertWorkItem(ctx context.Context, item WorkItem) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	now := time.Now().UTC()
	if stringsTrim(item.ID) == "" {
		return fmt.Errorf("work item id is required")
	}
	if stringsTrim(item.Type) == "" {
		item.Type = "task"
	}
	if stringsTrim(item.Title) == "" {
		item.Title = item.ID
	}
	if stringsTrim(item.Status) == "" {
		item.Status = "open"
	}
	if stringsTrim(item.Dependencies) == "" {
		item.Dependencies = "[]"
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO work_items (id, type, title, description, status, assignee, parent_id, convoy_id, dependencies, created_at, closed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			title = excluded.title,
			description = excluded.description,
			status = excluded.status,
			assignee = excluded.assignee,
			parent_id = excluded.parent_id,
			convoy_id = excluded.convoy_id,
			dependencies = excluded.dependencies,
			closed_at = excluded.closed_at`,
		stringsTrim(item.ID),
		stringsTrim(item.Type),
		stringsTrim(item.Title),
		item.Description,
		stringsTrim(item.Status),
		stringsTrim(item.Assignee),
		stringsTrim(item.ParentID),
		stringsTrim(item.ConvoyID),
		item.Dependencies,
		item.CreatedAt.UTC().Unix(),
		timeToUnix(item.ClosedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert work item: %w", err)
	}
	return nil
}

func (s *Store) GetWorkItem(ctx context.Context, workItemID string) (WorkItem, error) {
	if s == nil || s.conn == nil {
		return WorkItem{}, fmt.Errorf("orchestration store is not initialized")
	}
	row := s.conn.QueryRowContext(
		ctx,
		`SELECT id, type, title, description, status, assignee, parent_id, convoy_id, dependencies, created_at, closed_at
		 FROM work_items
		 WHERE id = ?`,
		stringsTrim(workItemID),
	)
	item, err := scanWorkItem(row)
	if err != nil {
		return WorkItem{}, fmt.Errorf("get work item: %w", err)
	}
	return item, nil
}

func (s *Store) ListWorkItemsByAssignee(ctx context.Context, assignee string, limit int) ([]WorkItem, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT id, type, title, description, status, assignee, parent_id, convoy_id, dependencies, created_at, closed_at
		 FROM work_items
		 WHERE assignee = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		stringsTrim(assignee),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list work items by assignee: %w", err)
	}
	defer rows.Close()
	return scanWorkItemRows(rows)
}

func (s *Store) ListWorkItemsByStatus(ctx context.Context, statuses []string, limit int) ([]WorkItem, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	statuses = normalizeStringArgs(statuses)
	if len(statuses) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	placeholders := make([]string, 0, len(statuses))
	args := make([]any, 0, len(statuses)+1)
	for _, status := range statuses {
		placeholders = append(placeholders, "?")
		args = append(args, status)
	}
	args = append(args, limit)
	rows, err := s.conn.QueryContext(
		ctx,
		fmt.Sprintf(
			`SELECT id, type, title, description, status, assignee, parent_id, convoy_id, dependencies, created_at, closed_at
			   FROM work_items
			  WHERE status IN (%s)
			  ORDER BY created_at ASC
			  LIMIT ?`,
			strings.Join(placeholders, ","),
		),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list work items by status: %w", err)
	}
	defer rows.Close()
	return scanWorkItemRows(rows)
}

func (s *Store) ListWorkItemsByConvoy(ctx context.Context, convoyID string, limit int) ([]WorkItem, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	convoyID = stringsTrim(convoyID)
	if convoyID == "" {
		return nil, fmt.Errorf("convoy id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT id, type, title, description, status, assignee, parent_id, convoy_id, dependencies, created_at, closed_at
		   FROM work_items
		  WHERE convoy_id = ?
		  ORDER BY created_at ASC
		  LIMIT ?`,
		convoyID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list work items by convoy: %w", err)
	}
	defer rows.Close()
	return scanWorkItemRows(rows)
}

func (s *Store) SaveConvoy(ctx context.Context, convoy Convoy) (Convoy, error) {
	if s == nil || s.conn == nil {
		return Convoy{}, fmt.Errorf("orchestration store is not initialized")
	}
	now := time.Now().UTC()
	if stringsTrim(convoy.ID) == "" {
		convoy.ID = uuid.NewString()
	}
	if stringsTrim(convoy.Name) == "" {
		convoy.Name = convoy.ID
	}
	if stringsTrim(convoy.MergeStrategy) == "" {
		convoy.MergeStrategy = "direct"
	}
	if stringsTrim(convoy.Status) == "" {
		convoy.Status = "open"
	}
	if convoy.CreatedAt.IsZero() {
		convoy.CreatedAt = now
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO convoys (id, name, owner, notify, merge_strategy, status, created_at, closed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name,
		   owner = excluded.owner,
		   notify = excluded.notify,
		   merge_strategy = excluded.merge_strategy,
		   status = excluded.status,
		   closed_at = excluded.closed_at`,
		stringsTrim(convoy.ID),
		stringsTrim(convoy.Name),
		stringsTrim(convoy.Owner),
		stringsTrim(convoy.Notify),
		stringsTrim(convoy.MergeStrategy),
		stringsTrim(convoy.Status),
		convoy.CreatedAt.UTC().Unix(),
		timeToUnix(convoy.ClosedAt),
	)
	if err != nil {
		return Convoy{}, fmt.Errorf("save convoy: %w", err)
	}
	return convoy, nil
}

func (s *Store) GetConvoy(ctx context.Context, convoyID string) (Convoy, error) {
	if s == nil || s.conn == nil {
		return Convoy{}, fmt.Errorf("orchestration store is not initialized")
	}
	row := s.conn.QueryRowContext(
		ctx,
		`SELECT id, name, owner, notify, merge_strategy, status, created_at, closed_at
		   FROM convoys
		  WHERE id = ?`,
		stringsTrim(convoyID),
	)
	item, err := scanConvoy(row)
	if err != nil {
		return Convoy{}, fmt.Errorf("get convoy: %w", err)
	}
	return item, nil
}

func (s *Store) ListConvoys(ctx context.Context, statuses []string, limit int) ([]Convoy, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, name, owner, notify, merge_strategy, status, created_at, closed_at
	            FROM convoys
	           WHERE 1 = 1`
	args := make([]any, 0, len(statuses)+1)
	statuses = normalizeStringArgs(statuses)
	if len(statuses) > 0 {
		placeholders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, status)
		}
		query += ` AND status IN (` + strings.Join(placeholders, ",") + `)`
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list convoys: %w", err)
	}
	defer rows.Close()
	var items []Convoy
	for rows.Next() {
		item, err := scanConvoy(rows)
		if err != nil {
			return nil, fmt.Errorf("scan convoy: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate convoys: %w", err)
	}
	return items, nil
}

func (s *Store) AddConvoyTracks(ctx context.Context, convoyID string, workItemIDs []string) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	convoyID = stringsTrim(convoyID)
	if convoyID == "" {
		return fmt.Errorf("convoy id is required")
	}
	workItemIDs = normalizeStringArgs(workItemIDs)
	if len(workItemIDs) == 0 {
		return nil
	}
	now := time.Now().UTC().Unix()
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin convoy track transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for _, workItemID := range workItemIDs {
		if _, execErr := tx.ExecContext(
			ctx,
			`INSERT INTO convoy_tracks (convoy_id, work_item_id, added_at)
			 VALUES (?, ?, ?)
			 ON CONFLICT(convoy_id, work_item_id) DO NOTHING`,
			convoyID,
			workItemID,
			now,
		); execErr != nil {
			err = fmt.Errorf("add convoy track %s: %w", workItemID, execErr)
			return err
		}
		if _, execErr := tx.ExecContext(
			ctx,
			`UPDATE work_items
			    SET convoy_id = ?
			  WHERE id = ?`,
			convoyID,
			workItemID,
		); execErr != nil {
			err = fmt.Errorf("update work item convoy id %s: %w", workItemID, execErr)
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit convoy track transaction: %w", err)
	}
	return nil
}

func (s *Store) ListConvoyTracks(ctx context.Context, convoyID string) ([]ConvoyTrack, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT convoy_id, work_item_id, added_at
		   FROM convoy_tracks
		  WHERE convoy_id = ?
		  ORDER BY added_at ASC`,
		stringsTrim(convoyID),
	)
	if err != nil {
		return nil, fmt.Errorf("list convoy tracks: %w", err)
	}
	defer rows.Close()
	var items []ConvoyTrack
	for rows.Next() {
		var (
			item      ConvoyTrack
			addedUnix int64
		)
		if err := rows.Scan(&item.ConvoyID, &item.WorkItemID, &addedUnix); err != nil {
			return nil, fmt.Errorf("scan convoy track: %w", err)
		}
		if addedUnix > 0 {
			item.AddedAt = time.Unix(addedUnix, 0).UTC()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate convoy tracks: %w", err)
	}
	return items, nil
}

func (s *Store) UpsertAgentHook(ctx context.Context, hook AgentHook) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	if stringsTrim(hook.AgentID) == "" {
		return fmt.Errorf("agent hook agent_id is required")
	}
	if stringsTrim(hook.Status) == "" {
		hook.Status = "idle"
	}
	if hook.HookedAt.IsZero() && stringsTrim(hook.HookBeadID) != "" {
		hook.HookedAt = time.Now().UTC()
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO agent_hooks (agent_id, hook_bead_id, hooked_at, status)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(agent_id) DO UPDATE SET
		   hook_bead_id = excluded.hook_bead_id,
		   hooked_at = excluded.hooked_at,
		   status = excluded.status`,
		stringsTrim(hook.AgentID),
		stringsTrim(hook.HookBeadID),
		timeToUnix(hook.HookedAt),
		stringsTrim(hook.Status),
	)
	if err != nil {
		return fmt.Errorf("upsert agent hook: %w", err)
	}
	return nil
}

func (s *Store) GetAgentHook(ctx context.Context, agentID string) (AgentHook, error) {
	if s == nil || s.conn == nil {
		return AgentHook{}, fmt.Errorf("orchestration store is not initialized")
	}
	row := s.conn.QueryRowContext(
		ctx,
		`SELECT agent_id, hook_bead_id, hooked_at, status
		   FROM agent_hooks
		  WHERE agent_id = ?`,
		stringsTrim(agentID),
	)
	item, err := scanAgentHook(row)
	if err != nil {
		return AgentHook{}, fmt.Errorf("get agent hook: %w", err)
	}
	return item, nil
}

func (s *Store) ListAgentHooks(ctx context.Context, statuses []string, limit int) ([]AgentHook, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT agent_id, hook_bead_id, hooked_at, status
	            FROM agent_hooks
	           WHERE 1 = 1`
	args := make([]any, 0, len(statuses)+1)
	statuses = normalizeStringArgs(statuses)
	if len(statuses) > 0 {
		placeholders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, status)
		}
		query += ` AND status IN (` + strings.Join(placeholders, ",") + `)`
	}
	query += ` ORDER BY hooked_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agent hooks: %w", err)
	}
	defer rows.Close()
	var items []AgentHook
	for rows.Next() {
		item, err := scanAgentHook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent hook: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent hooks: %w", err)
	}
	return items, nil
}

func (s *Store) EnqueueDispatch(ctx context.Context, item DispatchQueueItem) (DispatchQueueItem, error) {
	if s == nil || s.conn == nil {
		return DispatchQueueItem{}, fmt.Errorf("orchestration store is not initialized")
	}
	now := time.Now().UTC()
	if stringsTrim(item.ID) == "" {
		item.ID = uuid.NewString()
	}
	if stringsTrim(item.SessionID) == "" {
		return DispatchQueueItem{}, fmt.Errorf("dispatch session_id is required")
	}
	if stringsTrim(item.Status) == "" {
		item.Status = "queued"
	}
	if stringsTrim(item.PayloadJSON) == "" {
		item.PayloadJSON = "{}"
	}
	if item.AvailableAt.IsZero() {
		item.AvailableAt = now
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO dispatch_queue (
			id, session_id, work_item_id, target_scope, status, priority, payload_json, retry_count, last_error,
			available_at, leased_by, leased_at, assigned_agent_id, submission_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID,
		stringsTrim(item.SessionID),
		stringsTrim(item.WorkItemID),
		stringsTrim(item.TargetScope),
		stringsTrim(item.Status),
		item.Priority,
		item.PayloadJSON,
		item.RetryCount,
		item.LastError,
		item.AvailableAt.UTC().Unix(),
		stringsTrim(item.LeasedBy),
		timeToUnix(item.LeasedAt),
		stringsTrim(item.AssignedAgentID),
		stringsTrim(item.SubmissionID),
		item.CreatedAt.UTC().Unix(),
		item.UpdatedAt.UTC().Unix(),
	)
	if err != nil {
		return DispatchQueueItem{}, fmt.Errorf("enqueue dispatch: %w", err)
	}
	return item, nil
}

func (s *Store) LeaseDispatch(ctx context.Context, leaseOwner string, limit int) ([]DispatchQueueItem, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	leaseOwner = stringsTrim(leaseOwner)
	if leaseOwner == "" {
		return nil, fmt.Errorf("lease owner is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin dispatch lease transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, session_id, work_item_id, target_scope, status, priority, payload_json, retry_count, last_error,
		        available_at, leased_by, leased_at, assigned_agent_id, submission_id, created_at, updated_at
		   FROM dispatch_queue
		  WHERE status = 'queued' AND available_at <= ?
		  ORDER BY priority ASC, created_at ASC
		  LIMIT ?`,
		time.Now().UTC().Unix(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select dispatch lease candidates: %w", err)
	}
	items, scanErr := scanDispatchRows(rows)
	rows.Close()
	if scanErr != nil {
		err = scanErr
		return nil, err
	}

	leased := make([]DispatchQueueItem, 0, len(items))
	now := time.Now().UTC()
	for _, item := range items {
		result, execErr := tx.ExecContext(
			ctx,
			`UPDATE dispatch_queue
			    SET status = 'leased', leased_by = ?, leased_at = ?, updated_at = ?
			  WHERE id = ? AND status = 'queued'`,
			leaseOwner,
			now.Unix(),
			now.Unix(),
			item.ID,
		)
		if execErr != nil {
			err = fmt.Errorf("lease dispatch item %s: %w", item.ID, execErr)
			return nil, err
		}
		affected, affErr := result.RowsAffected()
		if affErr != nil {
			err = fmt.Errorf("inspect dispatch lease rows affected: %w", affErr)
			return nil, err
		}
		if affected != 1 {
			continue
		}
		item.Status = "leased"
		item.LeasedBy = leaseOwner
		item.LeasedAt = now
		item.UpdatedAt = now
		leased = append(leased, item)
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit dispatch lease transaction: %w", err)
	}
	return leased, nil
}

func (s *Store) UpdateDispatch(ctx context.Context, item DispatchQueueItem) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	if stringsTrim(item.ID) == "" {
		return fmt.Errorf("dispatch id is required")
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now().UTC()
	}
	_, err := s.conn.ExecContext(
		ctx,
		`UPDATE dispatch_queue
		    SET status = ?,
		        retry_count = ?,
		        last_error = ?,
		        available_at = ?,
		        leased_by = ?,
		        leased_at = ?,
		        assigned_agent_id = ?,
		        submission_id = ?,
		        updated_at = ?
		  WHERE id = ?`,
		stringsTrim(item.Status),
		item.RetryCount,
		item.LastError,
		timeToUnix(item.AvailableAt),
		stringsTrim(item.LeasedBy),
		timeToUnix(item.LeasedAt),
		stringsTrim(item.AssignedAgentID),
		stringsTrim(item.SubmissionID),
		item.UpdatedAt.UTC().Unix(),
		item.ID,
	)
	if err != nil {
		return fmt.Errorf("update dispatch item: %w", err)
	}
	return nil
}

func (s *Store) ListDispatches(ctx context.Context, sessionID string, statuses []string, limit int) ([]DispatchQueueItem, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT id, session_id, work_item_id, target_scope, status, priority, payload_json, retry_count, last_error,
	                 available_at, leased_by, leased_at, assigned_agent_id, submission_id, created_at, updated_at
	            FROM dispatch_queue
	           WHERE 1 = 1`
	args := make([]any, 0, len(statuses)+2)
	if sessionID = stringsTrim(sessionID); sessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, sessionID)
	}
	statuses = normalizeStringArgs(statuses)
	if len(statuses) > 0 {
		placeholders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, status)
		}
		query += ` AND status IN (` + strings.Join(placeholders, ",") + `)`
	}
	query += ` ORDER BY priority ASC, created_at ASC LIMIT ?`
	args = append(args, limit)

	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dispatch queue: %w", err)
	}
	defer rows.Close()
	return scanDispatchRows(rows)
}

func (s *Store) ListDispatchesByWorkItem(ctx context.Context, workItemID string, statuses []string, limit int) ([]DispatchQueueItem, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	workItemID = stringsTrim(workItemID)
	if workItemID == "" {
		return nil, fmt.Errorf("work item id is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	query := `SELECT id, session_id, work_item_id, target_scope, status, priority, payload_json, retry_count, last_error,
	                 available_at, leased_by, leased_at, assigned_agent_id, submission_id, created_at, updated_at
	            FROM dispatch_queue
	           WHERE work_item_id = ?`
	args := []any{workItemID}
	statuses = normalizeStringArgs(statuses)
	if len(statuses) > 0 {
		placeholders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, status)
		}
		query += ` AND status IN (` + strings.Join(placeholders, ",") + `)`
	}
	query += ` ORDER BY created_at ASC LIMIT ?`
	args = append(args, limit)
	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dispatches by work item: %w", err)
	}
	defer rows.Close()
	return scanDispatchRows(rows)
}

func (s *Store) UpsertWorktreeRun(ctx context.Context, item WorktreeRun) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	now := time.Now().UTC()
	if stringsTrim(item.ID) == "" {
		item.ID = uuid.NewString()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	if stringsTrim(item.Policy) == "" {
		item.Policy = "shared_repo"
	}
	if stringsTrim(item.MetadataJSON) == "" {
		item.MetadataJSON = "{}"
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO worktree_runs (
			id, session_id, agent_id, parent_agent_id, kind, policy, status, repo_root, worktree_path, branch, base_ref,
			task_key, title, metadata_json, created_at, updated_at, landed_at, removed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			session_id = excluded.session_id,
			agent_id = excluded.agent_id,
			parent_agent_id = excluded.parent_agent_id,
			kind = excluded.kind,
			policy = excluded.policy,
			status = excluded.status,
			repo_root = excluded.repo_root,
			worktree_path = excluded.worktree_path,
			branch = excluded.branch,
			base_ref = excluded.base_ref,
			task_key = excluded.task_key,
			title = excluded.title,
			metadata_json = excluded.metadata_json,
			updated_at = excluded.updated_at,
			landed_at = excluded.landed_at,
			removed_at = excluded.removed_at`,
		stringsTrim(item.ID),
		stringsTrim(item.SessionID),
		stringsTrim(item.AgentID),
		stringsTrim(item.ParentAgentID),
		stringsTrim(item.Kind),
		stringsTrim(item.Policy),
		stringsTrim(item.Status),
		stringsTrim(item.RepoRoot),
		stringsTrim(item.WorktreePath),
		stringsTrim(item.Branch),
		stringsTrim(item.BaseRef),
		stringsTrim(item.TaskKey),
		stringsTrim(item.Title),
		item.MetadataJSON,
		item.CreatedAt.UTC().Unix(),
		item.UpdatedAt.UTC().Unix(),
		timeToUnix(item.LandedAt),
		timeToUnix(item.RemovedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert worktree run: %w", err)
	}
	return nil
}

func (s *Store) GetWorktreeRun(ctx context.Context, id string) (WorktreeRun, error) {
	if s == nil || s.conn == nil {
		return WorktreeRun{}, fmt.Errorf("orchestration store is not initialized")
	}
	row := s.conn.QueryRowContext(
		ctx,
		`SELECT id, session_id, agent_id, parent_agent_id, kind, policy, status, repo_root, worktree_path, branch, base_ref,
		        task_key, title, metadata_json, created_at, updated_at, landed_at, removed_at
		   FROM worktree_runs
		  WHERE id = ?`,
		stringsTrim(id),
	)
	item, err := scanWorktreeRun(row)
	if err != nil {
		return WorktreeRun{}, fmt.Errorf("get worktree run: %w", err)
	}
	return item, nil
}

func (s *Store) GetWorktreeRunByPath(ctx context.Context, worktreePath string) (WorktreeRun, error) {
	if s == nil || s.conn == nil {
		return WorktreeRun{}, fmt.Errorf("orchestration store is not initialized")
	}
	row := s.conn.QueryRowContext(
		ctx,
		`SELECT id, session_id, agent_id, parent_agent_id, kind, policy, status, repo_root, worktree_path, branch, base_ref,
		        task_key, title, metadata_json, created_at, updated_at, landed_at, removed_at
		   FROM worktree_runs
		  WHERE worktree_path = ?
		  ORDER BY updated_at DESC
		  LIMIT 1`,
		stringsTrim(worktreePath),
	)
	item, err := scanWorktreeRun(row)
	if err != nil {
		return WorktreeRun{}, fmt.Errorf("get worktree run by path: %w", err)
	}
	return item, nil
}

func (s *Store) ListWorktreeRuns(ctx context.Context, sessionID string, statuses []string, limit int) ([]WorktreeRun, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, session_id, agent_id, parent_agent_id, kind, policy, status, repo_root, worktree_path, branch, base_ref,
	                 task_key, title, metadata_json, created_at, updated_at, landed_at, removed_at
	            FROM worktree_runs
	           WHERE 1=1`
	args := make([]any, 0, 1+len(statuses))
	if sessionID = stringsTrim(sessionID); sessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, sessionID)
	}
	statuses = normalizeStringArgs(statuses)
	if len(statuses) > 0 {
		query += ` AND status IN (` + strings.TrimRight(strings.Repeat("?,", len(statuses)), ",") + `)`
		for _, status := range statuses {
			args = append(args, status)
		}
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list worktree runs: %w", err)
	}
	defer rows.Close()
	return scanWorktreeRunRows(rows)
}

func (s *Store) SaveCheckpoint(ctx context.Context, checkpoint SessionCheckpoint) (SessionCheckpoint, error) {
	if s == nil || s.conn == nil {
		return SessionCheckpoint{}, fmt.Errorf("orchestration store is not initialized")
	}
	now := time.Now().UTC()
	if stringsTrim(checkpoint.ID) == "" {
		checkpoint.ID = uuid.NewString()
	}
	if stringsTrim(checkpoint.SessionID) == "" {
		return SessionCheckpoint{}, fmt.Errorf("checkpoint session_id is required")
	}
	if stringsTrim(checkpoint.AgentID) == "" {
		return SessionCheckpoint{}, fmt.Errorf("checkpoint agent_id is required")
	}
	if stringsTrim(checkpoint.SummaryJSON) == "" {
		checkpoint.SummaryJSON = "{}"
	}
	if checkpoint.CreatedAt.IsZero() {
		checkpoint.CreatedAt = now
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO session_checkpoints (
			id, session_id, agent_id, work_item_id, parent_checkpoint_id, message_count, summary_json, audit_tail,
			pending_tasks_json, files_modified_json, mail_cursor, activity_cursor, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		checkpoint.ID,
		stringsTrim(checkpoint.SessionID),
		stringsTrim(checkpoint.AgentID),
		stringsTrim(checkpoint.WorkItemID),
		stringsTrim(checkpoint.ParentCheckpointID),
		checkpoint.MessageCount,
		checkpoint.SummaryJSON,
		checkpoint.AuditTail,
		firstNonEmptyJSON(checkpoint.PendingTasksJSON, "[]"),
		firstNonEmptyJSON(checkpoint.FilesModifiedJSON, "[]"),
		checkpoint.MailCursor,
		checkpoint.ActivityCursor,
		checkpoint.CreatedAt.UTC().Unix(),
	)
	if err != nil {
		return SessionCheckpoint{}, fmt.Errorf("save session checkpoint: %w", err)
	}
	return checkpoint, nil
}

func (s *Store) LatestCheckpoint(ctx context.Context, sessionID, agentID string) (SessionCheckpoint, error) {
	if s == nil || s.conn == nil {
		return SessionCheckpoint{}, fmt.Errorf("orchestration store is not initialized")
	}
	query := `SELECT id, session_id, agent_id, work_item_id, parent_checkpoint_id, message_count, summary_json, audit_tail,
	                 pending_tasks_json, files_modified_json, mail_cursor, activity_cursor, created_at
	            FROM session_checkpoints
	           WHERE session_id = ?`
	args := []any{stringsTrim(sessionID)}
	if agentID = stringsTrim(agentID); agentID != "" {
		query += ` AND agent_id = ?`
		args = append(args, agentID)
	}
	query += ` ORDER BY created_at DESC LIMIT 1`
	row := s.conn.QueryRowContext(ctx, query, args...)
	item, err := scanCheckpoint(row)
	if err != nil {
		return SessionCheckpoint{}, fmt.Errorf("get latest checkpoint: %w", err)
	}
	return item, nil
}

func (s *Store) ListCheckpoints(ctx context.Context, sessionID, agentID string, limit int) ([]SessionCheckpoint, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, session_id, agent_id, work_item_id, parent_checkpoint_id, message_count, summary_json, audit_tail,
	                 pending_tasks_json, files_modified_json, mail_cursor, activity_cursor, created_at
	            FROM session_checkpoints
	           WHERE session_id = ?`
	args := []any{stringsTrim(sessionID)}
	if agentID = stringsTrim(agentID); agentID != "" {
		query += ` AND agent_id = ?`
		args = append(args, agentID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	defer rows.Close()
	var items []SessionCheckpoint
	for rows.Next() {
		item, err := scanCheckpoint(rows)
		if err != nil {
			return nil, fmt.Errorf("scan checkpoint: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate checkpoints: %w", err)
	}
	return items, nil
}

func (s *Store) DeleteCheckpoints(ctx context.Context, ids []string) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	ids = normalizeStringArgs(ids)
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	_, err := s.conn.ExecContext(ctx, fmt.Sprintf(`DELETE FROM session_checkpoints WHERE id IN (%s)`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return fmt.Errorf("delete checkpoints: %w", err)
	}
	return nil
}

func (s *Store) SaveDecision(ctx context.Context, item DecisionRecord) (DecisionRecord, error) {
	if s == nil || s.conn == nil {
		return DecisionRecord{}, fmt.Errorf("orchestration store is not initialized")
	}
	if stringsTrim(item.ID) == "" {
		item.ID = uuid.NewString()
	}
	if stringsTrim(item.SessionID) == "" {
		return DecisionRecord{}, fmt.Errorf("decision session_id is required")
	}
	if stringsTrim(item.Category) == "" || stringsTrim(item.Key) == "" || stringsTrim(item.Value) == "" {
		return DecisionRecord{}, fmt.Errorf("decision category, key, and value are required")
	}
	if stringsTrim(item.Confidence) == "" {
		item.Confidence = "tentative"
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO decisions (id, session_id, category, key, value, confidence, source_checkpoint_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID,
		stringsTrim(item.SessionID),
		stringsTrim(item.Category),
		stringsTrim(item.Key),
		stringsTrim(item.Value),
		stringsTrim(item.Confidence),
		stringsTrim(item.SourceCheckpointID),
		item.CreatedAt.UTC().Unix(),
	)
	if err != nil {
		return DecisionRecord{}, fmt.Errorf("save decision: %w", err)
	}
	return item, nil
}

func (s *Store) ListDecisionRecords(ctx context.Context, sessionID string, limit int) ([]DecisionRecord, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT id, session_id, category, key, value, confidence, source_checkpoint_id, created_at
		   FROM decisions
		  WHERE session_id = ?
		  ORDER BY created_at DESC
		  LIMIT ?`,
		stringsTrim(sessionID),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}
	defer rows.Close()
	var items []DecisionRecord
	for rows.Next() {
		item, err := scanDecisionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan decision: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate decisions: %w", err)
	}
	return items, nil
}

func (s *Store) GetUserPreference(ctx context.Context, key string) (UserPreference, error) {
	if s == nil || s.conn == nil {
		return UserPreference{}, fmt.Errorf("orchestration store is not initialized")
	}
	row := s.conn.QueryRowContext(
		ctx,
		`SELECT key, value, confidence, source_session_id, updated_at
		   FROM user_preferences
		  WHERE key = ?`,
		stringsTrim(key),
	)
	item, err := scanUserPreference(row)
	if err != nil {
		return UserPreference{}, fmt.Errorf("get user preference: %w", err)
	}
	return item, nil
}

func (s *Store) UpsertUserPreference(ctx context.Context, item UserPreference) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("orchestration store is not initialized")
	}
	if stringsTrim(item.Key) == "" {
		return fmt.Errorf("user preference key is required")
	}
	if stringsTrim(item.Confidence) == "" {
		item.Confidence = "confirmed"
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now().UTC()
	}
	_, err := s.conn.ExecContext(
		ctx,
		`INSERT INTO user_preferences (key, value, confidence, source_session_id, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		   value = excluded.value,
		   confidence = excluded.confidence,
		   source_session_id = excluded.source_session_id,
		   updated_at = excluded.updated_at`,
		stringsTrim(item.Key),
		stringsTrim(item.Value),
		stringsTrim(item.Confidence),
		stringsTrim(item.SourceSessionID),
		item.UpdatedAt.UTC().Unix(),
	)
	if err != nil {
		return fmt.Errorf("upsert user preference: %w", err)
	}
	return nil
}

func (s *Store) ListUserPreferences(ctx context.Context, limit int) ([]UserPreference, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("orchestration store is not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.conn.QueryContext(
		ctx,
		`SELECT key, value, confidence, source_session_id, updated_at
		   FROM user_preferences
		  ORDER BY updated_at DESC
		  LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list user preferences: %w", err)
	}
	defer rows.Close()
	var items []UserPreference
	for rows.Next() {
		item, err := scanUserPreference(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user preference: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user preferences: %w", err)
	}
	return items, nil
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

func normalizeStringArgs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := stringsTrim(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmptyJSON(value string, fallback string) string {
	value = stringsTrim(value)
	if value == "" {
		return fallback
	}
	return value
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAgentState(row scanner) (AgentState, error) {
	var (
		state             AgentState
		lastHeartbeatUnix int64
		createdAtUnix     int64
		updatedAtUnix     int64
	)
	if err := row.Scan(
		&state.AgentID,
		&state.Role,
		&state.Status,
		&state.SessionID,
		&state.WorktreePath,
		&state.Branch,
		&state.HookBeadID,
		&state.ParentAgentID,
		&lastHeartbeatUnix,
		&createdAtUnix,
		&updatedAtUnix,
	); err != nil {
		return AgentState{}, err
	}
	if lastHeartbeatUnix > 0 {
		state.LastHeartbeat = time.Unix(lastHeartbeatUnix, 0).UTC()
	}
	if createdAtUnix > 0 {
		state.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
	}
	if updatedAtUnix > 0 {
		state.UpdatedAt = time.Unix(updatedAtUnix, 0).UTC()
	}
	return state, nil
}

func scanAgentStateRows(rows *sql.Rows) ([]AgentState, error) {
	var items []AgentState
	for rows.Next() {
		item, err := scanAgentState(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent state: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent states: %w", err)
	}
	return items, nil
}

func scanActivity(row scanner) (AgentActivity, error) {
	var (
		item        AgentActivity
		createdUnix int64
	)
	if err := row.Scan(&item.RowID, &item.ID, &item.AgentID, &item.EventType, &item.DetailsJSON, &createdUnix); err != nil {
		return AgentActivity{}, err
	}
	if createdUnix > 0 {
		item.CreatedAt = time.Unix(createdUnix, 0).UTC()
	}
	return item, nil
}

func scanMail(row scanner) (AgentMail, error) {
	var (
		item             AgentMail
		leaseExpiresUnix int64
		readInt          int
		createdUnix      int64
		readUnix         int64
		ackedUnix        int64
	)
	if err := row.Scan(
		&item.RowID,
		&item.ID,
		&item.Address,
		&item.ToAgent,
		&item.ResolvedToAgent,
		&item.FromAgent,
		&item.Subject,
		&item.Body,
		&item.Priority,
		&item.ThreadID,
		&item.DeliveryState,
		&item.DeliveryAttempts,
		&item.LeaseOwner,
		&leaseExpiresUnix,
		&readInt,
		&createdUnix,
		&readUnix,
		&ackedUnix,
	); err != nil {
		return AgentMail{}, err
	}
	item.Read = readInt != 0
	if item.Address == "" {
		item.Address = item.ToAgent
	}
	if item.ResolvedToAgent == "" {
		item.ResolvedToAgent = item.ToAgent
	}
	if createdUnix > 0 {
		item.CreatedAt = time.Unix(createdUnix, 0).UTC()
	}
	if leaseExpiresUnix > 0 {
		item.LeaseExpiresAt = time.Unix(leaseExpiresUnix, 0).UTC()
	}
	if readUnix > 0 {
		item.ReadAt = time.Unix(readUnix, 0).UTC()
	}
	if ackedUnix > 0 {
		item.AckedAt = time.Unix(ackedUnix, 0).UTC()
	}
	return item, nil
}

func scanMailRows(rows *sql.Rows) ([]AgentMail, error) {
	var items []AgentMail
	for rows.Next() {
		item, err := scanMail(rows)
		if err != nil {
			return nil, fmt.Errorf("scan mail: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mail: %w", err)
	}
	return items, nil
}

func scanActivityRows(rows *sql.Rows) ([]AgentActivity, error) {
	var items []AgentActivity
	for rows.Next() {
		item, err := scanActivity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity: %w", err)
	}
	return items, nil
}

func scanWorkItem(row scanner) (WorkItem, error) {
	var (
		item          WorkItem
		createdAtUnix int64
		closedAtUnix  int64
	)
	if err := row.Scan(
		&item.ID,
		&item.Type,
		&item.Title,
		&item.Description,
		&item.Status,
		&item.Assignee,
		&item.ParentID,
		&item.ConvoyID,
		&item.Dependencies,
		&createdAtUnix,
		&closedAtUnix,
	); err != nil {
		return WorkItem{}, err
	}
	if createdAtUnix > 0 {
		item.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
	}
	if closedAtUnix > 0 {
		item.ClosedAt = time.Unix(closedAtUnix, 0).UTC()
	}
	return item, nil
}

func scanWorkItemRows(rows *sql.Rows) ([]WorkItem, error) {
	var items []WorkItem
	for rows.Next() {
		item, err := scanWorkItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan work item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work items: %w", err)
	}
	return items, nil
}

func scanConvoy(row scanner) (Convoy, error) {
	var (
		item          Convoy
		createdAtUnix int64
		closedAtUnix  int64
	)
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Owner,
		&item.Notify,
		&item.MergeStrategy,
		&item.Status,
		&createdAtUnix,
		&closedAtUnix,
	); err != nil {
		return Convoy{}, err
	}
	if createdAtUnix > 0 {
		item.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
	}
	if closedAtUnix > 0 {
		item.ClosedAt = time.Unix(closedAtUnix, 0).UTC()
	}
	return item, nil
}

func scanAgentHook(row scanner) (AgentHook, error) {
	var (
		item       AgentHook
		hookedUnix int64
	)
	if err := row.Scan(&item.AgentID, &item.HookBeadID, &hookedUnix, &item.Status); err != nil {
		return AgentHook{}, err
	}
	if hookedUnix > 0 {
		item.HookedAt = time.Unix(hookedUnix, 0).UTC()
	}
	return item, nil
}

func scanDispatch(row scanner) (DispatchQueueItem, error) {
	var (
		item            DispatchQueueItem
		availableAtUnix int64
		leasedAtUnix    int64
		createdAtUnix   int64
		updatedAtUnix   int64
	)
	if err := row.Scan(
		&item.ID,
		&item.SessionID,
		&item.WorkItemID,
		&item.TargetScope,
		&item.Status,
		&item.Priority,
		&item.PayloadJSON,
		&item.RetryCount,
		&item.LastError,
		&availableAtUnix,
		&item.LeasedBy,
		&leasedAtUnix,
		&item.AssignedAgentID,
		&item.SubmissionID,
		&createdAtUnix,
		&updatedAtUnix,
	); err != nil {
		return DispatchQueueItem{}, err
	}
	if availableAtUnix > 0 {
		item.AvailableAt = time.Unix(availableAtUnix, 0).UTC()
	}
	if leasedAtUnix > 0 {
		item.LeasedAt = time.Unix(leasedAtUnix, 0).UTC()
	}
	if createdAtUnix > 0 {
		item.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
	}
	if updatedAtUnix > 0 {
		item.UpdatedAt = time.Unix(updatedAtUnix, 0).UTC()
	}
	return item, nil
}

func scanDispatchRows(rows *sql.Rows) ([]DispatchQueueItem, error) {
	var items []DispatchQueueItem
	for rows.Next() {
		item, err := scanDispatch(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dispatch item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dispatch items: %w", err)
	}
	return items, nil
}

func scanCheckpoint(row scanner) (SessionCheckpoint, error) {
	var (
		item          SessionCheckpoint
		createdAtUnix int64
	)
	if err := row.Scan(
		&item.ID,
		&item.SessionID,
		&item.AgentID,
		&item.WorkItemID,
		&item.ParentCheckpointID,
		&item.MessageCount,
		&item.SummaryJSON,
		&item.AuditTail,
		&item.PendingTasksJSON,
		&item.FilesModifiedJSON,
		&item.MailCursor,
		&item.ActivityCursor,
		&createdAtUnix,
	); err != nil {
		return SessionCheckpoint{}, err
	}
	if createdAtUnix > 0 {
		item.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
	}
	return item, nil
}

func scanWorktreeRun(row scanner) (WorktreeRun, error) {
	var (
		item          WorktreeRun
		createdAtUnix int64
		updatedAtUnix int64
		landedAtUnix  int64
		removedAtUnix int64
	)
	if err := row.Scan(
		&item.ID,
		&item.SessionID,
		&item.AgentID,
		&item.ParentAgentID,
		&item.Kind,
		&item.Policy,
		&item.Status,
		&item.RepoRoot,
		&item.WorktreePath,
		&item.Branch,
		&item.BaseRef,
		&item.TaskKey,
		&item.Title,
		&item.MetadataJSON,
		&createdAtUnix,
		&updatedAtUnix,
		&landedAtUnix,
		&removedAtUnix,
	); err != nil {
		return WorktreeRun{}, err
	}
	if createdAtUnix > 0 {
		item.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
	}
	if updatedAtUnix > 0 {
		item.UpdatedAt = time.Unix(updatedAtUnix, 0).UTC()
	}
	if landedAtUnix > 0 {
		item.LandedAt = time.Unix(landedAtUnix, 0).UTC()
	}
	if removedAtUnix > 0 {
		item.RemovedAt = time.Unix(removedAtUnix, 0).UTC()
	}
	return item, nil
}

func scanWorktreeRunRows(rows *sql.Rows) ([]WorktreeRun, error) {
	var items []WorktreeRun
	for rows.Next() {
		item, err := scanWorktreeRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan worktree run: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worktree runs: %w", err)
	}
	return items, nil
}

func scanDecisionRecord(row scanner) (DecisionRecord, error) {
	var (
		item          DecisionRecord
		createdAtUnix int64
	)
	if err := row.Scan(&item.ID, &item.SessionID, &item.Category, &item.Key, &item.Value, &item.Confidence, &item.SourceCheckpointID, &createdAtUnix); err != nil {
		return DecisionRecord{}, err
	}
	if createdAtUnix > 0 {
		item.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
	}
	return item, nil
}

func scanUserPreference(row scanner) (UserPreference, error) {
	var (
		item          UserPreference
		updatedAtUnix int64
	)
	if err := row.Scan(&item.Key, &item.Value, &item.Confidence, &item.SourceSessionID, &updatedAtUnix); err != nil {
		return UserPreference{}, err
	}
	if updatedAtUnix > 0 {
		item.UpdatedAt = time.Unix(updatedAtUnix, 0).UTC()
	}
	return item, nil
}
