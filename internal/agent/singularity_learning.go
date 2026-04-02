package agent

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/skills"
)

const (
	singularityDirName              = "singularity"
	singularityPolicyFileName       = "route_policies.json"
	singularityHistoryDirName       = "history"
	singularityAuditFileName        = "turn_audit.jsonl"
	singularityStoreVersion         = 1
	autoLearnedSkillPrefix          = "autolearn"
	maxSingularityPolicySnapshots   = 24
	minPolicyConfidenceForInjection = 55
	learnedPolicyDecayFactor        = 0.78
	learnedPolicyStateObserver      = "observer"
	learnedPolicyStateCandidate     = "candidate"
	learnedPolicyStatePromoted      = "promoted"
	learnedPolicyStateQuarantined   = "quarantined"
	learnedPolicyStateDemoted       = "demoted"
)

type learnedTaskFamily struct {
	ID       string   `json:"id"`
	GoalType string   `json:"goal_type"`
	Breadth  string   `json:"breadth"`
	Domains  []string `json:"domains,omitempty"`
}

type learnedRoutePolicy struct {
	TaskFamily                   string         `json:"task_family"`
	TaskFamilySlug               string         `json:"task_family_slug"`
	GoalType                     string         `json:"goal_type"`
	Breadth                      string         `json:"breadth"`
	Domains                      []string       `json:"domains,omitempty"`
	EvidenceCount                int            `json:"evidence_count"`
	SuccessCount                 int            `json:"success_count"`
	FailureCount                 int            `json:"failure_count"`
	StructuredSuccesses          int            `json:"structured_successes"`
	HarnessSuccesses             int            `json:"harness_successes"`
	ParallelSuccesses            int            `json:"parallel_successes"`
	IndexSuccesses               int            `json:"index_successes"`
	BashDiscoveryFailures        int            `json:"bash_discovery_failures"`
	BashDiscoverySuccess         int            `json:"bash_discovery_success"`
	ToolSuccessCounts            map[string]int `json:"tool_success_counts,omitempty"`
	ToolFailureCounts            map[string]int `json:"tool_failure_counts,omitempty"`
	PreferredDiscovery           []string       `json:"preferred_discovery,omitempty"`
	PreferredVerification        []string       `json:"preferred_verification,omitempty"`
	PreferredSkills              []string       `json:"preferred_skills,omitempty"`
	RequireHarness               bool           `json:"require_harness,omitempty"`
	PreferParallel               bool           `json:"prefer_parallel,omitempty"`
	PreferIndexCodebase          bool           `json:"prefer_index_codebase,omitempty"`
	ForbidBashDiscovery          bool           `json:"forbid_bash_discovery,omitempty"`
	RequireContextRead           bool           `json:"require_context_read,omitempty"`
	RequireExplicitPlan          bool           `json:"require_explicit_plan,omitempty"`
	RequirePostWriteVerification bool           `json:"require_post_write_verification,omitempty"`
	Confidence                   int            `json:"confidence"`
	RecentSuccessWeight          float64        `json:"recent_success_weight,omitempty"`
	RecentFailureWeight          float64        `json:"recent_failure_weight,omitempty"`
	RecentStructuredWeight       float64        `json:"recent_structured_weight,omitempty"`
	RecentHarnessWeight          float64        `json:"recent_harness_weight,omitempty"`
	RecentParallelWeight         float64        `json:"recent_parallel_weight,omitempty"`
	RecentIndexWeight            float64        `json:"recent_index_weight,omitempty"`
	RecentBashFailureWeight      float64        `json:"recent_bash_failure_weight,omitempty"`
	RecentBashSuccessWeight      float64        `json:"recent_bash_success_weight,omitempty"`
	RecentContextPenalty         float64        `json:"recent_context_penalty,omitempty"`
	RecentPlanningPenalty        float64        `json:"recent_planning_penalty,omitempty"`
	RecentPlanQualityPenalty     float64        `json:"recent_plan_quality_penalty,omitempty"`
	RecentArchitecturePenalty    float64        `json:"recent_architecture_penalty,omitempty"`
	RecentValidationPenalty      float64        `json:"recent_validation_penalty,omitempty"`
	RecentVerifierWeight         float64        `json:"recent_verifier_weight,omitempty"`
	RecentRecoveryPenalty        float64        `json:"recent_recovery_penalty,omitempty"`
	RecentTradeoffPenalty        float64        `json:"recent_tradeoff_penalty,omitempty"`
	RecentDecompositionPenalty   float64        `json:"recent_decomposition_penalty,omitempty"`
	RecentQualityGatePenalty     float64        `json:"recent_quality_gate_penalty,omitempty"`
	ContextFailures              int            `json:"context_failures,omitempty"`
	PlanningFailures             int            `json:"planning_failures,omitempty"`
	PlanQualityFailures          int            `json:"plan_quality_failures,omitempty"`
	ArchitectureFailures         int            `json:"architecture_failures,omitempty"`
	ValidationFailures           int            `json:"validation_failures,omitempty"`
	VerifierSuccesses            int            `json:"verifier_successes,omitempty"`
	RecoveryFailures             int            `json:"recovery_failures,omitempty"`
	TradeoffFailures             int            `json:"tradeoff_failures,omitempty"`
	DecompositionFailures        int            `json:"decomposition_failures,omitempty"`
	QuarantineCount              int            `json:"quarantine_count,omitempty"`
	LastLearningVerdict          string         `json:"last_learning_verdict,omitempty"`
	LastContextConfidence        int            `json:"last_context_confidence,omitempty"`
	LastPlanningConfidence       int            `json:"last_planning_confidence,omitempty"`
	LastDecompositionConfidence  int            `json:"last_decomposition_confidence,omitempty"`
	LastPlanQualityConfidence    int            `json:"last_plan_quality_confidence,omitempty"`
	LastArchitectureConfidence   int            `json:"last_architecture_confidence,omitempty"`
	LastValidationConfidence     int            `json:"last_validation_confidence,omitempty"`
	LastTradeoffConfidence       int            `json:"last_tradeoff_confidence,omitempty"`
	LastRecoveryConfidence       int            `json:"last_recovery_confidence,omitempty"`
	ChampionQualityScore         int            `json:"champion_quality_score,omitempty"`
	ChallengerWins               int            `json:"challenger_wins,omitempty"`
	ChallengerLosses             int            `json:"challenger_losses,omitempty"`
	PromotionState               string         `json:"promotion_state,omitempty"`
	AppliedCount                 int            `json:"applied_count,omitempty"`
	LastAppliedAt                string         `json:"last_applied_at,omitempty"`
	SkillName                    string         `json:"skill_name,omitempty"`
	SkillFilePath                string         `json:"skill_file_path,omitempty"`
	UpdatedAt                    string         `json:"updated_at,omitempty"`
}

type learnedPolicyStore struct {
	Version  int                           `json:"version"`
	Policies map[string]learnedRoutePolicy `json:"policies"`
}

type turnLearningTrace struct {
	SessionID            string
	Mode                 string
	WorkingDir           string
	Provider             string
	ReasoningEffort      string
	Prompt               string
	Family               learnedTaskFamily
	StartedAt            time.Time
	LoadedSkills         []string
	ActivePolicyID       string
	OrderedTools         []string
	ToolCalls            map[string]int
	ToolResults          map[string]int
	ToolErrorCodes       map[string]int
	StructuredEvidence   map[string]int
	ReadEvidence         map[string]int
	VerificationEvidence map[string]int
	BashDiscovery        int
	BlockedBash          int
	ValidationChecks     int
}

type completedTurnTrace struct {
	SessionID            string
	Mode                 string
	WorkingDir           string
	Provider             string
	ReasoningEffort      string
	Prompt               string
	Family               learnedTaskFamily
	StartedAt            time.Time
	LoadedSkills         []string
	ActivePolicyID       string
	OrderedTools         []string
	ToolCalls            map[string]int
	ToolResults          map[string]int
	ToolErrorCodes       map[string]int
	StructuredEvidence   map[string]int
	ReadEvidence         map[string]int
	VerificationEvidence map[string]int
	BashDiscovery        int
	BlockedBash          int
	ValidationChecks     int
	Status               string
	ClosureMode          string
	PhaseAtInterrupt     string
	ProviderFallbackUsed bool
	ResultText           string
	ResultSummary        string
	FinishedAt           time.Time
}

