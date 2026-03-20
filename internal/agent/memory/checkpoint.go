package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	pmem "github.com/duggal1/Sapphire-cli/internal/memory"
	"github.com/duggal1/Sapphire-cli/internal/message"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const (
	checkpointMessageThreshold = 50
	checkpointTimeThreshold    = 30 * time.Minute
	maxRecentCheckpoints       = 24
	maxOlderHighValue          = 8
	checkpointAnchorInterval   = 200
	resumeRetrievalTokens      = 1200
)

type MessageSource interface {
	List(ctx context.Context, sessionID string) ([]message.Message, error)
}

type CheckpointParams struct {
	SessionID      string
	AgentID        string
	WorkItemID     string
	AuditTail      string
	Phase          string
	Prompt         string
	Result         string
	Status         string
	Summary        map[string]any
	Force          bool
	MailCursor     int64
	ActivityCursor int64
}

type ResumeSnapshot struct {
	Checkpoint        orchestrationdb.SessionCheckpoint
	UserPreferences   []orchestrationdb.UserPreference
	Decisions         []orchestrationdb.DecisionRecord
	DecisionConflicts []string
	PendingTasks      []string
	FilesModified     []string
	RetrievalContext  string
}

type CheckpointService struct {
	store     *orchestrationdb.Store
	messages  MessageSource
	memory    MemoryService
	retrieval *pmem.System
	now       func() time.Time
}

