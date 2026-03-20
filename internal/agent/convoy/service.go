package convoy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const (
	StatusOpen           = "open"
	StatusStagedReady    = "staged_ready"
	StatusStagedWarnings = "staged_warnings"
	StatusClosed         = "closed"
)

type Hooks struct {
	EnsureDispatchForWorkItem func(ctx context.Context, item orchestrationdb.WorkItem) (string, error)
}

type Service struct {
	store *orchestrationdb.Store
	hooks Hooks
}

func NewService(store *orchestrationdb.Store, hooks Hooks) *Service {
	return &Service{store: store, hooks: hooks}
}

func (s *Service) CreateConvoy(ctx context.Context, name, owner, mergeStrategy string) (orchestrationdb.Convoy, error) {
	if s == nil || s.store == nil {
		return orchestrationdb.Convoy{}, fmt.Errorf("convoy service is not initialized")
	}
	if strings.TrimSpace(mergeStrategy) == "" {
		mergeStrategy = "direct"
	}
	return s.store.SaveConvoy(ctx, orchestrationdb.Convoy{
		ID:            "cv-" + uuid.NewString(),
		Name:          strings.TrimSpace(name),
		Owner:         strings.TrimSpace(owner),
		MergeStrategy: strings.TrimSpace(mergeStrategy),
		Status:        StatusOpen,
		CreatedAt:     time.Now().UTC(),
	})
}

func (s *Service) AddWorkItems(ctx context.Context, convoyID string, workItemIDs []string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("convoy service is not initialized")
	}
	return s.store.AddConvoyTracks(ctx, convoyID, workItemIDs)
}

func (s *Service) StageConvoy(ctx context.Context, convoyID string) (orchestrationdb.Convoy, []string, error) {
	convoy, items, err := s.loadTrackedItems(ctx, convoyID)
	if err != nil {
		return orchestrationdb.Convoy{}, nil, err
	}
	warnings := stageWarnings(ctx, s.store, items)
	if len(warnings) > 0 {
		convoy.Status = StatusStagedWarnings
	} else {
		convoy.Status = StatusStagedReady
	}
	convoy.ClosedAt = time.Time{}
	saved, err := s.store.SaveConvoy(ctx, convoy)
	return saved, warnings, err
}

func (s *Service) LaunchConvoy(ctx context.Context, convoyID string) (orchestrationdb.Convoy, error) {
	convoy, items, err := s.loadTrackedItems(ctx, convoyID)
	if err != nil {
		return orchestrationdb.Convoy{}, err
	}
	if convoy.Status == StatusClosed {
		return orchestrationdb.Convoy{}, fmt.Errorf("convoy %s is closed", convoyID)
	}
	convoy.Status = StatusOpen
	convoy.ClosedAt = time.Time{}
	convoy, err = s.store.SaveConvoy(ctx, convoy)
	if err != nil {
		return orchestrationdb.Convoy{}, err
	}
	if err := s.dispatchReadyWork(ctx, items); err != nil {
		return orchestrationdb.Convoy{}, err
	}
	return convoy, nil
}

func (s *Service) LandConvoy(ctx context.Context, convoyID string) (orchestrationdb.Convoy, error) {
	if s == nil || s.store == nil {
		return orchestrationdb.Convoy{}, fmt.Errorf("convoy service is not initialized")
	}
	convoy, err := s.store.GetConvoy(ctx, convoyID)
	if err != nil {
		return orchestrationdb.Convoy{}, err
	}
	convoy.Status = StatusClosed
	convoy.ClosedAt = time.Now().UTC()
	return s.store.SaveConvoy(ctx, convoy)
}

func (s *Service) CheckConvoyCompletion(ctx context.Context, convoyID string) (bool, error) {
	convoy, items, err := s.loadTrackedItems(ctx, convoyID)
	if err != nil {
		return false, err
	}
	if convoy.Status == StatusClosed {
		return true, nil
	}
	if allTrackedItemsClosed(items) {
		if _, err := s.LandConvoy(ctx, convoyID); err != nil {
			return false, err
		}
		return true, nil
	}
	if convoy.Status == StatusOpen {
		if err := s.dispatchReadyWork(ctx, items); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (s *Service) FindStrandedConvoys(ctx context.Context, limit int) ([]orchestrationdb.Convoy, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("convoy service is not initialized")
	}
	convoys, err := s.store.ListConvoys(ctx, []string{StatusOpen}, limit)
	if err != nil {
		return nil, err
	}
	stranded := make([]orchestrationdb.Convoy, 0, len(convoys))
	for _, convoy := range convoys {
		_, items, err := s.loadTrackedItems(ctx, convoy.ID)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			ready, err := s.isReadyForDispatch(ctx, item)
			if err != nil || !ready {
				continue
			}
			if active, err := s.hasActiveWork(ctx, item); err == nil && !active {
				stranded = append(stranded, convoy)
				break
			}
		}
	}
	return stranded, nil
}

