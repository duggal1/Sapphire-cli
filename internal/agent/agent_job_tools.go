package agent

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/filepathext"
	"github.com/google/uuid"
)

//go:embed tools/spawn_agents_on_csv.md
var spawnAgentsOnCSVDescription []byte

//go:embed tools/report_agent_job_result.md
var reportAgentJobResultDescription []byte

const (
	SpawnAgentsOnCSVToolName     = "spawn_agents_on_csv"
	ReportAgentJobResultToolName = "report_agent_job_result"
)

const (
	defaultAgentJobConcurrency = 16
	maxAgentJobConcurrency     = 64
	defaultAgentJobTimeout     = 30 * time.Minute
)

type SpawnAgentsOnCSVParams struct {
	CSVPath           string         `json:"csv_path" description:"Path to the input CSV file"`
	Instruction       string         `json:"instruction" description:"Instruction template with {column} placeholders"`
	IDColumn          string         `json:"id_column,omitempty" description:"Optional column used to generate item IDs"`
	OutputCSVPath     string         `json:"output_csv_path,omitempty" description:"Optional path to write the output CSV"`
	OutputSchema      map[string]any `json:"output_schema,omitempty" description:"Optional JSON schema or shape for worker output"`
	MaxConcurrency    int            `json:"max_concurrency,omitempty" description:"Maximum concurrent workers (default 16, capped by agent_max_threads)"`
	MaxWorkers        int            `json:"max_workers,omitempty" description:"Alias for max_concurrency"`
	MaxRuntimeSeconds int64          `json:"max_runtime_seconds,omitempty" description:"Maximum runtime per worker (seconds)"`
}

type ReportAgentJobResultParams struct {
	JobID  string         `json:"job_id" description:"Agent job id"`
	ItemID string         `json:"item_id" description:"Agent job item id"`
	Result map[string]any `json:"result" description:"Result object for this item"`
	Stop   bool           `json:"stop,omitempty" description:"Cancel remaining items after reporting this result"`
}

type spawnAgentsOnCSVResponse struct {
	JobID           string                   `json:"job_id"`
	Status          agentJobStatus           `json:"status"`
	OutputCSVPath   string                   `json:"output_csv_path"`
	TotalItems      int                      `json:"total_items"`
	CompletedItems  int                      `json:"completed_items"`
	FailedItems     int                      `json:"failed_items"`
	JobError        string                   `json:"job_error,omitempty"`
	FailedItemsInfo []agentJobFailureSummary `json:"failed_item_errors,omitempty"`
}

type agentJobFailureSummary struct {
	ItemID   string `json:"item_id"`
	SourceID string `json:"source_id,omitempty"`
	Error    string `json:"last_error"`
}

type reportAgentJobResultResponse struct {
	Accepted bool `json:"accepted"`
}

