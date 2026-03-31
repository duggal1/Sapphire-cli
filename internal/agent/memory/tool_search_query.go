package memory

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type ToolSearchMatch struct {
	Kind      string
	Path      string
	Name      string
	StableKey string
	Signature string
	Snippet   string
	Language  string
	Role      string
	Status    string
	StartLine int
	EndLine   int
	SizeBytes int64
	Symbols   int64
	Exported  bool
	Score     int
}

type toolSearchQuerySpec struct {
	Raw    string
	Lower  string
	Exact  []string
	Prefix []string
	Fuzzy  []string
	Terms  []string
}

func (c *Compiler) ToolSearch(ctx context.Context, workingDir, query string, limit int) (IndexStatus, []ToolSearchMatch, error) {
	if c == nil || c.conn == nil || c.q == nil {
		return IndexStatus{}, nil, fmt.Errorf("memory compiler is not initialized")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return IndexStatus{}, nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 12
	}

	status, err := c.IndexStatus(ctx, workingDir)
	if err != nil {
		return IndexStatus{}, nil, err
	}
	if !status.Available {
		return status, nil, fmt.Errorf("durable codebase graph is not available")
	}

	snapshot, err := captureRepoSnapshot(ctx, workingDir)
	if err != nil {
		return status, nil, err
	}
	scope, err := c.loadExistingScope(ctx, snapshot)
	if err != nil {
		return status, nil, err
	}

	spec := buildToolSearchQuerySpec(query)
	rawLimit := min(max(limit*6, 48), 160)

	symbolMatches, err := c.queryToolSearchSymbols(ctx, scope.ID, spec, rawLimit)
	if err != nil {
		return status, nil, err
	}
	fileMatches, err := c.queryToolSearchFiles(ctx, scope.ID, spec, rawLimit)
	if err != nil {
		return status, nil, err
	}

	allMatches := append(symbolMatches, fileMatches...)
	if len(allMatches) == 0 {
		return status, nil, nil
	}

	sort.SliceStable(allMatches, func(i, j int) bool {
		if allMatches[i].Score == allMatches[j].Score {
			if allMatches[i].Path == allMatches[j].Path {
				return allMatches[i].Name < allMatches[j].Name
			}
			return allMatches[i].Path < allMatches[j].Path
		}
		return allMatches[i].Score > allMatches[j].Score
	})

	if len(allMatches) > limit {
		allMatches = allMatches[:limit]
	}
	return status, allMatches, nil
}