func NewCheckpointService(store *orchestrationdb.Store, messages MessageSource, memory MemoryService, retrieval *pmem.System) *CheckpointService {
	return &CheckpointService{
		store:     store,
		messages:  messages,
		memory:    memory,
		retrieval: retrieval,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (s *CheckpointService) Record(ctx context.Context, params CheckpointParams) (orchestrationdb.SessionCheckpoint, bool, error) {
	if s == nil || s.store == nil || s.messages == nil {
		return orchestrationdb.SessionCheckpoint{}, false, nil
	}
	params.SessionID = strings.TrimSpace(params.SessionID)
	params.AgentID = strings.TrimSpace(params.AgentID)
	if params.SessionID == "" || params.AgentID == "" {
		return orchestrationdb.SessionCheckpoint{}, false, nil
	}

	messages, err := s.messages.List(ctx, params.SessionID)
	if err != nil {
		return orchestrationdb.SessionCheckpoint{}, false, err
	}
	messageCount := len(messages)

	latest, latestErr := s.store.LatestCheckpoint(ctx, params.SessionID, params.AgentID)
	if latestErr != nil && !errors.Is(latestErr, sql.ErrNoRows) {
		return orchestrationdb.SessionCheckpoint{}, false, latestErr
	}
	now := s.now()
	if !params.Force && latest.ID != "" {
		if messageCount-latest.MessageCount < checkpointMessageThreshold && now.Sub(latest.CreatedAt) < checkpointTimeThreshold {
			return latest, false, nil
		}
	}

	pendingTasks, filesModified, structuredSummary := s.extractStructuredState(ctx, params.SessionID)
	summary := cloneMap(params.Summary)
	if len(summary) == 0 {
		summary = map[string]any{}
	}
	if phase := strings.TrimSpace(params.Phase); phase != "" {
		summary["phase"] = phase
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		summary["status"] = status
	}
	if prompt := compactText(params.Prompt, 280); prompt != "" {
		summary["prompt"] = prompt
	}
	if result := compactText(params.Result, 280); result != "" {
		summary["result"] = result
	}
	if structuredSummary != "" {
		summary["summary"] = structuredSummary
	}
	summary["message_count"] = messageCount

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return orchestrationdb.SessionCheckpoint{}, false, err
	}
	pendingJSON, err := json.Marshal(pendingTasks)
	if err != nil {
		return orchestrationdb.SessionCheckpoint{}, false, err
	}
	filesJSON, err := json.Marshal(filesModified)
	if err != nil {
		return orchestrationdb.SessionCheckpoint{}, false, err
	}

	checkpoint := orchestrationdb.SessionCheckpoint{
		SessionID:          params.SessionID,
		AgentID:            params.AgentID,
		WorkItemID:         strings.TrimSpace(params.WorkItemID),
		ParentCheckpointID: latest.ID,
		MessageCount:       messageCount,
		SummaryJSON:        string(summaryJSON),
		AuditTail:          compactText(params.AuditTail, 2800),
		PendingTasksJSON:   string(pendingJSON),
		FilesModifiedJSON:  string(filesJSON),
		MailCursor:         firstNonZero(params.MailCursor, now.Unix()),
		ActivityCursor:     firstNonZero(params.ActivityCursor, now.Unix()),
		CreatedAt:          now,
	}
	saved, err := s.store.SaveCheckpoint(ctx, checkpoint)
	if err != nil {
		return orchestrationdb.SessionCheckpoint{}, false, err
	}
	for _, pref := range ExtractUserPreferences(messages, params.SessionID, now) {
		if err := s.applyUserPreference(ctx, pref); err != nil {
			return saved, true, err
		}
	}
	for _, decision := range ExtractDecisionRecords(messages, params.SessionID, saved.ID, now, s.memory) {
		if _, err := s.store.SaveDecision(ctx, decision); err != nil {
			return saved, true, err
		}
	}
	if err := s.pruneCheckpoints(ctx, params.SessionID, params.AgentID); err != nil {
		return saved, true, err
	}
	return saved, true, nil
}

func (s *CheckpointService) Resume(ctx context.Context, sessionID, agentID string) (ResumeSnapshot, error) {
	if s == nil || s.store == nil {
		return ResumeSnapshot{}, sql.ErrNoRows
	}
	checkpoint, err := s.store.LatestCheckpoint(ctx, sessionID, agentID)
	if err != nil {
		return ResumeSnapshot{}, err
	}
	preferences, err := s.store.ListUserPreferences(ctx, 20)
	if err != nil {
		return ResumeSnapshot{}, err
	}
	decisions, err := s.store.ListDecisionRecords(ctx, sessionID, 50)
	if err != nil {
		return ResumeSnapshot{}, err
	}
	resolvedDecisions, conflicts := resolveDecisionRecords(decisions)

	var pendingTasks []string
	if strings.TrimSpace(checkpoint.PendingTasksJSON) != "" {
		_ = json.Unmarshal([]byte(checkpoint.PendingTasksJSON), &pendingTasks)
	}
	var filesModified []string
	if strings.TrimSpace(checkpoint.FilesModifiedJSON) != "" {
		_ = json.Unmarshal([]byte(checkpoint.FilesModifiedJSON), &filesModified)
	}

	retrievalContext := ""
	if s.retrieval != nil {
		retrievalContext = strings.TrimSpace(s.retrieval.BuildContextInjectionForSession(ctx, sessionID, resumeRetrievalTokens))
	}

	return ResumeSnapshot{
		Checkpoint:        checkpoint,
		UserPreferences:   preferences,
		Decisions:         resolvedDecisions,
		DecisionConflicts: conflicts,
		PendingTasks:      pendingTasks,
		FilesModified:     filesModified,
		RetrievalContext:  retrievalContext,
	}, nil
}

func (s *CheckpointService) extractStructuredState(ctx context.Context, sessionID string) ([]string, []string, string) {
	if s == nil || s.memory == nil || strings.TrimSpace(sessionID) == "" {
		return nil, nil, ""
	}
	data, err := s.memory.GetStructuredSummary(ctx, sessionID)
	if err != nil || data == nil {
		return nil, nil, ""
	}
	var pendingTasks []string
	for _, item := range data.TodoStates {
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "done" || status == "completed" || status == "closed" {
			continue
		}
		if text := strings.TrimSpace(item.Content); text != "" {
			pendingTasks = append(pendingTasks, text)
		}
	}
	var filesModified []string
	for _, item := range data.FileChanges {
		if text := strings.TrimSpace(item.File); text != "" {
			filesModified = append(filesModified, text)
		}
	}
	var summaryParts []string
	for _, item := range limitDecisions(data.Decisions, 4) {
		decision := strings.TrimSpace(item.Decision)
		if decision == "" {
			continue
		}
		if rationale := strings.TrimSpace(item.Rationale); rationale != "" {
			summaryParts = append(summaryParts, decision+": "+rationale)
		} else {
			summaryParts = append(summaryParts, decision)
		}
	}
	return uniqueStrings(pendingTasks), uniqueStrings(filesModified), compactText(strings.Join(summaryParts, " | "), 420)
}

func (s *CheckpointService) applyUserPreference(ctx context.Context, pref orchestrationdb.UserPreference) error {
	current, err := s.store.GetUserPreference(ctx, pref.Key)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return s.store.UpsertUserPreference(ctx, pref)
	}
	if strings.EqualFold(strings.TrimSpace(current.Value), strings.TrimSpace(pref.Value)) {
		if confidenceRank(pref.Confidence) >= confidenceRank(current.Confidence) {
			return s.store.UpsertUserPreference(ctx, pref)
		}
		return nil
	}
	currentRank := confidenceRank(current.Confidence)
	newRank := confidenceRank(pref.Confidence)
	switch {
	case newRank > currentRank:
		return s.store.UpsertUserPreference(ctx, pref)
	case newRank == currentRank:
		pref.Confidence = "conflicted"
		return s.store.UpsertUserPreference(ctx, pref)
	default:
		return nil
	}
}