type singularityTurnAuditRecord struct {
	Timestamp                 string         `json:"timestamp"`
	SessionID                 string         `json:"session_id"`
	WorkingDir                string         `json:"working_dir,omitempty"`
	Mode                      string         `json:"mode,omitempty"`
	Provider                  string         `json:"provider,omitempty"`
	ReasoningEffort           string         `json:"reasoning_effort,omitempty"`
	TaskFamily                string         `json:"task_family"`
	GoalType                  string         `json:"goal_type"`
	Breadth                   string         `json:"breadth"`
	Status                    string         `json:"status"`
	ClosureMode               string         `json:"closure_mode,omitempty"`
	PhaseAtInterrupt          string         `json:"phase_at_interrupt,omitempty"`
	ProviderFallbackUsed      bool           `json:"provider_fallback_used,omitempty"`
	ActivePolicyID            string         `json:"active_policy_id,omitempty"`
	AppliedPolicy             bool           `json:"applied_policy"`
	PolicyState               string         `json:"policy_state,omitempty"`
	PolicyConfidence          int            `json:"policy_confidence,omitempty"`
	OrderedTools              []string       `json:"ordered_tools,omitempty"`
	ToolCalls                 map[string]int `json:"tool_calls,omitempty"`
	ToolResults               map[string]int `json:"tool_results,omitempty"`
	ToolErrorCodes            map[string]int `json:"tool_error_codes,omitempty"`
	StructuredEvidenceCount   int            `json:"structured_evidence_count,omitempty"`
	ReadEvidenceCount         int            `json:"read_evidence_count,omitempty"`
	VerificationEvidenceCount int            `json:"verification_evidence_count,omitempty"`
	LoadedSkills              []string       `json:"loaded_skills,omitempty"`
	BashDiscovery             int            `json:"bash_discovery,omitempty"`
	BlockedBashDiscovery      int            `json:"blocked_bash_discovery,omitempty"`
	ValidationChecks          int            `json:"validation_checks,omitempty"`
	ContextDiscipline         string         `json:"context_discipline,omitempty"`
	PlanningDiscipline        string         `json:"planning_discipline,omitempty"`
	DecompositionDiscipline   string         `json:"decomposition_discipline,omitempty"`
	PlanQualityDiscipline     string         `json:"plan_quality_discipline,omitempty"`
	ArchitectureDiscipline    string         `json:"architecture_discipline,omitempty"`
	ValidationDiscipline      string         `json:"validation_discipline,omitempty"`
	RecoveryDiscipline        string         `json:"recovery_discipline,omitempty"`
	TradeoffDiscipline        string         `json:"tradeoff_discipline,omitempty"`
	ExecutionRisk             string         `json:"execution_risk,omitempty"`
	LearningVerdict           string         `json:"learning_verdict,omitempty"`
	LearningBlockers          []string       `json:"learning_blockers,omitempty"`
	ContextConfidence         int            `json:"context_confidence,omitempty"`
	PlanningConfidence        int            `json:"planning_confidence,omitempty"`
	DecompositionConfidence   int            `json:"decomposition_confidence,omitempty"`
	PlanQualityConfidence     int            `json:"plan_quality_confidence,omitempty"`
	ArchitectureConfidence    int            `json:"architecture_confidence,omitempty"`
	ValidationConfidence      int            `json:"validation_confidence,omitempty"`
	TradeoffConfidence        int            `json:"tradeoff_confidence,omitempty"`
	RecoveryConfidence        int            `json:"recovery_confidence,omitempty"`
	ResultSummary             string         `json:"result_summary,omitempty"`
}

type kernelMutationBoundary struct {
	repoRoot       string
	learnableRoots []string
}

func newKernelMutationBoundary(repoRoot, dataDir string) kernelMutationBoundary {
	learnableRoots := []string{
		filepath.Clean(skills.ProjectSkillsDir(dataDir)),
		filepath.Clean(filepath.Join(dataDir, singularityDirName)),
	}
	return kernelMutationBoundary{
		repoRoot:       filepath.Clean(repoRoot),
		learnableRoots: learnableRoots,
	}
}

func (b kernelMutationBoundary) AllowsAutoWrite(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	abs := filepath.Clean(path)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(b.repoRoot, abs)
	}
	for _, root := range b.learnableRoots {
		if root == "" {
			continue
		}
		if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (b kernelMutationBoundary) IsKernelProtected(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	abs := filepath.Clean(path)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(b.repoRoot, abs)
	}
	if b.AllowsAutoWrite(abs) {
		return false
	}
	rel, err := filepath.Rel(b.repoRoot, abs)
	if err != nil {
		return true
	}
	rel = filepath.ToSlash(rel)
	protectedPrefixes := []string{
		"internal/agent/templates/",
		"internal/agent/tools/",
		"internal/agent/formulas/",
		"internal/config/",
	}
	protectedFiles := []string{
		"internal/agent/prompts.go",
		"internal/agent/tool_catalog_prompt.go",
	}
	for _, prefix := range protectedPrefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	for _, file := range protectedFiles {
		if rel == file {
			return true
		}
	}
	return false
}

type singularityManager struct {
	repoRoot   string
	dataDir    string
	policyDir  string
	historyDir string
	policyPath string
	auditPath  string
	skillRoot  string
	boundary   kernelMutationBoundary

	mu     sync.Mutex
	store  learnedPolicyStore
	active map[string]*turnLearningTrace
}

func newSingularityManager(cfg *config.Config) *singularityManager {
	if cfg == nil || cfg.Options == nil {
		return nil
	}
	dataDir := strings.TrimSpace(cfg.Options.DataDirectory)
	repoRoot := strings.TrimSpace(cfg.WorkingDir())
	if dataDir == "" || repoRoot == "" {
		return nil
	}
	manager := &singularityManager{
		repoRoot:   filepath.Clean(repoRoot),
		dataDir:    filepath.Clean(dataDir),
		policyDir:  filepath.Join(filepath.Clean(dataDir), singularityDirName),
		historyDir: filepath.Join(filepath.Clean(dataDir), singularityDirName, singularityHistoryDirName),
		policyPath: filepath.Join(filepath.Clean(dataDir), singularityDirName, singularityPolicyFileName),
		auditPath:  filepath.Join(filepath.Clean(dataDir), singularityDirName, singularityAuditFileName),
		skillRoot:  filepath.Clean(skills.ProjectSkillsDir(dataDir)),
		boundary:   newKernelMutationBoundary(repoRoot, dataDir),
		store: learnedPolicyStore{
			Version:  singularityStoreVersion,
			Policies: map[string]learnedRoutePolicy{},
		},
		active: map[string]*turnLearningTrace{},
	}
	manager.load()
	return manager
}

func (m *singularityManager) load() {
	if m == nil {
		return
	}
	data, err := os.ReadFile(m.policyPath)
	if err != nil {
		return
	}
	var store learnedPolicyStore
	if err := json.Unmarshal(data, &store); err != nil {
		return
	}
	if store.Policies == nil {
		store.Policies = map[string]learnedRoutePolicy{}
	}
	if store.Version == 0 {
		store.Version = singularityStoreVersion
	}
	for taskFamily, policy := range store.Policies {
		store.Policies[taskFamily] = normalizeLoadedLearnedRoutePolicy(policy)
	}
	m.store = store
}

