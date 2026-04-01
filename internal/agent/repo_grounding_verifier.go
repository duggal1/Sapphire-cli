package agent

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

type repoGroundingFileClaim struct {
	Path    string
	Symbols []string
}

type repoGroundingPackageClaim struct {
	Qualifier string
	Symbol    string
	Raw       string
}

func verifyRepoGroundingClaims(ctx context.Context, policy agenttools.LearnedToolPolicy, assistantText string) error {
	if !shouldVerifyRepoGrounding(policy) {
		return nil
	}
	assistantText = strings.TrimSpace(assistantText)
	if assistantText == "" {
		return nil
	}
	repoRoot := strings.TrimSpace(agenttools.GetWorkingDirFromContext(ctx))
	if repoRoot == "" {
		return nil
	}
	if info, err := os.Stat(repoRoot); err != nil || !info.IsDir() {
		return nil
	}

	normalizedText := stripRepoGroundingFencedCodeBlocks(assistantText)
	fileClaims := collectRepoGroundingFileClaims(repoRoot, normalizedText)
	packageClaims := collectRepoGroundingPackageClaims(normalizedText)
	if len(fileClaims) == 0 && len(packageClaims) == 0 {
		return nil
	}

	missing := make([]string, 0)
	missing = append(missing, verifyRepoGroundingFileClaims(repoRoot, fileClaims)...)
	missing = append(missing, verifyRepoGroundingPackageClaims(repoRoot, packageClaims)...)
	missing = uniqueSortedStrings(missing)
	if len(missing) == 0 {
		return nil
	}
	return &agenttools.TurnGuardrailError{
		Title:   "Repo Grounding Failed",
		Message: "This turn made repository-grounding claims that do not exist in the codebase: " + strings.Join(missing, ", ") + ". Re-read the cited files and ground the answer in real code before finishing.",
	}
}

func shouldVerifyRepoGrounding(policy agenttools.LearnedToolPolicy) bool {
	taskFamily := strings.ToLower(strings.TrimSpace(policy.TaskFamily))
	if taskFamily == "" {
		return false
	}
	if strings.HasPrefix(taskFamily, "initialize/") {
		return false
	}
	return strings.HasPrefix(taskFamily, "design/") ||
		strings.HasPrefix(taskFamily, "research/") ||
		strings.HasPrefix(taskFamily, "review/") ||
		strings.HasPrefix(taskFamily, "implementation/") ||
		strings.HasPrefix(taskFamily, "migration/")
}

