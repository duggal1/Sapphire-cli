package memory

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	MistakesFileName       = "MISTAKES.md"
	mistakeProtocolRelPath = ".sapphire/mistake.md"
)

//go:embed templates/mistake.md
var mistakeProtocolTemplate string

type MistakeRootCauseClass string

const (
	MistakeRootCauseHallucination      MistakeRootCauseClass = "HALLUCINATION"
	MistakeRootCauseContextGap         MistakeRootCauseClass = "CONTEXT_GAP"
	MistakeRootCauseComplexityOverload MistakeRootCauseClass = "COMPLEXITY_OVERLOAD"
	MistakeRootCauseWrongAssumption    MistakeRootCauseClass = "WRONG_ASSUMPTION"
	MistakeRootCauseOrchestrationFail  MistakeRootCauseClass = "ORCHESTRATION_FAILURE"
	MistakeRootCauseToolMisuse         MistakeRootCauseClass = "TOOL_MISUSE"
)

type MistakeSeverity string

const (
	MistakeSeverityLow      MistakeSeverity = "LOW"
	MistakeSeverityMedium   MistakeSeverity = "MEDIUM"
	MistakeSeverityHigh     MistakeSeverity = "HIGH"
	MistakeSeverityCritical MistakeSeverity = "CRITICAL"
)

type MistakeLogInput struct {
	Fingerprint    string
	Date           time.Time
	Task           string
	TaskDomain     string
	Agent          string
	Model          string
	Worktree       string
	WhatHappened   string
	RootCauseClass MistakeRootCauseClass
	RootCause      string
	DeepAnalysis   string
	WhyThisClass   string
	Severity       MistakeSeverity
	IsIgnorable    bool
	SolutionSteps  []string
	PreventionRule string
	StatusNote     string
	Resolved       bool
}

type MistakeEntry struct {
	Number         int
	Fingerprint    string
	Date           time.Time
	Task           string
	TaskDomain     string
	Agent          string
	Model          string
	Worktree       string
	RootCauseClass MistakeRootCauseClass
	Severity       MistakeSeverity
	PreventionRule string
	Resolved       bool
	Body           string
}

type MistakeRegister struct {
	Scope   string
	Entries []MistakeEntry
}

var (
	mistakeFileMu        sync.Mutex
	mistakeHeadingRE     = regexp.MustCompile(`(?m)^## MISTAKE-(\d{3})\s*$`)
	mistakeFingerprintRE = regexp.MustCompile(`(?m)^<!--\s*mistake_fingerprint:\s*([^\s]+)\s*-->`)
)

func MistakesPath(repoRoot string) string {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return ""
	}
	return filepath.Join(repoRoot, MistakesFileName)
}

func MistakeProtocolPath(repoRoot string) string {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return ""
	}
	return filepath.Join(repoRoot, filepath.FromSlash(mistakeProtocolRelPath))
}

func EnsureMistakeProtocol(repoRoot string) error {
	path := MistakeProtocolPath(repoRoot)
	if path == "" {
		return nil
	}
	if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) != "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := strings.TrimSpace(mistakeProtocolTemplate)
	if content == "" {
		return nil
	}
	return os.WriteFile(path, []byte(content+"\n"), 0o644)
}

func MistakesFileExists(repoRoot string) bool {
	path := MistakesPath(repoRoot)
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(data)) != ""
}

func BuildFailureFingerprint(sessionID string, turnIndex int, failure FailureEncountered) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(sessionID),
		fmt.Sprintf("%d", turnIndex),
		strings.TrimSpace(failure.WhatFailed),
		strings.TrimSpace(failure.RootCause),
		string(failure.NormalizedRootCauseClass()),
		strings.TrimSpace(failure.PreventionRule),
	}, "\n")))
	return "failure:" + hex.EncodeToString(sum[:8])
}

