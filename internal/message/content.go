package message

import (
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
)

type MessageRole string

const (
	Assistant MessageRole = "assistant"
	User      MessageRole = "user"
	System    MessageRole = "system"
	Tool      MessageRole = "tool"
)

type FinishReason string

const (
	FinishReasonEndTurn          FinishReason = "end_turn"
	FinishReasonMaxTokens        FinishReason = "max_tokens"
	FinishReasonToolUse          FinishReason = "tool_use"
	FinishReasonCanceled         FinishReason = "canceled"
	FinishReasonError            FinishReason = "error"
	FinishReasonPermissionDenied FinishReason = "permission_denied"

	// Should never happen
	FinishReasonUnknown FinishReason = "unknown"
)

type ContentPart interface {
	isPart()
}

type ReasoningContent struct {
	Thinking         string                             `json:"thinking"`
	Signature        string                             `json:"signature"`
	ThoughtSignature string                             `json:"thought_signature"` // Used for google
	ToolID           string                             `json:"tool_id"`           // Used for openrouter google models
	ResponsesData    *openai.ResponsesReasoningMetadata `json:"responses_data"`
	StartedAt        int64                              `json:"started_at,omitempty"`
	FinishedAt       int64                              `json:"finished_at,omitempty"`
}

func (tc ReasoningContent) String() string {
	return tc.Thinking
}
func (ReasoningContent) isPart() {}

type TextContent struct {
	Text string `json:"text"`
}

func (tc TextContent) String() string {
	return tc.Text
}

func (TextContent) isPart() {}

type SkillContextContent struct {
	Skills []string `json:"skills"`
}

func (SkillContextContent) isPart() {}

type ImageURLContent struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

func (iuc ImageURLContent) String() string {
	return iuc.URL
}

func (ImageURLContent) isPart() {}

type BinaryContent struct {
	Path     string
	MIMEType string
	Data     []byte
}

func (bc BinaryContent) String(p catwalk.InferenceProvider) string {
	base64Encoded := base64.StdEncoding.EncodeToString(bc.Data)
	if p == catwalk.InferenceProviderOpenAI {
		return "data:" + bc.MIMEType + ";base64," + base64Encoded
	}
	return base64Encoded
}

func (BinaryContent) isPart() {}

type ToolCall struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Input            string `json:"input"`
	ProviderExecuted bool   `json:"provider_executed"`
	Finished         bool   `json:"finished"`
}

func (ToolCall) isPart() {}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	Data       string `json:"data"`
	MIMEType   string `json:"mime_type"`
	Metadata   string `json:"metadata"`
	IsError    bool   `json:"is_error"`
}

func (ToolResult) isPart() {}

type Finish struct {
	Reason  FinishReason `json:"reason"`
	Time    int64        `json:"time"`
	Message string       `json:"message,omitempty"`
	Details string       `json:"details,omitempty"`
	// Gemini-specific usage metadata
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens,omitempty"`
	ThoughtsTokens   int64 `json:"thoughts_tokens,omitempty"`
	CachedTokens     int64 `json:"cached_tokens,omitempty"`
	// Timing metrics for Gemini
	StartTimeMs     int64   `json:"start_time_ms,omitempty"`
	EndTimeMs       int64   `json:"end_time_ms,omitempty"`
	TokensPerSecond float64 `json:"tokens_per_second,omitempty"`
	AvgLatencyMs    float64 `json:"avg_latency_ms,omitempty"`
	ThinkingEffort  string  `json:"thinking_effort,omitempty"`
}

func (Finish) isPart() {}

type Message struct {
	ID               string
	Role             MessageRole
	SessionID        string
	Parts            []ContentPart
	Model            string
	Provider         string
	CreatedAt        int64
	UpdatedAt        int64
	IsSummaryMessage bool
}

func (m *Message) Content() TextContent {
	for _, part := range m.Parts {
		if c, ok := part.(TextContent); ok {
			return c
		}
	}
	return TextContent{}
}

func (m *Message) ReasoningContent() ReasoningContent {
	for _, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			return c
		}
	}
	return ReasoningContent{}
}

