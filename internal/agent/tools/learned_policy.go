package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type learnedToolPolicyKey string

const LearnedToolPolicyContextKey learnedToolPolicyKey = "learned_tool_policy"

type toolUsageStateKey string

const ToolUsageStateContextKey toolUsageStateKey = "tool_usage_state"

// LearnedToolPolicy carries low-latency route guardrails compiled from prior
// turns. It intentionally stays narrow: only enforce rules with a clear,
// already-available structured-tool alternative.
type LearnedToolPolicy struct {
	TaskFamily                   string `json:"task_family,omitempty"`
	Reason                       string `json:"reason,omitempty"`
	ForbidBashDiscovery          bool   `json:"forbid_bash_discovery,omitempty"`
	PreferStructuredDiscovery    bool   `json:"prefer_structured_discovery,omitempty"`
	RequireContextRead           bool   `json:"require_context_read,omitempty"`
	RequireExplicitPlan          bool   `json:"require_explicit_plan,omitempty"`
	RequirePostWriteVerification bool   `json:"require_post_write_verification,omitempty"`
}

func GetLearnedToolPolicyFromContext(ctx context.Context) LearnedToolPolicy {
	return getContextValue(ctx, LearnedToolPolicyContextKey, LearnedToolPolicy{})
}

func (p LearnedToolPolicy) HasGuardrails() bool {
	return p.ForbidBashDiscovery || p.PreferStructuredDiscovery || p.RequireContextRead || p.RequireExplicitPlan || p.RequirePostWriteVerification
}

func (p LearnedToolPolicy) GuidanceReason() string {
	reason := strings.TrimSpace(p.Reason)
	if reason != "" {
		return reason
	}
	if taskFamily := strings.TrimSpace(p.TaskFamily); taskFamily != "" {
		return "learned route policy for task family " + taskFamily
	}
	return "learned route policy"
}

type ToolUsageState struct {
	mu                          sync.Mutex
	counts                      map[string]int
	planPublished               bool
	pendingArtifactVerification map[string]struct{}
	structuredEvidence          map[string]struct{}
	readEvidence                map[string]struct{}
	verificationEvidence        map[string]struct{}
	deterministicToolCounts     map[string]int
	deterministicTotalCalls     int
	deterministicReadCounts     map[string]int
	deterministicWriteCounts    map[string]int
	deterministicBlindWrites    map[string]int
	deterministicCreatedFiles   map[string]struct{}
	deterministicModifiedFiles  map[string]struct{}
}

var (
	sharedToolUsageMu        sync.Mutex
	sharedToolUsageBySession = map[string]*ToolUsageState{}
)

func NewToolUsageState() *ToolUsageState {
	return &ToolUsageState{
		counts:                      map[string]int{},
		pendingArtifactVerification: map[string]struct{}{},
		structuredEvidence:          map[string]struct{}{},
		readEvidence:                map[string]struct{}{},
		verificationEvidence:        map[string]struct{}{},
		deterministicToolCounts:     map[string]int{},
		deterministicReadCounts:     map[string]int{},
		deterministicWriteCounts:    map[string]int{},
		deterministicBlindWrites:    map[string]int{},
		deterministicCreatedFiles:   map[string]struct{}{},
		deterministicModifiedFiles:  map[string]struct{}{},
	}
}

func ResetSharedToolUsageState(sessionID string) *ToolUsageState {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return NewToolUsageState()
	}
	sharedToolUsageMu.Lock()
	defer sharedToolUsageMu.Unlock()
	state := NewToolUsageState()
	sharedToolUsageBySession[sessionID] = state
	return state
}

func ClearSharedToolUsageState(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	sharedToolUsageMu.Lock()
	defer sharedToolUsageMu.Unlock()
	delete(sharedToolUsageBySession, sessionID)
}

func SharedToolUsageState(sessionID string) *ToolUsageState {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	sharedToolUsageMu.Lock()
	defer sharedToolUsageMu.Unlock()
	if state, ok := sharedToolUsageBySession[sessionID]; ok && state != nil {
		return state
	}
	state := NewToolUsageState()
	sharedToolUsageBySession[sessionID] = state
	return state
}

