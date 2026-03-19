package tools

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/fsext"
	"github.com/charmbracelet/sapphire/internal/permission"
	"github.com/charmbracelet/sapphire/internal/shell"
)

// BashParams defines the parameters for the bash execution tool.
type BashParams struct {
	Description     string   `json:"description" description:"A brief description of what the command does, try to keep it under 30 characters or so"`
	Command         string   `json:"command" description:"The command to execute"`
	WorkingDir      string   `json:"working_dir,omitempty" description:"The working directory to execute the command in (defaults to current directory)"`
	RunInBackground bool     `json:"run_in_background,omitempty" description:"Set to true (boolean) to run this command in the background. Use job_output to read the output later."`
	Backend         string   `json:"backend,omitempty" description:"Execution backend: 'posix' (default, mvdan/sh emulation) or 'native' (os/exec native shell)."`
	Justification   string   `json:"justification,omitempty" description:"Why this command is needed (for audit trail)"`
	PrefixRule      []string `json:"prefix_rule,omitempty" description:"Commands to prepend (e.g., ['timeout', '30'] for timeout)"`
}

func (p *BashParams) UnmarshalJSON(data []byte) error {
	type rawBashParams struct {
		Description      string   `json:"description"`
		Command          string   `json:"command"`
		Cmd              string   `json:"cmd"`
		BashCommand      string   `json:"bash_command"`
		Script           string   `json:"script"`
		WorkingDir       string   `json:"working_dir"`
		WorkingDirectory string   `json:"working_directory"`
		RunInBackground  bool     `json:"run_in_background"`
		Background       bool     `json:"background"`
		Backend          string   `json:"backend"`
		Justification    string   `json:"justification"`
		PrefixRule       []string `json:"prefix_rule"`
	}

	var raw rawBashParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.Description = strings.TrimSpace(raw.Description)
	p.Command = strings.TrimSpace(cmp.Or(raw.Command, raw.Cmd, raw.BashCommand, raw.Script))
	p.WorkingDir = strings.TrimSpace(cmp.Or(raw.WorkingDir, raw.WorkingDirectory))
	p.RunInBackground = raw.RunInBackground || raw.Background
	p.Backend = strings.TrimSpace(raw.Backend)
	p.Justification = strings.TrimSpace(raw.Justification)
	p.PrefixRule = raw.PrefixRule
	return nil
}

type BashPermissionsParams struct {
	Description     string   `json:"description"`
	Command         string   `json:"command"`
	WorkingDir      string   `json:"working_dir"`
	RunInBackground bool     `json:"run_in_background"`
	Backend         string   `json:"backend,omitempty"`
	Justification   string   `json:"justification,omitempty"`
	PrefixRule      []string `json:"prefix_rule,omitempty"`
}

type BashResponseMetadata struct {
	StartTime        int64  `json:"start_time"`
	EndTime          int64  `json:"end_time"`
	Output           string `json:"output"`
	Description      string `json:"description"`
	WorkingDirectory string `json:"working_directory"`
	Background       bool   `json:"background,omitempty"`
	ShellID          string `json:"shell_id,omitempty"`
	Justification    string `json:"justification,omitempty"`
}

const (
	BashToolName = "bash"

	MaxOutputLength = 30000
	BashNoOutput    = "no output"
	bashTimeout     = 20 * time.Second
	foregroundGrace = 750 * time.Millisecond
)

//go:embed bash.tpl
var bashDescriptionTmpl []byte

var bashDescriptionTpl = template.Must(
	template.New("bashDescription").
		Parse(string(bashDescriptionTmpl)),
)

type bashDescriptionData struct {
	BannedCommands  string
	MaxOutputLength int
	Attribution     config.Attribution
	ModelName       string
}

var bannedCommands = []string{
	// Network/Download tools
	"alias",
	"aria2c",
	"axel",
	"chrome",
	"curl",
	"curlie",
	"firefox",
	"http-prompt",
	"httpie",
	"links",
	"lynx",
	"nc",
	"safari",
	"scp",
	"ssh",
	"telnet",
	"w3m",
	"wget",
	"xh",

	// System administration
	"doas",
	"su",
	"sudo",

	// Package managers
	"apk",
	"apt",
	"apt-cache",
	"apt-get",
	"dnf",
	"dpkg",
	"emerge",
	"home-manager",
	"makepkg",
	"opkg",
	"pacman",
	"paru",
	"pkg",
	"pkg_add",
	"pkg_delete",
	"portage",
	"rpm",
	"yay",
	"yum",
	"zypper",

	// System modification
	"at",
	"batch",
	"chkconfig",
	"crontab",
	"fdisk",
	"mkfs",
	"mount",
	"parted",
	"service",
	"systemctl",
	"umount",

	// Network configuration
	"firewall-cmd",
	"ifconfig",
	"ip",
	"iptables",
	"netstat",
	"pfctl",
	"route",
	"ufw",
}