func buildToolSearchQuerySpec(query string) toolSearchQuerySpec {
	raw := strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
	if raw == "" {
		return toolSearchQuerySpec{}
	}
	lower := strings.ToLower(raw)
	normalizedPath := filepath.ToSlash(strings.TrimPrefix(raw, "./"))
	base := filepath.Base(normalizedPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	terms := scoreTerms(lower)

	exact := uniqueToolSearchValues(raw, normalizedPath, base)
	if stem != "" && stem != base {
		exact = uniqueToolSearchValues(append(exact, stem)...)
	}
	if len(terms) == 1 {
		exact = uniqueToolSearchValues(append(exact, terms[0])...)
	}

	prefix := uniqueToolSearchValues(exact...)
	if len(terms) >= 2 {
		prefix = uniqueToolSearchValues(append(prefix, terms[0]+" "+terms[1])...)
	}
	for _, term := range terms {
		if len(term) < 3 {
			continue
		}
		prefix = uniqueToolSearchValues(append(prefix, term)...)
		if len(prefix) >= 6 {
			break
		}
	}

	fuzzy := uniqueToolSearchValues(lower)
	if len(terms) >= 2 {
		fuzzy = uniqueToolSearchValues(append(fuzzy, terms[0]+" "+terms[1])...)
	}
	for _, term := range terms {
		if len(term) < 3 {
			continue
		}
		fuzzy = uniqueToolSearchValues(append(fuzzy, term)...)
		if len(fuzzy) >= 4 {
			break
		}
	}

	return toolSearchQuerySpec{
		Raw:    raw,
		Lower:  lower,
		Exact:  exact,
		Prefix: prefix,
		Fuzzy:  fuzzy,
		Terms:  terms,
	}
}

func uniqueToolSearchValues(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(filepath.ToSlash(value))
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (c *Compiler) queryToolSearchSymbols(ctx context.Context, scopeID string, spec toolSearchQuerySpec, limit int) ([]ToolSearchMatch, error) {
	if limit <= 0 {
		limit = 12
	}
	results := make([]ToolSearchMatch, 0, min(limit, 48))
	seen := make(map[string]struct{}, limit*2)
	stages := []struct {
		name   string
		values []string
		limit  int
		build  func([]string) (string, []any)
	}{
		{name: "exact", values: spec.Exact, limit: min(max(limit, 10), 32), build: buildExactToolSearchSymbolClause},
		{name: "prefix", values: spec.Prefix, limit: min(max(limit, 14), 40), build: buildPrefixToolSearchSymbolClause},
		{name: "fuzzy", values: spec.Fuzzy, limit: min(max(limit*2, 24), 80), build: buildFuzzyToolSearchSymbolClause},
	}
	for _, stage := range stages {
		clause, args := stage.build(stage.values)
		if clause == "" {
			continue
		}
		stageMatches, err := c.queryToolSearchSymbolStage(ctx, scopeID, clause, args, stage.limit, spec)
		if err != nil {
			return nil, err
		}
		results = appendToolSearchMatches(results, seen, stageMatches, limit)
		if stage.name != "fuzzy" && len(results) >= max(limit, 8) {
			break
		}
	}
	sortToolSearchMatches(results)
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (c *Compiler) queryToolSearchSymbolStage(ctx context.Context, scopeID, clause string, args []any, limit int, spec toolSearchQuerySpec) ([]ToolSearchMatch, error) {
	rows, err := c.conn.QueryContext(ctx, `
		SELECT
			f.path,
			COALESCE(f.language, ''),
			COALESCE(f.role, ''),
			COALESCE(f.status, ''),
			COALESCE(f.size_bytes, 0),
			COALESCE(f.symbol_count, 0),
			COALESCE(s.name, ''),
			COALESCE(s.kind, ''),
			COALESCE(s.stable_key, ''),
			COALESCE(s.signature, ''),
			COALESCE(s.doc, ''),
			COALESCE(s.start_line, 0),
			COALESCE(s.end_line, 0),
			COALESCE(s.exported, 0),
			COALESCE(s.status, '')
		FROM memory_repo_symbols s
		JOIN memory_repo_files f ON f.id = s.file_id
		WHERE s.scope_id = ?
			AND (`+clause+`)
		LIMIT ?`,
		append(append([]any{scopeID}, args...), limit)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]ToolSearchMatch, 0)
	seen := map[string]struct{}{}
	for rows.Next() {
		var path string
		var language string
		var role string
		var status string
		var sizeBytes int64
		var symbols int64
		var name string
		var kind string
		var stableKey string
		var signature string
		var doc string
		var startLine int
		var endLine int
		var exportedInt int
		var symbolStatus string
		if err := rows.Scan(&path, &language, &role, &status, &sizeBytes, &symbols, &name, &kind, &stableKey, &signature, &doc, &startLine, &endLine, &exportedInt, &symbolStatus); err != nil {
			return nil, err
		}
		match := ToolSearchMatch{
			Kind:      defaultToolSearchKind(kind, "symbol"),
			Path:      filepath.ToSlash(strings.TrimSpace(path)),
			Name:      strings.TrimSpace(name),
			StableKey: strings.TrimSpace(stableKey),
			Signature: strings.TrimSpace(signature),
			Snippet:   trimPreview(doc, 180),
			Language:  strings.TrimSpace(language),
			Role:      strings.TrimSpace(role),
			Status:    defaultToolSearchKind(strings.TrimSpace(symbolStatus), strings.TrimSpace(status)),
			StartLine: startLine,
			EndLine:   endLine,
			SizeBytes: sizeBytes,
			Symbols:   symbols,
			Exported:  exportedInt > 0,
		}
		match.Score = scoreToolSearchMatch(spec, match)
		key := strings.Join([]string{match.Kind, match.Path, match.Name, match.StableKey, match.Signature}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, match)
	}
	sortToolSearchMatches(results)
	return results, rows.Err()
}

func buildExactToolSearchSymbolClause(values []string) (string, []any) {
	return buildToolSearchClause(values, func(value string) []toolSearchPredicate {
		preds := []toolSearchPredicate{
			{sql: "s.name = ?", arg: value},
			{sql: "s.stable_key = ?", arg: value},
		}
		if looksToolSearchPathValue(value) {
			preds = append(preds, toolSearchPredicate{sql: "f.path = ?", arg: filepath.ToSlash(strings.TrimPrefix(value, "./"))})
		}
		return preds
	})
}

func buildPrefixToolSearchSymbolClause(values []string) (string, []any) {
	return buildToolSearchClause(values, func(value string) []toolSearchPredicate {
		if len(strings.TrimSpace(value)) < 3 {
			return nil
		}
		pattern := escapeLike(value) + "%"
		preds := []toolSearchPredicate{
			{sql: "s.name LIKE ? ESCAPE '\\'", arg: pattern},
			{sql: "s.stable_key LIKE ? ESCAPE '\\'", arg: pattern},
		}
		if looksToolSearchPathValue(value) {
			preds = append(preds, toolSearchPredicate{sql: "f.path LIKE ? ESCAPE '\\'", arg: pattern})
		}
		return preds
	})
}

func buildFuzzyToolSearchSymbolClause(values []string) (string, []any) {
	return buildToolSearchClause(values, func(value string) []toolSearchPredicate {
		if len(strings.TrimSpace(value)) < 3 {
			return nil
		}
		like := "%" + escapeLike(strings.ToLower(value)) + "%"
		return []toolSearchPredicate{
			{sql: "lower(s.name) LIKE ? ESCAPE '\\'", arg: like},
			{sql: "lower(s.stable_key) LIKE ? ESCAPE '\\'", arg: like},
			{sql: "lower(s.signature) LIKE ? ESCAPE '\\'", arg: like},
			{sql: "lower(s.doc) LIKE ? ESCAPE '\\'", arg: like},
			{sql: "lower(f.path) LIKE ? ESCAPE '\\'", arg: like},
		}
	})
}

func (c *Compiler) queryToolSearchFiles(ctx context.Context, scopeID string, spec toolSearchQuerySpec, limit int) ([]ToolSearchMatch, error) {
	if limit <= 0 {
		limit = 12
	}
	results := make([]ToolSearchMatch, 0, min(limit, 48))
	seen := make(map[string]struct{}, limit*2)
	stages := []struct {
		name   string
		values []string
		limit  int
		build  func([]string) (string, []any)
	}{
		{name: "exact", values: spec.Exact, limit: min(max(limit, 10), 32), build: buildExactToolSearchFileClause},
		{name: "prefix", values: spec.Prefix, limit: min(max(limit, 14), 40), build: buildPrefixToolSearchFileClause},
		{name: "fuzzy", values: spec.Fuzzy, limit: min(max(limit*2, 24), 80), build: buildFuzzyToolSearchFileClause},
	}
	for _, stage := range stages {
		clause, args := stage.build(stage.values)
		if clause == "" {
			continue
		}
		stageMatches, err := c.queryToolSearchFileStage(ctx, scopeID, clause, args, stage.limit, spec)
		if err != nil {
			return nil, err
		}
		results = appendToolSearchMatches(results, seen, stageMatches, limit)
		if stage.name != "fuzzy" && len(results) >= max(limit, 8) {
			break
		}
	}
	sortToolSearchMatches(results)
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (c *Compiler) queryToolSearchFileStage(ctx context.Context, scopeID, clause string, args []any, limit int, spec toolSearchQuerySpec) ([]ToolSearchMatch, error) {
	rows, err := c.conn.QueryContext(ctx, `
		SELECT
			COALESCE(path, ''),
			COALESCE(language, ''),
			COALESCE(role, ''),
			COALESCE(status, ''),
			COALESCE(size_bytes, 0),
			COALESCE(symbol_count, 0)
		FROM memory_repo_files
		WHERE scope_id = ?
			AND (`+clause+`)
		LIMIT ?`,
		append(append([]any{scopeID}, args...), limit)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]ToolSearchMatch, 0)
	seen := map[string]struct{}{}
	for rows.Next() {
		var path string
		var language string
		var role string
		var status string
		var sizeBytes int64
		var symbolCount int64
		if err := rows.Scan(&path, &language, &role, &status, &sizeBytes, &symbolCount); err != nil {
			return nil, err
		}
		match := ToolSearchMatch{
			Kind:      "file",
			Path:      filepath.ToSlash(strings.TrimSpace(path)),
			Snippet:   trimPreview(fmt.Sprintf("role=%s status=%s symbols=%d", strings.TrimSpace(role), strings.TrimSpace(status), symbolCount), 180),
			Language:  strings.TrimSpace(language),
			Role:      strings.TrimSpace(role),
			Status:    strings.TrimSpace(status),
			SizeBytes: sizeBytes,
			Symbols:   symbolCount,
		}
		match.Score = scoreToolSearchMatch(spec, match)
		if _, ok := seen[match.Path]; ok {
			continue
		}
		seen[match.Path] = struct{}{}
		results = append(results, match)
	}
	sortToolSearchMatches(results)
	return results, rows.Err()
}

func buildExactToolSearchFileClause(values []string) (string, []any) {
	return buildToolSearchClause(values, func(value string) []toolSearchPredicate {
		normalized := filepath.ToSlash(strings.TrimPrefix(value, "./"))
		return []toolSearchPredicate{
			{sql: "path = ?", arg: normalized},
			{sql: "language = ?", arg: strings.ToLower(value)},
			{sql: "role = ?", arg: strings.ToLower(value)},
		}
	})
}

func buildPrefixToolSearchFileClause(values []string) (string, []any) {
	return buildToolSearchClause(values, func(value string) []toolSearchPredicate {
		if len(strings.TrimSpace(value)) < 3 {
			return nil
		}
		normalized := filepath.ToSlash(strings.TrimPrefix(value, "./"))
		if !looksToolSearchPathValue(normalized) {
			return nil
		}
		return []toolSearchPredicate{
			{sql: "path LIKE ? ESCAPE '\\'", arg: escapeLike(normalized) + "%"},
		}
	})
}

func buildFuzzyToolSearchFileClause(values []string) (string, []any) {
	return buildToolSearchClause(values, func(value string) []toolSearchPredicate {
		if len(strings.TrimSpace(value)) < 3 {
			return nil
		}
		like := "%" + escapeLike(strings.ToLower(value)) + "%"
		return []toolSearchPredicate{
			{sql: "lower(path) LIKE ? ESCAPE '\\'", arg: like},
			{sql: "lower(language) LIKE ? ESCAPE '\\'", arg: like},
			{sql: "lower(role) LIKE ? ESCAPE '\\'", arg: like},
		}
	})
}

type toolSearchPredicate struct {
	sql string
	arg any
}

func buildToolSearchClause(values []string, builder func(string) []toolSearchPredicate) (string, []any) {
	clauses := make([]string, 0, len(values)*3)
	args := make([]any, 0, len(values)*3)
	for _, value := range values {
		for _, predicate := range builder(value) {
			if strings.TrimSpace(predicate.sql) == "" {
				continue
			}
			clauses = append(clauses, predicate.sql)
			args = append(args, predicate.arg)
		}
	}
	return strings.Join(clauses, " OR "), args
}

func appendToolSearchMatches(results []ToolSearchMatch, seen map[string]struct{}, items []ToolSearchMatch, limit int) []ToolSearchMatch {
	for _, item := range items {
		key := strings.Join([]string{item.Kind, item.Path, item.Name, item.StableKey, item.Signature}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, item)
		if len(results) >= max(limit*3, 24) {
			break
		}
	}
	return results
}

func sortToolSearchMatches(matches []ToolSearchMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			if matches[i].Path == matches[j].Path {
				if matches[i].Name == matches[j].Name {
					return matches[i].Kind < matches[j].Kind
				}
				return matches[i].Name < matches[j].Name
			}
			return matches[i].Path < matches[j].Path
		}
		return matches[i].Score > matches[j].Score
	})
}

