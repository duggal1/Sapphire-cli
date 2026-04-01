package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/config"
)

type SingularityPolicyInfo struct {
	TaskFamily                   string   `json:"task_family"`
	GoalType                     string   `json:"goal_type"`
	Breadth                      string   `json:"breadth"`
	Domains                      []string `json:"domains,omitempty"`
	PromotionState               string   `json:"promotion_state"`
	Confidence                   int      `json:"confidence"`
	EvidenceCount                int      `json:"evidence_count"`
	SuccessCount                 int      `json:"success_count"`
	FailureCount                 int      `json:"failure_count"`
	AppliedCount                 int      `json:"applied_count"`
	RequireHarness               bool     `json:"require_harness"`
	PreferParallel               bool     `json:"prefer_parallel"`
	PreferIndexCodebase          bool     `json:"prefer_index_codebase"`
	ForbidBashDiscovery          bool     `json:"forbid_bash_discovery"`
	RequireExplicitPlan          bool     `json:"require_explicit_plan"`
	RequirePostWriteVerification bool     `json:"require_post_write_verification"`
	PreferredDiscovery           []string `json:"preferred_discovery,omitempty"`
	PreferredSkills              []string `json:"preferred_skills,omitempty"`
	SkillName                    string   `json:"skill_name,omitempty"`
	SkillFilePath                string   `json:"skill_file_path,omitempty"`
	LastAppliedAt                string   `json:"last_applied_at,omitempty"`
	UpdatedAt                    string   `json:"updated_at,omitempty"`
}

type SingularityPolicyStoreInfo struct {
	PolicyPath    string                  `json:"policy_path"`
	HistoryDir    string                  `json:"history_dir"`
	AuditPath     string                  `json:"audit_path"`
	SnapshotCount int                     `json:"snapshot_count"`
	Policies      []SingularityPolicyInfo `json:"policies"`
}

type SingularityPolicyFieldChange struct {
	TaskFamily string `json:"task_family"`
	Field      string `json:"field"`
	Before     string `json:"before"`
	After      string `json:"after"`
}

type SingularityPolicyDiff struct {
	CurrentPath   string                         `json:"current_path"`
	PreviousPath  string                         `json:"previous_path,omitempty"`
	SnapshotCount int                            `json:"snapshot_count"`
	Added         []SingularityPolicyInfo        `json:"added,omitempty"`
	Removed       []SingularityPolicyInfo        `json:"removed,omitempty"`
	Changed       []SingularityPolicyFieldChange `json:"changed,omitempty"`
}

type SingularityResetResult struct {
	PolicyPath      string   `json:"policy_path"`
	RemovedPolicies []string `json:"removed_policies"`
	RemovedSkills   []string `json:"removed_skills,omitempty"`
}

type SingularityAuditInfo struct {
	AuditPath string                     `json:"audit_path"`
	Records   []SingularityTurnAuditInfo `json:"records"`
}

type SingularityTurnAuditInfo struct {
	Timestamp            string         `json:"timestamp"`
	SessionID            string         `json:"session_id"`
	TaskFamily           string         `json:"task_family"`
	GoalType             string         `json:"goal_type"`
	Breadth              string         `json:"breadth"`
	Status               string         `json:"status"`
	ActivePolicyID       string         `json:"active_policy_id,omitempty"`
	AppliedPolicy        bool           `json:"applied_policy"`
	PolicyState          string         `json:"policy_state,omitempty"`
	PolicyConfidence     int            `json:"policy_confidence,omitempty"`
	OrderedTools         []string       `json:"ordered_tools,omitempty"`
	ToolCalls            map[string]int `json:"tool_calls,omitempty"`
	ToolResults          map[string]int `json:"tool_results,omitempty"`
	ToolErrorCodes       map[string]int `json:"tool_error_codes,omitempty"`
	LoadedSkills         []string       `json:"loaded_skills,omitempty"`
	BashDiscovery        int            `json:"bash_discovery,omitempty"`
	BlockedBashDiscovery int            `json:"blocked_bash_discovery,omitempty"`
	ValidationChecks     int            `json:"validation_checks,omitempty"`
	ContextDiscipline    string         `json:"context_discipline,omitempty"`
	PlanningDiscipline   string         `json:"planning_discipline,omitempty"`
	ValidationDiscipline string         `json:"validation_discipline,omitempty"`
	RecoveryDiscipline   string         `json:"recovery_discipline,omitempty"`
	TradeoffDiscipline   string         `json:"tradeoff_discipline,omitempty"`
	ExecutionRisk        string         `json:"execution_risk,omitempty"`
	ResultSummary        string         `json:"result_summary,omitempty"`
}