func (m *singularityManager) LookupPolicy(prompt string) (learnedRoutePolicy, learnedTaskFamily, bool) {
	family := classifyLearnedTaskFamily(prompt)
	if m == nil {
		return learnedRoutePolicy{}, family, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	policy, ok := m.store.Policies[family.ID]
	if !ok {
		return learnedRoutePolicy{}, family, false
	}
	policy = normalizeLoadedLearnedRoutePolicy(policy)
	if !isInjectableLearnedPolicy(policy) {
		return learnedRoutePolicy{}, family, false
	}
	return policy, family, true
}

func (m *singularityManager) StartTurn(sessionID, prompt, workingDir string, loadedSkills []string, policy learnedRoutePolicy) learnedTaskFamily {
	return m.StartTurnWithModeAndModel(sessionID, prompt, workingDir, loadedSkills, policy, planmode.DefaultSessionMode, config.SelectedModel{})
}

func (m *singularityManager) StartTurnWithMode(sessionID, prompt, workingDir string, loadedSkills []string, policy learnedRoutePolicy, mode planmode.SessionMode) learnedTaskFamily {
	return m.StartTurnWithModeAndModel(sessionID, prompt, workingDir, loadedSkills, policy, mode, config.SelectedModel{})
}

func (m *singularityManager) StartTurnWithModeAndModel(sessionID, prompt, workingDir string, loadedSkills []string, policy learnedRoutePolicy, mode planmode.SessionMode, modelCfg config.SelectedModel) learnedTaskFamily {
	family := classifyLearnedTaskFamily(prompt)
	if m == nil || !shouldTrackLearnedTurn(prompt, family) {
		return family
	}
	trace := &turnLearningTrace{
		SessionID:            strings.TrimSpace(sessionID),
		Mode:                 string(planmode.NormalizeMode(mode)),
		WorkingDir:           strings.TrimSpace(workingDir),
		Provider:             strings.TrimSpace(modelCfg.Provider),
		ReasoningEffort:      strings.TrimSpace(modelCfg.ReasoningEffort),
		Prompt:               strings.TrimSpace(prompt),
		Family:               family,
		StartedAt:            time.Now().UTC(),
		LoadedSkills:         compactLearnedStrings(loadedSkills),
		ActivePolicyID:       strings.TrimSpace(policy.TaskFamily),
		ToolCalls:            map[string]int{},
		ToolResults:          map[string]int{},
		ToolErrorCodes:       map[string]int{},
		StructuredEvidence:   map[string]int{},
		ReadEvidence:         map[string]int{},
		VerificationEvidence: map[string]int{},
	}
	m.mu.Lock()
	m.active[trace.SessionID] = trace
	m.mu.Unlock()
	return family
}

func (m *singularityManager) RecordToolCall(sessionID, toolName, rawInput string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	trace := m.active[strings.TrimSpace(sessionID)]
	if trace == nil {
		return
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return
	}
	trace.OrderedTools = append(trace.OrderedTools, toolName)
	m.applyObservedToolCall(trace, toolName, rawInput)
}

func (m *singularityManager) ReconcileToolExecution(sessionID, requestedToolName, requestedInput, executedToolName, executedInput string) {
	if m == nil {
		return
	}
	requestedToolName = strings.TrimSpace(requestedToolName)
	executedToolName = strings.TrimSpace(executedToolName)
	if requestedToolName == "" || executedToolName == "" || requestedToolName == executedToolName {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	trace := m.active[strings.TrimSpace(sessionID)]
	if trace == nil {
		return
	}

	m.removeObservedToolCall(trace, requestedToolName, requestedInput)
	replaced := false
	for idx := len(trace.OrderedTools) - 1; idx >= 0; idx-- {
		if strings.TrimSpace(trace.OrderedTools[idx]) == requestedToolName {
			trace.OrderedTools[idx] = executedToolName
			replaced = true
			break
		}
	}
	if !replaced {
		trace.OrderedTools = append(trace.OrderedTools, executedToolName)
	}
	m.applyObservedToolCall(trace, executedToolName, executedInput)
}

func (m *singularityManager) RecordToolResult(sessionID, toolName, content, metadata string, isError bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	trace := m.active[strings.TrimSpace(sessionID)]
	if trace == nil {
		return
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return
	}
	trace.ToolResults[toolName]++
	if !isError {
		return
	}
	meta, ok := tools.ParseToolErrorMetadata(metadata)
	if !ok {
		meta, ok = tools.DeriveToolErrorMetadata(toolName, content)
	}
	if !ok || strings.TrimSpace(meta.Code) == "" {
		return
	}
	trace.ToolErrorCodes[meta.Code]++
	if toolName == tools.BashToolName && meta.Code == "learned_route_policy" {
		trace.BlockedBash++
	}
}

func (m *singularityManager) applyObservedToolCall(trace *turnLearningTrace, toolName, rawInput string) {
	if trace == nil {
		return
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return
	}
	trace.ToolCalls[toolName]++
	recordTraceContextEvidence(trace, toolName, rawInput, 1)
	if toolName == tools.BashToolName {
		if command := extractObservedBashCommand(rawInput); command != "" {
			if looksLikeObservedDiscoveryBash(command) {
				trace.BashDiscovery++
			}
			if looksLikeValidationShellCommand(command) {
				trace.ValidationChecks++
			}
		}
	}
	if strings.Contains(strings.ToLower(rawInput), "agents.md") || strings.Contains(strings.ToLower(rawInput), "agent.md") {
		switch toolName {
		case tools.ViewToolName, tools.SingleViewToolName, tools.AgenticViewToolName, tools.RGToolName, tools.GrepToolName:
			trace.ValidationChecks++
		}
	}
	if toolName == "lsp_diagnostics" {
		trace.ValidationChecks++
	}
}

func (m *singularityManager) removeObservedToolCall(trace *turnLearningTrace, toolName, rawInput string) {
	if trace == nil {
		return
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return
	}
	if current := trace.ToolCalls[toolName]; current > 1 {
		trace.ToolCalls[toolName] = current - 1
	} else {
		delete(trace.ToolCalls, toolName)
	}
	recordTraceContextEvidence(trace, toolName, rawInput, -1)
	if toolName == tools.BashToolName {
		if command := extractObservedBashCommand(rawInput); command != "" {
			if looksLikeObservedDiscoveryBash(command) && trace.BashDiscovery > 0 {
				trace.BashDiscovery--
			}
			if looksLikeValidationShellCommand(command) && trace.ValidationChecks > 0 {
				trace.ValidationChecks--
			}
		}
	}
	if strings.Contains(strings.ToLower(rawInput), "agents.md") || strings.Contains(strings.ToLower(rawInput), "agent.md") {
		switch toolName {
		case tools.ViewToolName, tools.SingleViewToolName, tools.AgenticViewToolName, tools.RGToolName, tools.GrepToolName:
			if trace.ValidationChecks > 0 {
				trace.ValidationChecks--
			}
		}
	}
	if toolName == "lsp_diagnostics" && trace.ValidationChecks > 0 {
		trace.ValidationChecks--
	}
}

func recordTraceContextEvidence(trace *turnLearningTrace, toolName, rawInput string, delta int) {
	if trace == nil || delta == 0 {
		return
	}
	input := parseObservedToolInput(rawInput)
	if len(input) == 0 {
		return
	}
	evidence := tools.ExtractContextEvidence(toolName, input)
	adjustTraceEvidence(trace.StructuredEvidence, evidence.Structured, delta)
	adjustTraceEvidence(trace.ReadEvidence, evidence.Read, delta)
	adjustTraceEvidence(trace.VerificationEvidence, evidence.Verification, delta)
}

func adjustTraceEvidence(target map[string]int, values []string, delta int) {
	if len(values) == 0 || target == nil {
		return
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		next := target[value] + delta
		if next <= 0 {
			delete(target, value)
			continue
		}
		target[value] = next
	}
}

func (m *singularityManager) FinishTurn(sessionID, status, resultSummary string) *completedTurnTrace {
	return m.FinishTurnWithMetadata(sessionID, status, resultSummary, turnCompletionMetadata{})
}

func (m *singularityManager) FinishTurnWithMetadata(sessionID, status, resultSummary string, meta turnCompletionMetadata) *completedTurnTrace {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	trace := m.active[strings.TrimSpace(sessionID)]
	delete(m.active, strings.TrimSpace(sessionID))
	m.mu.Unlock()
	if trace == nil {
		return nil
	}
	return &completedTurnTrace{
		SessionID:            trace.SessionID,
		Mode:                 trace.Mode,
		WorkingDir:           trace.WorkingDir,
		Provider:             trace.Provider,
		ReasoningEffort:      trace.ReasoningEffort,
		Prompt:               trace.Prompt,
		Family:               trace.Family,
		StartedAt:            trace.StartedAt,
		LoadedSkills:         append([]string{}, trace.LoadedSkills...),
		ActivePolicyID:       trace.ActivePolicyID,
		OrderedTools:         append([]string{}, trace.OrderedTools...),
		ToolCalls:            cloneStringIntMap(trace.ToolCalls),
		ToolResults:          cloneStringIntMap(trace.ToolResults),
		ToolErrorCodes:       cloneStringIntMap(trace.ToolErrorCodes),
		StructuredEvidence:   cloneStringIntMap(trace.StructuredEvidence),
		ReadEvidence:         cloneStringIntMap(trace.ReadEvidence),
		VerificationEvidence: cloneStringIntMap(trace.VerificationEvidence),
		BashDiscovery:        trace.BashDiscovery,
		BlockedBash:          trace.BlockedBash,
		ValidationChecks:     trace.ValidationChecks,
		Status:               strings.TrimSpace(status),
		ClosureMode:          strings.TrimSpace(meta.ClosureMode),
		PhaseAtInterrupt:     strings.TrimSpace(meta.PhaseAtInterrupt),
		ProviderFallbackUsed: meta.ProviderFallbackUsed,
		ResultText:           compactLearnedText(strings.TrimSpace(resultSummary), 2400),
		ResultSummary:        compactLearnedText(strings.TrimSpace(resultSummary), 600),
		FinishedAt:           time.Now().UTC(),
	}
}

func (m *singularityManager) CompileTurn(_ context.Context, trace *completedTurnTrace) {
	_ = m.CompileTurnSync(context.Background(), trace)
}

func (m *singularityManager) CompileTurnSync(_ context.Context, trace *completedTurnTrace) error {
	if m == nil || trace == nil {
		return nil
	}
	if !shouldTrackLearnedTurn(trace.Prompt, trace.Family) {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	policy := m.store.Policies[trace.Family.ID]
	policy = mergeCompletedTurnIntoPolicy(policy, trace)
	if err := m.materializeLearnedSkill(&policy); err == nil {
		if policy.SkillName != "" {
			policy.PreferredSkills = []string{policy.SkillName}
		}
	}
	m.store.Policies[trace.Family.ID] = policy
	if err := m.persistLocked(); err != nil {
		return err
	}
	if err := m.appendTurnAuditLocked(trace, policy); err != nil {
		return err
	}
	return nil
}

func (m *singularityManager) persistLocked() error {
	if m == nil {
		return nil
	}
	if err := os.MkdirAll(m.policyDir, 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(m.store, "", "  ")
	if err != nil {
		return err
	}
	current, err := os.ReadFile(m.policyPath)
	switch {
	case err == nil:
		if bytes.Equal(current, payload) {
			return nil
		}
		if err := m.snapshotPolicyHistoryLocked(current); err != nil {
			return err
		}
	case os.IsNotExist(err):
	default:
		return err
	}
	return os.WriteFile(m.policyPath, payload, 0o644)
}

func (m *singularityManager) snapshotPolicyHistoryLocked(current []byte) error {
	if m == nil || len(bytes.TrimSpace(current)) == 0 {
		return nil
	}
	if err := os.MkdirAll(m.historyDir, 0o755); err != nil {
		return err
	}
	snapshotPath := filepath.Join(
		m.historyDir,
		fmt.Sprintf("route_policies-%s.json", time.Now().UTC().Format("20060102T150405.000000000Z")),
	)
	if err := os.WriteFile(snapshotPath, current, 0o644); err != nil {
		return err
	}
	return trimSingularityHistory(m.historyDir, maxSingularityPolicySnapshots)
}

func (m *singularityManager) RenderPromptHints(policy learnedRoutePolicy) (string, []string) {
	if m == nil || !isInjectableLearnedPolicy(policy) {
		return "", nil
	}
	parts := []string{renderLearnedRoutePolicyBlock(policy)}
	var activeSkills []string
	if skillContext, skillName := m.loadSkillContext(policy); skillContext != "" {
		parts = append(parts, skillContext)
		activeSkills = append(activeSkills, skillName)
	}
	return strings.Join(parts, "\n\n"), compactLearnedStrings(activeSkills)
}

func (m *singularityManager) loadSkillContext(policy learnedRoutePolicy) (string, string) {
	if m == nil {
		return "", ""
	}
	path := strings.TrimSpace(policy.SkillFilePath)
	if path == "" || !m.boundary.AllowsAutoWrite(path) || m.boundary.IsKernelProtected(path) {
		return "", ""
	}
	skill, err := skills.Parse(path)
	if err != nil || strings.TrimSpace(skill.Instructions) == "" {
		return "", ""
	}
	return "<learned_project_skill>\n" + strings.TrimSpace(skill.Instructions) + "\n</learned_project_skill>", strings.TrimSpace(skill.Name)
}

func (m *singularityManager) LearnedToolPolicy(policy learnedRoutePolicy) tools.LearnedToolPolicy {
	if !isInjectableLearnedPolicy(policy) {
		return tools.LearnedToolPolicy{}
	}
	return tools.LearnedToolPolicy{
		TaskFamily:                   policy.TaskFamily,
		Reason:                       "learned route policy for recurring " + policy.TaskFamily + " turns",
		ForbidBashDiscovery:          policy.ForbidBashDiscovery,
		PreferStructuredDiscovery:    policy.RequireContextRead,
		RequireContextRead:           policy.RequireContextRead,
		RequireExplicitPlan:          policy.RequireExplicitPlan,
		RequirePostWriteVerification: policy.RequirePostWriteVerification,
	}
}

func (m *singularityManager) LearnedHarnessRequirement(prompt string, policy learnedRoutePolicy) *tools.HarnessRequirement {
	if !isInjectableLearnedPolicy(policy) || !policy.RequireHarness {
		return nil
	}
	requirement := buildHarnessRequirement(prompt)
	requirement.Required = true
	requirement.Reason = "learned route policy for recurring " + policy.TaskFamily + " turns"
	requirement.ComplexityScore = max(requirement.ComplexityScore, 3)
	requirement.Task = strings.TrimSpace(prompt)
	return &requirement
}

func classifyLearnedTaskFamily(prompt string) learnedTaskFamily {
	decision := evaluateSubAgentLaunch(prompt)
	goal := inferHarnessGoalType(prompt, "")
	if isInitializationStylePrompt(prompt) {
		return learnedTaskFamily{
			ID:       "initialize/broad/codebase",
			GoalType: "initialize",
			Breadth:  "broad",
			Domains:  []string{"codebase"},
		}
	}
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	breadth := "focused"
	if hasAnySignal(normalized, subAgentCodebaseSignals) || hasAnySignal(normalized, subAgentDependencySignals) || hasAnySignal(normalized, subAgentRiskSignals) || decision.Complexity >= 4 {
		breadth = "broad"
	}
	if (goal == "design" || goal == "research") && hasAnySignal(normalized, []string{"repository", "repo", "codebase", "across", "compare", "trade-off", "tradeoff", "auth", "billing", "search", "investigate"}) {
		breadth = "broad"
	}
	domains := append([]string{}, decision.Domains...)
	if len(domains) == 0 {
		domains = []string{"general"}
	}
	sort.Strings(domains)
	return learnedTaskFamily{
		ID:       fmt.Sprintf("%s/%s/%s", goal, breadth, strings.Join(domains, "+")),
		GoalType: goal,
		Breadth:  breadth,
		Domains:  domains,
	}
}

func shouldTrackLearnedTurn(prompt string, family learnedTaskFamily) bool {
	decision := evaluateSubAgentLaunch(prompt)
	if decision.Complexity >= 2 {
		return true
	}
	return family.GoalType == "initialize" || family.Breadth == "broad"
}

func mergeCompletedTurnIntoPolicy(existing learnedRoutePolicy, trace *completedTurnTrace) learnedRoutePolicy {
	policy := normalizeLoadedLearnedRoutePolicy(existing)
	if policy.TaskFamily == "" {
		policy.TaskFamily = trace.Family.ID
		policy.TaskFamilySlug = sanitizeLearnedSlug(trace.Family.ID)
		policy.GoalType = trace.Family.GoalType
		policy.Breadth = trace.Family.Breadth
		policy.Domains = append([]string{}, trace.Family.Domains...)
		policy.ToolSuccessCounts = map[string]int{}
		policy.ToolFailureCounts = map[string]int{}
	} else {
		if policy.ToolSuccessCounts == nil {
			policy.ToolSuccessCounts = map[string]int{}
		}
		if policy.ToolFailureCounts == nil {
			policy.ToolFailureCounts = map[string]int{}
		}
	}

	policy = applyRecentLearnedPolicyDecay(policy)
	policy.EvidenceCount++
	if strings.TrimSpace(trace.ActivePolicyID) == trace.Family.ID {
		policy.AppliedCount++
		policy.LastAppliedAt = time.Now().UTC().Format(time.RFC3339)
	}
	success := strings.EqualFold(strings.TrimSpace(trace.Status), "completed")
	seenTools := uniqueLearnedToolNames(trace.OrderedTools)
	structuredDiscovery := hasStructuredDiscovery(trace)
	harnessSuccess := containsTool(seenTools, tools.RunHarnessToolName)
	indexSuccess := containsTool(seenTools, tools.IndexCodebaseToolName)
	parallelSuccess := containsAnyTool(seenTools, SpawnAgentToolName, WaitAgentsToolName, CollectResultToolName)
	assessment := assessSingularityCognition(trace)
	experience := compileSingularityExperience(trace)
	learningVerdict := evaluateSingularityLearningVerdict(trace, assessment)
	applyLearningVector(&policy, learningVerdict)
	qualityScore := deriveLearnedQualityScore(learningVerdict)
	if success && learningVerdict.Decision == singularityLearningVerdictAccepted {
		policy.SuccessCount++
		policy.RecentSuccessWeight += 1
		for _, toolName := range seenTools {
			policy.ToolSuccessCounts[toolName]++
		}
		if structuredDiscovery {
			policy.StructuredSuccesses++
			policy.RecentStructuredWeight += 1
		}
		if harnessSuccess {
			policy.HarnessSuccesses++
			policy.RecentHarnessWeight += 1
		}
		if indexSuccess {
			policy.IndexSuccesses++
			policy.RecentIndexWeight += 1
		}
		if parallelSuccess {
			policy.ParallelSuccesses++
			policy.RecentParallelWeight += 1
		}
		if experience.Verification.Discipline == "strong" {
			policy.VerifierSuccesses++
			policy.RecentVerifierWeight += 1
		}
		if trace.BashDiscovery > 0 {
			policy.BashDiscoverySuccess += trace.BashDiscovery
			policy.RecentBashSuccessWeight += float64(trace.BashDiscovery)
		}
		policy = recordChampionChallengerOutcome(policy, qualityScore, true)
	} else if success {
		policy.QuarantineCount++
		policy.RecentQualityGatePenalty += 1
		policy = recordChampionChallengerOutcome(policy, qualityScore, false)
	} else {
		policy.FailureCount++
		policy.RecentFailureWeight += 1
		for _, toolName := range seenTools {
			policy.ToolFailureCounts[toolName]++
		}
		if trace.BashDiscovery > 0 {
			policy.BashDiscoveryFailures += trace.BashDiscovery
			policy.RecentBashFailureWeight += float64(trace.BashDiscovery)
		}
		policy = recordChampionChallengerOutcome(policy, qualityScore, false)
	}
	if trace.BlockedBash > 0 {
		policy.BashDiscoveryFailures += trace.BlockedBash
		policy.RecentBashFailureWeight += float64(trace.BlockedBash)
	}
	policy = applyCognitiveAssessment(policy, assessment)
	policy.LastLearningVerdict = learningVerdict.Decision

	policy.PreferredDiscovery = derivePreferredDiscoveryTools(policy)
	policy.PreferredVerification = mergeLearnedDiscoveryPreference(experience.Verification.PreferredTools, policy.PreferredVerification, 4)
	policy.RequireHarness = deriveHarnessRequirement(policy)
	policy.PreferParallel = deriveParallelPreference(policy)
	policy.PreferIndexCodebase = deriveIndexPreference(policy)
	policy.ForbidBashDiscovery = deriveBashDiscoveryBan(policy)
	policy.RequireContextRead = deriveContextReadRequirement(policy)
	policy.RequireExplicitPlan = deriveExplicitPlanRequirement(policy)
	policy.RequirePostWriteVerification = derivePostWriteVerificationRequirement(policy)
	if promptMentionsAgentsArtifact(trace.Prompt) {
		policy.RequirePostWriteVerification = true
	}
	policy.Confidence = derivePolicyConfidence(policy)
	policy.PromotionState = derivePromotionState(policy)
	policy.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return policy
}

func derivePreferredDiscoveryTools(policy learnedRoutePolicy) []string {
	candidates := []string{
		"tool_search",
		tools.RGFilesToolName,
		tools.RGToolName,
		tools.AgenticViewToolName,
		tools.LSToolName,
	}
	type scoredTool struct {
		name  string
		score int
	}
	scored := make([]scoredTool, 0, len(candidates))
	for _, name := range candidates {
		if policy.ToolSuccessCounts[name] <= 0 {
			continue
		}
		scored = append(scored, scoredTool{name: name, score: policy.ToolSuccessCounts[name]})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].name < scored[j].name
		}
		return scored[i].score > scored[j].score
	})
	out := make([]string, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.name)
	}
	if len(out) > 4 {
		out = out[:4]
	}
	if policy.GoalType == "initialize" && policy.Breadth == "broad" {
		out = mergeLearnedDiscoveryPreference([]string{
			tools.ToolSearchToolName,
			tools.AgenticViewToolName,
			tools.RGFilesToolName,
			tools.RGToolName,
			tools.LSToolName,
		}, out, 5)
	}
	return out
}