func (m *Message) ImageURLContent() []ImageURLContent {
	imageURLContents := make([]ImageURLContent, 0)
	for _, part := range m.Parts {
		if c, ok := part.(ImageURLContent); ok {
			imageURLContents = append(imageURLContents, c)
		}
	}
	return imageURLContents
}

func (m *Message) SkillContext() *SkillContextContent {
	for _, part := range m.Parts {
		if c, ok := part.(SkillContextContent); ok {
			return &c
		}
	}
	return nil
}

func (m *Message) BinaryContent() []BinaryContent {
	binaryContents := make([]BinaryContent, 0)
	for _, part := range m.Parts {
		if c, ok := part.(BinaryContent); ok {
			binaryContents = append(binaryContents, c)
		}
	}
	return binaryContents
}

func (m *Message) ToolCalls() []ToolCall {
	toolCalls := make([]ToolCall, 0)
	for _, part := range m.Parts {
		if c, ok := part.(ToolCall); ok {
			toolCalls = append(toolCalls, c)
		}
	}
	return toolCalls
}

func (m *Message) ToolResults() []ToolResult {
	toolResults := make([]ToolResult, 0)
	for _, part := range m.Parts {
		if c, ok := part.(ToolResult); ok {
			toolResults = append(toolResults, c)
		}
	}
	return toolResults
}

func (m *Message) IsFinished() bool {
	for _, part := range m.Parts {
		if _, ok := part.(Finish); ok {
			return true
		}
	}
	return false
}

func (m *Message) FinishPart() *Finish {
	for _, part := range m.Parts {
		if c, ok := part.(Finish); ok {
			return &c
		}
	}
	return nil
}

func (m *Message) FinishReason() FinishReason {
	for _, part := range m.Parts {
		if c, ok := part.(Finish); ok {
			return c.Reason
		}
	}
	return ""
}

func (m *Message) IsThinking() bool {
	if m.ReasoningContent().Thinking != "" && m.Content().Text == "" && !m.IsFinished() {
		return true
	}
	return false
}

func (m *Message) AppendContent(delta string) {
	found := false
	for i, part := range m.Parts {
		if c, ok := part.(TextContent); ok {
			m.Parts[i] = TextContent{Text: c.Text + delta}
			found = true
		}
	}
	if !found {
		m.Parts = append(m.Parts, TextContent{Text: delta})
	}
}

func (m *Message) AppendReasoningContent(delta string) {
	found := false
	for i, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			m.Parts[i] = ReasoningContent{
				Thinking:   c.Thinking + delta,
				Signature:  c.Signature,
				StartedAt:  c.StartedAt,
				FinishedAt: c.FinishedAt,
			}
			found = true
		}
	}
	if !found {
		m.Parts = append(m.Parts, ReasoningContent{
			Thinking:  delta,
			StartedAt: time.Now().Unix(),
		})
	}
}

func (m *Message) AppendThoughtSignature(signature string, toolCallID string) {
	for i, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			m.Parts[i] = ReasoningContent{
				Thinking:         c.Thinking,
				ThoughtSignature: c.ThoughtSignature + signature,
				ToolID:           toolCallID,
				Signature:        c.Signature,
				StartedAt:        c.StartedAt,
				FinishedAt:       c.FinishedAt,
			}
			return
		}
	}
	m.Parts = append(m.Parts, ReasoningContent{ThoughtSignature: signature})
}

func (m *Message) AppendReasoningSignature(signature string) {
	for i, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			m.Parts[i] = ReasoningContent{
				Thinking:   c.Thinking,
				Signature:  c.Signature + signature,
				StartedAt:  c.StartedAt,
				FinishedAt: c.FinishedAt,
			}
			return
		}
	}
	m.Parts = append(m.Parts, ReasoningContent{Signature: signature})
}

func (m *Message) SetReasoningResponsesData(data *openai.ResponsesReasoningMetadata) {
	for i, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			m.Parts[i] = ReasoningContent{
				Thinking:      c.Thinking,
				ResponsesData: data,
				StartedAt:     c.StartedAt,
				FinishedAt:    c.FinishedAt,
			}
			return
		}
	}
}

func (m *Message) FinishThinking() {
	for i, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			if c.FinishedAt == 0 {
				m.Parts[i] = ReasoningContent{
					Thinking:   c.Thinking,
					Signature:  c.Signature,
					StartedAt:  c.StartedAt,
					FinishedAt: time.Now().Unix(),
				}
			}
			return
		}
	}
}

