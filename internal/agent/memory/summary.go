package memory

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/message"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

var (
	preferPattern    = regexp.MustCompile(`(?i)\bi prefer\s+([^.!?\n]+)`)
	namePattern      = regexp.MustCompile(`(?i)\bmy name is\s+([^.!?\n]+)`)
	alwaysUsePattern = regexp.MustCompile(`(?i)\balways use\s+([^.!?\n]+)`)
	databasePattern  = regexp.MustCompile(`(?i)\b(?:use|using|go with|let'?s use|we decided on)\s+(postgres(?:ql)?|sqlite|mysql)\b`)
)

func ExtractUserPreferences(messages []message.Message, sessionID string, now time.Time) []orchestrationdb.UserPreference {
	var items []orchestrationdb.UserPreference
	for _, msg := range messages {
		if msg.Role != message.User {
			continue
		}
		text := strings.TrimSpace(msg.Content().Text)
		if text == "" {
			continue
		}
		if match := firstMatch(namePattern, text); match != "" {
			items = append(items, orchestrationdb.UserPreference{
				Key:             "user.name",
				Value:           match,
				Confidence:      "confirmed",
				SourceSessionID: sessionID,
				UpdatedAt:       now,
			})
		}
		if match := firstMatch(preferPattern, text); match != "" {
			items = append(items, orchestrationdb.UserPreference{
				Key:             "preference.general",
				Value:           match,
				Confidence:      "confirmed",
				SourceSessionID: sessionID,
				UpdatedAt:       now,
			})
		}
		if match := firstMatch(alwaysUsePattern, text); match != "" {
			items = append(items, orchestrationdb.UserPreference{
				Key:             "preference.always_use",
				Value:           match,
				Confidence:      "confirmed",
				SourceSessionID: sessionID,
				UpdatedAt:       now,
			})
		}
	}
	return items
}

func ExtractDecisionRecords(messages []message.Message, sessionID, sourceCheckpointID string, now time.Time, memorySvc MemoryService) []orchestrationdb.DecisionRecord {
	var items []orchestrationdb.DecisionRecord
	for _, msg := range messages {
		text := strings.TrimSpace(msg.Content().Text)
		if text == "" {
			continue
		}
		if match := firstMatch(databasePattern, text); match != "" {
			items = append(items, orchestrationdb.DecisionRecord{
				SessionID:          sessionID,
				Category:           "architecture",
				Key:                "database",
				Value:              normalizeDecisionValue(match),
				Confidence:         "confirmed",
				SourceCheckpointID: sourceCheckpointID,
				CreatedAt:          now,
			})
		}
	}
	if memorySvc != nil {
		if summary, err := memorySvc.GetStructuredSummary(context.Background(), sessionID); err == nil && summary != nil {
			for _, decision := range summary.Decisions {
				key := strings.TrimSpace(decision.Symbol)
				if key == "" {
					key = strings.TrimSpace(decision.File)
				}
				if key == "" {
					key = fmt.Sprintf("decision-%d", len(items)+1)
				}
				value := strings.TrimSpace(decision.Decision)
				if value == "" {
					continue
				}
				items = append(items, orchestrationdb.DecisionRecord{
					SessionID:          sessionID,
					Category:           "structured_summary",
					Key:                key,
					Value:              value,
					Confidence:         "confirmed",
					SourceCheckpointID: sourceCheckpointID,
					CreatedAt:          now,
				})
			}
		}
	}
	return dedupeDecisionRecords(items)
}

func firstMatch(pattern *regexp.Regexp, text string) string {
	matches := pattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func normalizeDecisionValue(text string) string {
	text = strings.TrimSpace(strings.ToLower(text))
	switch text {
	case "postgres":
		return "postgresql"
	default:
		return text
	}
}

func dedupeDecisionRecords(items []orchestrationdb.DecisionRecord) []orchestrationdb.DecisionRecord {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]orchestrationdb.DecisionRecord, 0, len(items))
	for _, item := range items {
		key := item.Category + "|" + item.Key + "|" + item.Value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}
