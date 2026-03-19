package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/message"
)

const subAgentMetadataTag = "subagent_metadata"

type subAgentMetadata struct {
	AssignmentID     string   `json:"assignment_id,omitempty"`
	WorktreePath     string   `json:"worktree_path,omitempty"`
	Branch           string   `json:"branch,omitempty"`
	WriteManifest    []string `json:"write_manifest,omitempty"`
	DefinitionOfDone string   `json:"definition_of_done,omitempty"`
	TestCommand      string   `json:"test_command,omitempty"`
}

func (c *coordinator) recordSubAgentMetadata(ctx context.Context, sessionID string, meta subAgentMetadata) error {
	if sessionID == "" {
		return nil
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encode sub-agent metadata: %w", err)
	}
	text := fmt.Sprintf("<%s>%s</%s>", subAgentMetadataTag, payload, subAgentMetadataTag)
	_, err = c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.System,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	})
	if err != nil {
		return fmt.Errorf("store sub-agent metadata: %w", err)
	}
	return nil
}

func (c *coordinator) loadSubAgentMetadata(ctx context.Context, sessionID string) (subAgentMetadata, bool) {
	var meta subAgentMetadata
	if sessionID == "" {
		return meta, false
	}
	msgs, err := c.messages.List(ctx, sessionID)
	if err != nil {
		return meta, false
	}
	for _, msg := range msgs {
		if msg.Role != message.System {
			continue
		}
		for _, part := range msg.Parts {
			textPart, ok := part.(message.TextContent)
			if !ok {
				continue
			}
			if payload, ok := extractTaggedPayload(textPart.Text, subAgentMetadataTag); ok {
				if err := json.Unmarshal([]byte(payload), &meta); err == nil {
					return meta, true
				}
			}
		}
	}
	return meta, false
}

func extractTaggedPayload(text, tag string) (string, bool) {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	start := strings.Index(text, open)
	if start == -1 {
		return "", false
	}
	end := strings.Index(text, close)
	if end == -1 || end <= start {
		return "", false
	}
	start += len(open)
	return strings.TrimSpace(text[start:end]), true
}

func normalizeWriteManifest(baseRoot, worktreeRoot string, manifest []string) []string {
	if manifest == nil {
		return nil
	}
	baseRoot = filepath.Clean(baseRoot)
	worktreeRoot = filepath.Clean(worktreeRoot)
	out := make([]string, 0, len(manifest))
	for _, entry := range manifest {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if filepath.IsAbs(entry) && baseRoot != "" && worktreeRoot != "" {
			if rel, err := filepath.Rel(baseRoot, entry); err == nil && !strings.HasPrefix(rel, "..") {
				entry = filepath.Join(worktreeRoot, rel)
			}
		}
		out = append(out, entry)
	}
	return out
}