func GetToolUsageStateFromContext(ctx context.Context) *ToolUsageState {
	value := ctx.Value(ToolUsageStateContextKey)
	if state, ok := value.(*ToolUsageState); ok && state != nil {
		return state
	}
	if sessionID := GetSessionFromContext(ctx); strings.TrimSpace(sessionID) != "" {
		return SharedToolUsageState(sessionID)
	}
	return nil
}

func (s *ToolUsageState) Increment(toolName string) {
	if s == nil {
		return
	}
	toolName = normalizeToolName(toolName)
	if toolName == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[toolName]++
}

func (s *ToolUsageState) Count(toolName string) int {
	if s == nil {
		return 0
	}
	toolName = normalizeToolName(toolName)
	if toolName == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts[toolName]
}

func (s *ToolUsageState) Total(toolNames ...string) int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	total := 0
	for _, toolName := range toolNames {
		total += s.counts[normalizeToolName(toolName)]
	}
	return total
}

func (s *ToolUsageState) MarkPlanPublished() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.planPublished = true
}

func (s *ToolUsageState) HasPublishedPlan() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planPublished
}

func (s *ToolUsageState) MarkArtifactWrite(path string) {
	if s == nil {
		return
	}
	normalized, ok := normalizeArtifactVerificationPath(path)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingArtifactVerification == nil {
		s.pendingArtifactVerification = map[string]struct{}{}
	}
	s.pendingArtifactVerification[normalized] = struct{}{}
}

func (s *ToolUsageState) MarkArtifactVerified(path string) {
	if s == nil {
		return
	}
	normalized, ok := normalizeArtifactVerificationPath(path)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pendingArtifactVerification, normalized)
}

func (s *ToolUsageState) MarkAllArtifactsVerified() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.pendingArtifactVerification)
}

func (s *ToolUsageState) MarkStructuredEvidence(values ...string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.structuredEvidence == nil {
		s.structuredEvidence = map[string]struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		s.structuredEvidence[value] = struct{}{}
	}
}

func (s *ToolUsageState) MarkReadEvidence(values ...string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readEvidence == nil {
		s.readEvidence = map[string]struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		s.readEvidence[value] = struct{}{}
	}
}

func (s *ToolUsageState) MarkVerificationEvidence(values ...string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.verificationEvidence == nil {
		s.verificationEvidence = map[string]struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		s.verificationEvidence[value] = struct{}{}
	}
}

func (s *ToolUsageState) StructuredEvidenceCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.structuredEvidence)
}

func (s *ToolUsageState) ReadEvidenceCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.readEvidence)
}

func (s *ToolUsageState) VerificationEvidenceCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.verificationEvidence)
}

type DeterministicLoopMetrics struct {
	TotalCalls       int            `json:"total_calls"`
	UniqueToolNames  []string       `json:"unique_tool_names,omitempty"`
	ReadCounts       map[string]int `json:"read_counts,omitempty"`
	WriteCounts      map[string]int `json:"write_counts,omitempty"`
	BlindWriteCounts map[string]int `json:"blind_write_counts,omitempty"`
	CreatedFiles     []string       `json:"created_files,omitempty"`
	ModifiedFiles    []string       `json:"modified_files,omitempty"`
}

func (s *ToolUsageState) RecordDeterministicToolCall(toolName string) {
	if s == nil {
		return
	}
	toolName = normalizeToolName(toolName)
	if toolName == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deterministicToolCounts == nil {
		s.deterministicToolCounts = map[string]int{}
	}
	s.deterministicToolCounts[toolName]++
	s.deterministicTotalCalls++
}

func (s *ToolUsageState) RecordDeterministicRead(path string) {
	if s == nil {
		return
	}
	path, ok := normalizeArtifactVerificationPath(path)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deterministicReadCounts == nil {
		s.deterministicReadCounts = map[string]int{}
	}
	s.deterministicReadCounts[path]++
}

