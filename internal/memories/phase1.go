package memories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/sapphire/internal/db"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/google/uuid"
)

func (s *Service) runPhase1(ctx context.Context) error {
	claimed, err := s.q.ClaimStage1JobsForStartup(ctx, db.ClaimStage1JobsForStartupParams{
		ClaimedBy:      nullableString(uuid.NewString()),
		LeaseExpiresAt: nullableInt64(nowUnix() + int64(defaultPhase1Lease.Seconds())),
		Now:            nowUnix(),
		LimitCount:     defaultPhase1Limit,
	})
	if err != nil {
		return fmt.Errorf("claim stage1 jobs: %w", err)
	}
	if len(claimed) == 0 {
		return nil
	}

	var (
		wg      sync.WaitGroup
		errOnce sync.Once
		runErr  error
		sem     = make(chan struct{}, defaultPhase1Limit)
	)
	for _, job := range claimed {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := s.processStage1Job(ctx, job); err != nil {
				errOnce.Do(func() { runErr = err })
			}
		}()
	}
	wg.Wait()
	return runErr
}

func (s *Service) processStage1Job(ctx context.Context, job db.MemoryStage1Job) error {
	runner := s.getPhase1Runner()
	if runner == nil {
		_ = s.q.MarkStage1JobFailed(ctx, db.MarkStage1JobFailedParams{
			RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
			SessionID:  job.SessionID,
		})
		return fmt.Errorf("phase1 runner not configured")
	}

	msgs, err := s.messages.List(ctx, job.SessionID)
	if err != nil {
		_ = s.q.MarkStage1JobFailed(ctx, db.MarkStage1JobFailedParams{
			RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
			SessionID:  job.SessionID,
		})
		return fmt.Errorf("list session messages for stage1: %w", err)
	}
	if len(msgs) == 0 {
		if err := s.q.MarkStage1JobNoOutput(ctx, job.SessionID); err != nil {
			return fmt.Errorf("mark stage1 no output: %w", err)
		}
		return nil
	}

	sess, err := s.sessions.Get(ctx, job.SessionID)
	if err != nil {
		_ = s.q.MarkStage1JobFailed(ctx, db.MarkStage1JobFailedParams{
			RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
			SessionID:  job.SessionID,
		})
		return fmt.Errorf("get session for stage1: %w", err)
	}

	rendered, err := serializeMessagesForMemory(msgs)
	if err != nil {
		_ = s.q.MarkStage1JobFailed(ctx, db.MarkStage1JobFailedParams{
			RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
			SessionID:  job.SessionID,
		})
		return fmt.Errorf("serialize messages for stage1: %w", err)
	}
	if strings.TrimSpace(rendered) == "" {
		if err := s.q.MarkStage1JobNoOutput(ctx, job.SessionID); err != nil {
			return fmt.Errorf("mark stage1 no output: %w", err)
		}
		return nil
	}

	output, err := runner(ctx, Phase1Invocation{
		SystemPrompt: StageOneSystemPrompt(),
		UserPrompt: BuildStageOneInputPrompt(
			"session://"+job.SessionID,
			job.Cwd,
			rendered,
		),
	})
	if err != nil {
		_ = s.q.MarkStage1JobFailed(ctx, db.MarkStage1JobFailedParams{
			RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
			SessionID:  job.SessionID,
		})
		return fmt.Errorf("run stage1 extraction: %w", err)
	}
	output.RawMemory = strings.TrimSpace(output.RawMemory)
	output.RolloutSummary = strings.TrimSpace(output.RolloutSummary)
	output.RolloutSlug = sanitizeSlug(output.RolloutSlug)
	if output.RawMemory == "" || output.RolloutSummary == "" {
		deleted, deleteErr := s.q.DeleteStage1OutputBySessionID(ctx, job.SessionID)
		if deleteErr != nil {
			return fmt.Errorf("delete stage1 output on no-op: %w", deleteErr)
		}
		if err := s.q.MarkStage1JobNoOutput(ctx, job.SessionID); err != nil {
			return fmt.Errorf("mark stage1 no output: %w", err)
		}
		if deleted > 0 {
			if err := s.q.MarkPhase2Dirty(ctx); err != nil {
				return fmt.Errorf("mark phase2 dirty after deleting stage1 output: %w", err)
			}
		}
		return nil
	}

	rolloutSlug := output.RolloutSlug
	if rolloutSlug == "" {
		rolloutSlug = sanitizeSlug(sess.Title)
	}
	filename := canonicalRolloutSummaryFilename(max(job.UpdatedAt, nowUnix()), job.SessionID, rolloutSlug)
	rolloutSummaryFile := filepathJoin("rollout_summaries", filename)

	if err := s.q.UpsertStage1Output(ctx, db.UpsertStage1OutputParams{
		SessionID:          job.SessionID,
		SourceUpdatedAt:    job.UpdatedAt,
		RawMemory:          output.RawMemory,
		RolloutSummary:     output.RolloutSummary,
		RolloutSlug:        rolloutSlug,
		RolloutSummaryFile: rolloutSummaryFile,
	}); err != nil {
		_ = s.q.MarkStage1JobFailed(ctx, db.MarkStage1JobFailedParams{
			RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
			SessionID:  job.SessionID,
		})
		return fmt.Errorf("upsert stage1 output: %w", err)
	}
	if err := s.q.MarkStage1JobSucceeded(ctx, job.SessionID); err != nil {
		return fmt.Errorf("mark stage1 succeeded: %w", err)
	}
	if err := s.q.MarkPhase2Dirty(ctx); err != nil {
		return fmt.Errorf("mark phase2 dirty after stage1: %w", err)
	}
	return nil
}

func serializeMessagesForMemory(msgs []message.Message) (string, error) {
	serialized := make([]map[string]any, 0, len(msgs))
	for _, msg := range msgs {
		text := strings.TrimSpace(msg.Content().Text)
		if text == "" && len(msg.ToolCalls()) == 0 && len(msg.ToolResults()) == 0 {
			continue
		}
		item := map[string]any{
			"role": string(msg.Role),
			"text": text,
		}
		toolCalls := make([]map[string]string, 0, len(msg.ToolCalls()))
		for _, call := range msg.ToolCalls() {
			toolCalls = append(toolCalls, map[string]string{
				"id":    call.ID,
				"name":  call.Name,
				"input": strings.TrimSpace(call.Input),
			})
		}
		toolResults := make([]map[string]string, 0, len(msg.ToolResults()))
		for _, result := range msg.ToolResults() {
			toolResults = append(toolResults, map[string]string{
				"tool_call_id": result.ToolCallID,
				"name":         result.Name,
				"content":      strings.TrimSpace(result.Content),
			})
		}
		if len(toolCalls) > 0 {
			item["tool_calls"] = toolCalls
		}
		if len(toolResults) > 0 {
			item["tool_results"] = toolResults
		}
		serialized = append(serialized, item)
	}
	data, err := json.MarshalIndent(serialized, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func filepathJoin(parts ...string) string {
	return strings.Join(parts, "/")
}