func (c *coordinator) spawnAgentsOnCSVTool(ctx context.Context) (fantasy.AgentTool, error) {
	_ = ctx
	return fantasy.NewParallelAgentTool(
		SpawnAgentsOnCSVToolName,
		string(spawnAgentsOnCSVDescription),
		func(ctx context.Context, params SpawnAgentsOnCSVParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			if strings.TrimSpace(params.CSVPath) == "" {
				return fantasy.NewTextErrorResponse("csv_path is required"), nil
			}
			if strings.TrimSpace(params.Instruction) == "" {
				return fantasy.NewTextErrorResponse("instruction is required"), nil
			}
			sessionID := tools.GetSessionFromContext(ctx)
			if strings.TrimSpace(sessionID) == "" {
				return fantasy.NewTextErrorResponse("session id missing from context"), nil
			}
			workingDir := tools.GetWorkingDirFromContext(ctx)
			if workingDir == "" {
				workingDir = c.cfg.WorkingDir()
			}
			inputPath := filepathext.SmartJoin(workingDir, params.CSVPath)
			content, err := os.ReadFile(inputPath)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to read csv input: %v", err)), nil
			}
			headers, rows, err := parseAgentJobCSV(string(content))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if len(headers) == 0 {
				return fantasy.NewTextErrorResponse("csv input must include a header row"), nil
			}
			if err := ensureUniqueHeaders(headers); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			idColumnIndex := -1
			if params.IDColumn != "" {
				for idx, header := range headers {
					if header == params.IDColumn {
						idColumnIndex = idx
						break
					}
				}
				if idColumnIndex == -1 {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("id_column %s was not found in csv headers", params.IDColumn)), nil
				}
			}

			items := make([]*agentJobItem, 0, len(rows))
			seenIDs := map[string]struct{}{}
			for idx, row := range rows {
				if len(row) != len(headers) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("csv row %d has %d fields but header has %d", idx+2, len(row), len(headers))), nil
				}
				rowIndex := idx + 1
				sourceID := ""
				if idColumnIndex >= 0 && idColumnIndex < len(row) {
					sourceID = strings.TrimSpace(row[idColumnIndex])
				}
				baseID := sourceID
				if baseID == "" {
					baseID = fmt.Sprintf("row-%d", rowIndex)
				}
				itemID := baseID
				for suffix := 2; ; suffix++ {
					if _, ok := seenIDs[itemID]; !ok {
						seenIDs[itemID] = struct{}{}
						break
					}
					itemID = fmt.Sprintf("%s-%d", baseID, suffix)
				}
				rowMap := make(map[string]string, len(headers))
				for i, header := range headers {
					rowMap[header] = row[i]
				}
				items = append(items, &agentJobItem{
					ItemID:   itemID,
					SourceID: sourceID,
					RowIndex: idx,
					Row:      rowMap,
					Status:   agentJobItemStatusPending,
				})
			}

			jobID := uuid.New().String()
			outputPath := params.OutputCSVPath
			if strings.TrimSpace(outputPath) == "" {
				outputPath = defaultAgentJobOutputPath(inputPath, jobID)
			} else {
				outputPath = filepathext.SmartJoin(workingDir, outputPath)
			}

			maxRuntime := defaultAgentJobTimeout
			if params.MaxRuntimeSeconds > 0 {
				maxRuntime = time.Duration(params.MaxRuntimeSeconds) * time.Second
			} else if params.MaxRuntimeSeconds < 0 {
				return fantasy.NewTextErrorResponse("max_runtime_seconds must be >= 1"), nil
			}

			var outputSchemaRaw json.RawMessage
			if params.OutputSchema != nil {
				if data, err := json.Marshal(params.OutputSchema); err == nil {
					outputSchemaRaw = data
				}
			}

			job := &agentJob{
				ID:              jobID,
				ParentSessionID: sessionID,
				Instruction:     strings.TrimSpace(params.Instruction),
				InputHeaders:    headers,
				OutputCSVPath:   outputPath,
				OutputSchemaRaw: outputSchemaRaw,
				Status:          agentJobStatusPending,
				Items:           items,
				ItemsByID:       make(map[string]*agentJobItem, len(items)),
				MaxRuntime:      maxRuntime,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
			for _, item := range items {
				item.JobID = jobID
				job.ItemsByID[item.ItemID] = item
			}
			c.agentJobs.create(job)

			concurrency := normalizeAgentJobConcurrency(params.MaxConcurrency, params.MaxWorkers, c.subAgentThreadLimit())
			job.Status = agentJobStatusRunning
			if err := c.runAgentJob(ctx, job, concurrency); err != nil {
				job.markFailed(err.Error())
			}

			if err := writeAgentJobCSV(job); err != nil {
				job.markFailed(fmt.Sprintf("failed to write output csv: %v", err))
			}

			progress := job.snapshotProgress()
			jobError := strings.TrimSpace(job.LastError)
			failedSummaries := summarizeAgentJobFailures(job, 5)
			resp := spawnAgentsOnCSVResponse{
				JobID:          job.ID,
				Status:         job.Status,
				OutputCSVPath:  job.OutputCSVPath,
				TotalItems:     progress.TotalItems,
				CompletedItems: progress.CompletedItems,
				FailedItems:    progress.FailedItems,
			}
			if jobError != "" {
				resp.JobError = jobError
			}
			if len(failedSummaries) > 0 {
				resp.FailedItemsInfo = failedSummaries
			}
			payload, _ := json.Marshal(resp)
			return fantasy.NewTextResponse(string(payload)), nil
		},
	), nil
}

