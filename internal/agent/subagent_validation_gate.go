package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

const (
	validationGateTimeout   = 30 * time.Second
	validationDiffMaxLines  = 500
	validationBuildTimeout  = 60 * time.Second
	validationTestTimeout   = 120 * time.Second
)

// validationResult holds the outcome of a worktree validation gate.
type validationResult struct {
	Passed      bool   `json:"passed"`
	DiffSummary string `json:"diff_summary,omitempty"`
	BuildOutput string `json:"build_output,omitempty"`
	TestOutput  string `json:"test_output,omitempty"`
	Errors      string `json:"errors,omitempty"`
	HasChanges  bool   `json:"has_changes"`
}

// validateWorktreeResult runs the validation gate on a completed sub-agent worktree.
// It diffs against the base branch, runs build, and optionally runs a test command.
// Validation is best-effort and never blocks — it returns a report even on failure.
func validateWorktreeResult(ctx context.Context, worktreeDir, baseBranch, testCommand string) validationResult {
	result := validationResult{Passed: true}

	// Phase 1: Git diff stat
	diffCtx, diffCancel := context.WithTimeout(ctx, validationGateTimeout)
	defer diffCancel()
	diffRef := "HEAD"
	if baseBranch != "" {
		diffRef = baseBranch
	}
	diffStat, err := runWorktreeCommand(diffCtx, worktreeDir, "git", "diff", "--stat", diffRef)
	if err != nil {
		result.DiffSummary = fmt.Sprintf("diff failed: %s", err)
	} else {
		diffStat = strings.TrimSpace(diffStat)
		if diffStat == "" {
			result.DiffSummary = "no changes"
			result.HasChanges = false
		} else {
			result.HasChanges = true
			lines := strings.Split(diffStat, "\n")
			if len(lines) > validationDiffMaxLines {
				result.DiffSummary = strings.Join(lines[:validationDiffMaxLines], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-validationDiffMaxLines)
			} else {
				result.DiffSummary = diffStat
			}
		}
	}

	// Phase 2: Build verification
	buildCmd := detectBuildCommand(worktreeDir)
	if buildCmd != "" {
		buildCtx, buildCancel := context.WithTimeout(ctx, validationBuildTimeout)
		defer buildCancel()
		buildOut, buildErr := runWorktreeShellCommand(buildCtx, worktreeDir, buildCmd)
		if buildErr != nil {
			result.Passed = false
			result.BuildOutput = truncateOutput(buildOut, 2000)
			if result.Errors == "" {
				result.Errors = fmt.Sprintf("build failed: %s", buildErr)
			}
		} else {
			result.BuildOutput = "build passed"
		}
	} else {
		result.BuildOutput = "no build command detected"
	}

	// Phase 3: Test verification (optional)
	testCmd := strings.TrimSpace(testCommand)
	if testCmd == "" {
		testCmd = detectTestCommand(worktreeDir)
	}
	if testCmd != "" {
		testCtx, testCancel := context.WithTimeout(ctx, validationTestTimeout)
		defer testCancel()
		testOut, testErr := runWorktreeShellCommand(testCtx, worktreeDir, testCmd)
		if testErr != nil {
			result.Passed = false
			result.TestOutput = truncateOutput(testOut, 2000)
			if result.Errors == "" {
				result.Errors = fmt.Sprintf("tests failed: %s", testErr)
			} else {
				result.Errors += fmt.Sprintf("; tests failed: %s", testErr)
			}
		} else {
			result.TestOutput = "tests passed"
		}
	} else {
		result.TestOutput = "no test command detected"
	}

	return result
}

// formatValidationReport formats a validation result into text for the sub-agent output.
func formatValidationReport(result validationResult) string {
	builder := &strings.Builder{}
	builder.WriteString("\n--- VALIDATION GATE ---\n")

	status := "PASSED"
	if !result.Passed {
		status = "FAILED"
	}
	builder.WriteString(fmt.Sprintf("Status: %s\n", status))

	if result.HasChanges {
		builder.WriteString(fmt.Sprintf("Changes: yes\n"))
	} else {
		builder.WriteString(fmt.Sprintf("Changes: none\n"))
	}

	if result.DiffSummary != "" {
		builder.WriteString(fmt.Sprintf("Diff: %s\n", result.DiffSummary))
	}
	if result.BuildOutput != "" {
		builder.WriteString(fmt.Sprintf("Build: %s\n", result.BuildOutput))
	}
	if result.TestOutput != "" {
		builder.WriteString(fmt.Sprintf("Test: %s\n", result.TestOutput))
	}
	if result.Errors != "" {
		builder.WriteString(fmt.Sprintf("Errors: %s\n", result.Errors))
	}

	builder.WriteString("--- END VALIDATION ---\n")
	return builder.String()
}

// runWorktreeCommand executes a command in a worktree directory and returns stdout.
func runWorktreeCommand(ctx context.Context, worktreeDir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = worktreeDir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// runWorktreeShellCommand executes a shell command string in a worktree directory.
func runWorktreeShellCommand(ctx context.Context, worktreeDir, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = worktreeDir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// detectBuildCommand inspects the worktree to determine the build command.
func detectBuildCommand(worktreeDir string) string {
	// Go project
	if fileExists(worktreeDir, "go.mod") {
		return "go build ./..."
	}
	// Node project
	if fileExists(worktreeDir, "package.json") {
		if fileExists(worktreeDir, "node_modules/.package-lock.json") || fileExists(worktreeDir, "package-lock.json") {
			return "npm run build --if-present"
		}
		if fileExists(worktreeDir, "yarn.lock") {
			return "yarn build --if-present"
		}
		return "npm run build --if-present"
	}
	// Rust project
	if fileExists(worktreeDir, "Cargo.toml") {
		return "cargo build"
	}
	// Python project
	if fileExists(worktreeDir, "pyproject.toml") || fileExists(worktreeDir, "setup.py") {
		return "python -m py_compile $(find . -name '*.py' -not -path './.*' | head -20)"
	}
	return ""
}

// detectTestCommand inspects the worktree to determine the test command.
func detectTestCommand(worktreeDir string) string {
	// Go project
	if fileExists(worktreeDir, "go.mod") {
		return "go test ./... -count=1 -short"
	}
	// Node project
	if fileExists(worktreeDir, "package.json") {
		return "npm test --if-present"
	}
	// Rust project
	if fileExists(worktreeDir, "Cargo.toml") {
		return "cargo test"
	}
	return ""
}

// fileExists checks if a file exists in the given directory.
func fileExists(dir, name string) bool {
	info, err := exec.Command("test", "-f", dir+"/"+name).CombinedOutput()
	_ = info
	return err == nil
}

// truncateOutput truncates output to maxLen and adds a suffix if truncated.
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "\n... (truncated)"
}

// runValidationGateAsync runs the validation gate asynchronously and logs the result.
// It returns the validation result on the provided channel.
func runValidationGateAsync(ctx context.Context, worktreeDir, baseBranch, testCommand string, resultCh chan<- validationResult) {
	go func() {
		vCtx, vCancel := context.WithTimeout(ctx, validationBuildTimeout+validationTestTimeout+validationGateTimeout)
		defer vCancel()
		result := validateWorktreeResult(vCtx, worktreeDir, baseBranch, testCommand)
		if !result.Passed {
			slog.Warn("Validation gate failed for worktree",
				"worktree", worktreeDir,
				"errors", result.Errors,
			)
		} else {
			slog.Info("Validation gate passed for worktree", "worktree", worktreeDir)
		}
		select {
		case resultCh <- result:
		default:
		}
	}()
}