// bashDescription generates the tool description by executing the template with system-specific banned commands.
func bashDescription(attribution *config.Attribution, modelName string) string {
	bannedCommandsStr := strings.Join(bannedCommands, ", ")
	var out bytes.Buffer
	if err := bashDescriptionTpl.Execute(&out, bashDescriptionData{
		BannedCommands:  bannedCommandsStr,
		MaxOutputLength: MaxOutputLength,
		Attribution:     *attribution,
		ModelName:       modelName,
	}); err != nil {
		// this should never happen.
		panic("failed to execute bash description template: " + err.Error())
	}
	return out.String()
}

func blockFuncs() []shell.BlockFunc {
	return []shell.BlockFunc{
		shell.CommandsBlocker(bannedCommands),

		// System package managers
		shell.ArgumentsBlocker("apk", []string{"add"}, nil),
		shell.ArgumentsBlocker("apt", []string{"install"}, nil),
		shell.ArgumentsBlocker("apt-get", []string{"install"}, nil),
		shell.ArgumentsBlocker("dnf", []string{"install"}, nil),
		shell.ArgumentsBlocker("pacman", nil, []string{"-S"}),
		shell.ArgumentsBlocker("pkg", []string{"install"}, nil),
		shell.ArgumentsBlocker("yum", []string{"install"}, nil),
		shell.ArgumentsBlocker("zypper", []string{"install"}, nil),

		// Language-specific package managers
		shell.ArgumentsBlocker("brew", []string{"install"}, nil),
		shell.ArgumentsBlocker("cargo", []string{"install"}, nil),
		shell.ArgumentsBlocker("gem", []string{"install"}, nil),
		shell.ArgumentsBlocker("go", []string{"install"}, nil),
		shell.ArgumentsBlocker("npm", []string{"install"}, []string{"--global"}),
		shell.ArgumentsBlocker("npm", []string{"install"}, []string{"-g"}),
		shell.ArgumentsBlocker("pip", []string{"install"}, []string{"--user"}),
		shell.ArgumentsBlocker("pip3", []string{"install"}, []string{"--user"}),
		shell.ArgumentsBlocker("pnpm", []string{"add"}, []string{"--global"}),
		shell.ArgumentsBlocker("pnpm", []string{"add"}, []string{"-g"}),
		shell.ArgumentsBlocker("yarn", []string{"global", "add"}, nil),

		// `go test -exec` can run arbitrary commands
		shell.ArgumentsBlocker("go", []string{"test"}, []string{"-exec"}),
	}
}

func isForbiddenGitAgentCommand(command string) (bool, string) {
	segments := splitCommandSegments(command)
	for _, seg := range segments {
		fields := strings.Fields(seg)
		if len(fields) < 2 || fields[0] != "git" {
			continue
		}
		switch fields[1] {
		case "push":
			return true, "git push is blocked for agents; push remains human-controlled"
		case "merge":
			return true, "git merge is blocked for agents; integration remains human-controlled"
		case "rebase":
			return true, "git rebase is blocked for agents"
		case "restore":
			return true, "git restore is blocked for agents"
		case "clean":
			return true, "git clean is blocked for agents"
		case "reset":
			for _, arg := range fields[2:] {
				if strings.EqualFold(arg, "--hard") {
					return true, "git reset --hard is blocked for agents"
				}
			}
		case "worktree":
			if len(fields) >= 3 && fields[2] == "remove" {
				return true, "git worktree remove is blocked for agents"
			}
		case "branch":
			for _, arg := range fields[2:] {
				if arg == "-D" || arg == "-d" {
					return true, "git branch deletion is blocked for agents"
				}
			}
		}
	}
	return false, ""
}

