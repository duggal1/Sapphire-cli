package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

const agentJobPollInterval = 250 * time.Millisecond

func (c *coordinator) runAgentJob(ctx context.Context, job *agentJob, maxConcurrency int) error {
	if job == nil {
		return errors.New("job is required")
	}
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	parentSessionID := job.ParentSessionID
	if parentSessionID == "" {
		parentSessionID = tools.GetSessionFromContext(ctx)
	}
	ticker := time.NewTicker(agentJobPollInterval)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		progress := job.snapshotProgress()
		if progress.PendingItems == 0 && progress.RunningItems == 0 {
			break
		}

		if !job.hasCancelRequest() {
			spawnSlots := maxConcurrency - progress.RunningItems
			if parentSessionID != "" {
				threadLimit := c.subAgentThreadLimit()
				if threadLimit > 0 {
					active := c.activeSubAgentCount(parentSessionID)
					available := threadLimit - active
					if available < spawnSlots {
						spawnSlots = available
					}
				}
			}
			if spawnSlots > 0 {
				for i := 0; i < spawnSlots; i++ {
					item := job.nextPendingItem()
					if item == nil {
						break
					}
					if err := c.spawnAgentJobWorker(ctx, job, item, parentSessionID); err != nil {
						job.failItem(item, err.Error())
						continue
					}
				}
			}
		}

		c.reapFinishedAgentJobWorkers(job)
		c.failTimedOutAgentJobWorkers(job)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}

	if job.hasCancelRequest() {
		job.markCancelled("cancelled by worker request")
	} else {
		job.markCompleted()
	}
	return nil
}

func (c *coordinator) spawnAgentJobWorker(ctx context.Context, job *agentJob, item *agentJobItem, parentSessionID string) error {
	prompt, err := buildAgentJobWorkerPrompt(job, item)
	if err != nil {
		return err
	}
	agentID, _, err := c.spawnSubAgent(ctx, parentSessionID, spawnAgentOptions{
		Prompt:      prompt,
		Title:       fmt.Sprintf("Agent Job %s", job.ID),
		Worktree:    false,
		ForkContext: false,
	})
	if err != nil {
		return err
	}
	job.setItemRunning(item, agentID)
	return nil
}

func (c *coordinator) reapFinishedAgentJobWorkers(job *agentJob) {
	job.mu.Lock()
	items := append([]*agentJobItem{}, job.Items...)
	job.mu.Unlock()

	for _, item := range items {
		if item.Status != agentJobItemStatusRunning {
			continue
		}
		runner, err := c.getSubAgent(item.AssignedID)
		if err != nil {
			job.failItem(item, "assigned worker not found")
			continue
		}
		runner.mu.Lock()
		status := runner.status
		runner.mu.Unlock()
		if status == subAgentStatusRunning || status == subAgentStatusQueued {
			continue
		}
		if item.Result == nil {
			job.failItem(item, "worker completed without calling report_agent_job_result")
		}
		_ = c.closeSubAgent(item.AssignedID)
	}
}

func (c *coordinator) failTimedOutAgentJobWorkers(job *agentJob) {
	job.mu.Lock()
	items := append([]*agentJobItem{}, job.Items...)
	timeout := job.MaxRuntime
	job.mu.Unlock()

	if timeout <= 0 {
		return
	}
	now := time.Now()
	for _, item := range items {
		if item.Status != agentJobItemStatusRunning {
			continue
		}
		if item.StartedAt.IsZero() {
			continue
		}
		if now.Sub(item.StartedAt) < timeout {
			continue
		}
		job.failItem(item, fmt.Sprintf("worker exceeded max runtime of %s", timeout))
		_ = c.closeSubAgent(item.AssignedID)
	}
}

func buildAgentJobWorkerPrompt(job *agentJob, item *agentJobItem) (string, error) {
	if job == nil || item == nil {
		return "", errors.New("job and item are required")
	}
	rowJSON, err := json.MarshalIndent(item.Row, "", "  ")
	if err != nil {
		return "", err
	}
	schema := "{}"
	if len(job.OutputSchemaRaw) > 0 {
		schema = string(job.OutputSchemaRaw)
	}
	instruction := renderAgentJobInstruction(job.Instruction, item.Row)
	prompt := fmt.Sprintf(
		"You are processing one item for a batch agent job.\n"+
			"Job ID: %s\n"+
			"Item ID: %s\n\n"+
			"Task instruction:\n%s\n\n"+
			"Input row (JSON):\n%s\n\n"+
			"Expected result schema (JSON Schema or {}):\n%s\n\n"+
			"You MUST call the `report_agent_job_result` tool exactly once with:\n"+
			"1. `job_id` = \"%s\"\n"+
			"2. `item_id` = \"%s\"\n"+
			"3. `result` = a JSON object that contains your analysis result for this row.\n\n"+
			"If you need to stop the job early, include `stop` = true in the tool call.\n\n"+
			"After the tool call succeeds, stop.",
		job.ID,
		item.ItemID,
		instruction,
		string(rowJSON),
		schema,
		job.ID,
		item.ItemID,
	)
	return prompt, nil
}

func renderAgentJobInstruction(instruction string, row map[string]string) string {
	const openSentinel = "__SAPPHIRE_OPEN_BRACE__"
	const closeSentinel = "__SAPPHIRE_CLOSE_BRACE__"
	rendered := strings.ReplaceAll(strings.ReplaceAll(instruction, "{{", openSentinel), "}}", closeSentinel)
	for key, value := range row {
		placeholder := fmt.Sprintf("{%s}", key)
		rendered = strings.ReplaceAll(rendered, placeholder, value)
	}
	rendered = strings.ReplaceAll(rendered, openSentinel, "{")
	rendered = strings.ReplaceAll(rendered, closeSentinel, "}")
	return rendered
}