func (s *Service) loadTrackedItems(ctx context.Context, convoyID string) (orchestrationdb.Convoy, []orchestrationdb.WorkItem, error) {
	if s == nil || s.store == nil {
		return orchestrationdb.Convoy{}, nil, fmt.Errorf("convoy service is not initialized")
	}
	convoy, err := s.store.GetConvoy(ctx, convoyID)
	if err != nil {
		return orchestrationdb.Convoy{}, nil, err
	}
	items, err := s.store.ListWorkItemsByConvoy(ctx, convoyID, 500)
	if err != nil {
		return orchestrationdb.Convoy{}, nil, err
	}
	return convoy, items, nil
}

func (s *Service) dispatchReadyWork(ctx context.Context, items []orchestrationdb.WorkItem) error {
	if s == nil || s.hooks.EnsureDispatchForWorkItem == nil {
		return nil
	}
	for _, item := range items {
		ready, err := s.isReadyForDispatch(ctx, item)
		if err != nil || !ready {
			continue
		}
		active, err := s.hasActiveWork(ctx, item)
		if err != nil || active {
			continue
		}
		if _, err := s.hooks.EnsureDispatchForWorkItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) isReadyForDispatch(ctx context.Context, item orchestrationdb.WorkItem) (bool, error) {
	status := normalizeStatus(item.Status)
	if status != "open" {
		return false, nil
	}
	deps := parseDependencies(item.Dependencies)
	for _, depID := range deps {
		dep, err := s.store.GetWorkItem(ctx, depID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		depStatus := normalizeStatus(dep.Status)
		if depStatus != "closed" && depStatus != "completed" {
			return false, nil
		}
	}
	return true, nil
}

func (s *Service) hasActiveWork(ctx context.Context, item orchestrationdb.WorkItem) (bool, error) {
	dispatches, err := s.store.ListDispatchesByWorkItem(ctx, item.ID, []string{"queued", "leased", "running"}, 10)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if len(dispatches) > 0 {
		return true, nil
	}
	if assignee := strings.TrimSpace(item.Assignee); assignee != "" {
		hook, err := s.store.GetAgentHook(ctx, assignee)
		if err == nil && strings.TrimSpace(hook.HookBeadID) == item.ID && normalizeStatus(hook.Status) != "idle" {
			return true, nil
		}
	}
	return false, nil
}

func allTrackedItemsClosed(items []orchestrationdb.WorkItem) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		status := normalizeStatus(item.Status)
		if status != "closed" && status != "completed" {
			return false
		}
	}
	return true
}

func stageWarnings(ctx context.Context, store *orchestrationdb.Store, items []orchestrationdb.WorkItem) []string {
	if len(items) == 0 {
		return []string{"convoy has no tracked work items"}
	}
	warnings := make([]string, 0, len(items))
	readyCount := 0
	for _, item := range items {
		status := normalizeStatus(item.Status)
		if status == "blocked" {
			warnings = append(warnings, fmt.Sprintf("work item %s is blocked", item.ID))
			continue
		}
		if status == "closed" || status == "completed" {
			continue
		}
		ready := true
		for _, depID := range parseDependencies(item.Dependencies) {
			dep, err := store.GetWorkItem(ctx, depID)
			if err != nil || (normalizeStatus(dep.Status) != "closed" && normalizeStatus(dep.Status) != "completed") {
				ready = false
				break
			}
		}
		if ready && status == "open" {
			readyCount++
		}
	}
	if readyCount == 0 {
		warnings = append(warnings, "no tracked work items are dispatchable")
	}
	return uniqueStrings(warnings)
}

func parseDependencies(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, `"`))
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func normalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
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