func ListSingularityPolicies(cfg *config.Config) (SingularityPolicyStoreInfo, error) {
	manager, err := loadSingularityManagerForAdmin(cfg)
	if err != nil {
		return SingularityPolicyStoreInfo{}, err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	info := SingularityPolicyStoreInfo{
		PolicyPath:    manager.policyPath,
		HistoryDir:    manager.historyDir,
		AuditPath:     manager.auditPath,
		SnapshotCount: countSingularitySnapshots(manager.historyDir),
		Policies:      make([]SingularityPolicyInfo, 0, len(manager.store.Policies)),
	}
	for _, policy := range manager.store.Policies {
		info.Policies = append(info.Policies, summarizeLearnedRoutePolicy(normalizeLoadedLearnedRoutePolicy(policy)))
	}
	sort.Slice(info.Policies, func(i, j int) bool {
		if info.Policies[i].Confidence == info.Policies[j].Confidence {
			return info.Policies[i].TaskFamily < info.Policies[j].TaskFamily
		}
		return info.Policies[i].Confidence > info.Policies[j].Confidence
	})
	return info, nil
}

func GetSingularityPolicy(cfg *config.Config, selector string) (SingularityPolicyInfo, error) {
	manager, err := loadSingularityManagerForAdmin(cfg)
	if err != nil {
		return SingularityPolicyInfo{}, err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	key, err := resolveLearnedPolicySelector(manager.store.Policies, selector)
	if err != nil {
		return SingularityPolicyInfo{}, err
	}
	policy, ok := manager.store.Policies[key]
	if !ok {
		return SingularityPolicyInfo{}, os.ErrNotExist
	}
	return summarizeLearnedRoutePolicy(normalizeLoadedLearnedRoutePolicy(policy)), nil
}

func DiffSingularityPolicies(cfg *config.Config, selector string) (SingularityPolicyDiff, error) {
	manager, err := loadSingularityManagerForAdmin(cfg)
	if err != nil {
		return SingularityPolicyDiff{}, err
	}
	manager.mu.Lock()
	currentPolicies := cloneLearnedPolicyMap(manager.store.Policies)
	currentPath := manager.policyPath
	historyDir := manager.historyDir
	manager.mu.Unlock()

	diff := SingularityPolicyDiff{
		CurrentPath:   currentPath,
		SnapshotCount: countSingularitySnapshots(historyDir),
	}
	previousPath, previousStore, err := loadLatestSingularitySnapshot(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return diff, nil
		}
		return SingularityPolicyDiff{}, err
	}
	diff.PreviousPath = previousPath

	if strings.TrimSpace(selector) != "" {
		currentKey, currentErr := resolveLearnedPolicySelector(currentPolicies, selector)
		previousKey, previousErr := resolveLearnedPolicySelector(previousStore.Policies, selector)
		switch {
		case currentErr == nil:
			currentPolicies = map[string]learnedRoutePolicy{currentKey: currentPolicies[currentKey]}
		case previousErr == nil:
			currentPolicies = map[string]learnedRoutePolicy{}
		default:
			return SingularityPolicyDiff{}, currentErr
		}
		if previousErr == nil {
			previousStore.Policies = map[string]learnedRoutePolicy{previousKey: previousStore.Policies[previousKey]}
		} else {
			previousStore.Policies = map[string]learnedRoutePolicy{}
		}
	}

	for taskFamily, current := range currentPolicies {
		previous, ok := previousStore.Policies[taskFamily]
		if !ok {
			diff.Added = append(diff.Added, summarizeLearnedRoutePolicy(normalizeLoadedLearnedRoutePolicy(current)))
			continue
		}
		diff.Changed = append(diff.Changed, diffLearnedPolicyFields(taskFamily, normalizeLoadedLearnedRoutePolicy(previous), normalizeLoadedLearnedRoutePolicy(current))...)
	}
	for taskFamily, previous := range previousStore.Policies {
		if _, ok := currentPolicies[taskFamily]; ok {
			continue
		}
		diff.Removed = append(diff.Removed, summarizeLearnedRoutePolicy(normalizeLoadedLearnedRoutePolicy(previous)))
	}

	sort.Slice(diff.Added, func(i, j int) bool { return diff.Added[i].TaskFamily < diff.Added[j].TaskFamily })
	sort.Slice(diff.Removed, func(i, j int) bool { return diff.Removed[i].TaskFamily < diff.Removed[j].TaskFamily })
	sort.Slice(diff.Changed, func(i, j int) bool {
		if diff.Changed[i].TaskFamily == diff.Changed[j].TaskFamily {
			return diff.Changed[i].Field < diff.Changed[j].Field
		}
		return diff.Changed[i].TaskFamily < diff.Changed[j].TaskFamily
	})
	return diff, nil
}

func ResetSingularityPolicies(cfg *config.Config, selector string, resetAll bool) (SingularityResetResult, error) {
	manager, err := loadSingularityManagerForAdmin(cfg)
	if err != nil {
		return SingularityResetResult{}, err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()

	result := SingularityResetResult{
		PolicyPath: manager.policyPath,
	}
	if resetAll {
		taskFamilies := sortedLearnedPolicyKeys(manager.store.Policies)
		for _, taskFamily := range taskFamilies {
			policy := manager.store.Policies[taskFamily]
			removedSkill, err := manager.removeLearnedSkillLocked(policy)
			if err != nil {
				return SingularityResetResult{}, err
			}
			if removedSkill != "" {
				result.RemovedSkills = append(result.RemovedSkills, removedSkill)
			}
			result.RemovedPolicies = append(result.RemovedPolicies, taskFamily)
		}
		manager.store.Policies = map[string]learnedRoutePolicy{}
		return result, manager.persistLocked()
	}

	key, err := resolveLearnedPolicySelector(manager.store.Policies, selector)
	if err != nil {
		return SingularityResetResult{}, err
	}
	policy := manager.store.Policies[key]
	removedSkill, err := manager.removeLearnedSkillLocked(policy)
	if err != nil {
		return SingularityResetResult{}, err
	}
	if removedSkill != "" {
		result.RemovedSkills = append(result.RemovedSkills, removedSkill)
	}
	delete(manager.store.Policies, key)
	result.RemovedPolicies = append(result.RemovedPolicies, key)
	return result, manager.persistLocked()
}

func ListSingularityAudit(cfg *config.Config, selector string, limit int) (SingularityAuditInfo, error) {
	manager, err := loadSingularityManagerForAdmin(cfg)
	if err != nil {
		return SingularityAuditInfo{}, err
	}
	auditPath := manager.auditPath
	data, err := os.ReadFile(auditPath)
	if err != nil {
		if os.IsNotExist(err) {
			return SingularityAuditInfo{AuditPath: auditPath}, nil
		}
		return SingularityAuditInfo{}, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	records := make([]SingularityTurnAuditInfo, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record singularityTurnAuditRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return SingularityAuditInfo{}, err
		}
		if selector != "" {
			if !auditRecordMatchesSelector(record, selector) {
				continue
			}
		}
		records = append(records, summarizeTurnAuditRecord(record))
	}
	if limit > 0 && len(records) > limit {
		records = records[len(records)-limit:]
	}
	return SingularityAuditInfo{
		AuditPath: auditPath,
		Records:   records,
	}, nil
}

func loadSingularityManagerForAdmin(cfg *config.Config) (*singularityManager, error) {
	manager := newSingularityManager(cfg)
	if manager == nil {
		return nil, fmt.Errorf("singularity learning is unavailable for this config")
	}
	return manager, nil
}

func summarizeLearnedRoutePolicy(policy learnedRoutePolicy) SingularityPolicyInfo {
	policy = normalizeLoadedLearnedRoutePolicy(policy)
	return SingularityPolicyInfo{
		TaskFamily:                   policy.TaskFamily,
		GoalType:                     policy.GoalType,
		Breadth:                      policy.Breadth,
		Domains:                      append([]string{}, policy.Domains...),
		PromotionState:               normalizeLearnedPromotionState(policy.PromotionState, policy.Confidence, policy.EvidenceCount),
		Confidence:                   policy.Confidence,
		EvidenceCount:                policy.EvidenceCount,
		SuccessCount:                 policy.SuccessCount,
		FailureCount:                 policy.FailureCount,
		AppliedCount:                 policy.AppliedCount,
		RequireHarness:               policy.RequireHarness,
		PreferParallel:               policy.PreferParallel,
		PreferIndexCodebase:          policy.PreferIndexCodebase,
		ForbidBashDiscovery:          policy.ForbidBashDiscovery,
		RequireExplicitPlan:          policy.RequireExplicitPlan,
		RequirePostWriteVerification: policy.RequirePostWriteVerification,
		PreferredDiscovery:           append([]string{}, policy.PreferredDiscovery...),
		PreferredSkills:              append([]string{}, policy.PreferredSkills...),
		SkillName:                    policy.SkillName,
		SkillFilePath:                policy.SkillFilePath,
		LastAppliedAt:                policy.LastAppliedAt,
		UpdatedAt:                    policy.UpdatedAt,
	}
}

func summarizeTurnAuditRecord(record singularityTurnAuditRecord) SingularityTurnAuditInfo {
	return SingularityTurnAuditInfo{
		Timestamp:            record.Timestamp,
		SessionID:            record.SessionID,
		TaskFamily:           record.TaskFamily,
		GoalType:             record.GoalType,
		Breadth:              record.Breadth,
		Status:               record.Status,
		ActivePolicyID:       record.ActivePolicyID,
		AppliedPolicy:        record.AppliedPolicy,
		PolicyState:          record.PolicyState,
		PolicyConfidence:     record.PolicyConfidence,
		OrderedTools:         append([]string{}, record.OrderedTools...),
		ToolCalls:            cloneStringIntMap(record.ToolCalls),
		ToolResults:          cloneStringIntMap(record.ToolResults),
		ToolErrorCodes:       cloneStringIntMap(record.ToolErrorCodes),
		LoadedSkills:         append([]string{}, record.LoadedSkills...),
		BashDiscovery:        record.BashDiscovery,
		BlockedBashDiscovery: record.BlockedBashDiscovery,
		ValidationChecks:     record.ValidationChecks,
		ContextDiscipline:    record.ContextDiscipline,
		PlanningDiscipline:   record.PlanningDiscipline,
		ValidationDiscipline: record.ValidationDiscipline,
		RecoveryDiscipline:   record.RecoveryDiscipline,
		TradeoffDiscipline:   record.TradeoffDiscipline,
		ExecutionRisk:        record.ExecutionRisk,
		ResultSummary:        record.ResultSummary,
	}
}

func auditRecordMatchesSelector(record singularityTurnAuditRecord, selector string) bool {
	selector = strings.TrimSpace(strings.ToLower(selector))
	if selector == "" {
		return true
	}
	return strings.Contains(strings.ToLower(record.TaskFamily), selector) || strings.EqualFold(record.ActivePolicyID, selector)
}

func diffLearnedPolicyFields(taskFamily string, before, after learnedRoutePolicy) []SingularityPolicyFieldChange {
	var changes []SingularityPolicyFieldChange
	appendChange := func(field, previous, current string) {
		if previous == current {
			return
		}
		changes = append(changes, SingularityPolicyFieldChange{
			TaskFamily: taskFamily,
			Field:      field,
			Before:     previous,
			After:      current,
		})
	}
	appendChange("promotion_state", normalizeLearnedPromotionState(before.PromotionState, before.Confidence, before.EvidenceCount), normalizeLearnedPromotionState(after.PromotionState, after.Confidence, after.EvidenceCount))
	appendChange("confidence", fmt.Sprintf("%d", before.Confidence), fmt.Sprintf("%d", after.Confidence))
	appendChange("applied_count", fmt.Sprintf("%d", before.AppliedCount), fmt.Sprintf("%d", after.AppliedCount))
	appendChange("require_harness", fmt.Sprintf("%t", before.RequireHarness), fmt.Sprintf("%t", after.RequireHarness))
	appendChange("prefer_parallel", fmt.Sprintf("%t", before.PreferParallel), fmt.Sprintf("%t", after.PreferParallel))
	appendChange("prefer_index_codebase", fmt.Sprintf("%t", before.PreferIndexCodebase), fmt.Sprintf("%t", after.PreferIndexCodebase))
	appendChange("forbid_bash_discovery", fmt.Sprintf("%t", before.ForbidBashDiscovery), fmt.Sprintf("%t", after.ForbidBashDiscovery))
	appendChange("require_explicit_plan", fmt.Sprintf("%t", before.RequireExplicitPlan), fmt.Sprintf("%t", after.RequireExplicitPlan))
	appendChange("require_post_write_verification", fmt.Sprintf("%t", before.RequirePostWriteVerification), fmt.Sprintf("%t", after.RequirePostWriteVerification))
	appendChange("preferred_discovery", strings.Join(before.PreferredDiscovery, ","), strings.Join(after.PreferredDiscovery, ","))
	appendChange("preferred_skills", strings.Join(before.PreferredSkills, ","), strings.Join(after.PreferredSkills, ","))
	appendChange("skill_name", before.SkillName, after.SkillName)
	return changes
}

func resolveLearnedPolicySelector(policies map[string]learnedRoutePolicy, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", fmt.Errorf("task family is required")
	}
	if _, ok := policies[selector]; ok {
		return selector, nil
	}
	lowerSelector := strings.ToLower(selector)
	var matches []string
	for taskFamily := range policies {
		if strings.Contains(strings.ToLower(taskFamily), lowerSelector) {
			matches = append(matches, taskFamily)
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no learned policy matches %q", selector)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("policy selector %q is ambiguous: %s", selector, strings.Join(matches, ", "))
	}
}

func loadLatestSingularitySnapshot(historyDir string) (string, learnedPolicyStore, error) {
	snapshotPath, err := latestSingularitySnapshotPath(historyDir)
	if err != nil {
		return "", learnedPolicyStore{}, err
	}
	store, err := readLearnedPolicyStoreFile(snapshotPath)
	return snapshotPath, store, err
}

func latestSingularitySnapshotPath(historyDir string) (string, error) {
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "route_policies-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return "", os.ErrNotExist
	}
	sort.Strings(names)
	return filepath.Join(historyDir, names[len(names)-1]), nil
}

func countSingularitySnapshots(historyDir string) int {
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "route_policies-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		count++
	}
	return count
}

