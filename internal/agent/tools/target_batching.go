package tools

import "strings"

func normalizeBatchTargets(primary string, many []string, fallback string) []string {
	raw := make([]string, 0, len(many)+1)
	if trimmed := strings.TrimSpace(primary); trimmed != "" {
		raw = append(raw, trimmed)
	}
	for _, value := range many {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			raw = append(raw, trimmed)
		}
	}
	if len(raw) == 0 {
		if trimmed := strings.TrimSpace(fallback); trimmed != "" {
			raw = append(raw, trimmed)
		}
	}
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	targets := make([]string, 0, len(raw))
	for _, value := range raw {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		targets = append(targets, value)
	}
	return targets
}

func boundedParallelism(count, limit int) int {
	if count <= 0 {
		return 1
	}
	if limit <= 0 {
		limit = 1
	}
	if count < limit {
		return count
	}
	return limit
}
