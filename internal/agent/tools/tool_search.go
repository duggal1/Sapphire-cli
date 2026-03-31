package tools

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/fantasy"
)

const ToolSearchToolName = "tool_search"

const (
	defaultToolSearchLimit      = 8
	maxToolSearchLimit          = 20
	maxToolSearchIndexedQueries = 4
	maxToolSearchFileQueries    = 4
	maxToolSearchTextQueries    = 3
)

type ToolSearchParams struct {
	Query   string   `json:"query" description:"Natural-language, symbol-name, or filename query used to locate the exact code file or function"`
	Path    string   `json:"path,omitempty" description:"Optional root directory to restrict the search"`
	Paths   []string `json:"paths,omitempty" description:"Optional list of roots to search in one parallel call"`
	Include string   `json:"include,omitempty" description:"Optional file glob for text fallback searches"`
	Limit   int      `json:"limit,omitempty" description:"Maximum number of ranked results to return. Defaults to 8 and is capped at 20."`
}

type ToolSearchIndexedLookup func(context.Context, string, int) (ToolSearchIndexedResult, error)

type ToolSearchIndexedResult struct {
	Available bool
	Message   string
	Matches   []ToolSearchIndexedMatch
}

type ToolSearchIndexedMatch struct {
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

//go:embed tool_search.md
var toolSearchToolDescription []byte

var toolSearchStopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "but": {},
	"broken": {}, "bug": {}, "bugs": {}, "by": {}, "change": {}, "code": {}, "codebase": {},
	"error": {}, "errors": {}, "exact": {}, "failing": {}, "failure": {}, "file": {},
	"files": {}, "find": {}, "fix": {}, "fixed": {}, "for": {}, "from": {}, "function": {},
	"functions": {}, "help": {}, "i": {}, "implement": {}, "in": {}, "into": {}, "is": {},
	"issue": {}, "issues": {}, "it": {}, "locate": {}, "me": {}, "need": {}, "of": {},
	"on": {}, "or": {}, "please": {}, "repo": {}, "repository": {}, "search": {},
	"symbol": {}, "symbols": {}, "task": {}, "that": {}, "the": {}, "this": {}, "to": {},
	"update": {}, "use": {}, "want": {}, "with": {}, "without": {}, "you": {},
}

type toolSearchQueryPlan struct {
	All     []string
	Indexed []string
	Files   []string
	Text    []string
}

type toolSearchCandidate struct {
	Path         string
	IndexedScore int
	FileScore    int
	TextScore    int
	IndexedHits  int
	TextHits     int
	HasIndexed   bool
	BestIndexed  ToolSearchIndexedMatch
	HasText      bool
	BestText     grepMatch
}

func NewToolSearchTool(description, workingDir string, indexedLookup ToolSearchIndexedLookup) fantasy.AgentTool {
	if strings.TrimSpace(description) == "" {
		description = string(toolSearchToolDescription)
	}
	return fantasy.NewParallelAgentTool(
		ToolSearchToolName,
		description,
		func(ctx context.Context, params ToolSearchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}
			query := strings.TrimSpace(params.Query)
			if query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}
			limit := params.Limit
			if limit <= 0 {
				limit = defaultToolSearchLimit
			}
			if limit > maxToolSearchLimit {
				limit = maxToolSearchLimit
			}
			searchPaths := normalizeBatchTargets(params.Path, params.Paths, workingDir)
			if len(searchPaths) == 0 {
				searchPaths = []string{workingDir}
			}
			plan := buildToolSearchQueryPlan(query)
			stageLimit := minInt(maxToolSearchLimit, maxInt(limit*2, defaultToolSearchLimit))

			candidates := make(map[string]*toolSearchCandidate)
			indexMessages := make([]string, 0, 2)
			errors := make([]string, 0)
			fileTruncated := false
			textTruncated := false
			stopReason := "query variants exhausted"

			if indexedLookup != nil {
				for _, variant := range plan.Indexed {
					indexedResult, err := indexedLookup(ctx, variant, stageLimit)
					if err != nil {
						indexMessages = append(indexMessages, "index error: "+err.Error())
						continue
					}
					if strings.TrimSpace(indexedResult.Message) != "" {
						indexMessages = append(indexMessages, indexedResult.Message)
					}
					if !indexedResult.Available {
						break
					}
					filtered := filterIndexedMatchesToRoots(indexedResult.Matches, searchPaths)
					addIndexedToolSearchCandidates(candidates, filtered)
					if shouldStopToolSearchAfterLocatorStage(candidates, limit) {
						stopReason = "strong indexed candidates found; skipped broader fallback stages"
						break
					}
				}
			} else {
				indexMessages = append(indexMessages, "Durable codebase graph is not configured.")
			}

			if !shouldStopToolSearchAfterLocatorStage(candidates, limit) {
				for _, variant := range plan.Files {
					fileResults, truncated, fileErrors := rgFilesAcrossRoots(ctx, variant, searchPaths, stageLimit)
					fileTruncated = fileTruncated || truncated
					errors = append(errors, fileErrors...)
					addFilenameToolSearchCandidates(candidates, fileResults)
					if shouldStopToolSearchAfterLocatorStage(candidates, limit) {
						stopReason = "strong indexed/path candidates found; skipped text fallback"
						break
					}
				}
			}

			if !shouldStopToolSearchAfterLocatorStage(candidates, limit) {
				for _, variant := range plan.Text {
					textMatches, truncated, textErrors := rgSearchAcrossRoots(ctx, RGParams{
						Pattern:     variant,
						Paths:       searchPaths,
						Include:     params.Include,
						LiteralText: true,
						Limit:       stageLimit,
					}, searchPaths, stageLimit)
					textTruncated = textTruncated || truncated
					errors = append(errors, textErrors...)
					addTextToolSearchCandidates(candidates, textMatches)
					if shouldStopToolSearchAfterTextStage(candidates, limit) {
						break
					}
				}
				if len(candidates) > 0 && stopReason == "query variants exhausted" {
					stopReason = "text fallback used to resolve remaining ambiguity"
				}
			}

			ranked := rankToolSearchCandidates(candidates, limit)
			return fantasy.NewTextResponse(formatToolSearchOutput(query, searchPaths, plan, ranked, uniqueToolSearchStrings(indexMessages), uniqueToolSearchStrings(errors), stopReason, fileTruncated, textTruncated)), nil
		},
	)
}

