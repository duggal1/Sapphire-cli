package memory

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	shortHashAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	shortHashSpace    = 14776336 // 62^4
	rolloutSlugMaxLen = 60
)

// RolloutSummaryFileStemFromParts implements the literal 1:1 filename generation logic from Codex-rs storage.rs.
func RolloutSummaryFileStemFromParts(threadID string, sourceUpdatedAt time.Time, rolloutSlug *string) string {
	timestampFragment := sourceUpdatedAt.Format("2006-01-02T15-04-05")
	var shortHashSeed uint32

	if u, err := uuid.Parse(threadID); err == nil {
		// Literal: timestamp from UUID if possible, but for Sapphire we trust the passed sourceUpdatedAt
		// to avoid complex UUID version/timestamp extraction variance.
		// short_hash_seed = (thread_uuid.as_u128() & 0xFFFF_FFFF) as u32
		bytes := u[:]
		// Get last 4 bytes for the uint32 seed (as_u128() & 0xFFFFFFFF)
		shortHashSeed = uint32(bytes[12])<<24 | uint32(bytes[13])<<16 | uint32(bytes[14])<<8 | uint32(bytes[15])
		
		// Note: Codex-rs uses the UUID timestamp if it can parse it. 
		// For Sapphire, we use the sourceUpdatedAt which is already the "source_updated_at".
	} else {
		// Fallback hash for non-UUID strings
		for i := 0; i < len(threadID); i++ {
			shortHashSeed = shortHashSeed*31 + uint32(threadID[i])
		}
	}

	shortHashValue := shortHashSeed % shortHashSpace
	var shortHashChars [4]byte
	for i := 3; i >= 0; i-- {
		shortHashChars[i] = shortHashAlphabet[shortHashValue%62]
		shortHashValue /= 62
	}
	shortHash := string(shortHashChars[:])

	filePrefix := fmt.Sprintf("%s-%s", timestampFragment, shortHash)

	if rolloutSlug == nil || *rolloutSlug == "" {
		return filePrefix
	}

	// Slug sanitization: alphanumeric to lowercase, everything else to underscore
	slug := strings.ToLower(*rolloutSlug)
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "_")
	slug = strings.Trim(slug, "_")

	if len(slug) > rolloutSlugMaxLen {
		slug = slug[:rolloutSlugMaxLen]
		slug = strings.TrimRight(slug, "_")
	}

	if slug == "" {
		return filePrefix
	}

	return fmt.Sprintf("%s-%s", filePrefix, slug)
}

// FormatRawMemoryEntryHeader returns the literal markdown header for an entry in raw_memories.md.
func FormatRawMemoryEntryHeader(threadID, updatedAtRFC3339, cwd, rolloutPath, rolloutSummaryFile string) string {
	return fmt.Sprintf("## Thread `%s`\nupdated_at: %s\ncwd: %s\nrollout_path: %s\nrollout_summary_file: %s\n\n",
		threadID, updatedAtRFC3339, cwd, rolloutPath, rolloutSummaryFile)
}

// FormatRolloutSummaryHeader returns the literal header for a rollout_summaries/*.md file.
func FormatRolloutSummaryHeader(threadID, updatedAtRFC3339, rolloutPath, cwd, gitBranch string) string {
	header := fmt.Sprintf("thread_id: %s\nupdated_at: %s\nrollout_path: %s\ncwd: %s\n",
		threadID, updatedAtRFC3339, rolloutPath, cwd)
	if gitBranch != "" {
		header += fmt.Sprintf("git_branch: %s\n", gitBranch)
	}
	return header + "\n"
}
