package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type MemoryRepoScope struct {
	ID            string
	RepoRoot      string
	ScopePath     string
	Branch        string
	HeadCommit    string
	Dirty         bool
	ChangedFiles  []string
	LatestEpoch   int64
	LastIndexedAt int64
}

type UpsertMemoryRepoScopeParams struct {
	ID            string
	RepoRoot      string
	ScopePath     string
	Branch        string
	HeadCommit    string
	Dirty         bool
	ChangedFiles  []string
	LatestEpoch   int64
	LastIndexedAt int64
	CreatedAt     int64
	UpdatedAt     int64
}

type MemoryRepoFile struct {
	ID          string
	ScopeID     string
	Path        string
	Language    string
	Role        string
	Status      string
	ContentHash string
	ModTimeUnix int64
	SizeBytes   int64
	SymbolCount int64
	Imports     []string
	Facts       map[string]any
	UpdatedAt   int64
	CreatedAt   int64
}

type UpsertMemoryRepoFileParams struct {
	ID          string
	ScopeID     string
	Path        string
	Language    string
	Role        string
	Status      string
	ContentHash string
	ModTimeUnix int64
	SizeBytes   int64
	SymbolCount int
	Imports     []string
	Facts       map[string]any
	UpdatedAt   int64
	CreatedAt   int64
}

type MemoryRepoSymbol struct {
	ID          string
	ScopeID     string
	FileID      string
	StableKey   string
	Name        string
	Kind        string
	Signature   string
	Doc         string
	StartLine   int64
	EndLine     int64
	Exported    bool
	Status      string
	Fingerprint string
	UpdatedAt   int64
	CreatedAt   int64
}

type InsertMemoryRepoSymbolParams struct {
	ID          string
	ScopeID     string
	FileID      string
	StableKey   string
	Name        string
	Kind        string
	Signature   string
	Doc         string
	StartLine   int
	EndLine     int
	Exported    bool
	Status      string
	Fingerprint string
	UpdatedAt   int64
	CreatedAt   int64
}

type MemoryRepoEdge struct {
	ID          string
	ScopeID     string
	FromFile    string
	FromSymbol  string
	EdgeType    string
	ToFile      string
	ToSymbol    string
	ToSymbolKey string
	Metadata    map[string]any
	UpdatedAt   int64
	CreatedAt   int64
}

type InsertMemoryRepoEdgeParams struct {
	ID          string
	ScopeID     string
	FromFile    string
	FromSymbol  string
	EdgeType    string
	ToFile      string
	ToSymbol    string
	ToSymbolKey string
	Metadata    map[string]any
	UpdatedAt   int64
	CreatedAt   int64
}

type InsertMemoryIndexEpochParams struct {
	ID           string
	ScopeID      string
	Epoch        int64
	HeadCommit   string
	ChangedFiles []string
	RemovedFiles []string
	FileCount    int
	Status       string
	CreatedAt    int64
	CompletedAt  int64
}

type MemoryIndexEpoch struct {
	ID           string
	ScopeID      string
	Epoch        int64
	HeadCommit   string
	ChangedFiles []string
	RemovedFiles []string
	FileCount    int64
	Status       string
	CreatedAt    int64
	CompletedAt  int64
}

type InsertMemoryHandoffParams struct {
	ID             string
	SessionID      string
	AgentID        string
	RepoScopeID    string
	CheckpointID   string
	Status         string
	Objective      string
	Plan           []string
	Blockers       []string
	Uncertainties  []string
	TouchedFiles   []string
	TouchedSymbols []string
	SubAgents      []string
	Validation     map[string]any
	RepoSnapshot   map[string]any
	NextActions    []string
	ArtifactPath   string
	CreatedAt      int64
}

type MemoryHandoff struct {
	ID             string
	SessionID      string
	AgentID        string
	RepoScopeID    string
	CheckpointID   string
	Status         string
	Objective      string
	Plan           []string
	Blockers       []string
	Uncertainties  []string
	TouchedFiles   []string
	TouchedSymbols []string
	SubAgents      []string
	Validation     map[string]any
	RepoSnapshot   map[string]any
	NextActions    []string
	ArtifactPath   string
	CreatedAt      int64
}

type InsertMemoryBootPacketParams struct {
	ID            string
	SessionID     string
	AgentID       string
	RepoScopeID   string
	TaskHash      string
	ArtifactPath  string
	RequiredReads []byte
	CreatedAt     int64
}

type MemoryBootPacket struct {
	ID            string
	SessionID     string
	AgentID       string
	RepoScopeID   string
	TaskHash      string
	ArtifactPath  string
	RequiredReads []byte
	CreatedAt     int64
}

type InsertMemoryProvenanceParams struct {
	ID               string
	RepoScopeID      string
	SessionID        string
	AgentID          string
	SourceKind       string
	ArtifactPath     string
	ToolName         string
	ToolOutputRef    string
	HandoffID        string
	SubAgentReportID string
	FilePath         string
	SymbolKey        string
	StartLine        int
	EndLine          int
	HeadCommit       string
	IndexEpoch       int64
	Metadata         map[string]any
	CreatedAt        int64
}

