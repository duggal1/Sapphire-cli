package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

//go:embed tools/spawn_agent.md
var spawnAgentDescription []byte

//go:embed tools/resume_agent.md
var resumeAgentDescription []byte

//go:embed tools/send_input.md
var sendInputDescription []byte

//go:embed tools/wait.md
var waitAgentsDescription []byte

//go:embed tools/collect_result.md
var collectResultDescription []byte

//go:embed tools/close_agent.md
var closeAgentDescription []byte

const (
	SpawnAgentToolName    = "spawn_agent"
	ResumeAgentToolName   = "resume_agent"
	SendInputToolName     = "send_input"
	WaitAgentsToolName    = "wait"
	CollectResultToolName = "collect_result"
	CloseAgentToolName    = "close_agent"
)

type SpawnAgentParams struct {
	Message          string   `json:"message,omitempty" description:"Initial task or prompt for the sub-agent"`
	Items            []string `json:"items,omitempty" description:"Optional structured input items; text items are flattened into the initial prompt"`
	Title            string   `json:"title,omitempty" description:"Optional session title for the sub-agent"`
	Isolation        string   `json:"isolation,omitempty" description:"Execution isolation mode. Use 'worktree' only when explicit git worktree isolation is required."`
	Worktree         *bool    `json:"worktree,omitempty" description:"Run in an isolated git worktree (default false)"`
	WorktreePath     string   `json:"worktree_path,omitempty" description:"Optional worktree path (defaults to repo-root/.sapphire/worktrees/agent/<id>/<task>)"`
	Branch           string   `json:"branch,omitempty" description:"Optional branch name for the worktree"`
	WriteManifest    []string `json:"write_manifest,omitempty" description:"Allowed write paths (relative to repo root). Empty list = read-only."`
	DefinitionOfDone string   `json:"definition_of_done,omitempty" description:"Acceptance criteria for completion"`
	Agent            string   `json:"agent,omitempty" description:"Agent profile to use (coder or task)"`
	Model            string   `json:"model,omitempty" description:"Optional model override (format provider:model or model)"`
	ReasoningEffort  string   `json:"reasoning_effort,omitempty" description:"Optional reasoning effort override (low, medium, high)"`
	ForkContext      *bool    `json:"fork_context,omitempty" description:"Copy recent parent context into the sub-agent session (default false)"`
}

type ResumeAgentParams struct {
	ID      string   `json:"id" description:"Agent id returned by spawn_agent"`
	Message string   `json:"message,omitempty" description:"Optional prompt to continue the resumed sub-agent"`
	Items   []string `json:"items,omitempty" description:"Optional structured follow-up input items"`
}

type SendInputParams struct {
	ID        string   `json:"id" description:"Agent id returned by spawn_agent"`
	Message   string   `json:"message,omitempty" description:"Follow-up task or prompt for the sub-agent"`
	Items     []string `json:"items,omitempty" description:"Optional structured follow-up input items"`
	Interrupt bool     `json:"interrupt,omitempty" description:"Interrupt current run before sending"`
}

type WaitAgentsParams struct {
	IDs       []string `json:"ids,omitempty" description:"Agent ids to wait for"`
	TimeoutMS int64    `json:"timeout_ms,omitempty" description:"Timeout in milliseconds (default 30000)"`
}

type CollectResultParams struct {
	IDs []string `json:"ids,omitempty" description:"Agent ids to collect results from"`
}

type CloseAgentParams struct {
	ID string `json:"id" description:"Agent id returned by spawn_agent"`
}