func (m *Message) ThinkingDuration() time.Duration {
	reasoning := m.ReasoningContent()
	if reasoning.StartedAt == 0 {
		return 0
	}

	endTime := reasoning.FinishedAt
	if endTime == 0 {
		endTime = time.Now().Unix()
	}

	return time.Duration(endTime-reasoning.StartedAt) * time.Second
}

// GeminiUsageMetadata returns Gemini-specific usage metadata if available.
// Returns nil if the message doesn't contain Gemini usage data.
func (m *Message) GeminiUsageMetadata() map[string]any {
	finish := m.FinishPart()
	if finish.PromptTokens == 0 && finish.CompletionTokens == 0 && finish.TotalTokens == 0 {
		return nil
	}

	metadata := make(map[string]any)
	if finish.PromptTokens > 0 {
		metadata["promptTokens"] = finish.PromptTokens
	}
	if finish.CompletionTokens > 0 {
		metadata["completionTokens"] = finish.CompletionTokens
	}
	if finish.TotalTokens > 0 {
		metadata["totalTokens"] = finish.TotalTokens
	}
	if finish.ThoughtsTokens > 0 {
		metadata["thoughtsTokens"] = finish.ThoughtsTokens
	}
	if finish.CachedTokens > 0 {
		metadata["cachedTokens"] = finish.CachedTokens
	}
	if finish.TokensPerSecond > 0 {
		metadata["tokensPerSecond"] = fmt.Sprintf("%.2f", finish.TokensPerSecond)
	}
	if finish.AvgLatencyMs > 0 {
		metadata["avgLatencyMs"] = fmt.Sprintf("%.0fms", finish.AvgLatencyMs)
	}
	if finish.EndTimeMs > 0 && finish.StartTimeMs > 0 {
		durationMs := finish.EndTimeMs - finish.StartTimeMs
		metadata["totalDuration"] = fmt.Sprintf("%dms", durationMs)
	}

	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func (m *Message) FinishToolCall(toolCallID string) {
	for i, part := range m.Parts {
		if c, ok := part.(ToolCall); ok {
			if c.ID == toolCallID {
				m.Parts[i] = ToolCall{
					ID:       c.ID,
					Name:     c.Name,
					Input:    c.Input,
					Finished: true,
				}
				return
			}
		}
	}
}

func (m *Message) AppendToolCallInput(toolCallID string, inputDelta string) {
	for i, part := range m.Parts {
		if c, ok := part.(ToolCall); ok {
			if c.ID == toolCallID {
				m.Parts[i] = ToolCall{
					ID:       c.ID,
					Name:     c.Name,
					Input:    c.Input + inputDelta,
					Finished: c.Finished,
				}
				return
			}
		}
	}
}

func (m *Message) AddToolCall(tc ToolCall) {
	for i, part := range m.Parts {
		if c, ok := part.(ToolCall); ok {
			if c.ID == tc.ID {
				m.Parts[i] = tc
				return
			}
		}
	}
	m.Parts = append(m.Parts, tc)
}

func (m *Message) SetToolCalls(tc []ToolCall) {
	// remove any existing tool call part it could have multiple
	parts := make([]ContentPart, 0)
	for _, part := range m.Parts {
		if _, ok := part.(ToolCall); ok {
			continue
		}
		parts = append(parts, part)
	}
	m.Parts = parts
	for _, toolCall := range tc {
		m.Parts = append(m.Parts, toolCall)
	}
}

func (m *Message) AddToolResult(tr ToolResult) {
	m.Parts = append(m.Parts, tr)
}

func (m *Message) SetToolResults(tr []ToolResult) {
	for _, toolResult := range tr {
		m.Parts = append(m.Parts, toolResult)
	}
}

// Clone returns a deep copy of the message with an independent Parts slice.
// This prevents race conditions when the message is modified concurrently.
func (m *Message) Clone() Message {
	clone := *m
	clone.Parts = make([]ContentPart, len(m.Parts))
	copy(clone.Parts, m.Parts)
	return clone
}

func (m *Message) AddFinish(reason FinishReason, message, details string) {
	// remove any existing finish part
	for i, part := range m.Parts {
		if _, ok := part.(Finish); ok {
			m.Parts = slices.Delete(m.Parts, i, i+1)
			break
		}
	}

	m.Parts = append(m.Parts, Finish{
		Reason:  reason,
		Message: message,
		Details: details,
		Time:    time.Now().Unix(),
	})
}

// AddFinishWithMetadata adds a finish part with comprehensive usage metadata and performance metrics.
func (m *Message) AddFinishWithMetadata(
	reason FinishReason,
	message, details string,
	promptTokens, completionTokens, totalTokens, thoughtsTokens, cachedTokens int64,
	startTimeMs, endTimeMs int64,
	thinkingEffort string,
) {
	// remove any existing finish part
	for i, part := range m.Parts {
		if _, ok := part.(Finish); ok {
			m.Parts = slices.Delete(m.Parts, i, i+1)
			break
		}
	}

	// Calculate performance metrics
	durationMs := endTimeMs - startTimeMs
	tokensPerSecond := float64(0)
	avgLatencyMs := float64(0)

	if durationMs > 0 {
		// Only calculate tokensPerSecond and avgLatencyMs if there's meaningful output and duration.
		// For very short responses, these metrics can appear skewed due to fixed overhead.
		const minMeaningfulTokens = 5
		if completionTokens >= minMeaningfulTokens {
			tokensPerSecond = float64(completionTokens) / (float64(durationMs) / 1000.0)
			avgLatencyMs = float64(durationMs) / float64(completionTokens)
		}
	}

	m.Parts = append(m.Parts, Finish{
		Reason:           reason,
		Message:          message,
		Details:          details,
		Time:             time.Now().Unix(),
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		ThoughtsTokens:   thoughtsTokens,
		CachedTokens:     cachedTokens,
		StartTimeMs:      startTimeMs,
		EndTimeMs:        endTimeMs,
		TokensPerSecond:  tokensPerSecond,
		AvgLatencyMs:     avgLatencyMs,
		ThinkingEffort:   thinkingEffort,
	})
}

// AddFinishWithGeminiMetadata adds a finish part with Gemini usage metadata.
// This captures token counts, timing, and performance metrics from Gemini API responses.
func (m *Message) AddFinishWithGeminiMetadata(
	reason FinishReason,
	message, details string,
	promptTokens, completionTokens, totalTokens, thoughtsTokens, cachedTokens int64,
	startTimeMs, endTimeMs int64,
	thinkingEffort string,
) {
	m.AddFinishWithMetadata(reason, message, details, promptTokens, completionTokens, totalTokens, thoughtsTokens, cachedTokens, startTimeMs, endTimeMs, thinkingEffort)
}

func (m *Message) AddImageURL(url, detail string) {
	m.Parts = append(m.Parts, ImageURLContent{URL: url, Detail: detail})
}

func (m *Message) AddBinary(mimeType string, data []byte) {
	m.Parts = append(m.Parts, BinaryContent{MIMEType: mimeType, Data: data})
}

func (m *Message) SetSkillContext(skills []string) {
	parts := make([]ContentPart, 0, len(m.Parts)+1)
	for _, part := range m.Parts {
		if _, ok := part.(SkillContextContent); ok {
			continue
		}
		parts = append(parts, part)
	}
	if len(skills) > 0 {
		parts = append(parts, SkillContextContent{Skills: skills})
	}
	m.Parts = parts
}

func PromptWithTextAttachments(prompt string, attachments []Attachment) string {
	var sb strings.Builder
	sb.WriteString(prompt)
	addedAttachments := false
	for _, content := range attachments {
		if !content.IsText() {
			continue
		}
		if !addedAttachments {
			sb.WriteString("\n<system_info>The files below have been attached by the user, consider them in your response</system_info>\n")
			addedAttachments = true
		}
		if content.FilePath != "" {
			fmt.Fprintf(&sb, "<file path='%s'>\n", content.FilePath)
		} else {
			sb.WriteString("<file>\n")
		}
		sb.WriteString("\n")
		sb.Write(content.Content)
		sb.WriteString("\n</file>\n")
	}
	return sb.String()
}

func (m *Message) ToAIMessage() []fantasy.Message {
	var messages []fantasy.Message
	switch m.Role {
	case User:
		var parts []fantasy.MessagePart
		text := strings.TrimSpace(m.Content().Text)
		var textAttachments []Attachment
		for _, content := range m.BinaryContent() {
			if !strings.HasPrefix(content.MIMEType, "text/") {
				continue
			}
			textAttachments = append(textAttachments, Attachment{
				FilePath: content.Path,
				MimeType: content.MIMEType,
				Content:  content.Data,
			})
		}
		text = PromptWithTextAttachments(text, textAttachments)
		if text != "" {
			parts = append(parts, fantasy.TextPart{Text: text})
		}
		for _, content := range m.BinaryContent() {
			// skip text attachements
			if strings.HasPrefix(content.MIMEType, "text/") {
				continue
			}
			parts = append(parts, fantasy.FilePart{
				Filename:  content.Path,
				Data:      content.Data,
				MediaType: content.MIMEType,
			})
		}
		messages = append(messages, fantasy.Message{
			Role:    fantasy.MessageRoleUser,
			Content: parts,
		})
	case Assistant:
		googleReasoningByToolID := make(map[string]*google.ReasoningMetadata)
		for _, part := range m.Parts {
			reasoning, ok := part.(ReasoningContent)
			if !ok || reasoning.ThoughtSignature == "" || reasoning.ToolID == "" {
				continue
			}
			googleReasoningByToolID[reasoning.ToolID] = &google.ReasoningMetadata{
				Signature: reasoning.ThoughtSignature,
				ToolID:    reasoning.ToolID,
			}
		}

		var parts []fantasy.MessagePart
		for _, part := range m.Parts {
			switch content := part.(type) {
			case TextContent:
				text := strings.TrimSpace(content.Text)
				if text != "" {
					parts = append(parts, fantasy.TextPart{Text: text})
				}
			case ReasoningContent:
				if content.Thinking == "" && content.Signature == "" && content.ResponsesData == nil && content.ThoughtSignature == "" {
					continue
				}
				reasoningPart := fantasy.ReasoningPart{Text: content.Thinking, ProviderOptions: fantasy.ProviderOptions{}}
				if content.Signature != "" {
					reasoningPart.ProviderOptions[anthropic.Name] = &anthropic.ReasoningOptionMetadata{
						Signature: content.Signature,
					}
				}
				if content.ResponsesData != nil {
					reasoningPart.ProviderOptions[openai.Name] = content.ResponsesData
				}
				if content.ThoughtSignature != "" {
					reasoningPart.ProviderOptions[google.Name] = &google.ReasoningMetadata{
						Signature: content.ThoughtSignature,
						ToolID:    content.ToolID,
					}
				}
				parts = append(parts, reasoningPart)
			case ToolCall:
				toolCallPart := fantasy.ToolCallPart{
					ToolCallID:       content.ID,
					ToolName:         content.Name,
					Input:            content.Input,
					ProviderExecuted: content.ProviderExecuted,
				}
				if metadata, ok := googleReasoningByToolID[content.ID]; ok {
					toolCallPart.ProviderOptions = fantasy.ProviderOptions{
						google.Name: metadata,
					}
				}
				parts = append(parts, toolCallPart)
			}
		}
		messages = append(messages, fantasy.Message{
			Role:    fantasy.MessageRoleAssistant,
			Content: parts,
		})
	case Tool:
		var parts []fantasy.MessagePart
		for _, result := range m.ToolResults() {
			var content fantasy.ToolResultOutputContent
			if result.IsError {
				content = fantasy.ToolResultOutputContentError{
					Error: errors.New(result.Content),
				}
			} else if result.Data != "" {
				content = fantasy.ToolResultOutputContentMedia{
					Data:      result.Data,
					MediaType: result.MIMEType,
				}
			} else {
				content = fantasy.ToolResultOutputContentText{
					Text: result.Content,
				}
			}
			parts = append(parts, fantasy.ToolResultPart{
				ToolCallID: result.ToolCallID,
				Output:     content,
			})
		}
		messages = append(messages, fantasy.Message{
			Role:    fantasy.MessageRoleTool,
			Content: parts,
		})
	}
	return messages
}
