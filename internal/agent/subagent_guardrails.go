package agent

import (
	"regexp"
	"strings"
)

type subAgentLaunchDecision struct {
	Allowed        bool
	Reason         string
	Complexity     int
	Domains        []string
	Parallelizable bool
	TaskKey        string
}

var (
	subAgentGreetingPhrases = map[string]struct{}{
		"hi":             {},
		"hello":          {},
		"hey":            {},
		"yo":             {},
		"sup":            {},
		"ping":           {},
		"test":           {},
		"thanks":         {},
		"thank you":      {},
		"ok":             {},
		"okay":           {},
		"cool":           {},
		"good morning":   {},
		"good afternoon": {},
		"good evening":   {},
		"how are you":    {},
		"whats up":       {},
		"what's up":      {},
	}

	subAgentGreetingWords = map[string]struct{}{
		"hi":     {},
		"hello":  {},
		"hey":    {},
		"yo":     {},
		"sup":    {},
		"ping":   {},
		"test":   {},
		"thanks": {},
		"thank":  {},
		"ok":     {},
		"okay":   {},
		"cool":   {},
	}

	subAgentCodebaseSignals = []string{
		"scan",
		"inventory",
		"survey",
		"find relevant files",
		"file discovery",
		"locate files",
		"search codebase",
		"codebase map",
		"codebase mapping",
		"repo map",
		"repo mapping",
		"map the codebase",
		"map the repo",
		"across the repo",
		"across the codebase",
		"repo-wide",
		"codebase-wide",
		"entire repo",
		"whole repo",
	}

	subAgentDependencySignals = []string{
		"dependency",
		"dependencies",
		"trace",
		"tracing",
		"call path",
		"call-path",
		"control flow",
		"data flow",
		"root cause",
		"regression",
		"stack trace",
	}

	subAgentSourceSignals = []string{
		"docs",
		"documentation",
		"spec",
		"specification",
		"rfc",
		"changelog",
		"release notes",
		"api reference",
		"reference docs",
	}

	subAgentMultiSourceSignals = []string{
		"multiple sources",
		"independent sources",
		"compare sources",
		"cross check",
		"cross-check",
		"triangulate",
		"corroborate",
		"verify against",
		"validate against",
	}

	subAgentTestSignals = []string{
		"test",
		"tests",
		"lint",
		"linting",
		"build",
		"ci",
		"validation",
		"benchmark",
		"benchmarks",
		"compile",
	}

	subAgentMonitoringSignals = []string{
		"monitor",
		"monitoring",
		"watch",
		"tail",
		"observe",
		"profiling",
		"long-running",
		"continuous",
		"soak",
		"telemetry",
		"metrics",
		"logs",
		"log file",
	}

	subAgentEnvPrepSignals = []string{
		"environment",
		"env",
		"setup",
		"bootstrap",
		"provision",
		"prepare",
		"install",
		"deps",
		"dependencies",
		"install dependencies",
		"dependencies install",
		"setup environments",
		"prepare environments",
		"configure environment",
	}

	subAgentDiagnosticsSignals = []string{
		"diagnostic",
		"diagnostics",
		"system state",
		"system info",
		"environment data",
		"query api",
		"call api",
		"fetch api",
		"api request",
		"http request",
		"curl",
		"web search",
		"search the web",
		"google",
		"duckduckgo",
		"logs",
		"tail logs",
		"system metrics",
		"script",
		"run script",
		"execute script",
	}

	subAgentLightweightSignals = []string{
		"list files",
		"list file",
		"list directory",
		"list directories",
		"ls",
		"grep",
		"rg ",
		"ripgrep",
		"search for",
		"find references",
		"find usages",
		"find occurrences",
		"locate file",
		"locate files",
		"show file",
		"open file",
		"view file",
		"read file",
		"cat ",
		"print file",
		"display file",
	}

	subAgentRiskSignals = []string{
		"risk",
		"risks",
		"edge case",
		"edge cases",
		"validation",
		"verify",
		"verification",
		"rollout",
		"regression",
		"incident",
		"refactor",
		"migration",
	}

	subAgentConjunctions = []string{
		" and ",
		" then ",
		" also ",
		" plus ",
		" as well as ",
		" while ",
		" alongside ",
		" & ",
	}

	subAgentDomainSignals = map[string][]string{
		"frontend": {
			"ui", "frontend", "react", "tailwind", "css", "layout", "tui", "terminal ui",
		},
		"backend": {
			"backend", "api", "server", "handler", "service", "router", "grpc", "rpc",
		},
		"database": {
			"db", "database", "sql", "sqlite", "postgres", "mysql", "schema", "migration",
		},
		"infra": {
			"infra", "deploy", "deployment", "docker", "kubernetes", "ci", "pipeline", "build",
		},
		"testing": {
			"test", "tests", "benchmark", "soak", "qa", "coverage",
		},
		"docs": {
			"docs", "documentation", "readme", "spec", "rfc",
		},
		"observability": {
			"logs", "logging", "metrics", "tracing", "monitoring", "telemetry",
		},
		"security": {
			"auth", "oauth", "permission", "security", "token", "secret",
		},
	}
)

func shouldAllowSubAgentLaunch(prompt string) (bool, string) {
	decision := evaluateSubAgentLaunch(prompt)
	return decision.Allowed, decision.Reason
}

