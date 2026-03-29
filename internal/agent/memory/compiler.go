package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	appdb "github.com/duggal1/Sapphire-cli/internal/db"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/google/uuid"
)

const (
	bootPacketVersion        = "memory.boot.v1"
	defaultGraphFileLimit    = 12
	defaultGraphSymbolLimit  = 40
	defaultGraphEdgeLimit    = 64
	defaultRequiredReadLimit = 10
	defaultCompileCacheTTL   = 45 * time.Second
	defaultPruneDelay        = 45 * time.Second
	defaultPruneInterval     = 5 * time.Minute
	defaultPruneTimeout      = 15 * time.Second
)

type Compiler struct {
	conn            *sql.DB
	q               *appdb.Queries
	store           *orchestrationdb.Store
	now             func() time.Time
	compileCacheMu  sync.Mutex
	compileCache    map[string]compiledPacketCacheEntry
	compileCacheTTL time.Duration
	pruneDelay      time.Duration
	pruneInterval   time.Duration
	pruneTimeout    time.Duration
	pruneMu         sync.Mutex
	pruneLastRun    map[string]time.Time
	pruneQueued     map[string]struct{}
}

type CompileRequest struct {
	SessionID           string
	AgentID             string
	WorkingDir          string
	Task                string
	ProjectConstitution string
	LongHorizonContext  string
	HistoricalContext   string
}

type IndexStatus struct {
	RepoRoot      string
	ScopePath     string
	Branch        string
	Available     bool
	IndexEpoch    int64
	LastIndexedAt time.Time
	Dirty         bool
	ChangedFiles  []string
	FileCount     int
}

type WarmRequest struct {
	WorkingDir string
	Force      bool
}

type WarmResult struct {
	Status          IndexStatus
	DiscoveredFiles int
	ChangedFiles    int
	RemovedFiles    int
	IndexedFiles    int
	Elapsed         time.Duration
}

type WarmProgress struct {
	Workspace       string
	Phase           string
	Message         string
	Active          bool
	Finished        bool
	FilesDiscovered int
	FilesProcessed  int
	FilesIndexed    int
	Percent         float64
	StartedAt       time.Time
	UpdatedAt       time.Time
	Error           string
}

type BootPacket struct {
	Version          string             `json:"version"`
	GeneratedAt      string             `json:"generated_at"`
	TaskClass        string             `json:"task_class"`
	RepoSnapshot     BootRepoSnapshot   `json:"repo_snapshot"`
	RuntimeState     BootRuntimeState   `json:"runtime_state"`
	Handoff          BootHandoffState   `json:"handoff"`
	RelevantPolicies []string           `json:"relevant_policies"`
	GraphSlice       BootGraphSlice     `json:"graph_slice"`
	RequiredReads    []BootRequiredRead `json:"required_reads"`
	WorkingSet       BootWorkingSet     `json:"working_set"`
	ArtifactPath     string             `json:"artifact_path,omitempty"`
}

type BootRepoSnapshot struct {
	RepoRoot       string   `json:"repo_root"`
	ScopePath      string   `json:"scope_path"`
	Branch         string   `json:"branch"`
	HeadCommit     string   `json:"head_commit"`
	IndexEpoch     int64    `json:"index_epoch"`
	Dirty          bool     `json:"dirty"`
	ChangedFiles   []string `json:"changed_files"`
	ActiveWorktree string   `json:"active_worktree,omitempty"`
}

type BootRuntimeState struct {
	CurrentTask      string                     `json:"current_task"`
	CurrentPlan      []string                   `json:"current_plan"`
	Blockers         []string                   `json:"blockers"`
	Uncertainties    []string                   `json:"uncertainties"`
	TouchedFiles     []string                   `json:"touched_files"`
	TouchedSymbols   []string                   `json:"touched_symbols"`
	RecentFindings   []string                   `json:"recent_findings,omitempty"`
	ValidationStatus map[string]any             `json:"validation_status"`
	ActiveSubAgents  []BootSubAgentRuntimeState `json:"active_subagents"`
	RecentDecisions  []string                   `json:"recent_decisions"`
}