func (c *coordinator) reportAgentJobResultTool(ctx context.Context) (fantasy.AgentTool, error) {
	_ = ctx
	return fantasy.NewParallelAgentTool(
		ReportAgentJobResultToolName,
		string(reportAgentJobResultDescription),
		func(ctx context.Context, params ReportAgentJobResultParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			if strings.TrimSpace(params.JobID) == "" {
				return fantasy.NewTextErrorResponse("job_id is required"), nil
			}
			if strings.TrimSpace(params.ItemID) == "" {
				return fantasy.NewTextErrorResponse("item_id is required"), nil
			}
			if params.Result == nil {
				return fantasy.NewTextErrorResponse("result must be a JSON object"), nil
			}
			sessionID := tools.GetSessionFromContext(ctx)
			accepted, err := c.agentJobs.reportResult(params.JobID, params.ItemID, sessionID, params.Result, params.Stop)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			payload, _ := json.Marshal(reportAgentJobResultResponse{Accepted: accepted})
			return fantasy.NewTextResponse(string(payload)), nil
		},
	), nil
}

func normalizeAgentJobConcurrency(requested, alias, threadLimit int) int {
	value := requested
	if value <= 0 {
		value = alias
	}
	if value <= 0 {
		value = defaultAgentJobConcurrency
	}
	if value > maxAgentJobConcurrency {
		value = maxAgentJobConcurrency
	}
	if threadLimit > 0 && value > threadLimit {
		value = threadLimit
	}
	if value < 1 {
		value = 1
	}
	return value
}

func parseAgentJobCSV(content string) ([]string, [][]string, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse csv headers: %w", err)
	}
	if len(headers) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}
	rows := make([][]string, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, csv.ErrFieldCount) {
			return nil, nil, fmt.Errorf("csv row has mismatched field count: %w", err)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse csv row: %w", err)
		}
		allEmpty := true
		for _, v := range record {
			if strings.TrimSpace(v) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}
		rows = append(rows, record)
	}
	return headers, rows, nil
}

func ensureUniqueHeaders(headers []string) error {
	seen := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		if _, ok := seen[header]; ok {
			return fmt.Errorf("csv header %s is duplicated", header)
		}
		seen[header] = struct{}{}
	}
	return nil
}

func defaultAgentJobOutputPath(inputPath, jobID string) string {
	stem := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	if stem == "" {
		stem = "agent_job_output"
	}
	return filepath.Join(filepath.Dir(inputPath), fmt.Sprintf("%s.agent-job-%s.csv", stem, jobID[:8]))
}

func renderAgentJobCSV(job *agentJob) (string, error) {
	headers := append([]string{}, job.InputHeaders...)
	headers = append(headers,
		"job_id",
		"item_id",
		"row_index",
		"source_id",
		"status",
		"attempt_count",
		"last_error",
		"result_json",
		"reported_at",
		"completed_at",
	)

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(headers); err != nil {
		return "", err
	}

	items := append([]*agentJobItem{}, job.Items...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].RowIndex < items[j].RowIndex
	})

	for _, item := range items {
		row := make([]string, 0, len(headers))
		for _, header := range job.InputHeaders {
			row = append(row, item.Row[header])
		}
		row = append(row,
			item.JobID,
			item.ItemID,
			fmt.Sprintf("%d", item.RowIndex),
			item.SourceID,
			string(item.Status),
			fmt.Sprintf("%d", item.AttemptCount),
			item.LastError,
			renderAgentJobResult(item.Result),
			formatAgentJobTime(item.ReportedAt),
			formatAgentJobTime(item.CompletedAt),
		)
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderAgentJobResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	data, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	return string(data)
}

func formatAgentJobTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}

func writeAgentJobCSV(job *agentJob) error {
	content, err := renderAgentJobCSV(job)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(job.OutputCSVPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(job.OutputCSVPath, []byte(content), 0o644)
}

func summarizeAgentJobFailures(job *agentJob, limit int) []agentJobFailureSummary {
	if limit <= 0 {
		return nil
	}
	summaries := make([]agentJobFailureSummary, 0, limit)
	for _, item := range job.Items {
		if item.Status != agentJobItemStatusFailed {
			continue
		}
		if strings.TrimSpace(item.LastError) == "" {
			continue
		}
		summaries = append(summaries, agentJobFailureSummary{
			ItemID:   item.ItemID,
			SourceID: item.SourceID,
			Error:    item.LastError,
		})
		if len(summaries) >= limit {
			break
		}
	}
	return summaries
}