func isTrivialSubAgentPrompt(prompt string) bool {
	trimmed := strings.Trim(prompt, "!?., ")
	if trimmed == "" {
		return true
	}
	if _, ok := subAgentGreetingPhrases[trimmed]; ok {
		return true
	}
	words := strings.Fields(trimmed)
	if len(words) == 0 {
		return true
	}
	if len(words) <= 3 {
		for _, word := range words {
			if _, ok := subAgentGreetingWords[word]; !ok {
				return false
			}
		}
		return true
	}
	return false
}

func looksLikeQuestion(prompt string) bool {
	if strings.HasSuffix(prompt, "?") {
		return true
	}
	for _, prefix := range []string{
		"what ",
		"why ",
		"how ",
		"explain ",
		"define ",
		"when ",
		"where ",
		"who ",
		"help ",
		"tell me ",
	} {
		if strings.HasPrefix(prompt, prefix) {
			return true
		}
	}
	return false
}

func isLightweightSingleOperation(prompt string, wordCount int) bool {
	if wordCount == 0 || wordCount > 6 {
		return false
	}
	if !hasAnySignal(prompt, subAgentLightweightSignals) {
		return false
	}
	for _, conj := range subAgentConjunctions {
		if strings.Contains(prompt, conj) {
			return false
		}
	}
	return true
}

func hasAnySignal(prompt string, signals []string) bool {
	for _, signal := range signals {
		if strings.Contains(prompt, signal) {
			return true
		}
	}
	return false
}

func evaluateSubAgentLaunch(prompt string) subAgentLaunchDecision {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	decision := subAgentLaunchDecision{
		Allowed: false,
		Reason:  "",
	}
	if normalized == "" {
		decision.Reason = "empty prompt"
		return decision
	}
	if isTrivialSubAgentPrompt(normalized) {
		decision.Reason = "trivial prompt"
		return decision
	}

	wordCount := len(strings.Fields(normalized))
	if isLightweightSingleOperation(normalized, wordCount) {
		decision.Reason = "single immediate operation"
		return decision
	}

	domains := detectSubAgentDomains(normalized)
	decision.Domains = domains
	decision.TaskKey = subAgentTaskKey(normalized)

	operational := hasAnySignal(normalized, subAgentCodebaseSignals) ||
		hasAnySignal(normalized, subAgentDependencySignals) ||
		hasAnySignal(normalized, subAgentSourceSignals) ||
		hasAnySignal(normalized, subAgentMultiSourceSignals) ||
		hasAnySignal(normalized, subAgentTestSignals) ||
		hasAnySignal(normalized, subAgentMonitoringSignals) ||
		hasAnySignal(normalized, subAgentEnvPrepSignals) ||
		hasAnySignal(normalized, subAgentDiagnosticsSignals)

	listCount := countListItems(prompt)
	sentenceCount := countSentences(normalized)
	decision.Parallelizable = len(domains) > 1 || listCount > 1 || hasAnySignal(normalized, []string{"parallel", "concurrent", "in parallel", "simultaneous"})

	decision.Complexity = complexityScore(wordCount, sentenceCount, listCount, len(domains), operational)

	if operational || decision.Complexity >= 4 || decision.Parallelizable {
		decision.Allowed = true
		return decision
	}

	if decision.Complexity <= 1 {
		decision.Reason = "too small for delegation"
		return decision
	}

	decision.Reason = "insufficient task complexity"
	return decision
}

func detectSubAgentDomains(prompt string) []string {
	if prompt == "" {
		return nil
	}
	var domains []string
	for name, signals := range subAgentDomainSignals {
		if hasAnySignal(prompt, signals) {
			domains = append(domains, name)
		}
	}
	return domains
}

func countSentences(prompt string) int {
	if prompt == "" {
		return 0
	}
	re := regexp.MustCompile(`[.!?]+`)
	return len(re.FindAllStringIndex(prompt, -1))
}

func countListItems(prompt string) int {
	if prompt == "" {
		return 0
	}
	lines := strings.Split(prompt, "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			count++
			continue
		}
		if len(line) > 2 && line[0] >= '0' && line[0] <= '9' && line[1] == '.' {
			count++
		}
	}
	return count
}

func complexityScore(wordCount, sentenceCount, listCount, domainCount int, operational bool) int {
	score := 0
	switch {
	case wordCount >= 120:
		score += 4
	case wordCount >= 80:
		score += 3
	case wordCount >= 40:
		score += 2
	case wordCount >= 20:
		score++
	}
	if sentenceCount >= 3 {
		score += 2
	} else if sentenceCount >= 2 {
		score++
	}
	if listCount >= 3 {
		score += 2
	} else if listCount >= 1 {
		score++
	}
	if domainCount >= 3 {
		score += 2
	} else if domainCount >= 2 {
		score++
	}
	if operational {
		score++
	}
	return score
}

func subAgentTaskKey(prompt string) string {
	if prompt == "" {
		return ""
	}
	normalized := strings.ToLower(prompt)
	normalized = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == ' ':
			return r
		default:
			return -1
		}
	}, normalized)
	fields := strings.Fields(normalized)
	if len(fields) > 18 {
		fields = fields[:18]
	}
	return strings.Join(fields, " ")
}