type BootSubAgentRuntimeState struct {
	AgentID       string `json:"agent_id"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	Branch        string `json:"branch,omitempty"`
	Worktree      string `json:"worktree,omitempty"`
	LastHeartbeat string `json:"last_heartbeat,omitempty"`
}

type BootHandoffState struct {
	LatestCheckpointAge string   `json:"latest_checkpoint_age,omitempty"`
	WhatWasDone         []string `json:"what_was_done"`
	WhatRemains         []string `json:"what_remains"`
	NextActions         []string `json:"next_actions"`
	Decisions           []string `json:"decisions"`
	Unresolved          []string `json:"unresolved"`
}

type BootGraphSlice struct {
	Files   []BootGraphFile   `json:"files"`
	Symbols []BootGraphSymbol `json:"symbols"`
	Edges   []BootGraphEdge   `json:"edges"`
}

type BootGraphFile struct {
	Path       string   `json:"path"`
	Language   string   `json:"language"`
	Role       string   `json:"role"`
	Status     string   `json:"status"`
	Imports    []string `json:"imports,omitempty"`
	Symbols    []string `json:"symbols,omitempty"`
	FactDigest []string `json:"fact_digest,omitempty"`
}

type BootGraphSymbol struct {
	Key       string `json:"key"`
	FilePath  string `json:"file_path"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Signature string `json:"signature,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Status    string `json:"status,omitempty"`
}

type BootGraphEdge struct {
	Type      string `json:"type"`
	From      string `json:"from"`
	To        string `json:"to"`
	Evidence  string `json:"evidence,omitempty"`
	Reference string `json:"reference,omitempty"`
}

type BootRequiredRead struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Reason    string `json:"reason"`
}

type BootWorkingSet struct {
	TaskPrompt          string   `json:"task_prompt"`
	SeedFiles           []string `json:"seed_files"`
	SeedSymbols         []string `json:"seed_symbols"`
	IncludedHistorical  bool     `json:"included_historical_context"`
	IncludedLongHorizon bool     `json:"included_long_horizon"`
}

type compiledGraph struct {
	files       map[string]storedRepoFile
	symbols     map[string]storedRepoSymbol
	symbolsBy   map[string][]storedRepoSymbol
	edges       []storedRepoEdge
	fileSymbols map[string][]storedRepoSymbol
}

type compiledPacketCacheEntry struct {
	packet      BootPacket
	snapshotKey string
	expiresAt   time.Time
}

type storedRepoScope struct {
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

type storedRepoFile struct {
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
}

type storedRepoSymbol struct {
	ID          string
	ScopeID     string
	FileID      string
	FilePath    string
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
}

type storedRepoEdge struct {
	Type        string
	FromFile    string
	FromSymbol  string
	ToFile      string
	ToSymbol    string
	ToSymbolKey string
	Metadata    map[string]any
}

type runtimeSnapshot struct {
	sessionInfo     appdb.Session
	readFiles       []string
	checkpoint      orchestrationdb.SessionCheckpoint
	checkpointOK    bool
	decisions       []orchestrationdb.DecisionRecord
	subAgents       []orchestrationdb.AgentState
	subAgentReports []appdb.MemorySubAgentReport
	findings        []appdb.MemoryFinding
	previousHandoff *persistedHandoff
}

type persistedHandoff struct {
	ID               string
	Objective        string
	Status           string
	Plan             []string
	Blockers         []string
	Uncertainties    []string
	TouchedFiles     []string
	TouchedSymbols   []string
	SubAgents        []string
	Validation       map[string]any
	NextActions      []string
	RepoSnapshotJSON string
	CreatedAt        time.Time
}

func NewCompiler(conn *sql.DB, store *orchestrationdb.Store) *Compiler {
	if conn == nil {
		return nil
	}
	return &Compiler{
		conn:            conn,
		q:               appdb.New(conn),
		store:           store,
		now:             func() time.Time { return time.Now().UTC() },
		compileCache:    map[string]compiledPacketCacheEntry{},
		compileCacheTTL: defaultCompileCacheTTL,
		pruneDelay:      defaultPruneDelay,
		pruneInterval:   defaultPruneInterval,
		pruneTimeout:    defaultPruneTimeout,
		pruneLastRun:    map[string]time.Time{},
		pruneQueued:     map[string]struct{}{},
	}
}

func (c *Compiler) RenderPromptInjection(ctx context.Context, req CompileRequest) string {
	packet, err := c.Compile(ctx, req)
	if err != nil {
		return ""
	}
	return renderBootPacketInjection(packet)
}

func (c *Compiler) RenderCachedPromptInjection(ctx context.Context, req CompileRequest) string {
	packet, ok := c.cachedCompilePacket(ctx, req)
	if !ok {
		return ""
	}
	return renderBootPacketInjection(packet)
}

func renderBootPacketInjection(packet BootPacket) string {
	raw, err := json.Marshal(packet)
	if err != nil {
		return ""
	}
	return "## COMPILED BOOT PACKET\n```json\n" + string(raw) + "\n```"
}

func (c *Compiler) Compile(ctx context.Context, req CompileRequest) (BootPacket, error) {
	if c == nil || c.conn == nil {
		return BootPacket{}, fmt.Errorf("memory compiler is not initialized")
	}
	if packet, ok := c.cachedCompilePacket(ctx, req); ok {
		return packet, nil
	}
	scope, err := c.ensureIndexedScope(ctx, req.WorkingDir)
	if err != nil {
		return BootPacket{}, err
	}
	graph, err := c.loadScopeGraph(ctx, scope.ID)
	if err != nil {
		return BootPacket{}, err
	}
	runtime, err := c.collectRuntimeSnapshot(ctx, req, scope)
	if err != nil {
		return BootPacket{}, err
	}

	taskClass := classifyTask(req.Task)
	seedFiles, seedSymbols := collectSliceSeeds(req.Task, runtime, scope, graph)
	slice, reads := buildGraphSlice(taskClass, seedFiles, seedSymbols, graph)
	policies := compileRelevantPolicies(scope.RepoRoot, req.ProjectConstitution, req.LongHorizonContext, req.HistoricalContext)
	packet := BootPacket{
		Version:     bootPacketVersion,
		GeneratedAt: c.now().Format(time.RFC3339),
		TaskClass:   taskClass,
		RepoSnapshot: BootRepoSnapshot{
			RepoRoot:       scope.RepoRoot,
			ScopePath:      scope.ScopePath,
			Branch:         scope.Branch,
			HeadCommit:     scope.HeadCommit,
			IndexEpoch:     scope.LatestEpoch,
			Dirty:          scope.Dirty,
			ChangedFiles:   limitStrings(scope.ChangedFiles, 16),
			ActiveWorktree: filepath.Base(scope.ScopePath),
		},
		RuntimeState:     buildRuntimeState(req, runtime),
		Handoff:          buildHandoffState(runtime),
		RelevantPolicies: policies,
		GraphSlice:       slice,
		RequiredReads:    reads,
		WorkingSet: BootWorkingSet{
			TaskPrompt:          strings.TrimSpace(req.Task),
			SeedFiles:           limitStrings(seedFiles, 12),
			SeedSymbols:         limitStrings(seedSymbols, 12),
			IncludedHistorical:  strings.TrimSpace(req.HistoricalContext) != "",
			IncludedLongHorizon: strings.TrimSpace(req.LongHorizonContext) != "",
		},
	}

	if artifactPath, err := c.writeBootPacketArtifact(scope.RepoRoot, packet); err == nil {
		packet.ArtifactPath = artifactPath
		_ = c.recordBootPacket(ctx, req, scope.ID, packet)
	}
	c.storeCompiledPacket(req, packet, scope)
	return packet, nil
}

func (c *Compiler) cachedCompilePacket(ctx context.Context, req CompileRequest) (BootPacket, bool) {
	if c == nil || c.compileCacheTTL <= 0 {
		return BootPacket{}, false
	}
	key := c.compileCacheKey(req)
	now := c.now()

	c.compileCacheMu.Lock()
	entry, ok := c.compileCache[key]
	if ok && !entry.expiresAt.After(now) {
		delete(c.compileCache, key)
		ok = false
	}
	c.compileCacheMu.Unlock()
	if !ok {
		return BootPacket{}, false
	}

	snapshot, err := captureRepoSnapshot(ctx, req.WorkingDir)
	if err != nil || !isCacheableCompileSnapshot(snapshot.Branch, snapshot.HeadCommit) || entry.snapshotKey != compileSnapshotKey(snapshot.RepoRoot, snapshot.ScopePath, snapshot.Branch, snapshot.HeadCommit, snapshot.Dirty, snapshot.ChangedFiles) {
		return BootPacket{}, false
	}
	return entry.packet, true
}

func (c *Compiler) storeCompiledPacket(req CompileRequest, packet BootPacket, scope storedRepoScope) {
	if c == nil || c.compileCacheTTL <= 0 || !isCacheableCompileSnapshot(scope.Branch, scope.HeadCommit) {
		return
	}
	key := c.compileCacheKey(req)
	entry := compiledPacketCacheEntry{
		packet:      packet,
		snapshotKey: compileSnapshotKey(scope.RepoRoot, scope.ScopePath, scope.Branch, scope.HeadCommit, scope.Dirty, scope.ChangedFiles),
		expiresAt:   c.now().Add(c.compileCacheTTL),
	}

	c.compileCacheMu.Lock()
	c.compileCache[key] = entry
	c.compileCacheMu.Unlock()
}

func (c *Compiler) compileCacheKey(req CompileRequest) string {
	return hashText(
		strings.TrimSpace(req.SessionID),
		strings.TrimSpace(req.AgentID),
		filepath.Clean(strings.TrimSpace(req.WorkingDir)),
		strings.TrimSpace(req.Task),
		strings.TrimSpace(req.ProjectConstitution),
		strings.TrimSpace(req.LongHorizonContext),
		strings.TrimSpace(req.HistoricalContext),
	)
}

func compileSnapshotKey(repoRoot, scopePath, branch, headCommit string, dirty bool, changedFiles []string) string {
	return hashText(
		strings.TrimSpace(repoRoot),
		strings.TrimSpace(scopePath),
		strings.TrimSpace(branch),
		strings.TrimSpace(headCommit),
		fmt.Sprintf("%t", dirty),
		strings.Join(changedFiles, "\n"),
	)
}

func isCacheableCompileSnapshot(branch, headCommit string) bool {
	return strings.TrimSpace(branch) != "" || strings.TrimSpace(headCommit) != ""
}

func (c *Compiler) IndexStatus(ctx context.Context, workingDir string) (IndexStatus, error) {
	if c == nil || c.conn == nil {
		return IndexStatus{}, fmt.Errorf("memory compiler is not initialized")
	}
	snapshot, err := captureRepoSnapshot(ctx, workingDir)
	if err != nil {
		return IndexStatus{}, err
	}
	status := IndexStatus{
		RepoRoot:     snapshot.RepoRoot,
		ScopePath:    snapshot.ScopePath,
		Branch:       snapshot.Branch,
		Dirty:        snapshot.Dirty,
		ChangedFiles: snapshot.ChangedFiles,
	}
	scope, err := c.loadExistingScope(ctx, snapshot)
	if err != nil {
		if errors.Is(err, errNoScope) {
			return status, nil
		}
		return IndexStatus{}, err
	}
	status.Available = true
	status.IndexEpoch = scope.LatestEpoch
	if scope.LastIndexedAt > 0 {
		status.LastIndexedAt = time.Unix(scope.LastIndexedAt, 0).UTC()
	}
	rows, err := c.q.ListMemoryRepoFilesByScope(ctx, scope.ID)
	if err != nil {
		return IndexStatus{}, err
	}
	status.FileCount = len(rows)
	return status, nil
}

func (c *Compiler) WarmCodebase(ctx context.Context, req WarmRequest, report func(WarmProgress)) (WarmResult, error) {
	if c == nil || c.conn == nil {
		return WarmResult{}, fmt.Errorf("memory compiler is not initialized")
	}
	startedAt := c.now()
	scope, stats, err := c.ensureIndexedScopeWithOptions(ctx, req.WorkingDir, indexOperationOptions{
		Force:  req.Force,
		Report: report,
	})
	if err != nil {
		if report != nil {
			phase := "failed"
			message := "Codebase graph indexing failed"
			errText := err.Error()
			percent := 1.0
			if errors.Is(err, context.Canceled) {
				phase = "canceled"
				message = "Codebase graph indexing stopped"
				errText = ""
				percent = 0
			}
			report(WarmProgress{
				Workspace: strings.TrimSpace(req.WorkingDir),
				Phase:     phase,
				Message:   message,
				Active:    false,
				Finished:  true,
				Error:     errText,
				Percent:   percent,
				StartedAt: startedAt,
				UpdatedAt: c.now(),
			})
		}
		return WarmResult{}, err
	}

	status, statusErr := c.IndexStatus(ctx, req.WorkingDir)
	if statusErr != nil {
		status = IndexStatus{
			RepoRoot:      scope.RepoRoot,
			ScopePath:     scope.ScopePath,
			Branch:        scope.Branch,
			Available:     true,
			IndexEpoch:    scope.LatestEpoch,
			LastIndexedAt: time.Unix(scope.LastIndexedAt, 0).UTC(),
			Dirty:         scope.Dirty,
			ChangedFiles:  scope.ChangedFiles,
			FileCount:     stats.DiscoveredFiles,
		}
	}

	message := "Durable codebase graph is ready"
	if stats.ChangedFiles == 0 && stats.RemovedFiles == 0 && stats.IndexedFiles == 0 {
		message = "Durable codebase graph is already up to date"
	}
	workspace := strings.TrimSpace(status.ScopePath)
	if workspace == "" {
		workspace = strings.TrimSpace(scope.ScopePath)
	}
	if workspace == "" {
		workspace = strings.TrimSpace(req.WorkingDir)
	}
	if report != nil {
		report(WarmProgress{
			Workspace:       workspace,
			Phase:           "ready",
			Message:         message,
			Active:          false,
			Finished:        true,
			FilesDiscovered: stats.DiscoveredFiles,
			FilesProcessed:  stats.DiscoveredFiles,
			FilesIndexed:    stats.IndexedFiles,
			Percent:         1,
			StartedAt:       startedAt,
			UpdatedAt:       c.now(),
		})
	}

	return WarmResult{
		Status:          status,
		DiscoveredFiles: stats.DiscoveredFiles,
		ChangedFiles:    stats.ChangedFiles,
		RemovedFiles:    stats.RemovedFiles,
		IndexedFiles:    stats.IndexedFiles,
		Elapsed:         c.now().Sub(startedAt),
	}, nil
}

func (c *Compiler) PersistHandoff(ctx context.Context, req CompileRequest) error {
	if c == nil || c.conn == nil || strings.TrimSpace(req.SessionID) == "" {
		return nil
	}
	packet, err := c.Compile(ctx, req)
	if err != nil {
		return err
	}
	_, _, err = c.persistHandoffPacket(ctx, req, packet)
	return err
}

func (c *Compiler) collectRuntimeSnapshot(ctx context.Context, req CompileRequest, scope storedRepoScope) (runtimeSnapshot, error) {
	var out runtimeSnapshot
	if strings.TrimSpace(req.SessionID) == "" {
		return out, nil
	}
	sessionInfo, err := c.q.GetSessionByID(ctx, req.SessionID)
	if err == nil {
		out.sessionInfo = sessionInfo
	}
	readFiles, err := c.q.ListSessionReadFiles(ctx, req.SessionID)
	if err == nil {
		out.readFiles = make([]string, 0, len(readFiles))
		for _, item := range readFiles {
			out.readFiles = append(out.readFiles, filepath.ToSlash(item.Path))
		}
	}
	if latest, err := c.latestCheckpoint(ctx, req.SessionID, req.AgentID); err == nil {
		out.checkpoint = latest
		out.checkpointOK = true
	}
	if c.store != nil {
		if decisions, err := c.store.ListDecisionRecords(ctx, req.SessionID, 24); err == nil {
			out.decisions = decisions
		}
		if states, err := c.store.ListAgentStatesByParent(ctx, mainAgentMailboxID(req.SessionID), 12); err == nil {
			out.subAgents = states
		}
	}
	if reports, err := c.q.ListMemorySubAgentReportsBySession(ctx, req.SessionID, 16); err == nil {
		out.subAgentReports = reports
	}
	if findings, err := c.q.ListMemoryFindingsBySession(ctx, req.SessionID, 24); err == nil {
		out.findings = findings
	}
	if handoff, err := c.latestHandoff(ctx, req.SessionID); err == nil {
		out.previousHandoff = handoff
	}
	return out, nil
}

func (c *Compiler) latestCheckpoint(ctx context.Context, sessionID, agentID string) (orchestrationdb.SessionCheckpoint, error) {
	if c == nil || c.store == nil || strings.TrimSpace(sessionID) == "" {
		return orchestrationdb.SessionCheckpoint{}, sql.ErrNoRows
	}
	if strings.TrimSpace(agentID) == "" {
		agentID = mainAgentMailboxID(sessionID)
	}
	return c.store.LatestCheckpoint(ctx, sessionID, agentID)
}

func (c *Compiler) latestHandoff(ctx context.Context, sessionID string) (*persistedHandoff, error) {
	item, err := c.q.GetLatestMemoryHandoffBySession(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	repoSnapshotJSON, _ := json.Marshal(item.RepoSnapshot)
	return &persistedHandoff{
		ID:               item.ID,
		Objective:        item.Objective,
		Status:           item.Status,
		Plan:             item.Plan,
		Blockers:         item.Blockers,
		Uncertainties:    item.Uncertainties,
		TouchedFiles:     item.TouchedFiles,
		TouchedSymbols:   item.TouchedSymbols,
		SubAgents:        item.SubAgents,
		Validation:       item.Validation,
		NextActions:      item.NextActions,
		RepoSnapshotJSON: string(repoSnapshotJSON),
		CreatedAt:        time.Unix(item.CreatedAt, 0).UTC(),
	}, nil
}

func buildRuntimeState(req CompileRequest, runtime runtimeSnapshot) BootRuntimeState {
	todos := parseSessionTodos(runtime.sessionInfo)
	plan := incompleteTodoTexts(todos)
	if len(plan) == 0 && runtime.previousHandoff != nil {
		plan = limitStrings(runtime.previousHandoff.Plan, 8)
	}
	blockers := collectRuntimeBlockers(runtime)
	uncertainties := []string{}
	if runtime.previousHandoff != nil {
		uncertainties = limitStrings(runtime.previousHandoff.Uncertainties, 8)
	}
	touchedFiles := uniqueSortedStrings(append(limitStrings(runtime.readFiles, 12), filesFromCheckpoint(runtime.checkpoint)...))
	for _, report := range runtime.subAgentReports {
		touchedFiles = append(touchedFiles, report.Files...)
	}
	if runtime.previousHandoff != nil {
		touchedFiles = uniqueSortedStrings(append(touchedFiles, runtime.previousHandoff.TouchedFiles...))
	}
	touchedSymbols := make([]string, 0, 12)
	for _, report := range runtime.subAgentReports {
		touchedSymbols = append(touchedSymbols, report.TouchedSymbols...)
	}
	if runtime.previousHandoff != nil {
		touchedSymbols = append(touchedSymbols, runtime.previousHandoff.TouchedSymbols...)
	}
	recentDecisions := make([]string, 0, len(runtime.decisions))
	for _, item := range limitDecisionRecords(runtime.decisions, 8) {
		recentDecisions = append(recentDecisions, strings.TrimSpace(item.Category+"."+item.Key+"="+item.Value))
	}
	for _, item := range runtime.findings {
		if item.Kind == "decision" {
			recentDecisions = append(recentDecisions, compactText(item.Content, 160))
		}
	}
	if len(recentDecisions) == 0 && runtime.previousHandoff != nil {
		recentDecisions = limitStrings(runtime.previousHandoff.NextActions, 4)
	}
	recentFindings := make([]string, 0, len(runtime.findings))
	for _, item := range runtime.findings {
		if item.Kind == "finding" || item.Kind == "uncertainty" {
			recentFindings = append(recentFindings, compactText(item.Content, 180))
		}
	}
	currentTask := strings.TrimSpace(req.Task)
	if currentTask == "" {
		currentTask = strings.TrimSpace(runtime.sessionInfo.Title)
	}
	if currentTask == "" && runtime.previousHandoff != nil {
		currentTask = strings.TrimSpace(runtime.previousHandoff.Objective)
	}
	return BootRuntimeState{
		CurrentTask:      currentTask,
		CurrentPlan:      plan,
		Blockers:         blockers,
		Uncertainties:    uncertainties,
		TouchedFiles:     limitStrings(touchedFiles, 16),
		TouchedSymbols:   limitStrings(uniqueSortedStrings(touchedSymbols), 12),
		RecentFindings:   limitStrings(uniqueSortedStrings(recentFindings), 8),
		ValidationStatus: buildValidationStatus(runtime),
		ActiveSubAgents:  renderSubAgentRuntime(runtime.subAgents),
		RecentDecisions:  limitStrings(uniqueSortedStrings(recentDecisions), 8),
	}
}

func buildHandoffState(runtime runtimeSnapshot) BootHandoffState {
	state := BootHandoffState{}
	if runtime.checkpointOK {
		state.LatestCheckpointAge = time.Since(runtime.checkpoint.CreatedAt).Truncate(time.Second).String()
		state.WhatWasDone = compactCheckpointFacts(runtime.checkpoint)
		state.WhatRemains = filesFromCheckpoint(runtime.checkpoint)
	}
	if runtime.previousHandoff != nil {
		if len(state.WhatRemains) == 0 {
			state.WhatRemains = limitStrings(runtime.previousHandoff.Plan, 8)
		}
		state.NextActions = limitStrings(runtime.previousHandoff.NextActions, 8)
		state.Unresolved = limitStrings(runtime.previousHandoff.Uncertainties, 8)
	}
	for _, report := range runtime.subAgentReports {
		if next := strings.TrimSpace(report.NextAction); next != "" {
			state.NextActions = append(state.NextActions, next)
		}
		if blocker := strings.TrimSpace(report.Blockers); blocker != "" {
			state.Unresolved = append(state.Unresolved, blocker)
		}
	}
	if len(state.NextActions) == 0 {
		state.NextActions = limitStrings(state.WhatRemains, 6)
	}
	for _, item := range limitDecisionRecords(runtime.decisions, 8) {
		state.Decisions = append(state.Decisions, strings.TrimSpace(item.Category+"."+item.Key+"="+item.Value))
	}
	state.NextActions = limitStrings(uniqueSortedStrings(state.NextActions), 8)
	state.Unresolved = limitStrings(uniqueSortedStrings(state.Unresolved), 8)
	return state
}

func collectSliceSeeds(task string, runtime runtimeSnapshot, scope storedRepoScope, graph compiledGraph) ([]string, []string) {
	fileSeeds := make([]string, 0, 16)
	symbolSeeds := make([]string, 0, 16)
	fileSeeds = append(fileSeeds, explicitFileMentions(task)...)
	fileSeeds = append(fileSeeds, limitStrings(scope.ChangedFiles, 8)...)
	fileSeeds = append(fileSeeds, limitStrings(runtime.readFiles, 8)...)
	if runtime.previousHandoff != nil {
		fileSeeds = append(fileSeeds, runtime.previousHandoff.TouchedFiles...)
		symbolSeeds = append(symbolSeeds, runtime.previousHandoff.TouchedSymbols...)
	}
	for _, report := range runtime.subAgentReports {
		fileSeeds = append(fileSeeds, report.Files...)
		symbolSeeds = append(symbolSeeds, report.TouchedSymbols...)
	}
	for _, sym := range explicitSymbolMentions(task) {
		if _, ok := graph.symbolsBy[strings.ToLower(sym)]; ok {
			symbolSeeds = append(symbolSeeds, sym)
		}
	}
	if len(fileSeeds) == 0 {
		for path := range graph.files {
			if strings.HasSuffix(path, "agent.go") || strings.HasSuffix(path, "coordinator.go") {
				fileSeeds = append(fileSeeds, path)
			}
		}
	}
	return uniqueSortedStrings(fileSeeds), uniqueSortedStrings(symbolSeeds)
}

func buildGraphSlice(taskClass string, seedFiles, seedSymbols []string, graph compiledGraph) (BootGraphSlice, []BootRequiredRead) {
	selectedFiles := make(map[string]struct{})
	selectedSymbols := make(map[string]struct{})

	for _, seed := range seedFiles {
		for path, file := range graph.files {
			if path == seed || strings.HasSuffix(path, seed) || filepath.Base(path) == filepath.Base(seed) {
				selectedFiles[file.Path] = struct{}{}
			}
		}
	}
	for _, seed := range seedSymbols {
		for _, symbol := range graph.symbolsBy[strings.ToLower(seed)] {
			selectedSymbols[symbol.StableKey] = struct{}{}
			selectedFiles[symbol.FilePath] = struct{}{}
		}
	}
	for filePath := range selectedFiles {
		for _, symbol := range limitStoredSymbols(graph.fileSymbols[filePath], 8) {
			selectedSymbols[symbol.StableKey] = struct{}{}
		}
	}

	allowedEdge := func(edgeType string) bool {
		switch taskClass {
		case "debug":
			return edgeType == "calls" || edgeType == "imports" || edgeType == "test_covers"
		case "performance":
			return edgeType == "calls" || edgeType == "imports" || edgeType == "config_controls"
		case "architecture":
			return edgeType == "imports" || edgeType == "implements" || edgeType == "test_covers"
		default:
			return edgeType == "calls" || edgeType == "imports" || edgeType == "test_covers" || edgeType == "implements"
		}
	}

	for _, edge := range graph.edges {
		if !allowedEdge(edge.Type) {
			continue
		}
		_, fromFileSelected := selectedFiles[edge.FromFile]
		_, fromSymSelected := selectedSymbols[edge.FromSymbol]
		if !fromFileSelected && !fromSymSelected {
			continue
		}
		if edge.ToFile != "" {
			selectedFiles[edge.ToFile] = struct{}{}
		}
		if edge.ToSymbolKey != "" {
			selectedSymbols[edge.ToSymbolKey] = struct{}{}
		}
	}

	fileList := make([]BootGraphFile, 0, len(selectedFiles))
	readList := make([]BootRequiredRead, 0, len(selectedFiles))
	for _, path := range sortedMapKeys(selectedFiles) {
		file := graph.files[path]
		symbolNames := make([]string, 0, len(graph.fileSymbols[path]))
		for _, symbol := range limitStoredSymbols(graph.fileSymbols[path], 6) {
			symbolNames = append(symbolNames, symbol.Name)
		}
		fileList = append(fileList, BootGraphFile{
			Path:       file.Path,
			Language:   file.Language,
			Role:       file.Role,
			Status:     file.Status,
			Imports:    limitStrings(file.Imports, 8),
			Symbols:    symbolNames,
			FactDigest: renderFactDigest(file.Facts),
		})
		if len(readList) < defaultRequiredReadLimit {
			readList = append(readList, BootRequiredRead{
				Path:   file.Path,
				Reason: reasonForFile(taskClass, file),
			})
		}
		if len(fileList) >= defaultGraphFileLimit {
			break
		}
	}

	symbolList := make([]BootGraphSymbol, 0, len(selectedSymbols))
	for _, key := range sortedMapKeys(selectedSymbols) {
		symbol := graph.symbols[key]
		if symbol.StableKey == "" {
			continue
		}
		symbolList = append(symbolList, BootGraphSymbol{
			Key:       symbol.StableKey,
			FilePath:  symbol.FilePath,
			Name:      symbol.Name,
			Kind:      symbol.Kind,
			Signature: symbol.Signature,
			StartLine: symbol.StartLine,
			EndLine:   symbol.EndLine,
			Status:    symbol.Status,
		})
		if len(symbolList) >= defaultGraphSymbolLimit {
			break
		}
	}

	for _, symbol := range symbolList {
		if len(readList) >= defaultRequiredReadLimit {
			break
		}
		readList = append(readList, BootRequiredRead{
			Path:      symbol.FilePath,
			StartLine: symbol.StartLine,
			EndLine:   symbol.EndLine,
			Reason:    "symbol neighborhood",
		})
	}

	edgeList := make([]BootGraphEdge, 0, len(graph.edges))
	for _, edge := range graph.edges {
		_, fromFileSelected := selectedFiles[edge.FromFile]
		_, fromSymSelected := selectedSymbols[edge.FromSymbol]
		if !fromFileSelected && !fromSymSelected {
			continue
		}
		ref := firstNonEmptyText(edge.ToSymbolKey, edge.ToSymbol, edge.ToFile)
		if ref == "" {
			continue
		}
		edgeList = append(edgeList, BootGraphEdge{
			Type:      edge.Type,
			From:      firstNonEmptyText(edge.FromSymbol, edge.FromFile),
			To:        ref,
			Evidence:  stringifyMetadataField(edge.Metadata, "evidence"),
			Reference: stringifyMetadataField(edge.Metadata, "import"),
		})
		if len(edgeList) >= defaultGraphEdgeLimit {
			break
		}
	}

	return BootGraphSlice{
		Files:   fileList,
		Symbols: symbolList,
		Edges:   edgeList,
	}, dedupeRequiredReads(readList)
}

func compileRelevantPolicies(repoRoot, constitution, longHorizonContext, historicalContext string) []string {
	policies := make([]string, 0, 16)
	if text := strings.TrimSpace(constitution); text != "" {
		policies = append(policies, splitPolicyLines(text, 8)...)
	}
	if text := strings.TrimSpace(longHorizonContext); text != "" {
		policies = append(policies, splitPolicyLines(text, 6)...)
	}
	if text := strings.TrimSpace(historicalContext); text != "" {
		policies = append(policies, splitPolicyLines(text, 4)...)
	}
	agentsPath := filepath.Join(repoRoot, "AGENTS.md")
	if data, err := os.ReadFile(agentsPath); err == nil {
		policies = append(policies, splitPolicyLines(string(data), 8)...)
	}
	return limitStrings(uniqueSortedStrings(policies), 16)
}

func splitPolicyLines(raw string, limit int) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, limit)
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "<") {
			continue
		}
		out = append(out, compactText(line, 180))
		if len(out) >= limit {
			break
		}
	}
	return out
}

func parseSessionTodos(item appdb.Session) []session.Todo {
	if !item.Todos.Valid || strings.TrimSpace(item.Todos.String) == "" {
		return nil
	}
	var todos []session.Todo
	if err := json.Unmarshal([]byte(item.Todos.String), &todos); err != nil {
		return nil
	}
	return todos
}

func incompleteTodoTexts(todos []session.Todo) []string {
	out := make([]string, 0, len(todos))
	for _, todo := range todos {
		if !session.IsRenderableTodo(todo) || session.IsTodoTerminalStatus(todo.Status) {
			continue
		}
		text := strings.TrimSpace(todo.Content)
		if text == "" {
			text = strings.TrimSpace(todo.ActiveForm)
		}
		if text != "" {
			out = append(out, text)
		}
	}
	return limitStrings(out, 8)
}

func collectRuntimeBlockers(runtime runtimeSnapshot) []string {
	blockers := make([]string, 0, 8)
	if runtime.previousHandoff != nil {
		blockers = append(blockers, runtime.previousHandoff.Blockers...)
	}
	for _, report := range runtime.subAgentReports {
		status := strings.ToLower(strings.TrimSpace(report.Status))
		if status == "blocked" || status == "timed_out" || status == "error" {
			blockers = append(blockers, firstNonEmptyText(report.Blockers, report.Summary, report.Progress))
		}
	}
	for _, agent := range runtime.subAgents {
		status := strings.ToLower(strings.TrimSpace(agent.Status))
		if status == "blocked" || status == "timed_out" || status == "failed" {
			blockers = append(blockers, strings.TrimSpace(agent.Role)+" "+status)
		}
	}
	return limitStrings(uniqueSortedStrings(blockers), 8)
}

func buildValidationStatus(runtime runtimeSnapshot) map[string]any {
	status := map[string]any{}
	if runtime.previousHandoff != nil && len(runtime.previousHandoff.Validation) > 0 {
		for key, value := range runtime.previousHandoff.Validation {
			status[key] = value
		}
	}
	if runtime.checkpointOK {
		var summary map[string]any
		if err := json.Unmarshal([]byte(runtime.checkpoint.SummaryJSON), &summary); err == nil {
			if v, ok := summary["status"]; ok {
				status["status"] = v
			}
			if v, ok := summary["result"]; ok {
				status["last_result"] = compactText(fmt.Sprint(v), 180)
			}
		}
	}
	if _, ok := status["status"]; !ok {
		status["status"] = "unknown"
	}
	return status
}

func renderSubAgentRuntime(items []orchestrationdb.AgentState) []BootSubAgentRuntimeState {
	out := make([]BootSubAgentRuntimeState, 0, len(items))
	for _, item := range items {
		out = append(out, BootSubAgentRuntimeState{
			AgentID:       item.AgentID,
			Role:          item.Role,
			Status:        item.Status,
			Branch:        item.Branch,
			Worktree:      filepath.Base(strings.TrimSpace(item.WorktreePath)),
			LastHeartbeat: item.LastHeartbeat.Format(time.RFC3339),
		})
	}
	return out
}

func filesFromCheckpoint(checkpoint orchestrationdb.SessionCheckpoint) []string {
	if strings.TrimSpace(checkpoint.FilesModifiedJSON) == "" {
		return nil
	}
	var files []string
	_ = json.Unmarshal([]byte(checkpoint.FilesModifiedJSON), &files)
	return limitStrings(files, 10)
}

func compactCheckpointFacts(checkpoint orchestrationdb.SessionCheckpoint) []string {
	if strings.TrimSpace(checkpoint.SummaryJSON) == "" {
		return nil
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(checkpoint.SummaryJSON), &summary); err != nil {
		return nil
	}
	facts := make([]string, 0, 4)
	for _, key := range []string{"phase", "status", "result", "summary"} {
		if value, ok := summary[key]; ok {
			facts = append(facts, key+": "+compactText(fmt.Sprint(value), 140))
		}
	}
	return facts
}

func reasonForFile(taskClass string, file storedRepoFile) string {
	switch taskClass {
	case "debug":
		return "debug path"
	case "performance":
		return "runtime path"
	case "architecture":
		return "architecture slice"
	default:
		if file.Role == "test" {
			return "test coverage"
		}
		return "task-local codebase slice"
	}
}

func renderFactDigest(facts map[string]any) []string {
	if len(facts) == 0 {
		return nil
	}
	keys := make([]string, 0, len(facts))
	for key := range facts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range limitStrings(keys, 4) {
		out = append(out, key+": "+compactText(fmt.Sprint(facts[key]), 72))
	}
	return out
}

func dedupeRequiredReads(reads []BootRequiredRead) []BootRequiredRead {
	seen := make(map[string]struct{}, len(reads))
	out := make([]BootRequiredRead, 0, len(reads))
	for _, item := range reads {
		key := fmt.Sprintf("%s:%d:%d", item.Path, item.StartLine, item.EndLine)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func limitDecisionRecords(items []orchestrationdb.DecisionRecord, limit int) []orchestrationdb.DecisionRecord {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func firstNonEmpty(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		default:
			return value
		}
	}
	return ""
}

func firstNonEmptyString(primary string, secondary []string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	for _, item := range secondary {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringifyMetadataField(meta map[string]any, key string) string {
	if len(meta) == 0 {
		return ""
	}
	if value, ok := meta[key]; ok {
		return compactText(fmt.Sprint(value), 120)
	}
	return ""
}

func limitStrings(items []string, limit int) []string {
	if len(items) == 0 {
		return nil
	}
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func uniqueSortedStrings(items []string) []string {
	set := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = filepath.ToSlash(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if _, ok := set[item]; ok {
			continue
		}
		set[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func sortedMapKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func hashText(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "::")))
	return hex.EncodeToString(sum[:])
}

func mainAgentMailboxID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	return "main:" + sessionID
}

func (c *Compiler) writeBootPacketArtifact(repoRoot string, packet BootPacket) (string, error) {
	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return "", err
	}
	return c.writeArtifact(repoRoot, "boot_packets", data)
}

func (c *Compiler) writeHandoffArtifact(repoRoot string, packet BootPacket) (string, error) {
	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return "", err
	}
	return c.writeArtifact(repoRoot, "handoffs", data)
}

func (c *Compiler) writeArtifact(repoRoot, kind string, data []byte) (string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return "", fmt.Errorf("repo root is required")
	}
	dir := filepath.Join(repoRoot, ".sapphire", "state", "memory", kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := c.now().Format("20060102T150405") + "-" + uuid.NewString() + ".json"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(path), nil
}

func (c *Compiler) recordBootPacket(ctx context.Context, req CompileRequest, scopeID string, packet BootPacket) error {
	readsJSON, _ := json.Marshal(packet.RequiredReads)
	bootPacketID := uuid.NewString()
	if err := c.q.InsertMemoryBootPacket(ctx, appdb.InsertMemoryBootPacketParams{
		ID:            bootPacketID,
		SessionID:     strings.TrimSpace(req.SessionID),
		AgentID:       strings.TrimSpace(req.AgentID),
		RepoScopeID:   scopeID,
		TaskHash:      hashText(strings.TrimSpace(req.Task), strings.TrimSpace(req.WorkingDir)),
		ArtifactPath:  packet.ArtifactPath,
		RequiredReads: readsJSON,
		CreatedAt:     c.now().Unix(),
	}); err != nil {
		return err
	}
	provenanceID, err := c.createProvenance(ctx, appdb.InsertMemoryProvenanceParams{
		ID:           uuid.NewString(),
		RepoScopeID:  scopeID,
		SessionID:    strings.TrimSpace(req.SessionID),
		AgentID:      strings.TrimSpace(req.AgentID),
		SourceKind:   "boot_packet",
		ArtifactPath: packet.ArtifactPath,
		HeadCommit:   packet.RepoSnapshot.HeadCommit,
		IndexEpoch:   packet.RepoSnapshot.IndexEpoch,
		Metadata: map[string]any{
			"task_hash":   hashText(strings.TrimSpace(req.Task), strings.TrimSpace(req.WorkingDir)),
			"task_class":  packet.TaskClass,
			"working_dir": req.WorkingDir,
		},
		CreatedAt: c.now().Unix(),
	})
	if err == nil {
		_ = c.linkFactProvenance(ctx, "boot_packet", bootPacketID, provenanceID)
	}
	c.scheduleDurableMemoryPrune(strings.TrimSpace(req.SessionID), scopeID)
	return nil
}

func (c *Compiler) scheduleDurableMemoryPrune(sessionID, scopeID string) {
	if c == nil || c.q == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	scopeID = strings.TrimSpace(scopeID)
	if sessionID == "" && scopeID == "" {
		return
	}

	key := sessionID + "|" + scopeID
	delay := c.pruneDelay
	now := c.now()

	c.pruneMu.Lock()
	if _, queued := c.pruneQueued[key]; queued {
		c.pruneMu.Unlock()
		return
	}
	if last := c.pruneLastRun[key]; !last.IsZero() {
		if since := now.Sub(last); since < c.pruneInterval {
			remaining := c.pruneInterval - since
			if remaining > delay {
				delay = remaining
			}
		}
	}
	c.pruneQueued[key] = struct{}{}
	c.pruneMu.Unlock()

	go func() {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			<-timer.C
		}

		ctx, cancel := context.WithTimeout(context.Background(), c.pruneTimeout)
		defer cancel()
		_ = c.pruneDurableMemory(ctx, sessionID, scopeID)

		c.pruneMu.Lock()
		c.pruneLastRun[key] = c.now()
		delete(c.pruneQueued, key)
		c.pruneMu.Unlock()
	}()
}

func (c *Compiler) lookupScopeID(ctx context.Context, repoRoot, scopePath, branch string) string {
	item, err := c.q.GetMemoryRepoScope(ctx, repoRoot, scopePath, branch)
	if err != nil {
		return ""
	}
	return item.ID
}

func parseJSONMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func parseJSONStringSlice(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func explicitFileMentions(task string) []string {
	fields := strings.FieldsFunc(task, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '"' || r == '\'' || r == '(' || r == ')'
	})
	files := make([]string, 0, 12)
	for _, field := range fields {
		field = strings.TrimSpace(strings.TrimSuffix(field, ":"))
		if strings.Count(field, "/") == 0 && !strings.Contains(field, ".") {
			continue
		}
		if strings.HasPrefix(field, "/") {
			field = filepath.ToSlash(field)
		}
		if strings.Contains(field, ".go") || strings.Contains(field, ".md") || strings.Contains(field, ".sql") || strings.Contains(field, ".json") || strings.Contains(field, ".yaml") || strings.Contains(field, ".yml") || strings.Contains(field, ".toml") || strings.Contains(field, ".ts") || strings.Contains(field, ".tsx") {
			files = append(files, filepath.ToSlash(field))
		}
	}
	return uniqueSortedStrings(files)
}

func explicitSymbolMentions(task string) []string {
	fields := strings.FieldsFunc(task, func(r rune) bool {
		return !(r == '_' || r == '.' || r == '-' || r == '/' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
	})
	symbols := make([]string, 0, 16)
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len(field) < 3 {
			continue
		}
		if strings.Contains(field, "/") || strings.Contains(field, ".go") {
			continue
		}
		if field == strings.ToLower(field) && !strings.Contains(field, "_") {
			continue
		}
		symbols = append(symbols, field)
	}
	return uniqueSortedStrings(symbols)
}

func classifyTask(task string) string {
	task = strings.ToLower(task)
	switch {
	case strings.Contains(task, "performance"), strings.Contains(task, "slow"), strings.Contains(task, "latency"), strings.Contains(task, "cpu"), strings.Contains(task, "memory"):
		return "performance"
	case strings.Contains(task, "debug"), strings.Contains(task, "fix"), strings.Contains(task, "error"), strings.Contains(task, "panic"), strings.Contains(task, "crash"):
		return "debug"
	case strings.Contains(task, "architecture"), strings.Contains(task, "design"), strings.Contains(task, "refactor"), strings.Contains(task, "codebase"), strings.Contains(task, "memory"):
		return "architecture"
	default:
		return "edit"
	}
}

func limitStoredSymbols(items []storedRepoSymbol, limit int) []storedRepoSymbol {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func scanStringSlice(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func scanBool(value int64) bool {
	return value != 0
}

var errNoScope = errors.New("memory repo scope not found")
