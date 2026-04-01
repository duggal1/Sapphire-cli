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
}

var (
	sharedToolUsageMu        sync.Mutex
	sharedToolUsageBySession = map[string]*ToolUsageState{}
)

func NewToolUsageState() *ToolUsageState {
	return &ToolUsageState{
		counts:                      map[string]int{},
		pendingArtifactVerification: map[string]struct{}{},
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
	for _, path := range extractArtifactWritePathsFromInput(canonical, input) {
		state.MarkArtifactWrite(resolveArtifactVerificationPath(ctx, path))
	}
	for _, path := range extractArtifactVerificationPathsFromInput(canonical, input) {
		state.MarkArtifactVerified(resolveArtifactVerificationPath(ctx, path))
	}
	if isArtifactValidationExecution(canonical, input) {
		state.MarkAllArtifactsVerified()
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

func normalizeArtifactVerificationPath(path string) (string, bool) {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." || path == "" {
		return "", false
	}
	return path, true
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