func NormalizeMistakeRootCauseClass(raw string) MistakeRootCauseClass {
	switch strings.ToUpper(cleanMistakeToken(raw)) {
	case string(MistakeRootCauseHallucination):
		return MistakeRootCauseHallucination
	case string(MistakeRootCauseContextGap):
		return MistakeRootCauseContextGap
	case string(MistakeRootCauseComplexityOverload):
		return MistakeRootCauseComplexityOverload
	case string(MistakeRootCauseWrongAssumption):
		return MistakeRootCauseWrongAssumption
	case string(MistakeRootCauseOrchestrationFail):
		return MistakeRootCauseOrchestrationFail
	case string(MistakeRootCauseToolMisuse):
		return MistakeRootCauseToolMisuse
	default:
		return ""
	}
}

func NormalizeMistakeSeverity(raw string) MistakeSeverity {
	switch strings.ToUpper(cleanMistakeToken(raw)) {
	case string(MistakeSeverityLow):
		return MistakeSeverityLow
	case string(MistakeSeverityMedium):
		return MistakeSeverityMedium
	case string(MistakeSeverityHigh):
		return MistakeSeverityHigh
	case string(MistakeSeverityCritical):
		return MistakeSeverityCritical
	default:
		return ""
	}
}

func LoadMistakeRegister(repoRoot string) (MistakeRegister, error) {
	path := MistakesPath(repoRoot)
	if path == "" {
		return MistakeRegister{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return MistakeRegister{Scope: filepath.Base(repoRoot)}, nil
		}
		return MistakeRegister{}, err
	}
	return parseMistakeRegister(filepath.Base(repoRoot), string(data)), nil
}

func HasLoggedMistakeFingerprint(repoRoot, fingerprint string) bool {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return false
	}
	register, err := LoadMistakeRegister(repoRoot)
	if err != nil {
		return false
	}
	for _, entry := range register.Entries {
		if entry.Fingerprint == fingerprint {
			return true
		}
	}
	return false
}

func AppendMistake(repoRoot string, input MistakeLogInput) (MistakeEntry, bool, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return MistakeEntry{}, false, nil
	}
	if err := EnsureMistakeProtocol(repoRoot); err != nil {
		return MistakeEntry{}, false, err
	}
	mistakeFileMu.Lock()
	defer mistakeFileMu.Unlock()

	register, err := LoadMistakeRegister(repoRoot)
	if err != nil {
		return MistakeEntry{}, false, err
	}
	if fingerprint := strings.TrimSpace(input.Fingerprint); fingerprint != "" {
		for _, entry := range register.Entries {
			if entry.Fingerprint == fingerprint {
				return entry, false, nil
			}
		}
	}

	nextNumber := len(register.Entries) + 1
	when := input.Date.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	entry := MistakeEntry{
		Number:         nextNumber,
		Fingerprint:    strings.TrimSpace(input.Fingerprint),
		Date:           when,
		Task:           firstNonEmptyStringValue(input.Task, input.WhatHappened, "Unspecified task"),
		TaskDomain:     firstNonEmptyStringValue(input.TaskDomain, "general"),
		Agent:          firstNonEmptyStringValue(input.Agent, "unknown"),
		Model:          firstNonEmptyStringValue(input.Model, "unknown"),
		Worktree:       firstNonEmptyStringValue(input.Worktree, "shared"),
		RootCauseClass: normalizeEntryRootCauseClass(input.RootCauseClass),
		Severity:       normalizeEntrySeverity(input.Severity),
		PreventionRule: strings.TrimSpace(input.PreventionRule),
		Resolved:       input.Resolved,
	}
	register.Entries = append(register.Entries, entry)

	rendered := renderMistakeRegister(filepath.Base(repoRoot), register.Entries, entry, input)
	if err := os.WriteFile(MistakesPath(repoRoot), []byte(rendered), 0o644); err != nil {
		return MistakeEntry{}, false, err
	}
	return entry, true, nil
}