func (s *ToolUsageState) RecordDeterministicWrite(path string, blind bool, created bool) {
	if s == nil {
		return
	}
	path, ok := normalizeArtifactVerificationPath(path)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deterministicWriteCounts == nil {
		s.deterministicWriteCounts = map[string]int{}
	}
	if s.deterministicBlindWrites == nil {
		s.deterministicBlindWrites = map[string]int{}
	}
	if s.deterministicCreatedFiles == nil {
		s.deterministicCreatedFiles = map[string]struct{}{}
	}
	if s.deterministicModifiedFiles == nil {
		s.deterministicModifiedFiles = map[string]struct{}{}
	}
	s.deterministicWriteCounts[path]++
	if blind {
		s.deterministicBlindWrites[path]++
	}
	if created {
		s.deterministicCreatedFiles[path] = struct{}{}
		delete(s.deterministicModifiedFiles, path)
		return
	}
	if _, alreadyCreated := s.deterministicCreatedFiles[path]; alreadyCreated {
		return
	}
	s.deterministicModifiedFiles[path] = struct{}{}
}

func (s *ToolUsageState) ResetDeterministicLoopMetrics() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deterministicToolCounts = map[string]int{}
	s.deterministicTotalCalls = 0
	s.deterministicReadCounts = map[string]int{}
	s.deterministicWriteCounts = map[string]int{}
	s.deterministicBlindWrites = map[string]int{}
	s.deterministicCreatedFiles = map[string]struct{}{}
	s.deterministicModifiedFiles = map[string]struct{}{}
}

func (s *ToolUsageState) SnapshotDeterministicLoopMetrics() DeterministicLoopMetrics {
	if s == nil {
		return DeterministicLoopMetrics{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	metrics := DeterministicLoopMetrics{
		TotalCalls:       s.deterministicTotalCalls,
		UniqueToolNames:  sortedMapKeys(s.deterministicToolCounts),
		ReadCounts:       cloneStringIntMapLocked(s.deterministicReadCounts),
		WriteCounts:      cloneStringIntMapLocked(s.deterministicWriteCounts),
		BlindWriteCounts: cloneStringIntMapLocked(s.deterministicBlindWrites),
		CreatedFiles:     sortedSetKeys(s.deterministicCreatedFiles),
		ModifiedFiles:    sortedSetKeys(s.deterministicModifiedFiles),
	}
	return metrics
}

func HasRequiredContextReadEvidence(state *ToolUsageState) bool {
	if state == nil {
		return false
	}
	hasStructuredDiscovery := state.StructuredEvidenceCount() > 0 || state.Total(ToolSearchToolName, RGFilesToolName, RGToolName, GlobToolName, GrepToolName) > 0
	hasCodeRead := state.ReadEvidenceCount() > 0 || state.Total(AgenticViewToolName, ViewToolName, SingleViewToolName) > 0
	return hasStructuredDiscovery && hasCodeRead
}

func HasPlanningSeedEvidence(state *ToolUsageState) bool {
	return HasRequiredContextReadEvidence(state)
}

func (s *ToolUsageState) PendingArtifactVerificationPaths() []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pendingArtifactVerification) == 0 {
		return nil
	}
	paths := make([]string, 0, len(s.pendingArtifactVerification))
	for path := range s.pendingArtifactVerification {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

type TurnGuardrailError struct {
	Title   string
	Message string
}

func (e *TurnGuardrailError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Message)
}

func ObserveSuccessfulTurnGuardrailResult(ctx context.Context, toolName string, rawInput string, isError bool) {
	if isError {
		return
	}
	state := GetToolUsageStateFromContext(ctx)
	if state == nil {
		return
	}
	canonical := canonicalToolNameForModePolicy(toolName)
	if canonical == "" {
		canonical = normalizeToolName(toolName)
	}
	if canonical == "" {
		return
	}
	if canonical == UpdatePlanToolName {
		state.MarkPlanPublished()
		return
	}
	input, err := parseToolInput(rawInput, canonical)
	if err != nil {
		return
	}
	recordContextEvidence(state, canonical, input)
	for _, path := range extractArtifactWritePathsFromInput(canonical, input) {
		state.MarkArtifactWrite(resolveArtifactVerificationPath(ctx, path))
	}
	for _, path := range extractArtifactVerificationPathsFromInput(canonical, input) {
		state.MarkArtifactVerified(resolveArtifactVerificationPath(ctx, path))
		state.MarkVerificationEvidence(resolveArtifactVerificationPath(ctx, path))
	}
	if isArtifactValidationExecution(canonical, input) {
		state.MarkAllArtifactsVerified()
		state.MarkVerificationEvidence("artifact_validation_execution")
	}
}

func RequirePostWriteVerificationCompletion(ctx context.Context, policy LearnedToolPolicy) error {
	if !policy.RequirePostWriteVerification {
		return nil
	}
	state := GetToolUsageStateFromContext(ctx)
	if state == nil {
		return nil
	}
	pending := state.PendingArtifactVerificationPaths()
	if len(pending) == 0 {
		return nil
	}
	return &TurnGuardrailError{
		Title:   "Verification Required",
		Message: fmt.Sprintf("The turn wrote %s but never verified the written artifact. Read it back with `single_view`, `view`, `agentic_view`, or `lsp_diagnostics`, or run a narrow verification command before completing.", strings.Join(pending, ", ")),
	}
}

func RequireContextReadCompletion(ctx context.Context, policy LearnedToolPolicy) error {
	if !policy.RequireContextRead {
		return nil
	}
	state := GetToolUsageStateFromContext(ctx)
	if state == nil || HasRequiredContextReadEvidence(state) {
		return nil
	}
	return &TurnGuardrailError{
		Title:   "More Evidence Required",
		Message: "This broad turn completed without enough repository evidence. Use structured discovery plus at least one real code read before concluding, delegating, or editing.",
	}
}

func RequireExplicitPlanCompletion(ctx context.Context, policy LearnedToolPolicy) error {
	if !policy.RequireExplicitPlan {
		return nil
	}
	state := GetToolUsageStateFromContext(ctx)
	if state == nil || state.HasPublishedPlan() {
		return nil
	}
	return &TurnGuardrailError{
		Title:   "Plan Required",
		Message: "This broad turn completed without publishing `update_plan`. After the first real repository evidence pass, use `update_plan` to lock the working plan before finishing.",
	}
}

func normalizeArtifactVerificationPath(path string) (string, bool) {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." || path == "" {
		return "", false
	}
	return path, true
}