func deriveHarnessRequirement(policy learnedRoutePolicy) bool {
	if policy.RecentSuccessWeight <= 0 && policy.SuccessCount == 0 {
		return false
	}
	if policy.GoalType == "initialize" && policy.Breadth == "broad" && policy.RecentStructuredWeight >= 0.8 {
		return true
	}
	if policy.Breadth == "broad" && policy.RecentHarnessWeight >= 0.9 {
		return true
	}
	return policy.RecentHarnessWeight >= 1.6 || policy.HarnessSuccesses >= 2
}

func deriveParallelPreference(policy learnedRoutePolicy) bool {
	if policy.RecentSuccessWeight <= 0 && policy.SuccessCount == 0 {
		return false
	}
	if policy.GoalType == "initialize" && policy.Breadth == "broad" && policy.RecentStructuredWeight >= 0.8 {
		return true
	}
	if policy.Breadth == "broad" && policy.RecentParallelWeight >= 0.9 {
		return true
	}
	return policy.RecentParallelWeight >= 1.6 || policy.ParallelSuccesses >= 2
}

func deriveIndexPreference(policy learnedRoutePolicy) bool {
	if policy.Breadth != "broad" {
		return false
	}
	return policy.RecentIndexWeight >= 0.8 || policy.IndexSuccesses >= 1
}