func (p *SpawnAgentParams) UnmarshalJSON(data []byte) error {
	type rawSpawnAgentParams struct {
		Message          string            `json:"message,omitempty"`
		Prompt           string            `json:"prompt,omitempty"`
		Task             string            `json:"task,omitempty"`
		Instruction      string            `json:"instruction,omitempty"`
		Items            []json.RawMessage `json:"items,omitempty"`
		Title            string            `json:"title,omitempty"`
		Isolation        string            `json:"isolation,omitempty"`
		Worktree         *bool             `json:"worktree,omitempty"`
		WorktreePath     string            `json:"worktree_path,omitempty"`
		WorktreeDir      string            `json:"worktree_dir,omitempty"`
		WorktreeDirAlt   string            `json:"worktree_directory,omitempty"`
		Branch           string            `json:"branch,omitempty"`
		BranchName       string            `json:"branch_name,omitempty"`
		WriteManifest    []string          `json:"write_manifest,omitempty"`
		Manifest         []string          `json:"manifest,omitempty"`
		AllowedFiles     []string          `json:"allowed_files,omitempty"`
		AllowedPaths     []string          `json:"allowed_paths,omitempty"`
		OwnedFiles       []string          `json:"owned_files,omitempty"`
		DefinitionOfDone string            `json:"definition_of_done,omitempty"`
		Done             string            `json:"done,omitempty"`
		Acceptance       string            `json:"acceptance_criteria,omitempty"`
		Agent            string            `json:"agent,omitempty"`
		AgentType        string            `json:"agent_type,omitempty"`
		AgentID          string            `json:"agent_id,omitempty"`
		AgentProfile     string            `json:"agent_profile,omitempty"`
		Model            string            `json:"model,omitempty"`
		ModelName        string            `json:"model_name,omitempty"`
		ReasoningEffort  string            `json:"reasoning_effort,omitempty"`
		Reasoning        string            `json:"reasoning,omitempty"`
		ForkContext      *bool             `json:"fork_context,omitempty"`
	}

	var raw rawSpawnAgentParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.Message = firstNonEmptyString(raw.Message, raw.Prompt, raw.Task, raw.Instruction)
	p.Items = parseCollabInputItems(raw.Items)
	if p.Message == "" && len(p.Items) > 0 {
		p.Message = strings.Join(p.Items, "\n")
	}
	p.Title = strings.TrimSpace(raw.Title)
	p.Isolation = strings.TrimSpace(raw.Isolation)
	p.Worktree = raw.Worktree
	p.WorktreePath = firstNonEmptyString(raw.WorktreePath, raw.WorktreeDir, raw.WorktreeDirAlt)
	p.Branch = firstNonEmptyString(raw.Branch, raw.BranchName)
	p.WriteManifest = firstNonEmptyStringSlice(raw.WriteManifest, raw.Manifest, raw.AllowedFiles, raw.AllowedPaths, raw.OwnedFiles)
	p.DefinitionOfDone = firstNonEmptyString(raw.DefinitionOfDone, raw.Done, raw.Acceptance)
	p.Agent = firstNonEmptyString(raw.Agent, raw.AgentType, raw.AgentID, raw.AgentProfile)
	p.Model = firstNonEmptyString(raw.Model, raw.ModelName)
	p.ReasoningEffort = firstNonEmptyString(raw.ReasoningEffort, raw.Reasoning)
	p.ForkContext = raw.ForkContext
	return nil
}

func flattenSpawnAgentItems(items []json.RawMessage) string {
	return strings.Join(parseCollabInputItems(items), "\n")
}

func (p *ResumeAgentParams) UnmarshalJSON(data []byte) error {
	type rawResumeAgentParams struct {
		ID      string            `json:"id,omitempty"`
		AgentID string            `json:"agent_id,omitempty"`
		Message string            `json:"message,omitempty"`
		Prompt  string            `json:"prompt,omitempty"`
		Task    string            `json:"task,omitempty"`
		Items   []json.RawMessage `json:"items,omitempty"`
	}

	var raw rawResumeAgentParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.ID = firstNonEmptyString(raw.ID, raw.AgentID)
	p.Message = firstNonEmptyString(raw.Message, raw.Prompt, raw.Task)
	p.Items = parseCollabInputItems(raw.Items)
	if p.Message == "" && len(p.Items) > 0 {
		p.Message = strings.Join(p.Items, "\n")
	}
	return nil
}

func (p *SendInputParams) UnmarshalJSON(data []byte) error {
	type rawSendInputParams struct {
		ID        string            `json:"id,omitempty"`
		AgentID   string            `json:"agent_id,omitempty"`
		Message   string            `json:"message,omitempty"`
		Prompt    string            `json:"prompt,omitempty"`
		Task      string            `json:"task,omitempty"`
		Items     []json.RawMessage `json:"items,omitempty"`
		Interrupt bool              `json:"interrupt,omitempty"`
	}

	var raw rawSendInputParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.ID = firstNonEmptyString(raw.ID, raw.AgentID)
	p.Message = firstNonEmptyString(raw.Message, raw.Prompt, raw.Task)
	p.Items = parseCollabInputItems(raw.Items)
	if p.Message == "" && len(p.Items) > 0 {
		p.Message = strings.Join(p.Items, "\n")
	}
	p.Interrupt = raw.Interrupt
	return nil
}