func stripRepoGroundingFencedCodeBlocks(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func collectRepoGroundingFileClaims(repoRoot, text string) []repoGroundingFileClaim {
	claims := map[string]map[string]struct{}{}
	for _, line := range strings.Split(text, "\n") {
		paths := extractRepoGroundingPaths(repoRoot, line)
		if len(paths) == 0 {
			continue
		}
		symbols := extractRepoGroundingLineSymbols(line)
		if len(symbols) == 0 {
			continue
		}
		for _, path := range paths {
			if claims[path] == nil {
				claims[path] = map[string]struct{}{}
			}
			for _, symbol := range symbols {
				claims[path][symbol] = struct{}{}
			}
		}
	}
	out := make([]repoGroundingFileClaim, 0, len(claims))
	for path, symbols := range claims {
		out = append(out, repoGroundingFileClaim{
			Path:    path,
			Symbols: mapKeysSorted(symbols),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

func collectRepoGroundingPackageClaims(text string) []repoGroundingPackageClaim {
	claims := map[string]repoGroundingPackageClaim{}
	for _, line := range strings.Split(text, "\n") {
		for _, claim := range extractRepoGroundingPackageClaimsFromLine(line) {
			key := claim.Qualifier + "." + claim.Symbol
			claims[key] = claim
		}
	}
	out := make([]repoGroundingPackageClaim, 0, len(claims))
	for _, claim := range claims {
		out = append(out, claim)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Qualifier == out[j].Qualifier {
			return out[i].Symbol < out[j].Symbol
		}
		return out[i].Qualifier < out[j].Qualifier
	})
	return out
}

func verifyRepoGroundingFileClaims(repoRoot string, claims []repoGroundingFileClaim) []string {
	if len(claims) == 0 {
		return nil
	}
	missing := make([]string, 0)
	for _, claim := range claims {
		path := filepath.Join(repoRoot, filepath.FromSlash(claim.Path))
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, symbol := range claim.Symbols {
			if !repoGroundingContainsIdentifier(string(content), symbol) {
				missing = append(missing, claim.Path+" -> "+symbol)
			}
		}
	}
	return missing
}

func verifyRepoGroundingPackageClaims(repoRoot string, claims []repoGroundingPackageClaim) []string {
	if len(claims) == 0 {
		return nil
	}
	localPackageDirs := discoverRepoGroundingLocalPackageDirs(repoRoot)
	targets := map[string]repoGroundingPackageClaim{}
	targetDirs := map[string][]string{}
	for _, claim := range claims {
		dirs := localPackageDirs[strings.ToLower(claim.Qualifier)]
		if len(dirs) == 0 || isRepoGroundingIgnoredQualifier(claim.Qualifier) {
			continue
		}
		key := strings.ToLower(claim.Qualifier) + "." + claim.Symbol
		targets[key] = claim
		targetDirs[key] = dirs
	}
	if len(targets) == 0 {
		return nil
	}
	found := scanRepoGroundingDirectories(targets, targetDirs)
	missing := make([]string, 0)
	for key, claim := range targets {
		if len(targetDirs[key]) == 0 {
			continue
		}
		if !found[key] {
			missing = append(missing, claim.Raw)
		}
	}
	return missing
}

func extractRepoGroundingPaths(repoRoot, line string) []string {
	tokens := strings.Fields(line)
	paths := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.Trim(token, "[](){}<>,:;.'\"`")
		if token == "" || !strings.Contains(token, ".") {
			continue
		}
		normalized, ok := normalizeRepoGroundingPath(repoRoot, token)
		if ok {
			paths = append(paths, normalized)
		}
	}
	return uniqueSortedStrings(paths)
}

func normalizeRepoGroundingPath(repoRoot, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if filepath.IsAbs(raw) {
		rel, err := filepath.Rel(repoRoot, raw)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", false
		}
		raw = rel
	}
	raw = filepath.ToSlash(filepath.Clean(raw))
	if raw == "." || raw == "" || strings.HasPrefix(raw, "../") {
		return "", false
	}
	if !repoGroundingLooksLikeFilePath(raw) {
		return "", false
	}
	return raw, true
}

func repoGroundingLooksLikeFilePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".kt", ".swift", ".md", ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func extractRepoGroundingLineSymbols(line string) []string {
	symbols := map[string]struct{}{}
	for _, segment := range extractRepoGroundingCodeSegments(line) {
		for _, symbol := range extractRepoGroundingSymbolsFromSegment(segment) {
			symbols[symbol] = struct{}{}
		}
	}
	return mapKeysSorted(symbols)
}

func extractRepoGroundingCodeSegments(line string) []string {
	segments := make([]string, 0)
	start := -1
	for i, r := range line {
		if r == '`' {
			if start >= 0 {
				segment := strings.TrimSpace(line[start:i])
				if segment != "" {
					segments = append(segments, segment)
				}
				start = -1
			} else {
				start = i + 1
			}
		}
	}
	return segments
}

func extractRepoGroundingSymbolsFromSegment(segment string) []string {
	segment = strings.TrimSpace(strings.TrimSuffix(segment, "()"))
	if segment == "" {
		return nil
	}
	if qualifier, symbol, ok := parseRepoGroundingQualifiedSymbol(segment); ok {
		_ = qualifier
		return []string{symbol}
	}
	if isRepoGroundingExportedIdentifier(segment) {
		return []string{segment}
	}
	return nil
}

func extractRepoGroundingPackageClaimsFromLine(line string) []repoGroundingPackageClaim {
	fields := strings.FieldsFunc(line, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', ',', ':', ';', '(', ')', '[', ']', '{', '}', '<', '>', '`', '"', '\'':
			return true
		default:
			return false
		}
	})
	claims := make([]repoGroundingPackageClaim, 0)
	for _, field := range fields {
		qualifier, symbol, ok := parseRepoGroundingQualifiedSymbol(field)
		if !ok {
			continue
		}
		claims = append(claims, repoGroundingPackageClaim{
			Qualifier: qualifier,
			Symbol:    symbol,
			Raw:       qualifier + "." + symbol,
		})
	}
	return claims
}

func parseRepoGroundingQualifiedSymbol(token string) (string, string, bool) {
	token = strings.TrimSpace(strings.Trim(token, ".,:;"))
	token = strings.TrimSuffix(token, "()")
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", "", false
	}
	qualifier := strings.TrimSpace(parts[0])
	symbol := strings.TrimSpace(parts[1])
	if !isRepoGroundingLowerIdentifier(qualifier) || !isRepoGroundingExportedIdentifier(symbol) {
		return "", "", false
	}
	return qualifier, symbol, true
}

func isRepoGroundingLowerIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		switch {
		case i == 0 && (r < 'a' || r > 'z'):
			return false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_':
		default:
			return false
		}
	}
	return true
}

func isRepoGroundingExportedIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		switch {
		case i == 0 && (r < 'A' || r > 'Z'):
			return false
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_':
		default:
			return false
		}
	}
	return true
}

func discoverRepoGroundingLocalPackageDirs(repoRoot string) map[string][]string {
	packages := map[string][]string{}
	_ = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipRepoGroundingDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !repoGroundingIsCodeFile(path) {
			return nil
		}
		dir := filepath.Dir(path)
		base := strings.ToLower(filepath.Base(dir))
		if !isRepoGroundingLowerIdentifier(base) {
			return nil
		}
		packages[base] = appendIfMissing(packages[base], dir)
		return nil
	})
	return packages
}

func scanRepoGroundingDirectories(targets map[string]repoGroundingPackageClaim, targetDirs map[string][]string) map[string]bool {
	found := map[string]bool{}
	for key, claim := range targets {
		symbol := claim.Symbol
		dirs := targetDirs[key]
		for _, dir := range dirs {
			if found[key] {
				break
			}
			_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || !repoGroundingIsCodeFile(path) {
					return nil
				}
				content, readErr := os.ReadFile(path)
				if readErr != nil {
					return nil
				}
				if repoGroundingContainsIdentifier(string(content), symbol) {
					found[key] = true
					return fs.SkipAll
				}
				return nil
			})
		}
	}
	return found
}

func repoGroundingIsCodeFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".kt", ".swift":
		return true
	default:
		return false
	}
}

func shouldSkipRepoGroundingDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".sapphire", "node_modules", "vendor", "dist", "build", "coverage", ".next", ".turbo", ".idea", ".vscode":
		return true
	default:
		return false
	}
}

func repoGroundingContainsIdentifier(content, symbol string) bool {
	start := 0
	for {
		index := strings.Index(content[start:], symbol)
		if index < 0 {
			return false
		}
		index += start
		beforeOK := index == 0 || !repoGroundingIsIdentifierChar(rune(content[index-1]))
		afterIndex := index + len(symbol)
		afterOK := afterIndex >= len(content) || !repoGroundingIsIdentifierChar(rune(content[afterIndex]))
		if beforeOK && afterOK {
			return true
		}
		start = index + len(symbol)
	}
}

func repoGroundingIsIdentifierChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
}

func isRepoGroundingIgnoredQualifier(qualifier string) bool {
	switch strings.ToLower(strings.TrimSpace(qualifier)) {
	case "context", "http", "sql", "fmt", "os", "io", "json", "strings", "errors", "time", "bytes", "sync", "atomic", "filepath", "regexp", "testing", "exec", "fs":
		return true
	default:
		return false
	}
}

func mapKeysSorted(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
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
	sort.Strings(out)
	return out
}

func appendIfMissing(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