func looksToolSearchPathValue(value string) bool {
	return strings.ContainsAny(value, `/\._-:`)
}

func escapeLike(input string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(strings.TrimSpace(input))
}

func scoreTerms(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		default:
			return true
		}
	})
	out := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len(field) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}

func scoreToolSearchMatch(spec toolSearchQuerySpec, match ToolSearchMatch) int {
	query := spec.Lower
	terms := spec.Terms
	pathLower := strings.ToLower(match.Path)
	nameLower := strings.ToLower(match.Name)
	stableKeyLower := strings.ToLower(match.StableKey)
	signatureLower := strings.ToLower(match.Signature)
	snippetLower := strings.ToLower(match.Snippet)
	baseLower := strings.ToLower(filepath.Base(match.Path))
	stemLower := strings.TrimSuffix(baseLower, filepath.Ext(baseLower))
	compactQuery := compactToolSearchToken(query)
	compactName := compactToolSearchToken(nameLower)
	compactStem := compactToolSearchToken(stemLower)

	score := 0
	switch {
	case nameLower != "" && nameLower == query:
		score += 320
	case stableKeyLower != "" && stableKeyLower == query:
		score += 300
	case compactName != "" && compactName == compactQuery:
		score += 290
	case compactStem != "" && compactStem == compactQuery:
		score += 272
	case stemLower == query:
		score += 280
	case baseLower == query:
		score += 260
	case pathLower == query:
		score += 240
	}
	if nameLower != "" && strings.HasPrefix(nameLower, query) {
		score += 180
	}
	if stableKeyLower != "" && strings.HasPrefix(stableKeyLower, query) {
		score += 160
	}
	if strings.HasPrefix(stemLower, query) {
		score += 150
	}
	if strings.HasPrefix(baseLower, query) {
		score += 132
	}
	if strings.HasPrefix(pathLower, query) {
		score += 120
	}
	if nameLower != "" && strings.Contains(nameLower, query) {
		score += 120
	}
	if stableKeyLower != "" && strings.Contains(stableKeyLower, query) {
		score += 110
	}
	if compactName != "" && compactQuery != "" && strings.Contains(compactName, compactQuery) {
		score += 132
	}
	if compactStem != "" && compactQuery != "" && strings.Contains(compactStem, compactQuery) {
		score += 116
	}
	if strings.Contains(baseLower, query) {
		score += 110
	}
	if strings.Contains(pathLower, query) {
		score += 90
	}
	if signatureLower != "" && strings.Contains(signatureLower, query) {
		score += 80
	}
	if snippetLower != "" && strings.Contains(snippetLower, query) {
		score += 50
	}

	coveredTerms := 0
	for _, term := range terms {
		if term == "" {
			continue
		}
		matched := false
		switch {
		case nameLower != "" && nameLower == term:
			score += 70
			matched = true
		case stableKeyLower != "" && strings.Contains(stableKeyLower, term):
			score += 44
			matched = true
		case nameLower != "" && strings.Contains(nameLower, term):
			score += 40
			matched = true
		case strings.Contains(stemLower, term):
			score += 38
			matched = true
		case containsToolSearchPathSegment(pathLower, term):
			score += 34
			matched = true
		case strings.Contains(baseLower, term):
			score += 32
			matched = true
		case strings.Contains(pathLower, term):
			score += 24
			matched = true
		case signatureLower != "" && strings.Contains(signatureLower, term):
			score += 16
			matched = true
		case snippetLower != "" && strings.Contains(snippetLower, term):
			score += 8
			matched = true
		}
		if matched {
			coveredTerms++
		}
	}
	score += coveredTerms * 18
	if coveredTerms > 1 && coveredTerms == len(terms) {
		score += 36
	}

	if match.Kind == "file" {
		score += 10
	} else {
		score += 22
	}
	if match.Exported {
		score += 6
	}
	if match.StartLine > 0 {
		score += 4
	}
	if match.Symbols > 0 {
		score += min(int(match.Symbols), 12)
	}
	switch match.Role {
	case "entrypoint", "orchestration", "ui", "code":
		score += 10
	case "test":
		if toolSearchQueryNeedsTestAssets(terms) {
			score += 20
		} else {
			score -= 10
		}
	case "config":
		if toolSearchQueryNeedsConfigAssets(terms) {
			score += 18
		} else {
			score -= 8
		}
	case "docs":
		if toolSearchQueryNeedsDocs(terms) {
			score += 12
		} else {
			score -= 10
		}
	}
	if strings.EqualFold(match.Status, "deprecated") {
		score -= 28
	}
	score -= toolSearchPathNoisePenalty(pathLower, terms)
	score -= toolSearchFileSizePenalty(match.SizeBytes)
	score -= min(max(strings.Count(pathLower, "/")-8, 0)*3, 18)
	if score < 0 {
		return 0
	}
	return score
}