func filterIndexedMatchesToRoots(matches []ToolSearchIndexedMatch, roots []string) []ToolSearchIndexedMatch {
	if len(roots) == 0 {
		return matches
	}
	filtered := make([]ToolSearchIndexedMatch, 0, len(matches))
	for _, match := range matches {
		if pathWithinRoots(match.Path, roots) {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

func pathWithinRoots(path string, roots []string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	for _, root := range roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		candidates := []string{path}
		if !filepath.IsAbs(path) {
			candidates = append([]string{filepath.Join(absRoot, path)}, candidates...)
		}
		for _, candidate := range candidates {
			absPath, err := filepath.Abs(candidate)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(absRoot, absPath)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

func formatPathSection(title string, paths []string) string {
	var output strings.Builder
	output.WriteString(title + ":")
	for _, path := range paths {
		output.WriteString("\n- " + filepath.ToSlash(path))
	}
	return output.String()
}

func buildToolSearchQueryPlan(query string) toolSearchQueryPlan {
	raw := strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
	if raw == "" {
		return toolSearchQueryPlan{}
	}
	terms := toolSearchMeaningfulTerms(raw)

	var plan toolSearchQueryPlan
	addPlanQuery := func(target *[]string, value string, maxCount int) {
		value = strings.TrimSpace(value)
		if value == "" || len(*target) >= maxCount {
			return
		}
		for _, existing := range *target {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		*target = append(*target, value)
	}

	if looksPreciseToolSearchQuery(raw) {
		addPlanQuery(&plan.Indexed, raw, maxToolSearchIndexedQueries)
		addPlanQuery(&plan.Text, raw, maxToolSearchTextQueries)
	}
	if len(terms) >= 2 {
		pair := terms[0] + " " + terms[1]
		addPlanQuery(&plan.Indexed, pair, maxToolSearchIndexedQueries)
		addPlanQuery(&plan.Text, pair, maxToolSearchTextQueries)
	}
	for _, term := range terms {
		addPlanQuery(&plan.Indexed, term, maxToolSearchIndexedQueries)
		addPlanQuery(&plan.Files, term, maxToolSearchFileQueries)
		if len(plan.Text) < maxToolSearchTextQueries {
			addPlanQuery(&plan.Text, term, maxToolSearchTextQueries)
		}
	}
	if looksPathLikeToolSearchQuery(raw) {
		addPlanQuery(&plan.Files, raw, maxToolSearchFileQueries)
	}

	if len(plan.Indexed) == 0 {
		addPlanQuery(&plan.Indexed, raw, maxToolSearchIndexedQueries)
	}
	if len(plan.Files) == 0 {
		addPlanQuery(&plan.Files, raw, maxToolSearchFileQueries)
	}
	if len(plan.Text) == 0 {
		addPlanQuery(&plan.Text, raw, maxToolSearchTextQueries)
	}

	plan.All = append(plan.All, plan.Indexed...)
	for _, query := range plan.Files {
		addPlanQuery(&plan.All, query, maxToolSearchIndexedQueries+maxToolSearchFileQueries+maxToolSearchTextQueries)
	}
	for _, query := range plan.Text {
		addPlanQuery(&plan.All, query, maxToolSearchIndexedQueries+maxToolSearchFileQueries+maxToolSearchTextQueries)
	}
	return plan
}

func toolSearchMeaningfulTerms(query string) []string {
	rawTerms := toolSearchScoreTerms(query)
	out := make([]string, 0, len(rawTerms))
	for _, term := range rawTerms {
		if len(term) < 3 {
			continue
		}
		if _, stop := toolSearchStopWords[term]; stop {
			continue
		}
		out = append(out, term)
		if len(out) >= 6 {
			break
		}
	}
	return out
}

func looksPreciseToolSearchQuery(query string) bool {
	if looksPathLikeToolSearchQuery(query) {
		return true
	}
	fields := strings.Fields(query)
	return len(fields) > 0 && len(fields) <= 4
}

func looksPathLikeToolSearchQuery(query string) bool {
	return strings.ContainsAny(query, `/\._-:`)
}

func addIndexedToolSearchCandidates(candidates map[string]*toolSearchCandidate, matches []ToolSearchIndexedMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].Path < matches[j].Path
		}
		return matches[i].Score > matches[j].Score
	})
	for i, match := range matches {
		path := filepath.ToSlash(strings.TrimSpace(match.Path))
		if path == "" {
			continue
		}
		candidate := ensureToolSearchCandidate(candidates, path)
		score := match.Score + maxInt(24, 72-(i*8))
		if score > candidate.IndexedScore {
			candidate.IndexedScore = score
			candidate.BestIndexed = match
			candidate.HasIndexed = true
		}
		candidate.IndexedHits++
	}
}

func addFilenameToolSearchCandidates(candidates map[string]*toolSearchCandidate, paths []string) {
	for i, path := range paths {
		candidate := ensureToolSearchCandidate(candidates, filepath.ToSlash(path))
		score := maxInt(48, 168-(i*12))
		if score > candidate.FileScore {
			candidate.FileScore = score
		}
	}
}

func addTextToolSearchCandidates(candidates map[string]*toolSearchCandidate, matches []grepMatch) {
	for i, match := range matches {
		path := filepath.ToSlash(match.path)
		if path == "" {
			continue
		}
		candidate := ensureToolSearchCandidate(candidates, path)
		score := maxInt(32, 108-(i*8))
		if score > candidate.TextScore {
			candidate.TextScore = score
			candidate.BestText = match
			candidate.HasText = true
		}
		candidate.TextHits++
	}
}

func ensureToolSearchCandidate(candidates map[string]*toolSearchCandidate, path string) *toolSearchCandidate {
	if existing, ok := candidates[path]; ok {
		return existing
	}
	candidate := &toolSearchCandidate{Path: path}
	candidates[path] = candidate
	return candidate
}

func shouldStopToolSearchAfterLocatorStage(candidates map[string]*toolSearchCandidate, limit int) bool {
	ranked := rankToolSearchCandidates(candidates, maxInt(5, limit))
	if len(ranked) == 0 {
		return false
	}
	if ranked[0].totalScore() >= 360 {
		return true
	}
	strong := 0
	for _, candidate := range ranked[:minInt(len(ranked), 5)] {
		if candidate.totalScore() >= 260 {
			strong++
		}
	}
	if limit <= 3 {
		return strong >= 2
	}
	return strong >= 3
}

func shouldStopToolSearchAfterTextStage(candidates map[string]*toolSearchCandidate, limit int) bool {
	ranked := rankToolSearchCandidates(candidates, maxInt(5, limit))
	if len(ranked) == 0 {
		return false
	}
	usable := 0
	for _, candidate := range ranked[:minInt(len(ranked), 5)] {
		if candidate.totalScore() >= 180 {
			usable++
		}
	}
	if limit <= 3 {
		return usable >= 2
	}
	return usable >= 3 || ranked[0].totalScore() >= 300
}

func rankToolSearchCandidates(candidates map[string]*toolSearchCandidate, limit int) []toolSearchCandidate {
	if len(candidates) == 0 {
		return nil
	}
	ranked := make([]toolSearchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		ranked = append(ranked, *candidate)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].totalScore() == ranked[j].totalScore() {
			return ranked[i].Path < ranked[j].Path
		}
		return ranked[i].totalScore() > ranked[j].totalScore()
	})
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func (candidate toolSearchCandidate) totalScore() int {
	score := candidate.IndexedScore + candidate.FileScore + candidate.TextScore
	sources := 0
	if candidate.IndexedScore > 0 {
		sources++
	}
	if candidate.FileScore > 0 {
		sources++
	}
	if candidate.TextScore > 0 {
		sources++
	}
	if sources > 1 {
		score += (sources - 1) * 25
	}
	if candidate.IndexedHits > 1 {
		score += minInt((candidate.IndexedHits-1)*8, 24)
	}
	if candidate.TextHits > 1 {
		score += minInt((candidate.TextHits-1)*4, 12)
	}
	return score
}

func (candidate toolSearchCandidate) location() string {
	location := candidate.Path
	switch {
	case candidate.HasIndexed && candidate.BestIndexed.StartLine > 0:
		location += fmt.Sprintf(":%d", candidate.BestIndexed.StartLine)
		if candidate.BestIndexed.EndLine > candidate.BestIndexed.StartLine {
			location += fmt.Sprintf("-%d", candidate.BestIndexed.EndLine)
		}
	case candidate.HasText && candidate.BestText.lineNum > 0:
		location += fmt.Sprintf(":%d", candidate.BestText.lineNum)
	}
	return location
}

func (candidate toolSearchCandidate) sourceSummary() string {
	sources := make([]string, 0, 3)
	if candidate.HasIndexed {
		label := "indexed"
		if kind := strings.TrimSpace(candidate.BestIndexed.Kind); kind != "" {
			label += ":" + kind
		}
		if name := strings.TrimSpace(candidate.BestIndexed.Name); name != "" {
			label += "(" + name + ")"
		}
		sources = append(sources, label)
	}
	if candidate.FileScore > 0 {
		sources = append(sources, "filename")
	}
	if candidate.TextScore > 0 {
		if candidate.TextHits > 1 {
			sources = append(sources, fmt.Sprintf("text x%d", candidate.TextHits))
		} else {
			sources = append(sources, "text")
		}
	}
	return strings.Join(sources, ", ")
}

func (candidate toolSearchCandidate) snippet() string {
	if candidate.HasIndexed {
		if snippet := strings.TrimSpace(candidate.BestIndexed.Snippet); snippet != "" {
			return trimToolSearchPreview(snippet, 180)
		}
		if signature := strings.TrimSpace(candidate.BestIndexed.Signature); signature != "" {
			return trimToolSearchPreview(signature, 180)
		}
	}
	if candidate.HasText {
		if text := strings.TrimSpace(candidate.BestText.lineText); text != "" {
			return trimToolSearchPreview(text, 180)
		}
	}
	return ""
}

func trimToolSearchPreview(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit]) + "..."
}