func deriveBashDiscoveryBan(policy learnedRoutePolicy) bool {
	if policy.GoalType == "initialize" && policy.Breadth == "broad" && policy.RecentStructuredWeight >= 0.8 {
		return true
	}
	if policy.StructuredSuccesses == 0 && policy.RecentStructuredWeight < 1 {
		return false
	}
	if policy.RecentStructuredWeight < 1 {
		return false
	}
	return policy.RecentBashFailureWeight > policy.RecentBashSuccessWeight+0.25
}

func deriveExplicitPlanRequirement(policy learnedRoutePolicy) bool {
	return policy.Breadth == "broad" && (policy.GoalType == "design" || policy.GoalType == "research" || policy.GoalType == "review" || policy.GoalType == "migration" || (policy.GoalType == "implementation" && policy.RecentHarnessWeight >= 0.75))
}

func derivePostWriteVerificationRequirement(policy learnedRoutePolicy) bool {
	return policy.GoalType == "initialize" && policy.Breadth == "broad"
}

func deriveContextReadRequirement(policy learnedRoutePolicy) bool {
	return policy.Breadth == "broad"
}

func applyCognitiveAssessment(policy learnedRoutePolicy, assessment singularityCognitiveAssessment) learnedRoutePolicy {
	applyPenalty := func(value string, failures *int, recent *float64) {
		if value != "weak" {
			return
		}
		(*failures)++
		*recent += 1
	}

	applyPenalty(assessment.ContextDiscipline, &policy.ContextFailures, &policy.RecentContextPenalty)
	applyPenalty(assessment.PlanningDiscipline, &policy.PlanningFailures, &policy.RecentPlanningPenalty)
	applyPenalty(assessment.DecompositionDiscipline, &policy.DecompositionFailures, &policy.RecentDecompositionPenalty)
	applyPenalty(assessment.PlanQualityDiscipline, &policy.PlanQualityFailures, &policy.RecentPlanQualityPenalty)
	applyPenalty(assessment.ArchitectureDiscipline, &policy.ArchitectureFailures, &policy.RecentArchitecturePenalty)
	applyPenalty(assessment.ValidationDiscipline, &policy.ValidationFailures, &policy.RecentValidationPenalty)
	applyPenalty(assessment.RecoveryDiscipline, &policy.RecoveryFailures, &policy.RecentRecoveryPenalty)
	applyPenalty(assessment.TradeoffDiscipline, &policy.TradeoffFailures, &policy.RecentTradeoffPenalty)
	return policy
}