func cloneStringIntMapLocked(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func sortedMapKeys(in map[string]int) []string {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSetKeys(in map[string]struct{}) []string {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func resolveArtifactVerificationPath(ctx context.Context, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.ToSlash(filepath.Clean(path))
	}
	if workingDir := strings.TrimSpace(GetWorkingDirFromContext(ctx)); workingDir != "" {
		return filepath.ToSlash(filepath.Clean(filepath.Join(workingDir, path)))
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func extractArtifactWritePathsFromInput(toolName string, input map[string]any) []string {
	if !isArtifactWriteTool(toolName) {
		return nil
	}
	return extractWritePaths(toolName, input)
}

func extractArtifactVerificationPathsFromInput(toolName string, input map[string]any) []string {
	switch toolName {
	case ViewToolName, SingleViewToolName, AgenticViewToolName:
		return extractViewPaths(input)
	case DiagnosticsToolName:
		return extractGenericPathFields(input)
	case RGToolName, GrepToolName:
		return extractGenericPathFields(input)
	case LSToolName, GlobToolName:
		return extractGenericPathFields(input)
	default:
		return nil
	}
}

func isArtifactWriteTool(toolName string) bool {
	switch toolName {
	case EditToolName, SingleEditToolName, AgenticEditToolName, ApplyPatchToolName, WriteToolName:
		return true
	default:
		return false
	}
}

func isArtifactValidationExecution(toolName string, input map[string]any) bool {
	switch toolName {
	case BashToolName:
		command, _ := input["command"].(string)
		return looksLikeVerificationCommand(command)
	default:
		return false
	}
}

func looksLikeVerificationCommand(command string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(command), " "))
	if normalized == "" {
		return false
	}
	verificationPrefixes := []string{
		"go test", "go build", "go vet",
		"cargo test", "cargo build", "cargo check", "cargo clippy",
		"npm test", "npm run test", "npm run build", "npm run lint", "npm run typecheck",
		"pnpm test", "pnpm run test", "pnpm build", "pnpm run build", "pnpm lint", "pnpm run lint", "pnpm typecheck", "pnpm run typecheck",
		"yarn test", "yarn build", "yarn lint", "yarn typecheck",
		"bun test", "bun run test", "bun run build", "bun run lint", "bun run typecheck",
		"pytest", "python -m pytest", "python -m unittest",
		"jest", "vitest", "eslint", "ruff check", "mypy", "golangci-lint",
		"make test", "make build", "make lint", "make check",
		"just test", "just build", "just lint", "just check",
	}
	for _, prefix := range verificationPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}