func containsToolSearchPathSegment(path, term string) bool {
	if term == "" {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		stem := strings.TrimSuffix(segment, filepath.Ext(segment))
		if segment == term || strings.Contains(segment, term) || stem == term || strings.Contains(stem, term) {
			return true
		}
	}
	return false
}

func compactToolSearchToken(value string) string {
	if value == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func toolSearchQueryNeedsTestAssets(terms []string) bool {
	return toolSearchQueryHasAnyTerm(terms, "test", "tests", "spec", "coverage", "mock", "fixture")
}

func toolSearchQueryNeedsConfigAssets(terms []string) bool {
	return toolSearchQueryHasAnyTerm(terms, "config", "configs", "configuration", "settings", "yaml", "yml", "json", "toml", "env")
}

func toolSearchQueryNeedsDocs(terms []string) bool {
	return toolSearchQueryHasAnyTerm(terms, "docs", "doc", "readme", "guide", "policy", "agents")
}

func toolSearchQueryHasAnyTerm(terms []string, candidates ...string) bool {
	if len(terms) == 0 || len(candidates) == 0 {
		return false
	}
	wanted := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		wanted[candidate] = struct{}{}
	}
	for _, term := range terms {
		if _, ok := wanted[term]; ok {
			return true
		}
	}
	return false
}