func parseCollabInputItems(items []json.RawMessage) []string {
	if len(items) == 0 {
		return nil
	}
	lines := make([]string, 0, len(items))
	for _, raw := range items {
		if len(raw) == 0 {
			continue
		}
		var plain string
		if err := json.Unmarshal(raw, &plain); err == nil {
			if text := strings.TrimSpace(plain); text != "" {
				lines = append(lines, text)
			}
			continue
		}
		var item struct {
			Text    string `json:"text,omitempty"`
			Content string `json:"content,omitempty"`
			Message string `json:"message,omitempty"`
		}
		if err := json.Unmarshal(raw, &item); err == nil {
			if text := firstNonEmptyString(item.Text, item.Content, item.Message); text != "" {
				lines = append(lines, text)
			}
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

func (p *WaitAgentsParams) UnmarshalJSON(data []byte) error {
	type rawWaitAgentsParams struct {
		IDs      []string `json:"ids,omitempty"`
		AgentIDs []string `json:"agent_ids,omitempty"`
		Timeout  int64    `json:"timeout_ms,omitempty"`
	}

	var raw rawWaitAgentsParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.IDs = firstNonEmptyStringSlice(raw.IDs, raw.AgentIDs)
	p.TimeoutMS = raw.Timeout
	return nil
}

func (p *CollectResultParams) UnmarshalJSON(data []byte) error {
	type rawCollectResultParams struct {
		IDs      []string `json:"ids,omitempty"`
		AgentIDs []string `json:"agent_ids,omitempty"`
	}

	var raw rawCollectResultParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.IDs = firstNonEmptyStringSlice(raw.IDs, raw.AgentIDs)
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptyStringSlice(groups ...[]string) []string {
	for _, group := range groups {
		normalized := normalizeStringSlice(group)
		if len(normalized) > 0 {
			return normalized
		}
	}
	return nil
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func resolveSpawnIsolation(worktree *bool, isolation string) (bool, error) {
	mode := strings.ToLower(strings.TrimSpace(isolation))
	switch mode {
	case "", "default":
		if worktree != nil {
			return *worktree, nil
		}
		return false, nil
	case "worktree":
		if worktree != nil && !*worktree {
			return false, errors.New("isolation=worktree conflicts with worktree=false")
		}
		return true, nil
	case "none", "shared":
		if worktree != nil && *worktree {
			return false, fmt.Errorf("isolation=%s conflicts with worktree=true", mode)
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported isolation %q; use worktree, none, or omit it", isolation)
	}
}

func (c *coordinator) spawnAgentTool(ctx context.Context) (fantasy.AgentTool, error) {
	_ = ctx
	control := c.subAgentControl()
	return fantasy.NewParallelAgentTool(
		SpawnAgentToolName,
		string(spawnAgentDescription),
		func(ctx context.Context, params SpawnAgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			if strings.TrimSpace(params.Message) == "" {
				return fantasy.NewTextErrorResponse("message is required"), nil
			}
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			useWorktree, err := resolveSpawnIsolation(params.Worktree, params.Isolation)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			worktreeSet := params.Worktree != nil || strings.TrimSpace(params.Isolation) != ""
			forkContext := false
			if params.ForkContext != nil {
				forkContext = *params.ForkContext
			}
			agentID, submissionID, err := control.spawn(ctx, sessionID, spawnAgentOptions{
				Prompt:           params.Message,
				PromptItems:      params.Items,
				Title:            params.Title,
				Worktree:         useWorktree,
				WorktreeSet:      worktreeSet,
				WorktreePath:     params.WorktreePath,
				Branch:           params.Branch,
				WriteManifest:    params.WriteManifest,
				DefinitionOfDone: params.DefinitionOfDone,
				AgentID:          params.Agent,
				Model:            params.Model,
				ReasoningEffort:  params.ReasoningEffort,
				ForkContext:      forkContext,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			status := subAgentStatusQueued
			workDir := ""
			startedAt := time.Time{}
			if runner, getErr := c.getSubAgent(agentID); getErr == nil {
				snapshot := runner.snapshot()
				status = snapshot.Status
				workDir = snapshot.WorkDir
				startedAt = snapshot.StartedAt
			}
			payload, _ := json.Marshal(map[string]any{
				"agent_id":      agentID,
				"submission_id": submissionID,
				"status":        status,
				"work_dir":      workDir,
				"started_at":    startedAt,
			})
			return fantasy.NewTextResponse(string(payload)), nil
		},
	), nil
}

func (c *coordinator) resumeAgentTool(ctx context.Context) (fantasy.AgentTool, error) {
	_ = ctx
	control := c.subAgentControl()
	return fantasy.NewParallelAgentTool(
		ResumeAgentToolName,
		string(resumeAgentDescription),
		func(ctx context.Context, params ResumeAgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			if params.ID == "" {
				return fantasy.NewTextErrorResponse("id is required"), nil
			}
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			submissionID, status, err := control.resume(ctx, sessionID, params.ID, firstNonEmptyString(params.Message, strings.Join(params.Items, "\n")))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			payload := map[string]any{
				"agent_id": params.ID,
				"status":   status,
			}
			if submissionID != "" {
				payload["submission_id"] = submissionID
			}
			if runner, getErr := c.getSubAgent(params.ID); getErr == nil {
				snapshot := runner.snapshot()
				payload["work_dir"] = snapshot.WorkDir
				payload["started_at"] = snapshot.StartedAt
			}
			encoded, _ := json.Marshal(payload)
			return fantasy.NewTextResponse(string(encoded)), nil
		},
	), nil
}

func (c *coordinator) sendInputTool(ctx context.Context) (fantasy.AgentTool, error) {
	_ = ctx
	control := c.subAgentControl()
	return fantasy.NewParallelAgentTool(
		SendInputToolName,
		string(sendInputDescription),
		func(ctx context.Context, params SendInputParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			if params.ID == "" {
				return fantasy.NewTextErrorResponse("id is required"), nil
			}
			if strings.TrimSpace(params.Message) == "" {
				return fantasy.NewTextErrorResponse("message is required"), nil
			}
			submissionID, err := control.sendInput(ctx, params.ID, params.Message, params.Items, params.Interrupt)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			payload, _ := json.Marshal(map[string]any{
				"submission_id": submissionID,
			})
			if runner, getErr := c.getSubAgent(params.ID); getErr == nil {
				snapshot := runner.snapshot()
				payload, _ = json.Marshal(map[string]any{
					"submission_id": submissionID,
					"work_dir":      snapshot.WorkDir,
					"started_at":    snapshot.StartedAt,
				})
			}
			return fantasy.NewTextResponse(string(payload)), nil
		},
	), nil
}

func (c *coordinator) waitAgentsTool(ctx context.Context) (fantasy.AgentTool, error) {
	_ = ctx
	control := c.subAgentControl()
	return fantasy.NewParallelAgentTool(
		WaitAgentsToolName,
		string(waitAgentsDescription),
		func(ctx context.Context, params WaitAgentsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			if len(params.IDs) == 0 {
				return fantasy.NewTextErrorResponse("ids are required"), nil
			}
			timeout := 60 * time.Second
			if params.TimeoutMS > 0 {
				timeout = time.Duration(params.TimeoutMS) * time.Millisecond
			}
			statuses, timedOut := control.wait(ctx, params.IDs, timeout)
			payload, _ := json.Marshal(map[string]any{
				"agents":    statuses,
				"timed_out": timedOut,
			})
			return fantasy.NewTextResponse(string(payload)), nil
		},
	), nil
}

func (c *coordinator) collectResultTool(ctx context.Context) (fantasy.AgentTool, error) {
	_ = ctx
	control := c.subAgentControl()
	return fantasy.NewParallelAgentTool(
		CollectResultToolName,
		string(collectResultDescription),
		func(ctx context.Context, params CollectResultParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			if len(params.IDs) == 0 {
				return fantasy.NewTextErrorResponse("ids are required"), nil
			}
			payload, _ := json.Marshal(map[string]any{
				"agents": control.collectResult(params.IDs),
			})
			return fantasy.NewTextResponse(string(payload)), nil
		},
	), nil
}

func (c *coordinator) closeAgentTool(ctx context.Context) (fantasy.AgentTool, error) {
	_ = ctx
	control := c.subAgentControl()
	return fantasy.NewParallelAgentTool(
		CloseAgentToolName,
		string(closeAgentDescription),
		func(ctx context.Context, params CloseAgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			if params.ID == "" {
				return fantasy.NewTextErrorResponse("id is required"), nil
			}
			if err := control.close(params.ID); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			payload, _ := json.Marshal(map[string]any{
				"agent_id": params.ID,
				"status":   subAgentStatusClosed,
			})
			return fantasy.NewTextResponse(string(payload)), nil
		},
	), nil
}