func splitCommandSegments(command string) []string {
	parts := strings.FieldsFunc(command, func(r rune) bool {
		switch r {
		case ';', '&', '|':
			return true
		default:
			return false
		}
	})
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		segment := strings.TrimSpace(part)
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func NewBashTool(permissions permission.Service, workingDir string, attribution *config.Attribution, modelName string) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		BashToolName,
		string(bashDescription(attribution, modelName)),
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}

			if !params.RunInBackground {
				toolCtx, cancel := context.WithTimeout(ctx, bashTimeout)
				defer cancel()
				ctx = toolCtx
			}

			params.Command = strings.TrimSpace(params.Command)
			if params.Command == "" {
				return fantasy.NewTextErrorResponse("command is required"), nil
			}

			if len(params.PrefixRule) > 0 {
				prefixStr := strings.Join(params.PrefixRule, " ")
				params.Command = prefixStr + " " + params.Command
			}

			// Determine working directory
			execWorkingDir := cmp.Or(params.WorkingDir, workingDir)
			if blocked, reason := isForbiddenGitAgentCommand(params.Command); blocked {
				return fantasy.NewTextErrorResponse(reason), nil
			}

			isSafeReadOnly := false
			cmdLower := strings.ToLower(params.Command)

			for _, safe := range safeCommands {
				if strings.HasPrefix(cmdLower, safe) {
					if len(cmdLower) == len(safe) || cmdLower[len(safe)] == ' ' || cmdLower[len(safe)] == '-' {
						isSafeReadOnly = true
						break
					}
				}
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for executing shell command")
			}
			if !isSafeReadOnly {
				p, err := permissions.Request(ctx,
					permission.CreatePermissionRequest{
						SessionID:   sessionID,
						Path:        execWorkingDir,
						ToolCallID:  call.ID,
						ToolName:    BashToolName,
						Action:      "execute",
						Description: fmt.Sprintf("Execute command: %s", params.Command),
						Params: BashPermissionsParams{
							Description:     params.Description,
							Command:         params.Command,
							WorkingDir:      params.WorkingDir,
							RunInBackground: params.RunInBackground,
							Backend:         params.Backend,
							Justification:   params.Justification,
							PrefixRule:      params.PrefixRule,
						},
					},
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !p {
					return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
				}
			}

			// If explicitly requested as background, start immediately with detached context
			if params.RunInBackground {
				startTime := time.Now()
				bgManager := shell.GetFastBackgroundShellManager()
				bgManager.Cleanup()
				// Use background context so it continues after tool returns
				bgShell, err := bgManager.Start(context.Background(), execWorkingDir, blockFuncs(), params.Command, params.Description, params.Backend)
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("error starting background shell: %w", err)
				}
				setLastBackgroundShellID(sessionID, bgShell.ID)

				// Immediate fast-failure check (no waiting).
				stdout, _, _, stderr, _, _, done, execErr := bgShell.GetOutputSince(0, 0)

				if !done {
					waitCtx, cancelWait := context.WithTimeout(ctx, foregroundGrace)
					_ = bgShell.WaitContext(waitCtx)
					cancelWait()
					stdout, stderr, done, execErr = bgShell.GetOutput()
				}

				if done {
					// Command failed or completed very quickly
					bgManager.Remove(bgShell.ID)

					interrupted := shell.IsInterrupt(execErr)
					exitCode := shell.ExitCode(execErr)
					if exitCode == 0 && !interrupted && execErr != nil {
						return fantasy.ToolResponse{}, fmt.Errorf("[Job %s] error executing command: %w", bgShell.ID, execErr)
					}

					stdout = formatOutput(stdout, stderr, execErr)

					metadata := BashResponseMetadata{
						StartTime:        startTime.UnixMilli(),
						EndTime:          time.Now().UnixMilli(),
						Output:           stdout,
						Description:      params.Description,
						Background:       params.RunInBackground,
						WorkingDirectory: bgShell.WorkingDir,
						Justification:    params.Justification,
					}
					if stdout == "" {
						return fantasy.WithResponseMetadata(fantasy.NewTextResponse(BashNoOutput), metadata), nil
					}
					stdout += fmt.Sprintf("\n\n<cwd>%s</cwd>", normalizeWorkingDir(bgShell.WorkingDir))
					return fantasy.WithResponseMetadata(fantasy.NewTextResponse(stdout), metadata), nil
				}

				// Still running after fast-failure check - return as background job
				metadata := BashResponseMetadata{
					StartTime:        startTime.UnixMilli(),
					EndTime:          time.Now().UnixMilli(),
					Description:      params.Description,
					WorkingDirectory: bgShell.WorkingDir,
					Background:       true,
					ShellID:          bgShell.ID,
					Justification:    params.Justification,
				}
				response := fmt.Sprintf("Background shell started with ID: %s\n\nUse job_output tool to view output or job_kill to terminate.", bgShell.ID)
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
			}

			// Start synchronous execution with auto-background support
			startTime := time.Now()

			// Start with detached context so it can survive if moved to background
			bgManager := shell.GetFastBackgroundShellManager()
			bgManager.Cleanup()

			// Military-grade safeguard: Check context before starting shell
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}

			bgShell, err := bgManager.Start(context.Background(), execWorkingDir, blockFuncs(), params.Command, params.Description, params.Backend)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error starting shell: %w", err)
			}

			watchDone := make(chan struct{})
			defer close(watchDone)
			go func() {
				select {
				case <-ctx.Done():
					_ = bgManager.Kill(bgShell.ID)
				case <-watchDone:
				}
			}()

			waitCtx, cancelWait := context.WithTimeout(ctx, foregroundGrace)
			_ = bgShell.WaitContext(waitCtx)
			cancelWait()

			var stdout, stderr string
			var done bool
			var execErr error
			stdout, stderr, done, execErr = bgShell.GetOutput()
			if !done {
				runtime.Gosched()
				stdout, stderr, done, execErr = bgShell.GetOutput()
			}
			if !done {
				select {
				case <-ctx.Done():
					bgManager.Kill(bgShell.ID)
					return fantasy.ToolResponse{}, ctx.Err()
				default:
				}
			}

			if done {
				// Command completed within threshold - return synchronously
				// Remove from background manager since we're returning directly
				// Don't call Kill() as it cancels the context and corrupts the exit code
				bgManager.Remove(bgShell.ID)

				interrupted := shell.IsInterrupt(execErr)
				exitCode := shell.ExitCode(execErr)
				if exitCode == 0 && !interrupted && execErr != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("[Job %s] error executing command: %w", bgShell.ID, execErr)
				}

				stdout = formatOutput(stdout, stderr, execErr)

				metadata := BashResponseMetadata{
					StartTime:        startTime.UnixMilli(),
					EndTime:          time.Now().UnixMilli(),
					Output:           stdout,
					Description:      params.Description,
					Background:       params.RunInBackground,
					WorkingDirectory: bgShell.WorkingDir,
					Justification:    params.Justification,
				}
				if stdout == "" {
					return fantasy.WithResponseMetadata(fantasy.NewTextResponse(BashNoOutput), metadata), nil
				}
				stdout += fmt.Sprintf("\n\n<cwd>%s</cwd>", normalizeWorkingDir(bgShell.WorkingDir))
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(stdout), metadata), nil
			}

			// Still running - keep as background job
			metadata := BashResponseMetadata{
				StartTime:        startTime.UnixMilli(),
				EndTime:          time.Now().UnixMilli(),
				Description:      params.Description,
				WorkingDirectory: bgShell.WorkingDir,
				Background:       true,
				ShellID:          bgShell.ID,
				Justification:    params.Justification,
			}
			setLastBackgroundShellID(sessionID, bgShell.ID)
			response := fmt.Sprintf("Command is taking longer than expected and has been moved to background.\n\nBackground shell ID: %s\n\nUse job_output tool to view output or job_kill to terminate.", bgShell.ID)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
		})
}

