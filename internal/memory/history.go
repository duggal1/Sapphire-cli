package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

const (
	defaultRecentEntryCount = 12
	maxViewMemoryEntries    = 5000
	maxEntriesPerStore      = 10000
)

const (
	historyKindUserPrompt        = "user_prompt"
	historyKindAssistantResponse = "assistant_response"
	historyKindToolCall          = "tool_call"
	historyKindToolResult        = "tool_result"
	historyKindDecision          = "decision"
)

const (
	metaSessionIDKey     = "meta/session_id"
	metaStoreIDKey       = "meta/store_id"
	metaParentStoreKey   = "meta/parent_store_id"
	metaStoreIndexKey    = "meta/store_index"
	metaEntryCountKey    = "meta/entry_count"
	metaCurrentTurnKey   = "meta/current_turn"
	metaCreatedAtKey     = "meta/created_at"
	metaUpdatedAtKey     = "meta/updated_at"
	metaSessionStatusKey = "meta/session_status"
)

type SessionHistoryEntry struct {
	EntryIndex uint64         `json:"entry_index"`
	TurnIndex  uint64         `json:"turn_index"`
	Timestamp  int64          `json:"timestamp"`
	SessionID  string         `json:"session_id"`
	Source     string         `json:"source"`
	Role       string         `json:"role"`
	Kind       string         `json:"kind"`
	ToolName   string         `json:"tool_name,omitempty"`
	Content    string         `json:"content"`
	IsError    bool           `json:"is_error,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ViewMemoryParams struct {
	Mode      string `json:"mode,omitempty" description:"recent, full_session, by_session, search, since, or decisions. Defaults to recent."`
	SessionID string `json:"session_id,omitempty" description:"Optional target session ID. Defaults to the current session."`
	Limit     int    `json:"limit,omitempty" description:"Maximum entries to return. Defaults to 12 for recent and 50 for search/since/decisions."`
	Query     string `json:"query,omitempty" description:"Search query used by search mode."`
	Since     string `json:"since,omitempty" description:"RFC3339 timestamp or unix seconds used by since mode."`
}

type ViewMemoryResult struct {
	Mode      string                `json:"mode"`
	SessionID string                `json:"session_id"`
	Sources   []string              `json:"sources"`
	Entries   []SessionHistoryEntry `json:"entries"`
}

type sessionHistoryManager struct {
	rootDir     string
	projectRoot string

	mu  sync.Mutex
	dbs map[string]*badger.DB
}

func newSessionHistoryManager(dataDir, projectRoot string) (*sessionHistoryManager, error) {
	rootDir := filepath.Join(dataDir, "memory", "sessions")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("memory history: create root dir: %w", err)
	}
	return &sessionHistoryManager{
		rootDir:     rootDir,
		projectRoot: projectRoot,
		dbs:         make(map[string]*badger.DB),
	}, nil
}

func (m *sessionHistoryManager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var closeErr error
	for storeID, db := range m.dbs {
		if db == nil {
			continue
		}
		if err := db.Close(); err != nil && closeErr == nil {
			closeErr = fmt.Errorf("close %s: %w", storeID, err)
		}
	}
	m.dbs = make(map[string]*badger.DB)
	return closeErr
}

func (m *sessionHistoryManager) RecordUserPrompt(ctx context.Context, sessionID, prompt string) error {
	return m.writeEntry(ctx, sessionID, historyKindUserPrompt, "user", "", prompt, false, nil, true)
}

func (m *sessionHistoryManager) RecordAssistantResponse(ctx context.Context, sessionID, content string) error {
	return m.writeEntry(ctx, sessionID, historyKindAssistantResponse, "assistant", "", content, false, nil, false)
}

func (m *sessionHistoryManager) RecordToolCall(ctx context.Context, sessionID, toolName, input string) error {
	return m.writeEntry(ctx, sessionID, historyKindToolCall, "assistant", toolName, input, false, nil, false)
}

func (m *sessionHistoryManager) RecordToolResult(ctx context.Context, sessionID, toolName, output string, isError bool) error {
	return m.writeEntry(ctx, sessionID, historyKindToolResult, "tool", toolName, output, isError, nil, false)
}

func (m *sessionHistoryManager) RecordDecision(ctx context.Context, sessionID, label, content string) error {
	meta := map[string]any{}
	if strings.TrimSpace(label) != "" {
		meta["label"] = label
	}
	return m.writeEntry(ctx, sessionID, historyKindDecision, "system", "", content, false, meta, false)
}

func (m *sessionHistoryManager) MarkSessionComplete(ctx context.Context, sessionID string) error {
	storeID, db, err := m.latestStore(ctx, sessionID)
	if err != nil {
		return err
	}
	return db.Update(func(txn *badger.Txn) error {
		if err := txn.Set([]byte(metaSessionStatusKey), []byte("complete")); err != nil {
			return err
		}
		if err := txn.Set([]byte(metaUpdatedAtKey), encodeUint64(uint64(timeNow().Unix()))); err != nil {
			return err
		}
		_ = storeID
		return nil
	})
}

func (m *sessionHistoryManager) CurrentTurn(ctx context.Context, sessionID string) (uint64, error) {
	_, db, err := m.latestStore(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	var turn uint64
	err = db.View(func(txn *badger.Txn) error {
		value, err := readUint64(txn, metaCurrentTurnKey)
		if err != nil {
			return err
		}
		turn = value
		return nil
	})
	if errors.Is(err, badger.ErrKeyNotFound) {
		return 0, nil
	}
	return turn, err
}

func (m *sessionHistoryManager) View(ctx context.Context, currentSessionID string, params ViewMemoryParams) (ViewMemoryResult, error) {
	mode := strings.TrimSpace(strings.ToLower(params.Mode))
	if mode == "" {
		mode = "recent"
	}

	targetSessionID := strings.TrimSpace(params.SessionID)
	if targetSessionID == "" {
		targetSessionID = strings.TrimSpace(currentSessionID)
	}
	if targetSessionID == "" {
		return ViewMemoryResult{}, fmt.Errorf("session_id is required")
	}

	entries, err := m.readEntries(ctx, targetSessionID)
	if err != nil {
		return ViewMemoryResult{}, err
	}

	limit := params.Limit
	if limit <= 0 {
		switch mode {
		case "recent":
			limit = defaultRecentEntryCount
		case "full_session", "by_session":
			limit = 0
		default:
			limit = 50
		}
	}
	if limit > maxViewMemoryEntries {
		limit = maxViewMemoryEntries
	}

	filtered := entries
	switch mode {
	case "recent":
		if len(filtered) > limit {
			filtered = filtered[len(filtered)-limit:]
		}
	case "full_session", "by_session":
		if len(filtered) > limit && params.Limit > 0 {
			filtered = filtered[:limit]
		}
	case "search":
		query := strings.ToLower(strings.TrimSpace(params.Query))
		if query == "" {
			return ViewMemoryResult{}, fmt.Errorf("query is required for search mode")
		}
		filtered = filterHistoryEntries(filtered, func(entry SessionHistoryEntry) bool {
			if strings.Contains(strings.ToLower(entry.Content), query) {
				return true
			}
			if strings.Contains(strings.ToLower(entry.ToolName), query) {
				return true
			}
			meta, _ := json.Marshal(entry.Metadata)
			return strings.Contains(strings.ToLower(string(meta)), query)
		})
		if len(filtered) > limit {
			filtered = filtered[len(filtered)-limit:]
		}
	case "since":
		since, err := parseSince(params.Since)
		if err != nil {
			return ViewMemoryResult{}, err
		}
		filtered = filterHistoryEntries(filtered, func(entry SessionHistoryEntry) bool {
			return entry.Timestamp >= since.Unix()
		})
		if len(filtered) > limit {
			filtered = filtered[len(filtered)-limit:]
		}
	case "decisions":
		filtered = filterHistoryEntries(filtered, func(entry SessionHistoryEntry) bool {
			return entry.Kind == historyKindDecision
		})
		if len(filtered) > limit {
			filtered = filtered[len(filtered)-limit:]
		}
	default:
		return ViewMemoryResult{}, fmt.Errorf("unsupported mode %q", mode)
	}

	sources := uniqueStrings(extractSources(filtered))
	return ViewMemoryResult{
		Mode:      mode,
		SessionID: targetSessionID,
		Sources:   sources,
		Entries:   filtered,
	}, nil
}

type sessionStateSnapshot struct {
	CurrentTask      string
	LastDecision     string
	LastModifiedFile string
	RecentTools      []string
}

func (m *sessionHistoryManager) BuildSessionState(ctx context.Context, sessionID string) (sessionStateSnapshot, error) {
	entries, err := m.readEntries(ctx, sessionID)
	if err != nil {
		return sessionStateSnapshot{}, err
	}

	var state sessionStateSnapshot
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		switch {
		case state.CurrentTask == "" && entry.Kind == historyKindUserPrompt:
			state.CurrentTask = truncate(entry.Content, 240)
		case state.LastDecision == "" && entry.Kind == historyKindDecision:
			state.LastDecision = truncate(entry.Content, 220)
		case state.LastModifiedFile == "" && (entry.Kind == historyKindToolCall || entry.Kind == historyKindToolResult):
			state.LastModifiedFile = inferLastModifiedFile(entry.Content)
		}
		if entry.ToolName != "" {
			state.RecentTools = append(state.RecentTools, entry.ToolName)
			if len(state.RecentTools) >= 5 {
				break
			}
		}
	}
	state.RecentTools = uniqueStrings(state.RecentTools)
	return state, nil
}

func (m *sessionHistoryManager) writeEntry(ctx context.Context, sessionID, kind, role, toolName, content string, isError bool, metadata map[string]any, incrementTurn bool) error {
	sessionID = strings.TrimSpace(sessionID)
	content = strings.TrimSpace(content)
	if sessionID == "" || content == "" {
		return nil
	}

	storeID, db, err := m.writableStore(ctx, sessionID, incrementTurn)
	if err != nil {
		return err
	}

	now := timeNow().Unix()
	return db.Update(func(txn *badger.Txn) error {
		currentTurn, err := readUint64(txn, metaCurrentTurnKey)
		if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		if incrementTurn {
			currentTurn++
			if err := txn.Set([]byte(metaCurrentTurnKey), encodeUint64(currentTurn)); err != nil {
				return err
			}
		}

		entryCount, err := readUint64(txn, metaEntryCountKey)
		if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		entryCount++
		if err := txn.Set([]byte(metaEntryCountKey), encodeUint64(entryCount)); err != nil {
			return err
		}
		if err := txn.Set([]byte(metaUpdatedAtKey), encodeUint64(uint64(now))); err != nil {
			return err
		}
		if err := txn.Set([]byte(metaSessionStatusKey), []byte("active")); err != nil {
			return err
		}

		entry := SessionHistoryEntry{
			EntryIndex: entryCount,
			TurnIndex:  currentTurn,
			Timestamp:  now,
			SessionID:  sessionID,
			Source:     storeID,
			Role:       role,
			Kind:       kind,
			ToolName:   toolName,
			Content:    content,
			IsError:    isError,
			Metadata:   metadata,
		}
		payload, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return txn.Set([]byte(entryKey(entryCount)), payload)
	})
}

func (m *sessionHistoryManager) writableStore(ctx context.Context, sessionID string, allowContinuation bool) (string, *badger.DB, error) {
	storeID, db, err := m.latestStore(ctx, sessionID)
	if err != nil {
		return "", nil, err
	}
	if !allowContinuation {
		return storeID, db, nil
	}

	var entryCount uint64
	if err := db.View(func(txn *badger.Txn) error {
		value, err := readUint64(txn, metaEntryCountKey)
		if errors.Is(err, badger.ErrKeyNotFound) {
			entryCount = 0
			return nil
		}
		if err != nil {
			return err
		}
		entryCount = value
		return nil
	}); err != nil {
		return "", nil, err
	}

	if entryCount < maxEntriesPerStore {
		return storeID, db, nil
	}

	storeIDs, err := m.storeIDsForSession(sessionID)
	if err != nil {
		return "", nil, err
	}
	nextIndex := len(storeIDs)

	var currentTurn uint64
	if err := db.View(func(txn *badger.Txn) error {
		value, err := readUint64(txn, metaCurrentTurnKey)
		if errors.Is(err, badger.ErrKeyNotFound) {
			currentTurn = 0
			return nil
		}
		if err != nil {
			return err
		}
		currentTurn = value
		return nil
	}); err != nil {
		return "", nil, err
	}

	nextStoreID := continuationStoreID(sessionID, nextIndex)
	nextDB, err := m.ensureStore(ctx, nextStoreID, sessionID, storeID, nextIndex, currentTurn)
	if err != nil {
		return "", nil, err
	}
	return nextStoreID, nextDB, nil
}

func (m *sessionHistoryManager) latestStore(ctx context.Context, sessionID string) (string, *badger.DB, error) {
	storeIDs, err := m.storeIDsForSession(sessionID)
	if err != nil {
		return "", nil, err
	}
	if len(storeIDs) == 0 {
		db, err := m.ensureStore(ctx, sessionID, sessionID, "", 0, 0)
		if err != nil {
			return "", nil, err
		}
		return sessionID, db, nil
	}
	latestID := storeIDs[len(storeIDs)-1]
	db, err := m.openStore(latestID)
	if err != nil {
		return "", nil, err
	}
	return latestID, db, nil
}

func (m *sessionHistoryManager) storeIDsForSession(sessionID string) ([]string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(m.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type indexedStore struct {
		id    string
		index int
	}
	var stores []indexedStore
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch {
		case name == sessionID:
			stores = append(stores, indexedStore{id: name, index: 0})
		case strings.HasPrefix(name, sessionID+"--cont-"):
			suffix := strings.TrimPrefix(name, sessionID+"--cont-")
			index, err := strconv.Atoi(suffix)
			if err != nil {
				continue
			}
			stores = append(stores, indexedStore{id: name, index: index})
		}
	}
	sort.Slice(stores, func(i, j int) bool { return stores[i].index < stores[j].index })

	result := make([]string, 0, len(stores))
	for _, store := range stores {
		result = append(result, store.id)
	}
	return result, nil
}

func (m *sessionHistoryManager) ensureStore(ctx context.Context, storeID, sessionID, parentStoreID string, storeIndex int, currentTurn uint64) (*badger.DB, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if db, ok := m.dbs[storeID]; ok && db != nil {
		return db, nil
	}

	storePath := filepath.Join(m.rootDir, storeID)
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		return nil, err
	}

	opts := badger.DefaultOptions(storePath)
	opts.Logger = nil
	opts.DetectConflicts = false

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	if err := db.Update(func(txn *badger.Txn) error {
		if err := ensureMetadata(txn, metaSessionIDKey, []byte(sessionID)); err != nil {
			return err
		}
		if err := ensureMetadata(txn, metaStoreIDKey, []byte(storeID)); err != nil {
			return err
		}
		if err := ensureMetadata(txn, metaParentStoreKey, []byte(parentStoreID)); err != nil {
			return err
		}
		if err := ensureMetadata(txn, metaStoreIndexKey, encodeUint64(uint64(storeIndex))); err != nil {
			return err
		}
		if err := ensureMetadata(txn, metaEntryCountKey, encodeUint64(0)); err != nil {
			return err
		}
		if err := ensureMetadata(txn, metaCurrentTurnKey, encodeUint64(currentTurn)); err != nil {
			return err
		}
		now := encodeUint64(uint64(timeNow().Unix()))
		if err := ensureMetadata(txn, metaCreatedAtKey, now); err != nil {
			return err
		}
		if err := ensureMetadata(txn, metaUpdatedAtKey, now); err != nil {
			return err
		}
		return ensureMetadata(txn, metaSessionStatusKey, []byte("active"))
	}); err != nil {
		_ = db.Close()
		return nil, err
	}

	m.dbs[storeID] = db
	return db, nil
}

func (m *sessionHistoryManager) openStore(storeID string) (*badger.DB, error) {
	m.mu.Lock()
	if db, ok := m.dbs[storeID]; ok && db != nil {
		m.mu.Unlock()
		return db, nil
	}
	m.mu.Unlock()
	return m.ensureStore(context.Background(), storeID, baseSessionID(storeID), "", continuationIndex(storeID), 0)
}

func (m *sessionHistoryManager) readEntries(ctx context.Context, sessionID string) ([]SessionHistoryEntry, error) {
	storeIDs, err := m.storeIDsForSession(sessionID)
	if err != nil {
		return nil, err
	}
	if len(storeIDs) == 0 {
		return nil, nil
	}

	var entries []SessionHistoryEntry
	for _, storeID := range storeIDs {
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		db, err := m.openStore(storeID)
		if err != nil {
			return nil, err
		}
		if err := db.View(func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()
			prefix := []byte("entries/")
			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				err := item.Value(func(val []byte) error {
					var entry SessionHistoryEntry
					if err := json.Unmarshal(val, &entry); err != nil {
						return err
					}
					if entry.Source == "" {
						entry.Source = storeID
					}
					if entry.SessionID == "" {
						entry.SessionID = sessionID
					}
					entries = append(entries, entry)
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Timestamp == entries[j].Timestamp {
			if entries[i].TurnIndex == entries[j].TurnIndex {
				if entries[i].Source == entries[j].Source {
					return entries[i].EntryIndex < entries[j].EntryIndex
				}
				return entries[i].Source < entries[j].Source
			}
			return entries[i].TurnIndex < entries[j].TurnIndex
		}
		return entries[i].Timestamp < entries[j].Timestamp
	})
	return entries, nil
}

func parseSince(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("since is required for since mode")
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid since value %q: use RFC3339 or unix seconds", raw)
	}
	return parsed.UTC(), nil
}

func filterHistoryEntries(entries []SessionHistoryEntry, keep func(SessionHistoryEntry) bool) []SessionHistoryEntry {
	filtered := make([]SessionHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		if keep(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func extractSources(entries []SessionHistoryEntry) []string {
	sources := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Source) != "" {
			sources = append(sources, entry.Source)
		}
	}
	return sources
}

func inferLastModifiedFile(raw string) string {
	for _, field := range strings.Fields(raw) {
		field = strings.Trim(field, "\"',()[]{}")
		if field == "" {
			continue
		}
		if !strings.Contains(field, "/") && !strings.Contains(field, ".") {
			continue
		}
		switch filepath.Ext(field) {
		case ".go", ".md", ".sql", ".toml", ".yaml", ".yml", ".json", ".ts", ".tsx", ".js", ".jsx", ".css", ".sh":
			return field
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func ensureMetadata(txn *badger.Txn, key string, value []byte) error {
	_, err := txn.Get([]byte(key))
	if err == nil {
		return nil
	}
	if !errors.Is(err, badger.ErrKeyNotFound) {
		return err
	}
	return txn.Set([]byte(key), value)
}

func readUint64(txn *badger.Txn, key string) (uint64, error) {
	item, err := txn.Get([]byte(key))
	if err != nil {
		return 0, err
	}
	var value uint64
	err = item.Value(func(val []byte) error {
		parsed, err := strconv.ParseUint(string(val), 10, 64)
		if err != nil {
			return err
		}
		value = parsed
		return nil
	})
	return value, err
}

func encodeUint64(v uint64) []byte {
	return []byte(strconv.FormatUint(v, 10))
}

func entryKey(index uint64) string {
	return fmt.Sprintf("entries/%020d", index)
}

func continuationStoreID(sessionID string, index int) string {
	return fmt.Sprintf("%s--cont-%04d", sessionID, index)
}

func baseSessionID(storeID string) string {
	if idx := strings.Index(storeID, "--cont-"); idx >= 0 {
		return storeID[:idx]
	}
	return storeID
}

func continuationIndex(storeID string) int {
	if idx := strings.Index(storeID, "--cont-"); idx >= 0 {
		value, err := strconv.Atoi(storeID[idx+8:])
		if err == nil {
			return value
		}
	}
	return 0
}