func PreventionRules(repoRoot string, limit int) []string {
	register, err := LoadMistakeRegister(repoRoot)
	if err != nil {
		return nil
	}
	var rules []string
	seen := map[string]struct{}{}
	for _, entry := range register.Entries {
		if entry.RootCauseClass == MistakeRootCauseHallucination {
			continue
		}
		rule := strings.TrimSpace(entry.PreventionRule)
		if rule == "" {
			continue
		}
		if _, ok := seen[rule]; ok {
			continue
		}
		seen[rule] = struct{}{}
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	if limit > 0 && len(rules) > limit {
		rules = rules[:limit]
	}
	return rules
}

func RenderPreventionRulesBlock(repoRoot string, limit int) string {
	rules := PreventionRules(repoRoot, limit)
	if len(rules) == 0 {
		return ""
	}
	lines := []string{"### Prevention Rules From MISTAKES.md"}
	for _, rule := range rules {
		lines = append(lines, "- "+rule)
	}
	return strings.Join(lines, "\n")
}

func parseMistakeRegister(scope, raw string) MistakeRegister {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	register := MistakeRegister{Scope: strings.TrimSpace(scope)}
	matches := mistakeHeadingRE.FindAllStringSubmatchIndex(raw, -1)
	for i, match := range matches {
		if len(match) < 4 {
			continue
		}
		number := parseThreeDigitNumber(raw[match[2]:match[3]])
		bodyStart := match[1]
		bodyEnd := len(raw)
		if i+1 < len(matches) && len(matches[i+1]) >= 2 {
			bodyEnd = matches[i+1][0]
		}
		if appendixStart := strings.Index(raw[bodyStart:bodyEnd], "\n## APPENDIX:"); appendixStart >= 0 {
			bodyEnd = bodyStart + appendixStart
		}
		section := raw[bodyStart:bodyEnd]
		entry := MistakeEntry{
			Number:         number,
			Fingerprint:    parseSectionFingerprint(section),
			Date:           parseSectionTime(matchSectionField(section, "**Date:**")),
			Task:           firstNonEmptyStringValue(matchSectionField(section, "**Task:**"), "Unspecified task"),
			TaskDomain:     firstNonEmptyStringValue(extractSectionBody(section, "### Task Domain"), "general"),
			Agent:          firstNonEmptyStringValue(parseInlineLabeledValue(section, "**Agent:**", "|"), "unknown"),
			Model:          firstNonEmptyStringValue(parseInlineLabeledValue(section, "**Model:**", "\n"), "unknown"),
			Worktree:       firstNonEmptyStringValue(matchSectionField(section, "**Worktree:**"), "shared"),
			RootCauseClass: NormalizeMistakeRootCauseClass(extractSectionBody(section, "### Root Cause Class")),
			Severity:       NormalizeMistakeSeverity(extractSectionBody(section, "### Severity")),
			PreventionRule: parsePreventionRule(extractSectionBody(section, "### Prevention Rule")),
			Resolved:       strings.Contains(strings.ToUpper(extractSectionBody(section, "### Status")), "RESOLVED"),
			Body:           strings.TrimSpace(section),
		}
		register.Entries = append(register.Entries, entry)
	}
	return register
}

func renderMistakeRegister(scope string, entries []MistakeEntry, newest MistakeEntry, input MistakeLogInput) string {
	lines := []string{
		fmt.Sprintf("# %s — Failure Intelligence Register", MistakesFileName),
		fmt.Sprintf("# Scope: %s | Reset: per repository | Version: %d", firstNonEmptyStringValue(scope, "repo"), len(entries)),
		"",
		"---",
		"",
		"## INDEX",
		"",
		"| # | Date | Task Domain | Root Cause Class | Severity | Resolved |",
		"|---|------|-------------|------------------|----------|----------|",
	}
	for _, entry := range entries {
		resolved := "no"
		if entry.Resolved {
			resolved = "yes"
		}
		lines = append(lines, fmt.Sprintf("| %03d | %s | %s | %s | %s | %s |",
			entry.Number,
			entry.Date.UTC().Format("2006-01-02"),
			entry.TaskDomain,
			entry.RootCauseClass,
			entry.Severity,
			resolved,
		))
	}

	lines = append(lines, "", "---")
	for _, entry := range entries {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("## MISTAKE-%03d", entry.Number))
		lines = append(lines, "")
		if entry.Number == newest.Number && strings.TrimSpace(entry.Fingerprint) != "" {
			lines = append(lines, fmt.Sprintf("<!-- mistake_fingerprint: %s -->", newest.Fingerprint))
			lines = append(lines, "")
		}
		if entry.Number == newest.Number {
			lines = append(lines, renderMistakeEntryBody(entry, input)...)
			continue
		}
		if entry.Body != "" {
			lines = append(lines, entry.Body)
		} else {
			lines = append(lines, renderExistingMistakeEntryBody(entry)...)
		}
	}
	lines = append(lines, "", "---", "")
	lines = append(lines, renderMistakeAppendix()...)
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func renderMistakeEntryBody(entry MistakeEntry, input MistakeLogInput) []string {
	lines := []string{
		fmt.Sprintf("**Date:** %s", entry.Date.UTC().Format(time.RFC3339)),
		fmt.Sprintf("**Task:** %s", entry.Task),
		fmt.Sprintf("**Agent:** %s | **Model:** %s", entry.Agent, entry.Model),
		fmt.Sprintf("**Worktree:** %s", entry.Worktree),
		"",
		"### Task Domain",
		entry.TaskDomain,
		"",
		"### What Happened",
		firstNonEmptyStringValue(input.WhatHappened, entry.Task),
		"",
		"### Root Cause Class",
		fmt.Sprintf("`%s`", entry.RootCauseClass),
		"",
		"### Root Cause — Deep Analysis",
		firstNonEmptyStringValue(input.DeepAnalysis, input.RootCause, "No deep analysis was recorded."),
		"",
		"### Why This Class, Not Another",
		firstNonEmptyStringValue(input.WhyThisClass, "Derived from the extracted failure evidence and taxonomy match."),
		"",
		"### Severity",
		fmt.Sprintf("`%s`", entry.Severity),
		"",
		"### Is It Ignorable?",
		strings.ToUpper(boolWord(input.IsIgnorable)),
		"",
		"### Solution — Permanent Fix",
	}
	if len(input.SolutionSteps) == 0 {
		lines = append(lines, "1. No permanent fix steps were recorded.")
	} else {
		for i, step := range input.SolutionSteps {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(step)))
		}
	}
	lines = append(lines,
		"",
		"### Prevention Rule",
		renderPreventionRule(entry.Number, entry.PreventionRule),
		"",
		"### Status",
		renderMistakeStatus(entry, input.StatusNote),
	)
	return lines
}