func (s *CheckpointService) pruneCheckpoints(ctx context.Context, sessionID, agentID string) error {
	checkpoints, err := s.store.ListCheckpoints(ctx, sessionID, agentID, 200)
	if err != nil {
		return err
	}
	if len(checkpoints) <= maxRecentCheckpoints {
		return nil
	}
	keep := make(map[string]struct{}, maxRecentCheckpoints+maxOlderHighValue)
	highValueCount := 0
	for idx, checkpoint := range checkpoints {
		if idx < maxRecentCheckpoints {
			keep[checkpoint.ID] = struct{}{}
			continue
		}
		if highValueCount < maxOlderHighValue && isHighValueCheckpoint(checkpoint) {
			keep[checkpoint.ID] = struct{}{}
			highValueCount++
		}
	}
	var deleteIDs []string
	for _, checkpoint := range checkpoints {
		if _, ok := keep[checkpoint.ID]; ok {
			continue
		}
		deleteIDs = append(deleteIDs, checkpoint.ID)
	}
	return s.store.DeleteCheckpoints(ctx, deleteIDs)
}

func resolveDecisionRecords(items []orchestrationdb.DecisionRecord) ([]orchestrationdb.DecisionRecord, []string) {
	if len(items) == 0 {
		return nil, nil
	}
	resolved := make(map[string]orchestrationdb.DecisionRecord, len(items))
	conflicts := make(map[string]string)
	for _, item := range items {
		key := strings.TrimSpace(item.Category) + "|" + strings.TrimSpace(item.Key)
		current, ok := resolved[key]
		if !ok {
			resolved[key] = item
			continue
		}
		if strings.EqualFold(strings.TrimSpace(current.Value), strings.TrimSpace(item.Value)) {
			if confidenceRank(item.Confidence) >= confidenceRank(current.Confidence) && item.CreatedAt.After(current.CreatedAt) {
				resolved[key] = item
			}
			continue
		}
		switch {
		case confidenceRank(item.Confidence) > confidenceRank(current.Confidence):
			resolved[key] = item
		case confidenceRank(item.Confidence) == confidenceRank(current.Confidence) && item.CreatedAt.After(current.CreatedAt):
			item.Confidence = "conflicted"
			resolved[key] = item
			conflicts[key] = current.Value + " <> " + item.Value
		default:
			conflicts[key] = current.Value + " <> " + item.Value
		}
	}
	out := make([]orchestrationdb.DecisionRecord, 0, len(resolved))
	for _, item := range resolved {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].Key < out[j].Key
		}
		return out[i].Category < out[j].Category
	})
	conflictList := make([]string, 0, len(conflicts))
	for key, value := range conflicts {
		conflictList = append(conflictList, key+" => "+value)
	}
	sort.Strings(conflictList)
	return out, conflictList
}

func isHighValueCheckpoint(checkpoint orchestrationdb.SessionCheckpoint) bool {
	var summary map[string]any
	_ = json.Unmarshal([]byte(checkpoint.SummaryJSON), &summary)
	status, _ := summary["status"].(string)
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "error" || status == "compacted" {
		return true
	}
	return checkpoint.MessageCount > 0 && checkpoint.MessageCount%checkpointAnchorInterval == 0
}

func confidenceRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "conflicted":
		return 3
	case "confirmed":
		return 2
	default:
		return 1
	}
}

func firstNonZero(primary, fallback int64) int64 {
	if primary != 0 {
		return primary
	}
	return fallback
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func compactText(text string, max int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if max > 0 && len(text) > max {
		return strings.TrimSpace(text[:max]) + "..."
	}
	return text
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func limitDecisions(items []Decision, max int) []Decision {
	if max <= 0 || len(items) <= max {
		return items
	}
	return items[:max]
}