// formatOutput formats the output of a completed command with error handling
func formatOutput(stdout, stderr string, execErr error) string {
	interrupted := shell.IsInterrupt(execErr)
	exitCode := shell.ExitCode(execErr)

	stdout = truncateOutput(stdout)
	stderr = truncateOutput(stderr)

	errorMessage := stderr
	if errorMessage == "" && execErr != nil {
		errorMessage = execErr.Error()
	}

	if interrupted {
		if errorMessage != "" {
			errorMessage += "\n"
		}
		errorMessage += "Command was aborted before completion"
	} else if exitCode != 0 {
		if errorMessage != "" {
			errorMessage += "\n"
		}
		errorMessage += fmt.Sprintf("Exit code %d", exitCode)
	}

	hasBothOutputs := stdout != "" && stderr != ""

	if hasBothOutputs {
		stdout += "\n"
	}

	if errorMessage != "" {
		stdout += "\n" + errorMessage
	}

	return stdout
}

func truncateOutput(content string) string {
	if len(content) <= MaxOutputLength {
		return content
	}

	halfLength := MaxOutputLength / 2
	start := content[:halfLength]
	end := content[len(content)-halfLength:]

	truncatedLinesCount := countLines(content[halfLength : len(content)-halfLength])
	return fmt.Sprintf("%s\n\n... [%d lines truncated] ...\n\n%s", start, truncatedLinesCount, end)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

func normalizeWorkingDir(path string) string {
	if runtime.GOOS == "windows" {
		path = strings.ReplaceAll(path, fsext.WindowsWorkingDirDrive(), "")
	}
	return filepath.ToSlash(path)
}

// JobListResponseMetadata is metadata for job list responses.
type JobListResponseMetadata struct {
	Jobs []struct {
		ShellID          string `json:"shell_id"`
		Status           string `json:"status"`
		Description      string `json:"description"`
		Command          string `json:"command"`
		WorkingDirectory string `json:"working_directory"`
	} `json:"jobs"`
}