func renderExistingMistakeEntryBody(entry MistakeEntry) []string {
	lines := []string{
		fmt.Sprintf("**Date:** %s", entry.Date.UTC().Format(time.RFC3339)),
		fmt.Sprintf("**Task:** %s", entry.Task),
		fmt.Sprintf("**Agent:** %s | **Model:** %s", entry.Agent, entry.Model),
		fmt.Sprintf("**Worktree:** %s", entry.Worktree),
		"",
		"### Task Domain",
		entry.TaskDomain,
		"",
		"### Root Cause Class",
		fmt.Sprintf("`%s`", entry.RootCauseClass),
		"",
		"### Severity",
		fmt.Sprintf("`%s`", entry.Severity),
		"",
		"### Prevention Rule",
		renderPreventionRule(entry.Number, entry.PreventionRule),
		"",
		"### Status",
		renderMistakeStatus(entry, ""),
	}
	return lines
}

func renderPreventionRule(number int, rule string) string {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return "No durable prevention rule was recorded."
	}
	ruleID := fmt.Sprintf("RULE-%03d", number)
	if !strings.HasPrefix(strings.ToUpper(rule), ruleID+":") {
		rule = ruleID + ": " + rule
	}
	return "> " + rule
}

func renderMistakeStatus(entry MistakeEntry, statusNote string) string {
	status := "PENDING"
	if entry.Resolved {
		status = "RESOLVED"
	}
	note := strings.TrimSpace(statusNote)
	if note == "" {
		if entry.Resolved {
			note = "Prevention rule persisted to durable memory."
		} else if entry.RootCauseClass == MistakeRootCauseHallucination {
			note = "Logged for reference only. No structural prevention rule was persisted."
		} else {
			note = "Prevention rule still needs durable persistence."
		}
	}
	return fmt.Sprintf("`%s` | %s", status, note)
}

