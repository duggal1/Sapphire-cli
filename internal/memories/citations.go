package memories

import (
	"regexp"
	"strings"
)

type Citation struct {
	Path string
	Note string
}

var rolloutIDPattern = regexp.MustCompile(`(?m)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)

func ExtractRolloutIDs(text string) []string {
	matches := rolloutIDPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		ids = append(ids, match)
	}
	return ids
}

func BuildCitationBlock(entries []Citation, rolloutIDs []string) string {
	var sb strings.Builder
	sb.WriteString("<oai-mem-citation>\n<citation_entries>\n")
	for _, entry := range entries {
		if entry.Path == "" {
			continue
		}
		sb.WriteString(entry.Path)
		if entry.Note != "" {
			sb.WriteString("|note=[")
			sb.WriteString(entry.Note)
			sb.WriteString("]")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("</citation_entries>\n<rollout_ids>\n")
	for _, id := range rolloutIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		sb.WriteString(id)
		sb.WriteString("\n")
	}
	sb.WriteString("</rollout_ids>\n</oai-mem-citation>")
	return sb.String()
}