func applyLearningVector(policy *learnedRoutePolicy, verdict singularityLearningVerdict) {
	if policy == nil {
		return
	}
	policy.LastContextConfidence = verdict.Vector.Context
	policy.LastPlanningConfidence = verdict.Vector.Planning
	policy.LastDecompositionConfidence = verdict.Vector.Decomposition
	policy.LastPlanQualityConfidence = verdict.Vector.PlanQuality
	policy.LastArchitectureConfidence = verdict.Vector.Architecture
	policy.LastValidationConfidence = verdict.Vector.Validation
	policy.LastTradeoffConfidence = verdict.Vector.Tradeoff
	policy.LastRecoveryConfidence = verdict.Vector.Recovery
}

func derivePolicyConfidence(policy learnedRoutePolicy) int {
	score := 10
	score += int(policy.RecentSuccessWeight * 24)
	score += int(policy.RecentStructuredWeight * 12)
	score += int(policy.RecentHarnessWeight * 8)
	score += int(policy.RecentParallelWeight * 6)
	score += int(policy.RecentIndexWeight * 4)
	score += int(policy.RecentVerifierWeight * 6)
	score += min(policy.AppliedCount*2, 6)
	if policy.GoalType == "initialize" && policy.Breadth == "broad" && policy.RecentStructuredWeight >= 0.8 {
		score += 12
	}
	score -= int(policy.RecentFailureWeight * 20)
	score -= int(policy.RecentBashFailureWeight * 8)
	score -= int(policy.RecentContextPenalty * 10)
	score -= int(policy.RecentPlanningPenalty * 10)
	score -= int(policy.RecentDecompositionPenalty * 9)
	score -= int(policy.RecentPlanQualityPenalty * 10)
	score -= int(policy.RecentArchitecturePenalty * 10)
	score -= int(policy.RecentValidationPenalty * 8)
	score -= int(policy.RecentRecoveryPenalty * 6)
	score -= int(policy.RecentTradeoffPenalty * 6)
	score -= int(policy.RecentQualityGatePenalty * 12)
	score += min(policy.ChallengerWins*2, 6)
	score -= min(policy.ChallengerLosses*2, 6)
	if policy.SuccessCount == 0 && policy.FailureCount < 2 {
		if policy.QuarantineCount > 0 {
			return max(0, min(35, score))
		}
		return 0
	}
	return max(0, min(95, score))
}

func derivePromotionState(policy learnedRoutePolicy) string {
	requiresChallenger := requiresChallengerPromotion(policy)
	switch {
	case policy.RecentQualityGatePenalty >= 0.8 || (policy.QuarantineCount > 0 && policy.LastLearningVerdict == singularityLearningVerdictQuarantined):
		return learnedPolicyStateQuarantined
	case policy.Confidence < 25:
		return learnedPolicyStateDemoted
	case policy.RecentContextPenalty >= 1.1 || policy.RecentPlanningPenalty >= 1.1 || policy.RecentDecompositionPenalty >= 1.1 || policy.RecentValidationPenalty >= 1.1:
		return learnedPolicyStateObserver
	case policy.RecentFailureWeight > policy.RecentSuccessWeight+0.4 && policy.Confidence < minPolicyConfidenceForInjection:
		return learnedPolicyStateDemoted
	case requiresChallenger && policy.ChallengerWins < 2:
		return learnedPolicyStateObserver
	case policy.Confidence >= 78 && policy.RecentSuccessWeight >= 1.8 && policy.RecentFailureWeight <= 0.7:
		if requiresChallenger && policy.ChallengerWins < 3 {
			return learnedPolicyStateCandidate
		}
		return learnedPolicyStatePromoted
	case policy.Confidence >= minPolicyConfidenceForInjection && policy.RecentSuccessWeight >= 0.75:
		if requiresChallenger && policy.ChallengerWins < 2 {
			return learnedPolicyStateObserver
		}
		return learnedPolicyStateCandidate
	default:
		return learnedPolicyStateObserver
	}
}

func isInjectableLearnedPolicy(policy learnedRoutePolicy) bool {
	if policy.Confidence < minPolicyConfidenceForInjection {
		return false
	}
	switch normalizeLearnedPromotionState(policy.PromotionState, policy.Confidence, policy.EvidenceCount) {
	case learnedPolicyStateCandidate, learnedPolicyStatePromoted:
		return true
	default:
		return false
	}
}

func normalizeLoadedLearnedRoutePolicy(policy learnedRoutePolicy) learnedRoutePolicy {
	if policy.ToolSuccessCounts == nil {
		policy.ToolSuccessCounts = map[string]int{}
	}
	if policy.ToolFailureCounts == nil {
		policy.ToolFailureCounts = map[string]int{}
	}
	if learnedPolicyRecentWeightsZero(policy) {
		policy.RecentSuccessWeight = minFloat64(float64(policy.SuccessCount), 2.5)
		policy.RecentFailureWeight = minFloat64(float64(policy.FailureCount), 2.5)
		policy.RecentStructuredWeight = minFloat64(float64(policy.StructuredSuccesses), 2.5)
		policy.RecentHarnessWeight = minFloat64(float64(policy.HarnessSuccesses), 2.0)
		policy.RecentParallelWeight = minFloat64(float64(policy.ParallelSuccesses), 2.0)
		policy.RecentIndexWeight = minFloat64(float64(policy.IndexSuccesses), 2.0)
		policy.RecentVerifierWeight = minFloat64(float64(policy.VerifierSuccesses), 2.0)
		policy.RecentBashFailureWeight = minFloat64(float64(policy.BashDiscoveryFailures), 2.0)
		policy.RecentBashSuccessWeight = minFloat64(float64(policy.BashDiscoverySuccess), 2.0)
	}
	policy.PromotionState = normalizeLearnedPromotionState(policy.PromotionState, policy.Confidence, policy.EvidenceCount)
	return policy
}

func normalizeLearnedPromotionState(state string, confidence int, evidence int) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case learnedPolicyStateObserver, learnedPolicyStateCandidate, learnedPolicyStatePromoted, learnedPolicyStateQuarantined, learnedPolicyStateDemoted:
		return strings.TrimSpace(strings.ToLower(state))
	}
	switch {
	case confidence >= 78:
		return learnedPolicyStatePromoted
	case confidence >= minPolicyConfidenceForInjection:
		return learnedPolicyStateCandidate
	case confidence > 0 || evidence > 0:
		return learnedPolicyStateObserver
	default:
		return learnedPolicyStateDemoted
	}
}