func toolSearchPathNoisePenalty(path string, terms []string) int {
	switch {
	case strings.Contains(path, "/node_modules/"), strings.Contains(path, "/vendor/"), strings.Contains(path, "/third_party/"):
		return 80
	case strings.Contains(path, "/dist/"), strings.Contains(path, "/build/"), strings.Contains(path, "/out/"), strings.Contains(path, "/coverage/"), strings.Contains(path, "/target/"), strings.Contains(path, "/.next/"), strings.Contains(path, "/.turbo/"):
		return 44
	case strings.Contains(path, ".pb.go"), strings.Contains(path, "/generated/"), strings.Contains(path, "_generated"), strings.Contains(path, ".gen."), strings.Contains(path, "/gen/"):
		if toolSearchQueryHasAnyTerm(terms, "generated", "generator", "proto", "protobuf", "pb", "codegen") {
			return 8
		}
		return 28
	case strings.Contains(path, "/testdata/"), strings.Contains(path, "/fixtures/"), strings.Contains(path, "/fixture/"), strings.Contains(path, "/mock/"), strings.Contains(path, "/mocks/"):
		if toolSearchQueryNeedsTestAssets(terms) {
			return 4
		}
		return 18
	default:
		return 0
	}
}

func toolSearchFileSizePenalty(sizeBytes int64) int {
	switch {
	case sizeBytes > 2*1024*1024:
		return 36
	case sizeBytes > 768*1024:
		return 24
	case sizeBytes > 256*1024:
		return 12
	default:
		return 0
	}
}

func trimPreview(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit]) + "..."
}

func defaultToolSearchKind(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

var _ = sql.ErrNoRows
