package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"mvdan.cc/sh/v3/shell"
)

func (c *coordinator) buildReadOnlyExplorationTools(ctx context.Context, workingDir string, allowed []string) ([]fantasy.AgentTool, error) {
	if c == nil {
		return nil, fmt.Errorf("coordinator is nil")
	}
	agentCfg, ok := c.cfg.Agents[config.AgentTask]
	if !ok {
		return nil, fmt.Errorf("task agent is not configured")
	}
	filteredAgent := agentCfg
	filteredAgent.AllowedTools = uniqueToolNames(allowed)
	toolset, err := c.buildToolsForWorkingDir(ctx, filteredAgent, workingDir)
	if err != nil {
		return nil, err
	}
	return wrapReadOnlyExplorationTools(toolset), nil
}

func uniqueToolNames(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	names := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func wrapReadOnlyExplorationTools(items []fantasy.AgentTool) []fantasy.AgentTool {
	if len(items) == 0 {
		return nil
	}
	wrapped := make([]fantasy.AgentTool, 0, len(items))
	for _, tool := range items {
		if tool == nil {
			continue
		}
		if tool.Info().Name == tools.BashToolName {
			wrapped = append(wrapped, newReadOnlyExplorationBashTool(tool))
			continue
		}
		wrapped = append(wrapped, tool)
	}
	return wrapped
}

type readOnlyExplorationTool struct {
	base fantasy.AgentTool
	info fantasy.ToolInfo
	run  func(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error)
}

func (t readOnlyExplorationTool) Info() fantasy.ToolInfo {
	return t.info
}

func (t readOnlyExplorationTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return t.run(ctx, call)
}

func (t readOnlyExplorationTool) ProviderOptions() fantasy.ProviderOptions {
	return t.base.ProviderOptions()
}

func (t readOnlyExplorationTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.base.SetProviderOptions(opts)
}

func newReadOnlyExplorationBashTool(base fantasy.AgentTool) fantasy.AgentTool {
	info := base.Info()
	info.Description = strings.TrimSpace(info.Description + "\n\nRead-only exploration only. File edits, shell redirection, and mutating git commands are blocked.")
	return readOnlyExplorationTool{
		base: base,
		info: info,
		run: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			var params tools.BashParams
			if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid bash input: %v", err)), nil
			}
			if err := validateReadOnlyExplorationCommand(params.Command, params.PrefixRule); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return base.Run(ctx, call)
		},
	}
}

func validateReadOnlyExplorationCommand(command string, prefixRule []string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("command is required")
	}

	combined := command
	if len(prefixRule) > 0 {
		combined = strings.TrimSpace(strings.Join(append(append([]string{}, prefixRule...), command), " "))
	}

	for _, token := range []string{"&&", "||", ";", "|", ">", "<", "`", "$("} {
		if strings.Contains(combined, token) {
			return fmt.Errorf("read-only exploration bash blocks shell control token %q", token)
		}
	}

	fields, err := shell.Fields(combined, nil)
	if err != nil || len(fields) == 0 {
		return fmt.Errorf("command %q could not be parsed safely", combined)
	}

	idx := 0
	for idx < len(fields) {
		switch fields[idx] {
		case "timeout", "nice", "nohup":
			idx++
			if idx < len(fields) && fields[idx-1] == "timeout" {
				idx++
			}
		default:
			goto validateCommand
		}
	}

validateCommand:
	if idx >= len(fields) {
		return fmt.Errorf("command %q does not contain an executable", combined)
	}

	cmd := fields[idx]
	args := fields[idx+1:]
	switch cmd {
	case "rg", "grep", "find", "cat", "ls", "head", "tail", "wc", "tree", "sed", "awk", "cut", "sort", "uniq", "stat", "file", "pwd", "realpath", "dirname", "basename":
		return validateReadOnlyUtilityArgs(cmd, args)
	case "git":
		return validateReadOnlyGitArgs(args)
	default:
		return fmt.Errorf("command %q is not allowed in read-only exploration mode", cmd)
	}
}

func validateReadOnlyUtilityArgs(command string, args []string) error {
	if command == "find" {
		for _, arg := range args {
			switch arg {
			case "-delete", "-exec", "-execdir", "-ok", "-okdir":
				return fmt.Errorf("find argument %q is blocked in read-only exploration mode", arg)
			}
		}
	}
	return nil
}

func validateReadOnlyGitArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("git subcommand is required in read-only exploration mode")
	}
	subcommand := args[0]
	rest := args[1:]
	switch subcommand {
	case "status", "log", "show", "diff", "grep", "ls-files", "rev-parse", "blame", "describe", "shortlog", "remote":
		return nil
	case "branch":
		for _, arg := range rest {
			if strings.HasPrefix(arg, "-d") || strings.HasPrefix(arg, "-D") || arg == "-m" || arg == "-M" || arg == "-c" || arg == "-C" {
				return fmt.Errorf("git branch argument %q is blocked in read-only exploration mode", arg)
			}
		}
		return nil
	case "tag":
		if len(rest) == 0 {
			return nil
		}
		for _, arg := range rest {
			if arg == "-d" || arg == "--delete" {
				return fmt.Errorf("git tag deletion is blocked in read-only exploration mode")
			}
		}
		return nil
	case "config":
		if len(rest) == 0 {
			return fmt.Errorf("git config requires a read-only flag in read-only exploration mode")
		}
		if rest[0] != "--get" && rest[0] != "--list" {
			return fmt.Errorf("git config %q is blocked in read-only exploration mode", rest[0])
		}
		return nil
	default:
		return fmt.Errorf("git subcommand %q is not allowed in read-only exploration mode", subcommand)
	}
}