func formatToolSearchOutput(query string, searchPaths []string, plan toolSearchQueryPlan, ranked []toolSearchCandidate, indexMessages, errors []string, stopReason string, fileTruncated, textTruncated bool) string {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Tool search results for %q\n", query))
	output.WriteString(fmt.Sprintf("- roots: %d\n", len(searchPaths)))
	if len(plan.All) > 0 {
		output.WriteString("- query_variants: " + strings.Join(plan.All, " | ") + "\n")
	}
	if len(indexMessages) > 0 {
		output.WriteString("- index: " + strings.Join(indexMessages, " | ") + "\n")
	}
	if strings.TrimSpace(stopReason) != "" {
		output.WriteString("- stop_reason: " + stopReason + "\n")
	}
	if fileTruncated {
		output.WriteString("- filename_search: truncated\n")
	}
	if textTruncated {
		output.WriteString("- text_search: truncated\n")
	}
	if len(ranked) == 0 {
		output.WriteString("No codebase matches found.")
		if len(errors) > 0 {
			output.WriteString("\n\nErrors:\n")
			output.WriteString(strings.Join(errors, "\n"))
		}
		return strings.TrimSpace(output.String())
	}

	output.WriteString("Top candidates:")
	for _, candidate := range ranked {
		output.WriteString(fmt.Sprintf("\n- %s | score=%d | sources=%s", candidate.location(), candidate.totalScore(), candidate.sourceSummary()))
		if snippet := candidate.snippet(); snippet != "" {
			output.WriteString("\n  ")
			output.WriteString(snippet)
		}
	}
	if len(errors) > 0 {
		output.WriteString("\n\nErrors:\n")
		output.WriteString(strings.Join(errors, "\n"))
	}
	return strings.TrimSpace(output.String())
}

func uniqueToolSearchStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