func renderMistakeAppendix() []string {
	return []string{
		"## APPENDIX: ROOT CAUSE TAXONOMY",
		"",
		"| Class | Definition | Ignorable? | Fix Target |",
		"|-------|------------|------------|------------|",
		"| `HALLUCINATION` | Model invented facts not in context | yes | nothing |",
		"| `CONTEXT_GAP` | Needed info existed but was outside the context window | no | boot packet / required reads |",
		"| `COMPLEXITY_OVERLOAD` | Task was too large for one agent turn | no | task decomposition |",
		"| `WRONG_ASSUMPTION` | Agent assumed something false about environment or state | no | prevention rule |",
		"| `ORCHESTRATION_FAILURE` | Multi-agent coordination failed | no | mailbox / worktree protocol |",
		"| `TOOL_MISUSE` | Wrong tool or mode was used for the job | no | prompt / mode guidance |",
		"",
		"## APPENDIX: RESOLUTION PROTOCOL",
		"",
		"1. Classify the failure using the root-cause taxonomy.",
		"2. Write a prevention rule in imperative form.",
		"3. If the class is not `HALLUCINATION`, persist the prevention rule via `save_memory` as an architectural decision.",
		"4. Mark the entry resolved only after the rule is persisted structurally.",
	}
}

func normalizeEntryRootCauseClass(class MistakeRootCauseClass) MistakeRootCauseClass {
	if normalized := NormalizeMistakeRootCauseClass(string(class)); normalized != "" {
		return normalized
	}
	return MistakeRootCauseWrongAssumption
}

func normalizeEntrySeverity(severity MistakeSeverity) MistakeSeverity {
	if normalized := NormalizeMistakeSeverity(string(severity)); normalized != "" {
		return normalized
	}
	return MistakeSeverityHigh
}

func parseSectionFingerprint(section string) string {
	match := mistakeFingerprintRE.FindStringSubmatch(section)
	if len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func matchSectionField(section, prefix string) string {
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func parseInlineLabeledValue(section, prefix, terminator string) string {
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		value := strings.TrimSpace(line[idx+len(prefix):])
		if terminator != "" {
			if end := strings.Index(value, terminator); end >= 0 {
				value = value[:end]
			}
		}
		return strings.TrimSpace(value)
	}
	if terminator == "" {
		return ""
	}
	return ""
}

func extractSectionBody(section, heading string) string {
	idx := strings.Index(section, heading)
	if idx < 0 {
		return ""
	}
	body := strings.TrimLeft(section[idx+len(heading):], "\n")
	if next := strings.Index(body, "\n### "); next >= 0 {
		body = body[:next]
	}
	return strings.TrimSpace(body)
}

func parsePreventionRule(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if line == "" {
			continue
		}
		return line
	}
	return ""
}

func parseSectionTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func parseThreeDigitNumber(raw string) int {
	raw = strings.TrimSpace(raw)
	if len(raw) != 3 {
		return 0
	}
	total := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0
		}
		total = total*10 + int(ch-'0')
	}
	return total
}

func boolWord(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func firstNonEmptyStringValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cleanMistakeToken(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "`")
	raw = strings.TrimSpace(raw)
	return raw
}