type MemoryProvenance struct {
	ID               string
	RepoScopeID      string
	SessionID        string
	AgentID          string
	SourceKind       string
	ArtifactPath     string
	ToolName         string
	ToolOutputRef    string
	HandoffID        string
	SubAgentReportID string
	FilePath         string
	SymbolKey        string
	StartLine        int64
	EndLine          int64
	HeadCommit       string
	IndexEpoch       int64
	Metadata         map[string]any
	CreatedAt        int64
}

type LinkMemoryFactProvenanceParams struct {
	FactKind     string
	FactID       string
	ProvenanceID string
	CreatedAt    int64
}

type InsertMemorySubAgentReportParams struct {
	ID              string
	SessionID       string
	ParentSessionID string
	AgentID         string
	AssignmentID    string
	SubmissionID    string
	RepoScopeID     string
	Status          string
	Summary         string
	Progress        string
	Risks           string
	Blockers        string
	NextAction      string
	Files           []string
	Commands        []string
	TouchedSymbols  []string
	RawResult       string
	ArtifactPath    string
	CreatedAt       int64
	UpdatedAt       int64
}

type MemorySubAgentReport struct {
	ID              string
	SessionID       string
	ParentSessionID string
	AgentID         string
	AssignmentID    string
	SubmissionID    string
	RepoScopeID     string
	Status          string
	Summary         string
	Progress        string
	Risks           string
	Blockers        string
	NextAction      string
	Files           []string
	Commands        []string
	TouchedSymbols  []string
	RawResult       string
	ArtifactPath    string
	CreatedAt       int64
	UpdatedAt       int64
}

type InsertMemoryFindingParams struct {
	ID             string
	SessionID      string
	AgentID        string
	RepoScopeID    string
	Kind           string
	Title          string
	Content        string
	FilePath       string
	SymbolKey      string
	Status         string
	SourceReportID string
	CreatedAt      int64
	UpdatedAt      int64
}

type MemoryFinding struct {
	ID             string
	SessionID      string
	AgentID        string
	RepoScopeID    string
	Kind           string
	Title          string
	Content        string
	FilePath       string
	SymbolKey      string
	Status         string
	SourceReportID string
	CreatedAt      int64
	UpdatedAt      int64
}

type InsertMemoryResumePointParams struct {
	ID                     string
	SessionID              string
	AgentID                string
	RepoScopeID            string
	HandoffID              string
	BootPacketArtifactPath string
	HandoffArtifactPath    string
	ContinuationPrompt     string
	OriginalPrompt         string
	ResumeReason           string
	Status                 string
	CreatedAt              int64
	ResumedAt              int64
}

type MemoryResumePoint struct {
	ID                     string
	SessionID              string
	AgentID                string
	RepoScopeID            string
	HandoffID              string
	BootPacketArtifactPath string
	HandoffArtifactPath    string
	ContinuationPrompt     string
	OriginalPrompt         string
	ResumeReason           string
	Status                 string
	CreatedAt              int64
	ResumedAt              int64
}

func (q *Queries) GetMemoryRepoScope(ctx context.Context, repoRoot, scopePath, branch string) (MemoryRepoScope, error) {
	row := q.queryRow(ctx, nil, `SELECT id, repo_root, scope_path, branch, head_commit, dirty, changed_files_json, latest_epoch, last_indexed_at
		FROM memory_repo_scopes
		WHERE repo_root = ? AND scope_path = ? AND branch = ?
		LIMIT 1`, repoRoot, scopePath, branch)
	var item MemoryRepoScope
	var dirty int64
	var changedJSON string
	if err := row.Scan(&item.ID, &item.RepoRoot, &item.ScopePath, &item.Branch, &item.HeadCommit, &dirty, &changedJSON, &item.LatestEpoch, &item.LastIndexedAt); err != nil {
		return MemoryRepoScope{}, err
	}
	item.Dirty = dirty != 0
	_ = json.Unmarshal([]byte(changedJSON), &item.ChangedFiles)
	return item, nil
}

func (q *Queries) UpsertMemoryRepoScope(ctx context.Context, arg UpsertMemoryRepoScopeParams) error {
	changedJSON, _ := json.Marshal(arg.ChangedFiles)
	_, err := q.exec(ctx, nil, `INSERT INTO memory_repo_scopes (
		id, repo_root, scope_path, branch, head_commit, dirty, changed_files_json,
		latest_epoch, last_indexed_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(repo_root, scope_path, branch) DO UPDATE SET
		head_commit = excluded.head_commit,
		dirty = excluded.dirty,
		changed_files_json = excluded.changed_files_json,
		latest_epoch = excluded.latest_epoch,
		last_indexed_at = excluded.last_indexed_at,
		updated_at = excluded.updated_at`,
		arg.ID, arg.RepoRoot, arg.ScopePath, arg.Branch, arg.HeadCommit, boolToInt64(arg.Dirty), string(changedJSON),
		arg.LatestEpoch, arg.LastIndexedAt, arg.CreatedAt, arg.UpdatedAt)
	return err
}

