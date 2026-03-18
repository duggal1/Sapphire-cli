package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
)

//go:embed apply_patch.md
var applyPatchDescription []byte

const ApplyPatchToolName = "apply_patch"

// ApplyPatchParams defines the parameters for the apply_patch tool.
type ApplyPatchParams struct {
	FilePath      string `json:"file_path" description:"The file to apply the patch to"`
	UnifiedDiff   string `json:"unified_diff" description:"The unified diff patch string to apply"`
	ExecutionMode string `json:"execution_mode,omitempty" description:"'direct' for Go memory manipulation or 'delegate' for system patch utility"`
	Justification string `json:"justification,omitempty" description:"Why this patch is being applied"`
}

func (p *ApplyPatchParams) UnmarshalJSON(data []byte) error {
	type rawApplyPatchParams struct {
		FilePath      string `json:"file_path"`
		File          string `json:"file"`
		Path          string `json:"path"`
		UnifiedDiff   string `json:"unified_diff"`
		Patch         string `json:"patch"`
		ExecutionMode string `json:"execution_mode"`
		Mode          string `json:"mode"`
		Justification string `json:"justification"`
	}

	var raw rawApplyPatchParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.FilePath = raw.FilePath
	if p.FilePath == "" {
		p.FilePath = raw.File
	}
	if p.FilePath == "" {
		p.FilePath = raw.Path
	}

	p.UnifiedDiff = raw.UnifiedDiff
	if p.UnifiedDiff == "" {
		p.UnifiedDiff = raw.Patch
	}

	p.ExecutionMode = raw.ExecutionMode
	if p.ExecutionMode == "" {
		p.ExecutionMode = raw.Mode
	}
	
	p.Justification = raw.Justification

	return nil
}

// NewApplyPatchTool creates a new tool for applying unified diffs directly or via delegate.
func NewApplyPatchTool(workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ApplyPatchToolName,
		string(applyPatchDescription),
		func(ctx context.Context, params ApplyPatchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}
			if params.UnifiedDiff == "" {
				return fantasy.NewTextErrorResponse("unified_diff is required"), nil
			}

			mode := strings.ToLower(strings.TrimSpace(params.ExecutionMode))
			if mode == "" {
				mode = "direct"
			}

			// Validate and resolve file path
			absPath := filepath.Join(workingDir, params.FilePath)
			if filepath.IsAbs(params.FilePath) {
				absPath = params.FilePath
			}

			// Verify file doesn't escape working dir constraints
			if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(workingDir)) {
				return fantasy.NewTextErrorResponse("cannot access files outside working directory"), nil
			}

			// Check file exists
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("file %s does not exist", params.FilePath)), nil
			}

			// File exists, proceed with mode.

			if mode == "delegate" {
				return applyPatchDelegate(ctx, absPath, params.UnifiedDiff, workingDir)
			}

			return applyPatchDirect(ctx, absPath, params.UnifiedDiff, workingDir)
		},
	)
}

func applyPatchDelegate(ctx context.Context, absPath, diff string, workingDir string) (fantasy.ToolResponse, error) {
	// 1. Write patch to temp file
	tmpFile, err := os.CreateTemp(workingDir, "agent_*.patch")
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("could not create temp patch file: %w", err)
	}
	patchPath := tmpFile.Name()
	defer os.Remove(patchPath)

	if _, err := tmpFile.WriteString(diff); err != nil {
		tmpFile.Close()
		return fantasy.ToolResponse{}, fmt.Errorf("could not write to temp patch file: %w", err)
	}
	tmpFile.Close()

	// 2. Execute patch command
	cmd := exec.CommandContext(ctx, "patch", "-p1", "-i", filepath.Base(patchPath))
	cmd.Dir = filepath.Dir(patchPath) // Run in same dir if possible, or adjust based on diff structure

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Provide some diagnostics back to the agent
		return fantasy.NewTextResponse(fmt.Sprintf("Failed to apply patch via delegate mode.\nError: %v\nOutput: %s", err, string(output))), nil
	}

	return fantasy.NewTextResponse(fmt.Sprintf("Successfully applied patch via delegate mode (system patch).\nOutput: %s", string(output))), nil
}

// applyPatchDirect uses the Codex-style fuzzy parser and applier built for Go memory manipulation.
func applyPatchDirect(ctx context.Context, absPath, diff string, workingDir string) (fantasy.ToolResponse, error) {
	hunks, err := ParsePatch(diff)
	if err != nil {
		return fantasy.NewTextResponse(fmt.Sprintf("Failed to parse patch: %v", err)), nil
	}

	affected, err := ApplyHunks(workingDir, hunks)
	if err != nil {
		return fantasy.NewTextResponse(fmt.Sprintf("Failed to apply patch: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString("Successfully applied unified diff patch using direct mode memory manipulation.\n")
	for p, action := range affected {
		// Just to be safe, get relative path nicely
		rel, err := filepath.Rel(workingDir, p)
		if err == nil {
			sb.WriteString(fmt.Sprintf("%s: %s\n", rel, action))
		} else {
			sb.WriteString(fmt.Sprintf("%s: %s\n", p, action))
		}
	}

	return fantasy.NewTextResponse(sb.String()), nil
}