func learnedPolicyRecentWeightsZero(policy learnedRoutePolicy) bool {
	return policy.RecentSuccessWeight == 0 &&
		policy.RecentFailureWeight == 0 &&
		policy.RecentStructuredWeight == 0 &&
		policy.RecentHarnessWeight == 0 &&
		policy.RecentParallelWeight == 0 &&
		policy.RecentIndexWeight == 0 &&
		policy.RecentVerifierWeight == 0 &&
		policy.RecentBashFailureWeight == 0 &&
		policy.RecentBashSuccessWeight == 0 &&
		policy.RecentContextPenalty == 0 &&
		policy.RecentPlanningPenalty == 0 &&
		policy.RecentDecompositionPenalty == 0 &&
		policy.RecentPlanQualityPenalty == 0 &&
		policy.RecentArchitecturePenalty == 0 &&
		policy.RecentValidationPenalty == 0 &&
		policy.RecentRecoveryPenalty == 0 &&
		policy.RecentTradeoffPenalty == 0 &&
		policy.RecentQualityGatePenalty == 0
}

func applyRecentLearnedPolicyDecay(policy learnedRoutePolicy) learnedRoutePolicy {
	policy.RecentSuccessWeight *= learnedPolicyDecayFactor
	policy.RecentFailureWeight *= learnedPolicyDecayFactor
	policy.RecentStructuredWeight *= learnedPolicyDecayFactor
	policy.RecentHarnessWeight *= learnedPolicyDecayFactor
	policy.RecentParallelWeight *= learnedPolicyDecayFactor
	policy.RecentIndexWeight *= learnedPolicyDecayFactor
	policy.RecentVerifierWeight *= learnedPolicyDecayFactor
	policy.RecentBashFailureWeight *= learnedPolicyDecayFactor
	policy.RecentBashSuccessWeight *= learnedPolicyDecayFactor
	policy.RecentContextPenalty *= learnedPolicyDecayFactor
	policy.RecentPlanningPenalty *= learnedPolicyDecayFactor
	policy.RecentDecompositionPenalty *= learnedPolicyDecayFactor
	policy.RecentPlanQualityPenalty *= learnedPolicyDecayFactor
	policy.RecentArchitecturePenalty *= learnedPolicyDecayFactor
	policy.RecentValidationPenalty *= learnedPolicyDecayFactor
	policy.RecentRecoveryPenalty *= learnedPolicyDecayFactor
	policy.RecentTradeoffPenalty *= learnedPolicyDecayFactor
	policy.RecentQualityGatePenalty *= learnedPolicyDecayFactor
	return policy
}

func deriveLearnedQualityScore(verdict singularityLearningVerdict) int {
	if verdict.Decision != singularityLearningVerdictAccepted {
		return 0
	}
	score := 0
	score += verdict.Vector.Context
	score += verdict.Vector.Planning
	score += verdict.Vector.Decomposition
	score += verdict.Vector.PlanQuality
	score += verdict.Vector.Architecture
	score += verdict.Vector.Validation
	score += verdict.Vector.Tradeoff
	score += verdict.Vector.Recovery
	return score / 8
}

func requiresChallengerPromotion(policy learnedRoutePolicy) bool {
	switch strings.TrimSpace(policy.GoalType) {
	case "design", "research", "review", "migration":
		return true
	case "implementation":
		return policy.Breadth == "broad"
	default:
		return false
	}
}

