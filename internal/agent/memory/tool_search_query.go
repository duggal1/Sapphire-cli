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
	Signature string
	Snippet   string
	Language  string
	Role      string
	StartLine int
	EndLine   int
	Score     int
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

	like := "%" + escapeLike(strings.ToLower(query)) + "%"
	rawLimit := max(limit*8, 64)

	symbolMatches, err := c.queryToolSearchSymbols(ctx, scope.ID, like, rawLimit)
	if err != nil {
		return status, nil, err
	}
	fileMatches, err := c.queryToolSearchFiles(ctx, scope.ID, like, rawLimit)
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

func (c *Compiler) queryToolSearchSymbols(ctx context.Context, scopeID, like string, limit int) ([]ToolSearchMatch, error) {
	rows, err := c.conn.QueryContext(ctx, `
		SELECT
			f.path,
			COALESCE(f.language, ''),
			COALESCE(f.role, ''),
			COALESCE(s.name, ''),
			COALESCE(s.kind, ''),
			COALESCE(s.signature, ''),
			COALESCE(s.doc, ''),
			COALESCE(s.start_line, 0),
			COALESCE(s.end_line, 0)
		FROM memory_repo_symbols s
		JOIN memory_repo_files f ON f.id = s.file_id
		WHERE s.scope_id = ?
			AND (
				lower(s.name) LIKE ? ESCAPE '\'
				OR lower(s.stable_key) LIKE ? ESCAPE '\'
				OR lower(s.signature) LIKE ? ESCAPE '\'
				OR lower(s.doc) LIKE ? ESCAPE '\'
				OR lower(f.path) LIKE ? ESCAPE '\'
			)
		LIMIT ?`,
		scopeID, like, like, like, like, like, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	query := strings.Trim(strings.ToLower(like), "%")
	terms := scoreTerms(query)
	results := make([]ToolSearchMatch, 0)
	seen := map[string]struct{}{}
	for rows.Next() {
		var path string
		var language string
		var role string
		var name string
		var kind string
		var signature string
		var doc string
		var startLine int
		var endLine int
		if err := rows.Scan(&path, &language, &role, &name, &kind, &signature, &doc, &startLine, &endLine); err != nil {
			return nil, err
		}
		match := ToolSearchMatch{
			Kind:      defaultToolSearchKind(kind, "symbol"),
			Path:      filepath.ToSlash(strings.TrimSpace(path)),
			Name:      strings.TrimSpace(name),
			Signature: strings.TrimSpace(signature),
			Snippet:   trimPreview(doc, 180),
			Language:  strings.TrimSpace(language),
			Role:      strings.TrimSpace(role),
			StartLine: startLine,
			EndLine:   endLine,
		}
		match.Score = scoreToolSearchMatch(query, terms, match)
		key := strings.Join([]string{match.Kind, match.Path, match.Name, match.Signature}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, match)
	}
	return results, rows.Err()
}

func (c *Compiler) queryToolSearchFiles(ctx context.Context, scopeID, like string, limit int) ([]ToolSearchMatch, error) {
	rows, err := c.conn.QueryContext(ctx, `
		SELECT
			COALESCE(path, ''),
			COALESCE(language, ''),
			COALESCE(role, ''),
			COALESCE(status, ''),
			COALESCE(symbol_count, 0)
		FROM memory_repo_files
		WHERE scope_id = ?
			AND (
				lower(path) LIKE ? ESCAPE '\'
				OR lower(language) LIKE ? ESCAPE '\'
				OR lower(role) LIKE ? ESCAPE '\'
			)
		LIMIT ?`,
		scopeID, like, like, like, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	query := strings.Trim(strings.ToLower(like), "%")
	terms := scoreTerms(query)
	results := make([]ToolSearchMatch, 0)
	seen := map[string]struct{}{}
	for rows.Next() {
		var path string
		var language string
		var role string
		var status string
		var symbolCount int64
		if err := rows.Scan(&path, &language, &role, &status, &symbolCount); err != nil {
			return nil, err
		}
		match := ToolSearchMatch{
			Kind:     "file",
			Path:     filepath.ToSlash(strings.TrimSpace(path)),
			Snippet:  trimPreview(fmt.Sprintf("role=%s status=%s symbols=%d", strings.TrimSpace(role), strings.TrimSpace(status), symbolCount), 180),
			Language: strings.TrimSpace(language),
			Role:     strings.TrimSpace(role),
		}
		match.Score = scoreToolSearchMatch(query, terms, match)
		if _, ok := seen[match.Path]; ok {
			continue
		}
		seen[match.Path] = struct{}{}
		results = append(results, match)
	}
	return results, rows.Err()
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

func scoreToolSearchMatch(query string, terms []string, match ToolSearchMatch) int {
	pathLower := strings.ToLower(match.Path)
	nameLower := strings.ToLower(match.Name)
	signatureLower := strings.ToLower(match.Signature)
	snippetLower := strings.ToLower(match.Snippet)
	baseLower := strings.ToLower(filepath.Base(match.Path))

	score := 0
	switch {
	case nameLower != "" && nameLower == query:
		score += 240
	case strings.TrimSuffix(baseLower, filepath.Ext(baseLower)) == query:
		score += 210
	case pathLower == query:
		score += 200
	}
	if nameLower != "" && strings.Contains(nameLower, query) {
		score += 120
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

	for _, term := range terms {
		switch {
		case nameLower != "" && strings.Contains(nameLower, term):
			score += 40
		case strings.Contains(baseLower, term):
			score += 32
		case strings.Contains(pathLower, term):
			score += 24
		case signatureLower != "" && strings.Contains(signatureLower, term):
			score += 16
		case snippetLower != "" && strings.Contains(snippetLower, term):
			score += 8
		}
	}

	if match.Kind == "file" {
		score += 6
	}
	if match.StartLine > 0 {
		score += 4
	}
	return score
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