func readLearnedPolicyStoreFile(path string) (learnedPolicyStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return learnedPolicyStore{Version: singularityStoreVersion, Policies: map[string]learnedRoutePolicy{}}, nil
		}
		return learnedPolicyStore{}, err
	}
	var store learnedPolicyStore
	if err := json.Unmarshal(data, &store); err != nil {
		return learnedPolicyStore{}, err
	}
	if store.Version == 0 {
		store.Version = singularityStoreVersion
	}
	if store.Policies == nil {
		store.Policies = map[string]learnedRoutePolicy{}
	}
	for taskFamily, policy := range store.Policies {
		store.Policies[taskFamily] = normalizeLoadedLearnedRoutePolicy(policy)
	}
	return store, nil
}

func cloneLearnedPolicyMap(in map[string]learnedRoutePolicy) map[string]learnedRoutePolicy {
	out := make(map[string]learnedRoutePolicy, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func sortedLearnedPolicyKeys(policies map[string]learnedRoutePolicy) []string {
	keys := make([]string, 0, len(policies))
	for key := range policies {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m *singularityManager) removeLearnedSkillLocked(policy learnedRoutePolicy) (string, error) {
	if m == nil {
		return "", nil
	}
	skillName := strings.TrimSpace(policy.SkillName)
	skillPath := strings.TrimSpace(policy.SkillFilePath)
	if skillName == "" || skillPath == "" || !strings.HasPrefix(skillName, autoLearnedSkillPrefix+"-") {
		return "", nil
	}
	if !m.boundary.AllowsAutoWrite(skillPath) || m.boundary.IsKernelProtected(skillPath) {
		return "", fmt.Errorf("auto-remove blocked for %s", skillPath)
	}
	skillDir := filepath.Dir(skillPath)
	if err := os.RemoveAll(skillDir); err != nil {
		return "", err
	}
	return skillDir, nil
}