func recordChampionChallengerOutcome(policy learnedRoutePolicy, qualityScore int, accepted bool) learnedRoutePolicy {
	if !requiresChallengerPromotion(policy) {
		return policy
	}
	if !accepted || qualityScore <= 0 {
		policy.ChallengerLosses++
		return policy
	}
	if policy.ChampionQualityScore <= 0 {
		policy.ChampionQualityScore = qualityScore
		policy.ChallengerWins++
		return policy
	}
	if qualityScore > policy.ChampionQualityScore {
		policy.ChampionQualityScore = qualityScore
		policy.ChallengerWins++
		return policy
	}
	if qualityScore >= policy.ChampionQualityScore-5 {
		policy.ChallengerWins++
		return policy
	}
	policy.ChallengerLosses++
	return policy
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func hasStructuredDiscovery(trace *completedTurnTrace) bool {
	return containsAnyTool(uniqueLearnedToolNames(trace.OrderedTools), tools.ToolSearchToolName, tools.RGFilesToolName, tools.RGToolName, tools.GlobToolName, tools.GrepToolName)
}

func uniqueLearnedToolNames(items []string) []string {
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

func mergeLearnedDiscoveryPreference(observed []string, defaults []string, limit int) []string {
	seen := make(map[string]struct{}, len(observed)+len(defaults))
	out := make([]string, 0, len(observed)+len(defaults))
	for _, name := range observed {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, name := range defaults {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func compactLearnedStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
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

func compactLearnedText(text string, max int) string {
	text = strings.TrimSpace(text)
	if text == "" || max <= 0 || len(text) <= max {
		return text
	}
	if max <= 1 {
		return text[:max]
	}
	return strings.TrimSpace(text[:max-1]) + "…"
}

func containsTool(items []string, name string) bool {
	for _, item := range items {
		if item == name {
			return true
		}
	}
	return false
}

func containsAnyTool(items []string, names ...string) bool {
	for _, name := range names {
		if containsTool(items, name) {
			return true
		}
	}
	return false
}

func (m *singularityManager) materializeLearnedSkill(policy *learnedRoutePolicy) error {
	if m == nil || policy == nil || !isInjectableLearnedPolicy(*policy) {
		return nil
	}
	skillName := learnedSkillNameForFamily(policy.TaskFamily)
	skillDir := filepath.Join(m.skillRoot, skillName)
	skillPath := filepath.Join(skillDir, skills.SkillFileName)
	if !m.boundary.AllowsAutoWrite(skillPath) || m.boundary.IsKernelProtected(skillPath) {
		return fmt.Errorf("auto-write blocked for %s", skillPath)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	content := renderLearnedSkillMarkdown(skillName, *policy)
	if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
		return err
	}
	policy.SkillName = skillName
	policy.SkillFilePath = skillPath
	return nil
}

func renderLearnedRoutePolicyBlock(policy learnedRoutePolicy) string {
	lines := []string{
		fmt.Sprintf("<learned_route_policy task_family=%q confidence=%d state=%q>", policy.TaskFamily, policy.Confidence, normalizeLearnedPromotionState(policy.PromotionState, policy.Confidence, policy.EvidenceCount)),
		"- Treat this as a recurring task family with proven route guidance.",
	}
	if policy.RequireHarness {
		lines = append(lines, "- Run `run_harness` before editing, execution, or delegation.")
	}
	if policy.RequireExplicitPlan {
		lines = append(lines, "- After the first real repository evidence pass, publish `update_plan` before continuing broad analysis, delegation, or execution.")
	}
	if len(policy.PreferredDiscovery) > 0 {
		lines = append(lines, "- Preferred discovery order: "+strings.Join(policy.PreferredDiscovery, " -> "))
	}
	if policy.RequireContextRead {
		lines = append(lines, "- Do not commit to edits, delegation, or conclusions until structured discovery and real code reads establish enough repository evidence.")
	}
	if len(policy.PreferredVerification) > 0 {
		lines = append(lines, "- Preferred verification tools: "+strings.Join(policy.PreferredVerification, " -> "))
	}
	if policy.PreferParallel {
		lines = append(lines, "- If the task is non-trivial, prefer parallel exploration or delegation instead of a single serial pass.")
	}
	if policy.PreferIndexCodebase {
		lines = append(lines, "- For broad repo work, prefer `index_codebase` when the durable graph is cold, dirty, or obviously too narrow.")
	}
	if policy.ForbidBashDiscovery {
		lines = append(lines, "- Do not use bash for repository discovery. Structured discovery tools already beat bash on this task family.")
	}
	if policy.RequirePostWriteVerification {
		lines = append(lines, "- If you write `AGENTS.md` or another repo instruction artifact, read it back before treating the turn as complete.")
	}
	lines = append(lines, "</learned_route_policy>")
	return strings.Join(lines, "\n")
}

func renderLearnedSkillMarkdown(skillName string, policy learnedRoutePolicy) string {
	description := fmt.Sprintf(
		"Auto-generated Sapphire project skill for recurring %s tasks. Use when this task family appears again and you need the proven route instead of improvising.",
		policy.TaskFamily,
	)
	lines := []string{
		"---",
		fmt.Sprintf("name: %s", skillName),
		fmt.Sprintf("description: %q", description),
		"metadata:",
		"  generated_by: singularity_learning",
		fmt.Sprintf("  task_family: %q", policy.TaskFamily),
		fmt.Sprintf("  confidence: %q", fmt.Sprintf("%d", policy.Confidence)),
		"---",
		"",
		fmt.Sprintf("# %s", skillName),
		"",
		"## Trigger",
		fmt.Sprintf("- Use when the task matches `%s`.", policy.TaskFamily),
		"- This skill is procedural only. It must not edit Sapphire's kernel prompt, tool schema, or preflight layers.",
		"",
		"## Route",
	}
	if policy.RequireHarness {
		lines = append(lines, "- Run `run_harness` before protected execution tools.")
	}
	if policy.RequireExplicitPlan {
		lines = append(lines, "- After the first real repository evidence pass, publish `update_plan` before continuing the broad analysis.")
	}
	if len(policy.PreferredDiscovery) > 0 {
		lines = append(lines, "- Discovery order: "+strings.Join(policy.PreferredDiscovery, " -> "))
	}
	if policy.RequireContextRead {
		lines = append(lines, "- Require real repository evidence before edits, delegation, or conclusions: at least one structured discovery pass and one concrete code read.")
	}
	if len(policy.PreferredVerification) > 0 {
		lines = append(lines, "- Verification order: "+strings.Join(policy.PreferredVerification, " -> "))
	}
	if policy.PreferParallel {
		lines = append(lines, "- For non-trivial scope, use parallel exploration or sub-agents instead of one long serial pass.")
	}
	if policy.PreferIndexCodebase {
		lines = append(lines, "- If repo-wide context is needed and the graph is cold or dirty, ask to run `index_codebase`.")
	}
	if policy.ForbidBashDiscovery {
		lines = append(lines, "- Do not use bash for discovery. Reserve bash for true fallback build/test/process work only.")
	}
	if policy.RequirePostWriteVerification {
		lines = append(lines, "- If this task writes `AGENTS.md` or another repo instruction artifact, read the written file back before considering the turn complete.")
	}
	lines = append(lines,
		"",
		"## Recovery",
		"- If repeated failures appear, strengthen `MISTAKES.md`, persist the prevention rule, and add a narrow `improvement_eval` before retrying.",
		"- Learn by writing or strengthening project skills and route policies, not by mutating Sapphire's kernel prompt stack.",
	)
	return strings.Join(lines, "\n")
}

func learnedSkillNameForFamily(taskFamily string) string {
	slug := sanitizeLearnedSlug(taskFamily)
	base := autoLearnedSkillPrefix + "-" + slug
	if len(base) <= skills.MaxNameLength {
		return base
	}
	hash := shortStableHash(taskFamily)
	trim := skills.MaxNameLength - len(autoLearnedSkillPrefix) - len(hash) - 2
	if trim < 8 {
		trim = 8
	}
	slug = slug[:trim]
	return autoLearnedSkillPrefix + "-" + slug + "-" + hash
}

func sanitizeLearnedSlug(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	var b strings.Builder
	lastDash := false
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "general"
	}
	return out
}

func shortStableHash(input string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(input)))
	return hex.EncodeToString(sum[:])[:8]
}

func cloneStringIntMap(in map[string]int) map[string]int {
	if len(in) == 0 {
		return map[string]int{}
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (m *singularityManager) appendTurnAuditLocked(trace *completedTurnTrace, policy learnedRoutePolicy) error {
	if m == nil || trace == nil {
		return nil
	}
	if err := os.MkdirAll(m.policyDir, 0o755); err != nil {
		return err
	}
	record := singularityTurnAuditRecord{
		Timestamp:                 trace.FinishedAt.UTC().Format(time.RFC3339),
		SessionID:                 trace.SessionID,
		WorkingDir:                trace.WorkingDir,
		Mode:                      trace.Mode,
		Provider:                  trace.Provider,
		ReasoningEffort:           trace.ReasoningEffort,
		TaskFamily:                trace.Family.ID,
		GoalType:                  trace.Family.GoalType,
		Breadth:                   trace.Family.Breadth,
		Status:                    trace.Status,
		ClosureMode:               trace.ClosureMode,
		PhaseAtInterrupt:          trace.PhaseAtInterrupt,
		ProviderFallbackUsed:      trace.ProviderFallbackUsed,
		ActivePolicyID:            trace.ActivePolicyID,
		AppliedPolicy:             strings.TrimSpace(trace.ActivePolicyID) == trace.Family.ID,
		PolicyState:               normalizeLearnedPromotionState(policy.PromotionState, policy.Confidence, policy.EvidenceCount),
		PolicyConfidence:          policy.Confidence,
		OrderedTools:              append([]string{}, trace.OrderedTools...),
		ToolCalls:                 cloneStringIntMap(trace.ToolCalls),
		ToolResults:               cloneStringIntMap(trace.ToolResults),
		ToolErrorCodes:            cloneStringIntMap(trace.ToolErrorCodes),
		StructuredEvidenceCount:   countPositiveTraceEvidence(trace.StructuredEvidence),
		ReadEvidenceCount:         countPositiveTraceEvidence(trace.ReadEvidence),
		VerificationEvidenceCount: countPositiveTraceEvidence(trace.VerificationEvidence),
		LoadedSkills:              append([]string{}, trace.LoadedSkills...),
		BashDiscovery:             trace.BashDiscovery,
		BlockedBashDiscovery:      trace.BlockedBash,
		ValidationChecks:          trace.ValidationChecks,
		ResultSummary:             trace.ResultSummary,
	}
	assessment := assessSingularityCognition(trace)
	learningVerdict := evaluateSingularityLearningVerdict(trace, assessment)
	record.ContextDiscipline = assessment.ContextDiscipline
	record.PlanningDiscipline = assessment.PlanningDiscipline
	record.DecompositionDiscipline = assessment.DecompositionDiscipline
	record.PlanQualityDiscipline = assessment.PlanQualityDiscipline
	record.ArchitectureDiscipline = assessment.ArchitectureDiscipline
	record.ValidationDiscipline = assessment.ValidationDiscipline
	record.RecoveryDiscipline = assessment.RecoveryDiscipline
	record.TradeoffDiscipline = assessment.TradeoffDiscipline
	record.ExecutionRisk = assessment.ExecutionRisk
	record.LearningVerdict = learningVerdict.Decision
	record.LearningBlockers = append([]string{}, learningVerdict.Blockers...)
	record.ContextConfidence = learningVerdict.Vector.Context
	record.PlanningConfidence = learningVerdict.Vector.Planning
	record.DecompositionConfidence = learningVerdict.Vector.Decomposition
	record.PlanQualityConfidence = learningVerdict.Vector.PlanQuality
	record.ArchitectureConfidence = learningVerdict.Vector.Architecture
	record.ValidationConfidence = learningVerdict.Vector.Validation
	record.TradeoffConfidence = learningVerdict.Vector.Tradeoff
	record.RecoveryConfidence = learningVerdict.Vector.Recovery
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	fh, err := os.OpenFile(m.auditPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		_ = fh.Close()
	}()
	if _, err := fh.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

func trimSingularityHistory(historyDir string, keep int) error {
	if keep <= 0 {
		return nil
	}
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	type snapshotEntry struct {
		name string
		path string
	}
	var snapshots []snapshotEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "route_policies-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		snapshots = append(snapshots, snapshotEntry{
			name: entry.Name(),
			path: filepath.Join(historyDir, entry.Name()),
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].name < snapshots[j].name
	})
	if len(snapshots) <= keep {
		return nil
	}
	for _, snapshot := range snapshots[:len(snapshots)-keep] {
		if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func extractObservedBashCommand(rawInput string) string {
	rawInput = strings.TrimSpace(rawInput)
	if rawInput == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(rawInput), &payload); err != nil {
		return ""
	}
	command, _ := payload["command"].(string)
	return strings.TrimSpace(command)
}

func looksLikeObservedDiscoveryBash(command string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	if command == "" {
		return false
	}
	prefixes := []string{
		"ls",
		"find ",
		"fd ",
		"rg ",
		"grep ",
		"cat ",
		"bat ",
		"sed -n ",
		"tree",
		"ack ",
		"ag ",
		"wc ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}
