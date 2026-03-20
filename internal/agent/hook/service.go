package hook

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

type Snapshot struct {
	Hook     orchestrationdb.AgentHook
	WorkItem orchestrationdb.WorkItem
}

type Service struct {
	store        *orchestrationdb.Store
	stateService *agentstate.Service
}

func NewService(store *orchestrationdb.Store, stateService *agentstate.Service) *Service {
	return &Service{
		store:        store,
		stateService: stateService,
	}
}

func (s *Service) AssignHook(ctx context.Context, agentID, workItemID string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("hook service is not initialized")
	}
	agentID = strings.TrimSpace(agentID)
	workItemID = strings.TrimSpace(workItemID)
	if agentID == "" || workItemID == "" {
		return fmt.Errorf("agent id and work item id are required")
	}
	now := time.Now().UTC()
	if err := s.store.UpsertAgentHook(ctx, orchestrationdb.AgentHook{
		AgentID:    agentID,
		HookBeadID: workItemID,
		HookedAt:   now,
		Status:     "hooked",
	}); err != nil {
		return err
	}
	if item, err := s.store.GetWorkItem(ctx, workItemID); err == nil {
		item.Assignee = agentID
		if strings.TrimSpace(item.Status) == "" {
			item.Status = "open"
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
		if err := s.store.UpsertWorkItem(ctx, item); err != nil {
			return err
		}
	}
	return s.syncAgentState(ctx, agentID, workItemID)
}

func (s *Service) MarkInProgress(ctx context.Context, agentID, workItemID string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("hook service is not initialized")
	}
	agentID = strings.TrimSpace(agentID)
	workItemID = strings.TrimSpace(workItemID)
	if agentID == "" || workItemID == "" {
		return fmt.Errorf("agent id and work item id are required")
	}
	hook, err := s.store.GetAgentHook(ctx, agentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s.AssignHook(ctx, agentID, workItemID)
		}
		return err
	}
	hook.HookBeadID = workItemID
	if hook.HookedAt.IsZero() {
		hook.HookedAt = time.Now().UTC()
	}
	hook.Status = "in_progress"
	if err := s.store.UpsertAgentHook(ctx, hook); err != nil {
		return err
	}
	return s.syncAgentState(ctx, agentID, workItemID)
}

func (s *Service) GetHook(ctx context.Context, agentID string) (Snapshot, error) {
	if s == nil || s.store == nil {
		return Snapshot{}, fmt.Errorf("hook service is not initialized")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return Snapshot{}, fmt.Errorf("agent id is required")
	}
	hook, err := s.store.GetAgentHook(ctx, agentID)
	if err != nil {
		return Snapshot{}, err
	}
	if strings.TrimSpace(hook.HookBeadID) == "" {
		return Snapshot{Hook: hook}, nil
	}
	item, err := s.store.GetWorkItem(ctx, hook.HookBeadID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, err
	}
	return Snapshot{Hook: hook, WorkItem: item}, nil
}

func (s *Service) ClearHook(ctx context.Context, agentID, workItemID string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("hook service is not initialized")
	}
	agentID = strings.TrimSpace(agentID)
	workItemID = strings.TrimSpace(workItemID)
	if agentID == "" {
		return fmt.Errorf("agent id is required")
	}
	hook, err := s.store.GetAgentHook(ctx, agentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if workItemID != "" && strings.TrimSpace(hook.HookBeadID) != workItemID {
		return nil
	}
	hook.HookBeadID = ""
	hook.Status = "idle"
	hook.HookedAt = time.Time{}
	if err := s.store.UpsertAgentHook(ctx, hook); err != nil {
		return err
	}
	if workItemID != "" {
		if item, err := s.store.GetWorkItem(ctx, workItemID); err == nil {
			if item.Assignee == agentID {
				item.Assignee = ""
				if err := s.store.UpsertWorkItem(ctx, item); err != nil {
					return err
				}
			}
		}
	}
	return s.syncAgentState(ctx, agentID, "")
}

func (s *Service) ScanHooks(ctx context.Context, statuses []string, limit int) ([]orchestrationdb.AgentHook, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("hook service is not initialized")
	}
	return s.store.ListAgentHooks(ctx, statuses, limit)
}

func (s *Service) DiffHook(ctx context.Context, agentID string) (string, error) {
	snapshot, err := s.GetHook(ctx, agentID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(snapshot.Hook.HookBeadID) == "" {
		return "hook empty", nil
	}
	parts := []string{
		fmt.Sprintf("agent: %s", snapshot.Hook.AgentID),
		fmt.Sprintf("hook_bead_id: %s", snapshot.Hook.HookBeadID),
		fmt.Sprintf("hook_status: %s", snapshot.Hook.Status),
		fmt.Sprintf("work_item_status: %s", snapshot.WorkItem.Status),
		fmt.Sprintf("work_item_assignee: %s", snapshot.WorkItem.Assignee),
	}
	return strings.Join(parts, "\n"), nil
}

func (s *Service) syncAgentState(ctx context.Context, agentID, workItemID string) error {
	if s == nil || s.stateService == nil || strings.TrimSpace(agentID) == "" {
		return nil
	}
	state, err := s.stateService.Status(ctx, agentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return nil
	}
	state.HookBeadID = strings.TrimSpace(workItemID)
	state.UpdatedAt = time.Now().UTC()
	if state.LastHeartbeat.IsZero() {
		state.LastHeartbeat = state.UpdatedAt
	}
	return s.stateService.Register(ctx, state)
}