func (q *Queries) ListMemoryRepoFilesByScope(ctx context.Context, scopeID string) ([]MemoryRepoFile, error) {
	rows, err := q.query(ctx, nil, `SELECT id, scope_id, path, language, role, status, content_hash, mod_time_unix, size_bytes, symbol_count, imports_json, facts_json, updated_at, created_at
		FROM memory_repo_files WHERE scope_id = ?`, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryRepoFile
	for rows.Next() {
		var item MemoryRepoFile
		var importsJSON, factsJSON string
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Path, &item.Language, &item.Role, &item.Status, &item.ContentHash, &item.ModTimeUnix, &item.SizeBytes, &item.SymbolCount, &importsJSON, &factsJSON, &item.UpdatedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(importsJSON), &item.Imports)
		_ = json.Unmarshal([]byte(factsJSON), &item.Facts)
		if item.Facts == nil {
			item.Facts = map[string]any{}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (q *Queries) UpsertMemoryRepoFile(ctx context.Context, arg UpsertMemoryRepoFileParams) error {
	importsJSON, _ := json.Marshal(arg.Imports)
	factsJSON, _ := json.Marshal(arg.Facts)
	_, err := q.exec(ctx, nil, `INSERT INTO memory_repo_files (
		id, scope_id, path, language, role, status, content_hash, mod_time_unix,
		size_bytes, symbol_count, imports_json, facts_json, updated_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(scope_id, path) DO UPDATE SET
		language = excluded.language,
		role = excluded.role,
		status = excluded.status,
		content_hash = excluded.content_hash,
		mod_time_unix = excluded.mod_time_unix,
		size_bytes = excluded.size_bytes,
		symbol_count = excluded.symbol_count,
		imports_json = excluded.imports_json,
		facts_json = excluded.facts_json,
		updated_at = excluded.updated_at`,
		arg.ID, arg.ScopeID, arg.Path, arg.Language, arg.Role, arg.Status, arg.ContentHash, arg.ModTimeUnix,
		arg.SizeBytes, arg.SymbolCount, string(importsJSON), string(factsJSON), arg.UpdatedAt, arg.CreatedAt)
	return err
}

func (q *Queries) DeleteMemoryRepoFileByScopeAndPath(ctx context.Context, scopeID, path string) error {
	_, err := q.exec(ctx, nil, `DELETE FROM memory_repo_files WHERE scope_id = ? AND path = ?`, scopeID, path)
	return err
}

func (q *Queries) DeleteMemoryRepoSymbolsByFile(ctx context.Context, scopeID, fileID string) error {
	_, err := q.exec(ctx, nil, `DELETE FROM memory_repo_symbols WHERE scope_id = ? AND file_id = ?`, scopeID, fileID)
	return err
}

func (q *Queries) InsertMemoryRepoSymbol(ctx context.Context, arg InsertMemoryRepoSymbolParams) error {
	_, err := q.exec(ctx, nil, `INSERT INTO memory_repo_symbols (
		id, scope_id, file_id, stable_key, name, kind, signature, doc, start_line, end_line,
		exported, status, fingerprint, updated_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.ID, arg.ScopeID, arg.FileID, arg.StableKey, arg.Name, arg.Kind, arg.Signature, arg.Doc,
		arg.StartLine, arg.EndLine, boolToInt64(arg.Exported), arg.Status, arg.Fingerprint, arg.UpdatedAt, arg.CreatedAt)
	return err
}

func (q *Queries) ListMemoryRepoSymbolsByScope(ctx context.Context, scopeID string) ([]MemoryRepoSymbol, error) {
	rows, err := q.query(ctx, nil, `SELECT id, scope_id, file_id, stable_key, name, kind, signature, doc, start_line, end_line, exported, status, fingerprint, updated_at, created_at
		FROM memory_repo_symbols WHERE scope_id = ?`, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryRepoSymbol
	for rows.Next() {
		var item MemoryRepoSymbol
		var exported int64
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.FileID, &item.StableKey, &item.Name, &item.Kind, &item.Signature, &item.Doc, &item.StartLine, &item.EndLine, &exported, &item.Status, &item.Fingerprint, &item.UpdatedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Exported = exported != 0
		out = append(out, item)
	}
	return out, rows.Err()
}

func (q *Queries) DeleteMemoryRepoEdgesForPaths(ctx context.Context, scopeID string, paths []string) error {
	paths = compactStringSlice(paths)
	if len(paths) == 0 {
		return nil
	}
	query, args := buildDynamicInQuery(
		`DELETE FROM memory_repo_edges WHERE scope_id = ? AND (from_file_path IN (%s) OR to_file_path IN (%s))`,
		[]any{scopeID},
		paths,
		paths,
	)
	_, err := q.exec(ctx, nil, query, args...)
	return err
}

func (q *Queries) InsertMemoryRepoEdge(ctx context.Context, arg InsertMemoryRepoEdgeParams) error {
	metadataJSON, _ := json.Marshal(arg.Metadata)
	_, err := q.exec(ctx, nil, `INSERT INTO memory_repo_edges (
		id, scope_id, from_file_path, from_symbol_key, edge_type, to_file_path, to_symbol_name, to_symbol_key,
		metadata_json, updated_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.ID, arg.ScopeID, arg.FromFile, arg.FromSymbol, arg.EdgeType, arg.ToFile, arg.ToSymbol, arg.ToSymbolKey,
		string(metadataJSON), arg.UpdatedAt, arg.CreatedAt)
	return err
}

func (q *Queries) ListMemoryRepoEdgesByScope(ctx context.Context, scopeID string) ([]MemoryRepoEdge, error) {
	rows, err := q.query(ctx, nil, `SELECT id, scope_id, from_file_path, from_symbol_key, edge_type, to_file_path, to_symbol_name, to_symbol_key, metadata_json, updated_at, created_at
		FROM memory_repo_edges WHERE scope_id = ?`, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryRepoEdge
	for rows.Next() {
		var item MemoryRepoEdge
		var metadataJSON string
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.FromFile, &item.FromSymbol, &item.EdgeType, &item.ToFile, &item.ToSymbol, &item.ToSymbolKey, &metadataJSON, &item.UpdatedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(metadataJSON), &item.Metadata)
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (q *Queries) RelinkMemoryRepoEdgesToChangedFiles(ctx context.Context, scopeID string, paths []string) error {
	paths = compactStringSlice(paths)
	if len(paths) == 0 {
		return nil
	}
	query, args := buildDynamicInQuery(
		`UPDATE memory_repo_edges
			SET to_symbol_key = COALESCE((
				SELECT stable_key
				FROM memory_repo_symbols s
				JOIN memory_repo_files f ON f.id = s.file_id
				WHERE s.scope_id = memory_repo_edges.scope_id
				  AND f.path = memory_repo_edges.to_file_path
				  AND lower(s.name) = lower(memory_repo_edges.to_symbol_name)
				ORDER BY s.start_line ASC
				LIMIT 1
			), '')
		  WHERE scope_id = ? AND to_file_path IN (%s)`,
		[]any{scopeID},
		paths,
	)
	_, err := q.exec(ctx, nil, query, args...)
	return err
}

func (q *Queries) InsertMemoryIndexEpoch(ctx context.Context, arg InsertMemoryIndexEpochParams) error {
	changedJSON, _ := json.Marshal(arg.ChangedFiles)
	removedJSON, _ := json.Marshal(arg.RemovedFiles)
	_, err := q.exec(ctx, nil, `INSERT INTO memory_index_epochs (
		id, scope_id, epoch, head_commit, changed_files_json, removed_files_json, file_count, status, created_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.ID, arg.ScopeID, arg.Epoch, arg.HeadCommit, string(changedJSON), string(removedJSON), arg.FileCount, arg.Status, arg.CreatedAt, arg.CompletedAt)
	return err
}

func (q *Queries) ListMemoryIndexEpochsByScope(ctx context.Context, scopeID string) ([]MemoryIndexEpoch, error) {
	rows, err := q.query(ctx, nil, `SELECT id, scope_id, epoch, head_commit, changed_files_json, removed_files_json, file_count, status, created_at, completed_at
		FROM memory_index_epochs WHERE scope_id = ? ORDER BY epoch DESC`, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryIndexEpoch
	for rows.Next() {
		var item MemoryIndexEpoch
		var changedJSON, removedJSON string
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Epoch, &item.HeadCommit, &changedJSON, &removedJSON, &item.FileCount, &item.Status, &item.CreatedAt, &item.CompletedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(changedJSON), &item.ChangedFiles)
		_ = json.Unmarshal([]byte(removedJSON), &item.RemovedFiles)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (q *Queries) DeleteMemoryIndexEpochsByIDs(ctx context.Context, ids []string) error {
	query, args := buildDeleteByIDsQuery(`DELETE FROM memory_index_epochs WHERE id IN (%s)`, ids)
	if query == "" {
		return nil
	}
	_, err := q.exec(ctx, nil, query, args...)
	return err
}

func (q *Queries) InsertMemoryHandoff(ctx context.Context, arg InsertMemoryHandoffParams) error {
	planJSON, _ := json.Marshal(arg.Plan)
	blockersJSON, _ := json.Marshal(arg.Blockers)
	uncertaintiesJSON, _ := json.Marshal(arg.Uncertainties)
	touchedFilesJSON, _ := json.Marshal(arg.TouchedFiles)
	touchedSymbolsJSON, _ := json.Marshal(arg.TouchedSymbols)
	subAgentsJSON, _ := json.Marshal(arg.SubAgents)
	validationJSON, _ := json.Marshal(arg.Validation)
	repoSnapshotJSON, _ := json.Marshal(arg.RepoSnapshot)
	nextActionsJSON, _ := json.Marshal(arg.NextActions)
	_, err := q.exec(ctx, nil, `INSERT INTO memory_handoffs (
		id, session_id, agent_id, repo_scope_id, checkpoint_id, status, objective,
		plan_json, blockers_json, uncertainties_json, touched_files_json, touched_symbols_json,
		subagents_json, validation_json, repo_snapshot_json, next_actions_json, artifact_path, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.ID, arg.SessionID, arg.AgentID, arg.RepoScopeID, arg.CheckpointID, arg.Status, arg.Objective,
		string(planJSON), string(blockersJSON), string(uncertaintiesJSON), string(touchedFilesJSON), string(touchedSymbolsJSON),
		string(subAgentsJSON), string(validationJSON), string(repoSnapshotJSON), string(nextActionsJSON), arg.ArtifactPath, arg.CreatedAt)
	return err
}

func (q *Queries) GetLatestMemoryHandoffBySession(ctx context.Context, sessionID string) (MemoryHandoff, error) {
	row := q.queryRow(ctx, nil, `SELECT id, session_id, agent_id, repo_scope_id, checkpoint_id, status, objective,
		plan_json, blockers_json, uncertainties_json, touched_files_json, touched_symbols_json,
		subagents_json, validation_json, repo_snapshot_json, next_actions_json, artifact_path, created_at
		FROM memory_handoffs WHERE session_id = ? ORDER BY created_at DESC LIMIT 1`, sessionID)
	var item MemoryHandoff
	var planJSON, blockersJSON, uncertaintiesJSON, touchedFilesJSON, touchedSymbolsJSON, subAgentsJSON, validationJSON, repoSnapshotJSON, nextActionsJSON string
	if err := row.Scan(&item.ID, &item.SessionID, &item.AgentID, &item.RepoScopeID, &item.CheckpointID, &item.Status, &item.Objective,
		&planJSON, &blockersJSON, &uncertaintiesJSON, &touchedFilesJSON, &touchedSymbolsJSON,
		&subAgentsJSON, &validationJSON, &repoSnapshotJSON, &nextActionsJSON, &item.ArtifactPath, &item.CreatedAt); err != nil {
		return MemoryHandoff{}, err
	}
	_ = json.Unmarshal([]byte(planJSON), &item.Plan)
	_ = json.Unmarshal([]byte(blockersJSON), &item.Blockers)
	_ = json.Unmarshal([]byte(uncertaintiesJSON), &item.Uncertainties)
	_ = json.Unmarshal([]byte(touchedFilesJSON), &item.TouchedFiles)
	_ = json.Unmarshal([]byte(touchedSymbolsJSON), &item.TouchedSymbols)
	_ = json.Unmarshal([]byte(subAgentsJSON), &item.SubAgents)
	_ = json.Unmarshal([]byte(validationJSON), &item.Validation)
	_ = json.Unmarshal([]byte(repoSnapshotJSON), &item.RepoSnapshot)
	_ = json.Unmarshal([]byte(nextActionsJSON), &item.NextActions)
	return item, nil
}

func (q *Queries) ListMemoryHandoffsBySession(ctx context.Context, sessionID string) ([]MemoryHandoff, error) {
	rows, err := q.query(ctx, nil, `SELECT id, session_id, agent_id, repo_scope_id, checkpoint_id, status, objective,
		plan_json, blockers_json, uncertainties_json, touched_files_json, touched_symbols_json,
		subagents_json, validation_json, repo_snapshot_json, next_actions_json, artifact_path, created_at
		FROM memory_handoffs WHERE session_id = ? ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryHandoff
	for rows.Next() {
		var item MemoryHandoff
		var planJSON, blockersJSON, uncertaintiesJSON, touchedFilesJSON, touchedSymbolsJSON, subAgentsJSON, validationJSON, repoSnapshotJSON, nextActionsJSON string
		if err := rows.Scan(&item.ID, &item.SessionID, &item.AgentID, &item.RepoScopeID, &item.CheckpointID, &item.Status, &item.Objective,
			&planJSON, &blockersJSON, &uncertaintiesJSON, &touchedFilesJSON, &touchedSymbolsJSON,
			&subAgentsJSON, &validationJSON, &repoSnapshotJSON, &nextActionsJSON, &item.ArtifactPath, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(planJSON), &item.Plan)
		_ = json.Unmarshal([]byte(blockersJSON), &item.Blockers)
		_ = json.Unmarshal([]byte(uncertaintiesJSON), &item.Uncertainties)
		_ = json.Unmarshal([]byte(touchedFilesJSON), &item.TouchedFiles)
		_ = json.Unmarshal([]byte(touchedSymbolsJSON), &item.TouchedSymbols)
		_ = json.Unmarshal([]byte(subAgentsJSON), &item.SubAgents)
		_ = json.Unmarshal([]byte(validationJSON), &item.Validation)
		_ = json.Unmarshal([]byte(repoSnapshotJSON), &item.RepoSnapshot)
		_ = json.Unmarshal([]byte(nextActionsJSON), &item.NextActions)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (q *Queries) DeleteMemoryHandoffsByIDs(ctx context.Context, ids []string) error {
	query, args := buildDeleteByIDsQuery(`DELETE FROM memory_handoffs WHERE id IN (%s)`, ids)
	if query == "" {
		return nil
	}
	_, err := q.exec(ctx, nil, query, args...)
	return err
}

func (q *Queries) InsertMemoryBootPacket(ctx context.Context, arg InsertMemoryBootPacketParams) error {
	_, err := q.exec(ctx, nil, `INSERT INTO memory_boot_packets (id, session_id, agent_id, repo_scope_id, task_hash, artifact_path, required_reads_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.ID, arg.SessionID, arg.AgentID, arg.RepoScopeID, arg.TaskHash, arg.ArtifactPath, string(arg.RequiredReads), arg.CreatedAt)
	return err
}

func (q *Queries) ListMemoryBootPacketsBySession(ctx context.Context, sessionID string) ([]MemoryBootPacket, error) {
	rows, err := q.query(ctx, nil, `SELECT id, session_id, agent_id, repo_scope_id, task_hash, artifact_path, required_reads_json, created_at
		FROM memory_boot_packets WHERE session_id = ? ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryBootPacket
	for rows.Next() {
		var item MemoryBootPacket
		var requiredReads string
		if err := rows.Scan(&item.ID, &item.SessionID, &item.AgentID, &item.RepoScopeID, &item.TaskHash, &item.ArtifactPath, &requiredReads, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.RequiredReads = []byte(requiredReads)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (q *Queries) DeleteMemoryBootPacketsByIDs(ctx context.Context, ids []string) error {
	query, args := buildDeleteByIDsQuery(`DELETE FROM memory_boot_packets WHERE id IN (%s)`, ids)
	if query == "" {
		return nil
	}
	_, err := q.exec(ctx, nil, query, args...)
	return err
}

func (q *Queries) InsertMemoryProvenance(ctx context.Context, arg InsertMemoryProvenanceParams) error {
	metadataJSON, _ := json.Marshal(arg.Metadata)
	_, err := q.exec(ctx, nil, `INSERT INTO memory_provenance (
		id, repo_scope_id, session_id, agent_id, source_kind, artifact_path, tool_name, tool_output_ref,
		handoff_id, subagent_report_id, file_path, symbol_key, start_line, end_line, head_commit, index_epoch,
		metadata_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.ID, nullableString(arg.RepoScopeID), arg.SessionID, arg.AgentID, arg.SourceKind, arg.ArtifactPath, arg.ToolName, arg.ToolOutputRef,
		nullableString(arg.HandoffID), nullableString(arg.SubAgentReportID), arg.FilePath, arg.SymbolKey, arg.StartLine, arg.EndLine, arg.HeadCommit, arg.IndexEpoch,
		string(metadataJSON), arg.CreatedAt)
	return err
}

func (q *Queries) LinkMemoryFactProvenance(ctx context.Context, arg LinkMemoryFactProvenanceParams) error {
	_, err := q.exec(ctx, nil, `INSERT OR REPLACE INTO memory_fact_provenance (fact_kind, fact_id, provenance_id, created_at)
		VALUES (?, ?, ?, ?)`, arg.FactKind, arg.FactID, arg.ProvenanceID, arg.CreatedAt)
	return err
}

func (q *Queries) DeleteMemoryFactProvenanceByFact(ctx context.Context, factKind, factID string) error {
	_, err := q.exec(ctx, nil, `DELETE FROM memory_fact_provenance WHERE fact_kind = ? AND fact_id = ?`, factKind, factID)
	return err
}

func (q *Queries) DeleteMemoryFactProvenanceByFacts(ctx context.Context, factKind string, ids []string) error {
	ids = compactStringSlice(ids)
	if len(ids) == 0 {
		return nil
	}
	query, args := buildDynamicInQuery(`DELETE FROM memory_fact_provenance WHERE fact_kind = ? AND fact_id IN (%s)`, []any{factKind}, ids)
	_, err := q.exec(ctx, nil, query, args...)
	return err
}

func (q *Queries) DeleteOrphanMemoryProvenance(ctx context.Context) error {
	_, err := q.exec(ctx, nil, `DELETE FROM memory_provenance WHERE id NOT IN (SELECT DISTINCT provenance_id FROM memory_fact_provenance)`)
	return err
}

func (q *Queries) InsertMemorySubAgentReport(ctx context.Context, arg InsertMemorySubAgentReportParams) error {
	filesJSON, _ := json.Marshal(arg.Files)
	commandsJSON, _ := json.Marshal(arg.Commands)
	touchedSymbolsJSON, _ := json.Marshal(arg.TouchedSymbols)
	_, err := q.exec(ctx, nil, `INSERT INTO memory_subagent_reports (
		id, session_id, parent_session_id, agent_id, assignment_id, submission_id, repo_scope_id,
		status, summary, progress, risks, blockers, next_action, files_json, commands_json,
		touched_symbols_json, raw_result, artifact_path, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.ID, arg.SessionID, arg.ParentSessionID, arg.AgentID, arg.AssignmentID, arg.SubmissionID, arg.RepoScopeID,
		arg.Status, arg.Summary, arg.Progress, arg.Risks, arg.Blockers, arg.NextAction, string(filesJSON), string(commandsJSON),
		string(touchedSymbolsJSON), arg.RawResult, arg.ArtifactPath, arg.CreatedAt, arg.UpdatedAt)
	return err
}

func (q *Queries) ListMemorySubAgentReportsBySession(ctx context.Context, sessionID string, limit int) ([]MemorySubAgentReport, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := q.query(ctx, nil, `SELECT id, session_id, parent_session_id, agent_id, assignment_id, submission_id, repo_scope_id,
		status, summary, progress, risks, blockers, next_action, files_json, commands_json,
		touched_symbols_json, raw_result, artifact_path, created_at, updated_at
		FROM memory_subagent_reports WHERE session_id = ? OR parent_session_id = ? ORDER BY created_at DESC LIMIT ?`, sessionID, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemorySubAgentReport
	for rows.Next() {
		var item MemorySubAgentReport
		var filesJSON, commandsJSON, touchedSymbolsJSON string
		if err := rows.Scan(&item.ID, &item.SessionID, &item.ParentSessionID, &item.AgentID, &item.AssignmentID, &item.SubmissionID, &item.RepoScopeID,
			&item.Status, &item.Summary, &item.Progress, &item.Risks, &item.Blockers, &item.NextAction, &filesJSON, &commandsJSON,
			&touchedSymbolsJSON, &item.RawResult, &item.ArtifactPath, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(filesJSON), &item.Files)
		_ = json.Unmarshal([]byte(commandsJSON), &item.Commands)
		_ = json.Unmarshal([]byte(touchedSymbolsJSON), &item.TouchedSymbols)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (q *Queries) DeleteMemorySubAgentReportsByIDs(ctx context.Context, ids []string) error {
	query, args := buildDeleteByIDsQuery(`DELETE FROM memory_subagent_reports WHERE id IN (%s)`, ids)
	if query == "" {
		return nil
	}
	_, err := q.exec(ctx, nil, query, args...)
	return err
}

func (q *Queries) InsertMemoryFinding(ctx context.Context, arg InsertMemoryFindingParams) error {
	_, err := q.exec(ctx, nil, `INSERT INTO memory_findings (
		id, session_id, agent_id, repo_scope_id, kind, title, content, file_path, symbol_key, status, source_report_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.ID, arg.SessionID, arg.AgentID, nullableString(arg.RepoScopeID), arg.Kind, arg.Title, arg.Content, arg.FilePath, arg.SymbolKey, arg.Status, nullableString(arg.SourceReportID), arg.CreatedAt, arg.UpdatedAt)
	return err
}

func (q *Queries) ListMemoryFindingsBySession(ctx context.Context, sessionID string, limit int) ([]MemoryFinding, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := q.query(ctx, nil, `SELECT id, session_id, agent_id, repo_scope_id, kind, title, content, file_path, symbol_key, status, source_report_id, created_at, updated_at
		FROM memory_findings WHERE session_id = ? ORDER BY created_at DESC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryFinding
	for rows.Next() {
		var item MemoryFinding
		var repoScopeID sql.NullString
		var sourceReportID sql.NullString
		if err := rows.Scan(&item.ID, &item.SessionID, &item.AgentID, &repoScopeID, &item.Kind, &item.Title, &item.Content, &item.FilePath, &item.SymbolKey, &item.Status, &sourceReportID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.RepoScopeID = repoScopeID.String
		item.SourceReportID = sourceReportID.String
		out = append(out, item)
	}
	return out, rows.Err()
}

func (q *Queries) DeleteMemoryFindingsByIDs(ctx context.Context, ids []string) error {
	query, args := buildDeleteByIDsQuery(`DELETE FROM memory_findings WHERE id IN (%s)`, ids)
	if query == "" {
		return nil
	}
	_, err := q.exec(ctx, nil, query, args...)
	return err
}

func (q *Queries) InsertMemoryResumePoint(ctx context.Context, arg InsertMemoryResumePointParams) error {
	_, err := q.exec(ctx, nil, `INSERT INTO memory_resume_points (
		id, session_id, agent_id, repo_scope_id, handoff_id, boot_packet_artifact_path, handoff_artifact_path,
		continuation_prompt, original_prompt, resume_reason, status, created_at, resumed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arg.ID, arg.SessionID, arg.AgentID, arg.RepoScopeID, arg.HandoffID, arg.BootPacketArtifactPath, arg.HandoffArtifactPath,
		arg.ContinuationPrompt, arg.OriginalPrompt, arg.ResumeReason, arg.Status, arg.CreatedAt, arg.ResumedAt)
	return err
}

func (q *Queries) GetMemoryResumePoint(ctx context.Context, id string) (MemoryResumePoint, error) {
	row := q.queryRow(ctx, nil, `SELECT id, session_id, agent_id, repo_scope_id, handoff_id, boot_packet_artifact_path, handoff_artifact_path,
		continuation_prompt, original_prompt, resume_reason, status, created_at, resumed_at
		FROM memory_resume_points WHERE id = ? LIMIT 1`, id)
	var item MemoryResumePoint
	if err := row.Scan(&item.ID, &item.SessionID, &item.AgentID, &item.RepoScopeID, &item.HandoffID, &item.BootPacketArtifactPath, &item.HandoffArtifactPath,
		&item.ContinuationPrompt, &item.OriginalPrompt, &item.ResumeReason, &item.Status, &item.CreatedAt, &item.ResumedAt); err != nil {
		return MemoryResumePoint{}, err
	}
	return item, nil
}

func (q *Queries) GetLatestPendingMemoryResumePointBySession(ctx context.Context, sessionID string) (MemoryResumePoint, error) {
	row := q.queryRow(ctx, nil, `SELECT id, session_id, agent_id, repo_scope_id, handoff_id, boot_packet_artifact_path, handoff_artifact_path,
		continuation_prompt, original_prompt, resume_reason, status, created_at, resumed_at
		FROM memory_resume_points
		WHERE session_id = ? AND status = 'pending'
		ORDER BY created_at DESC
		LIMIT 1`, sessionID)
	var item MemoryResumePoint
	if err := row.Scan(&item.ID, &item.SessionID, &item.AgentID, &item.RepoScopeID, &item.HandoffID, &item.BootPacketArtifactPath, &item.HandoffArtifactPath,
		&item.ContinuationPrompt, &item.OriginalPrompt, &item.ResumeReason, &item.Status, &item.CreatedAt, &item.ResumedAt); err != nil {
		return MemoryResumePoint{}, err
	}
	return item, nil
}

func (q *Queries) ListMemoryResumePointsBySession(ctx context.Context, sessionID string) ([]MemoryResumePoint, error) {
	rows, err := q.query(ctx, nil, `SELECT id, session_id, agent_id, repo_scope_id, handoff_id, boot_packet_artifact_path, handoff_artifact_path,
		continuation_prompt, original_prompt, resume_reason, status, created_at, resumed_at
		FROM memory_resume_points WHERE session_id = ? ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryResumePoint
	for rows.Next() {
		var item MemoryResumePoint
		if err := rows.Scan(&item.ID, &item.SessionID, &item.AgentID, &item.RepoScopeID, &item.HandoffID, &item.BootPacketArtifactPath, &item.HandoffArtifactPath,
			&item.ContinuationPrompt, &item.OriginalPrompt, &item.ResumeReason, &item.Status, &item.CreatedAt, &item.ResumedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (q *Queries) MarkMemoryResumePointResumed(ctx context.Context, id string, resumedAt int64) error {
	_, err := q.exec(ctx, nil, `UPDATE memory_resume_points SET status = 'resumed', resumed_at = ? WHERE id = ?`, resumedAt, id)
	return err
}

func (q *Queries) DeleteMemoryResumePointsByIDs(ctx context.Context, ids []string) error {
	query, args := buildDeleteByIDsQuery(`DELETE FROM memory_resume_points WHERE id IN (%s)`, ids)
	if query == "" {
		return nil
	}
	_, err := q.exec(ctx, nil, query, args...)
	return err
}

func compactStringSlice(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func buildDeleteByIDsQuery(pattern string, ids []string) (string, []any) {
	ids = compactStringSlice(ids)
	if len(ids) == 0 {
		return "", nil
	}
	args := make([]any, 0, len(ids))
	holders := make([]string, 0, len(ids))
	for _, id := range ids {
		holders = append(holders, "?")
		args = append(args, id)
	}
	return fmt.Sprintf(pattern, strings.Join(holders, ", ")), args
}

func buildDynamicInQuery(pattern string, prefixArgs []any, groups ...[]string) (string, []any) {
	args := append([]any{}, prefixArgs...)
	parts := make([]any, 0, len(groups))
	for _, group := range groups {
		group = compactStringSlice(group)
		holders := make([]string, 0, len(group))
		for _, item := range group {
			holders = append(holders, "?")
			args = append(args, item)
		}
		parts = append(parts, strings.Join(holders, ", "))
	}
	return fmt.Sprintf(pattern, parts...), args
}

func boolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return value
}
