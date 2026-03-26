package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"mvdan.cc/sh/v3/shell"
)

type ToolHandler func(context.Context, json.RawMessage, fantasy.ToolCall) (fantasy.ToolResponse, error)

type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
	Required    []string
	Parallel    bool
	Handler     ToolHandler
}

type Registry struct {
	mu    sync.RWMutex
	specs map[string]ToolSpec
}

func NewRegistry() *Registry {
	return &Registry{specs: make(map[string]ToolSpec)}
}

func (r *Registry) Register(spec ToolSpec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("tool name is required")
	}
	if spec.Handler == nil {
		return fmt.Errorf("tool %q handler is required", spec.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.specs[spec.Name]; exists {
		return fmt.Errorf("tool %q is already registered", spec.Name)
	}
	r.specs[spec.Name] = spec
	return nil
}

func (r *Registry) Get(name string) (ToolSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.specs[name]
	return spec, ok
}

func (r *Registry) Execute(ctx context.Context, name string, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	spec, ok := r.Get(name)
	if !ok {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("unknown tool %q", name)), nil
	}
	return spec.Handler(ctx, json.RawMessage(call.Input), call)
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.specs))
	for name := range r.specs {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (r *Registry) AgentTools(names ...string) []fantasy.AgentTool {
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	ordered := make([]string, 0, len(r.specs))
	for name := range r.specs {
		ordered = append(ordered, name)
	}
	slices.Sort(ordered)
	tools := make([]fantasy.AgentTool, 0, len(r.specs))
	for _, name := range ordered {
		if len(allowed) > 0 {
			if _, ok := allowed[name]; !ok {
				continue
			}
		}
		spec := r.specs[name]
		tools = append(tools, registryAgentTool{spec: spec})
	}
	return tools
}

type registryAgentTool struct {
	spec ToolSpec
}

func (t registryAgentTool) Info() fantasy.ToolInfo {
	parameters, required := normalizeToolInfoSchema(t.spec.Parameters, t.spec.Required)
	return fantasy.ToolInfo{
		Name:        t.spec.Name,
		Description: t.spec.Description,
		Parameters:  parameters,
		Required:    required,
		Parallel:    t.spec.Parallel,
	}
}

func (t registryAgentTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return t.spec.Handler(ctx, json.RawMessage(call.Input), call)
}

func (t registryAgentTool) ProviderOptions() fantasy.ProviderOptions {
	return nil
}

func (t registryAgentTool) SetProviderOptions(opts fantasy.ProviderOptions) {}

func normalizeToolInfoSchema(parameters map[string]any, required []string) (map[string]any, []string) {
	if len(parameters) == 0 {
		return nil, append([]string{}, required...)
	}

	if schemaType, ok := parameters["type"].(string); ok && schemaType == "object" {
		if props, ok := parameters["properties"].(map[string]any); ok {
			normalizedRequired := append([]string{}, required...)
			if len(normalizedRequired) == 0 {
				if nestedRequired, ok := parameters["required"].([]string); ok {
					normalizedRequired = append(normalizedRequired, nestedRequired...)
				}
			}
			return props, normalizedRequired
		}
	}

	return parameters, append([]string{}, required...)
}

type PlanQuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type PlanQuestion struct {
	Header   string               `json:"header"`
	ID       string               `json:"id"`
	Question string               `json:"question"`
	Options  []PlanQuestionOption `json:"options"`
}

type PlanUserInputRequest struct {
	Questions []PlanQuestion `json:"questions"`
}

type ExplorationLaunchRequest struct {
	Prompt          string `json:"prompt"`
	Title           string `json:"title,omitempty"`
	WorkItemID      string `json:"work_item_id,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type ReadFileArgs struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

type SearchCodebaseArgs struct {
	Query      string `json:"query"`
	Path       string `json:"path,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type ListDirectoryArgs struct {
	Path       string `json:"path,omitempty"`
	MaxDepth   int    `json:"max_depth,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type RunCommandArgs struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Path    string   `json:"path,omitempty"`
}

type PlanModeRegistryOptions struct {
	WorkingDir        string
	Sessions          session.Service
	OnQuestions       func(context.Context, []PlanQuestion) error
	LaunchExploration func(context.Context, ExplorationLaunchRequest) (string, error)
}

func NewPlanModeRegistry(opts PlanModeRegistryOptions) (*Registry, error) {
	registry := NewRegistry()
	if err := registry.Register(requestUserInputToolSpec(opts.OnQuestions)); err != nil {
		return nil, err
	}
	if err := registry.Register(launchExplorationToolSpec(opts.LaunchExploration)); err != nil {
		return nil, err
	}
	if err := registry.Register(readFileToolSpec(opts.WorkingDir)); err != nil {
		return nil, err
	}
	if err := registry.Register(searchCodebaseToolSpec(opts.WorkingDir)); err != nil {
		return nil, err
	}
	if err := registry.Register(listDirectoryToolSpec(opts.WorkingDir)); err != nil {
		return nil, err
	}
	return registry, nil
}

func updatePlanToolSpec(sessions session.Service) ToolSpec {
	return ToolSpec{
		Name:        "update_plan",
		Description: "Record the current implementation plan.",
		Parameters: objectSchema(map[string]any{
			"explanation": stringSchema("optional explanation"),
			"plan": map[string]any{
				"type": "array",
				"items": objectSchema(map[string]any{
					"step":   stringSchema("step label"),
					"status": stringSchema("pending, in_progress, or completed"),
				}, "step", "status"),
			},
		}, "plan"),
		Required: []string{"plan"},
		Handler: func(ctx context.Context, raw json.RawMessage, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if sessions == nil {
				return fantasy.ToolResponse{}, fmt.Errorf("session service is not configured")
			}
			var input map[string]any
			if err := json.Unmarshal(raw, &input); err != nil {
				return fantasy.ToolResponse{}, err
			}
			input = repairUpdatePlanInput(input)
			if err := validateUpdatePlanInputMap(input); err != nil {
				return fantasy.ToolResponse{}, err
			}
			var args UpdatePlanArgs
			if err := decodeInto(input, &args); err != nil {
				return fantasy.ToolResponse{}, err
			}
			args = NormalizeUpdatePlanArgs(args)
			if len(args.Plan) == 0 {
				return fantasy.NewTextResponse("Plan unchanged"), nil
			}
			if err := ValidatePlanItems(args.Plan); err != nil {
				return fantasy.ToolResponse{}, err
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required")
			}
			currentSession, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			currentSession.Todos = make([]session.Todo, len(args.Plan))
			for i, item := range args.Plan {
				currentSession.Todos[i] = session.Todo{
					ID:         fmt.Sprintf("plan-%d", i),
					Content:    item.Step,
					Status:     session.TodoStatus(item.Status),
					ActiveForm: item.Step,
				}
			}
			if _, err := sessions.Save(ctx, currentSession); err != nil {
				return fantasy.ToolResponse{}, err
			}
			return fantasy.NewTextResponse("Plan updated"), nil
		},
	}
}

func requestUserInputToolSpec(onQuestions func(context.Context, []PlanQuestion) error) ToolSpec {
	return ToolSpec{
		Name:        "request_user_input",
		Description: "Ask structured clarifying questions.",
		Parameters: objectSchema(map[string]any{
			"questions": map[string]any{
				"type": "array",
				"items": objectSchema(map[string]any{
					"header":   stringSchema("short header"),
					"id":       stringSchema("stable identifier"),
					"question": stringSchema("prompt to show"),
					"options": map[string]any{
						"type": "array",
						"items": objectSchema(map[string]any{
							"label":       stringSchema("option label"),
							"description": stringSchema("option description"),
						}, "label", "description"),
					},
				}, "header", "id", "question", "options"),
			},
		}, "questions"),
		Required: []string{"questions"},
		Handler: func(ctx context.Context, raw json.RawMessage, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			var args PlanUserInputRequest
			if err := json.Unmarshal(raw, &args); err != nil {
				return fantasy.ToolResponse{}, err
			}
			if len(args.Questions) == 0 || len(args.Questions) > 3 {
				return fantasy.ToolResponse{}, fmt.Errorf("questions must contain 1 to 3 items")
			}
			for _, question := range args.Questions {
				if len(question.Options) < 2 {
					return fantasy.ToolResponse{}, fmt.Errorf("question %q must have at least 2 options", question.ID)
				}
			}
			if onQuestions != nil {
				if err := onQuestions(ctx, args.Questions); err != nil {
					return fantasy.ToolResponse{}, err
				}
			}
			var builder strings.Builder
			for _, question := range args.Questions {
				builder.WriteString(question.Question)
				builder.WriteString("\n")
				for _, option := range question.Options {
					builder.WriteString("- ")
					builder.WriteString(option.Label)
					builder.WriteString(": ")
					builder.WriteString(option.Description)
					builder.WriteString("\n")
				}
				builder.WriteString("\n")
			}
			return fantasy.NewTextResponse(strings.TrimSpace(builder.String())), nil
		},
	}
}

func launchExplorationToolSpec(launch func(context.Context, ExplorationLaunchRequest) (string, error)) ToolSpec {
	return ToolSpec{
		Name:        "launch_exploration_agent",
		Description: "Launch a read-only exploration sub-agent in the background.",
		Parameters: objectSchema(map[string]any{
			"prompt":           stringSchema("exploration task"),
			"title":            stringSchema("optional title"),
			"work_item_id":     stringSchema("optional work item id"),
			"model":            stringSchema("optional model override"),
			"reasoning_effort": stringSchema("optional reasoning override"),
		}, "prompt"),
		Required: []string{"prompt"},
		Handler: func(ctx context.Context, raw json.RawMessage, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if launch == nil {
				return fantasy.ToolResponse{}, fmt.Errorf("exploration launcher is not configured")
			}
			var args ExplorationLaunchRequest
			if err := json.Unmarshal(raw, &args); err != nil {
				return fantasy.ToolResponse{}, err
			}
			agentID, err := launch(ctx, args)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			return fantasy.NewTextResponse(agentID), nil
		},
	}
}

func readFileToolSpec(workingDir string) ToolSpec {
	return ToolSpec{
		Name:        "read_file",
		Description: "Read a file from the codebase.",
		Parameters: objectSchema(map[string]any{
			"path":       stringSchema("file path"),
			"start_line": integerSchema("optional start line"),
			"end_line":   integerSchema("optional end line"),
		}, "path"),
		Required: []string{"path"},
		Handler: func(_ context.Context, raw json.RawMessage, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			var args ReadFileArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return fantasy.ToolResponse{}, err
			}
			path, err := resolvePath(workingDir, args.Path)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			content, err := os.ReadFile(path) //nolint:gosec
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			lines := strings.Split(string(content), "\n")
			start := max(1, args.StartLine)
			end := args.EndLine
			if end <= 0 || end > len(lines) {
				end = len(lines)
			}
			if start > end {
				return fantasy.ToolResponse{}, fmt.Errorf("invalid line range")
			}
			return fantasy.NewTextResponse(strings.Join(lines[start-1:end], "\n")), nil
		},
	}
}

func searchCodebaseToolSpec(workingDir string) ToolSpec {
	return ToolSpec{
		Name:        "search_codebase",
		Description: "Search the codebase with ripgrep.",
		Parameters: objectSchema(map[string]any{
			"query":       stringSchema("search pattern"),
			"path":        stringSchema("optional path"),
			"max_results": integerSchema("optional result cap"),
		}, "query"),
		Required: []string{"query"},
		Handler: func(ctx context.Context, raw json.RawMessage, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			var args SearchCodebaseArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return fantasy.ToolResponse{}, err
			}
			target, err := resolvePath(workingDir, strings.TrimSpace(args.Path))
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			limit := args.MaxResults
			if limit <= 0 {
				limit = 50
			}
			cmdArgs := []string{"--line-number", "--no-heading", "--color=never", "-m", fmt.Sprintf("%d", limit), args.Query, target}
			output, err := exec.CommandContext(ctx, "rg", cmdArgs...).CombinedOutput()
			if err != nil && len(output) == 0 {
				return fantasy.ToolResponse{}, err
			}
			return fantasy.NewTextResponse(strings.TrimSpace(string(output))), nil
		},
	}
}

func listDirectoryToolSpec(workingDir string) ToolSpec {
	return ToolSpec{
		Name:        "list_directory",
		Description: "List files and directories under a path.",
		Parameters: objectSchema(map[string]any{
			"path":        stringSchema("optional path"),
			"max_depth":   integerSchema("optional max depth"),
			"max_results": integerSchema("optional result cap"),
		}),
		Handler: func(_ context.Context, raw json.RawMessage, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			var args ListDirectoryArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return fantasy.ToolResponse{}, err
			}
			root, err := resolvePath(workingDir, strings.TrimSpace(args.Path))
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			maxDepth := args.MaxDepth
			if maxDepth <= 0 {
				maxDepth = 2
			}
			maxResults := args.MaxResults
			if maxResults <= 0 {
				maxResults = 200
			}
			baseDepth := strings.Count(filepath.Clean(root), string(os.PathSeparator))
			items := make([]string, 0, maxResults)
			err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if path == root {
					return nil
				}
				depth := strings.Count(filepath.Clean(path), string(os.PathSeparator)) - baseDepth
				if depth > maxDepth {
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				display := toDisplayPath(root, path)
				if entry.IsDir() {
					display += "/"
				}
				items = append(items, display)
				if len(items) >= maxResults {
					return fs.SkipAll
				}
				return nil
			})
			if err != nil && !errorsIs(err, fs.SkipAll) {
				return fantasy.ToolResponse{}, err
			}
			return fantasy.NewTextResponse(strings.Join(items, "\n")), nil
		},
	}
}

func runCommandToolSpec(workingDir string) ToolSpec {
	return ToolSpec{
		Name:        "run_command",
		Description: "Run a read-only terminal command for codebase exploration.",
		Parameters: objectSchema(map[string]any{
			"command": stringSchema("allowed command name"),
			"args": map[string]any{
				"type":  "array",
				"items": stringSchema("command argument"),
			},
			"path": stringSchema("optional working directory"),
		}, "command"),
		Required: []string{"command"},
		Handler: func(ctx context.Context, raw json.RawMessage, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			var args RunCommandArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return fantasy.ToolResponse{}, err
			}
			if err := validateReadOnlyCommand(args.Command, args.Args); err != nil {
				return fantasy.ToolResponse{}, err
			}
			dir, err := resolvePath(workingDir, strings.TrimSpace(args.Path))
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			cmd := exec.CommandContext(ctx, args.Command, args.Args...)
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()
			if err != nil && len(output) == 0 {
				return fantasy.ToolResponse{}, err
			}
			return fantasy.NewTextResponse(strings.TrimSpace(string(output))), nil
		},
	}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integerSchema(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func resolvePath(root, raw string) (string, error) {
	base := root
	if strings.TrimSpace(base) == "" {
		base = "."
	}
	if strings.TrimSpace(raw) == "" {
		return filepath.Abs(base)
	}
	candidate := raw
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(base, candidate)
	}
	candidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, candidate)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes working directory", raw)
	}
	return candidate, nil
}

func toDisplayPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func validateReadOnlyCommand(command string, args []string) error {
	allowed := map[string]struct{}{
		"grep": {},
		"rg":   {},
		"find": {},
		"cat":  {},
		"ls":   {},
		"head": {},
		"tail": {},
		"wc":   {},
		"tree": {},
	}
	if _, ok := allowed[command]; !ok {
		return fmt.Errorf("command %q is not allowed in plan mode", command)
	}
	for _, arg := range args {
		fields, err := shell.Fields(arg, nil)
		if err != nil || len(fields) > 1 {
			return fmt.Errorf("command argument %q must be a literal argument", arg)
		}
		if strings.ContainsAny(arg, "|;&><`$") {
			return fmt.Errorf("command argument %q contains a blocked shell token", arg)
		}
	}
	return nil
}

func errorsIs(err error, target error) bool {
	return errors.Is(err, target)
}
