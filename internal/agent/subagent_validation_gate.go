package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	validationGateTimeout     = 30 * time.Second
	validationDiffMaxLines    = 500
	validationBuildTimeout    = 60 * time.Second
	validationTestTimeout     = 120 * time.Second
	validationLintTimeout     = 60 * time.Second
	validationSecurityTimeout = 120 * time.Second
)

// validationResult holds the outcome of a worktree validation gate.
type validationResult struct {
	Passed         bool   `json:"passed"`
	DiffSummary    string `json:"diff_summary,omitempty"`
	BuildOutput    string `json:"build_output,omitempty"`
	TestOutput     string `json:"test_output,omitempty"`
	LintOutput     string `json:"lint_output,omitempty"`
	SecurityOutput string `json:"security_output,omitempty"`
	Errors         string `json:"errors,omitempty"`
	HasChanges     bool   `json:"has_changes"`
}

// validateWorktreeResult runs the validation gate on a completed sub-agent worktree.
// It diffs against the base branch, then runs build, test, lint, and security checks
// in parallel (they are independent). Validation is best-effort and never blocks —
// it returns a report even on failure.
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

	// Early exit: zero changes means no further validation needed
	if !result.HasChanges {
		result.BuildOutput = "no build command detected"
		result.TestOutput = "no test command detected"
		result.LintOutput = "no lint command detected"
		result.SecurityOutput = "no security command detected"
		return result
	}

	// Phases 2-5: Run build, test, lint, security in parallel
	// Use max individual timeout since they run concurrently
	vCtx, vCancel := context.WithTimeout(ctx, validationTestTimeout)
	defer vCancel()
	type checkResult struct {
		passed bool
		output string
		errMsg string
		check  string // "build", "test", "lint", "security"
	}
	checkCh := make(chan checkResult, 4)
	var checkWg sync.WaitGroup

	// Phase 2: Build verification
	buildCmd := detectBuildCommand(worktreeDir)
	if buildCmd != "" {
		checkWg.Add(1)
		go func() {
			defer checkWg.Done()
			buildCtx, buildCancel := context.WithTimeout(vCtx, validationBuildTimeout)
			defer buildCancel()
			buildOut, buildErr := runWorktreeShellCommand(buildCtx, worktreeDir, buildCmd)
			checkCh <- checkResult{
				passed: buildErr == nil,
				output: firstNonEmptyString(buildOut, "build passed"),
				errMsg: func() string {
					if buildErr != nil {
						return fmt.Sprintf("build failed: %s", buildErr)
					}
					return ""
				}(),
				check: "build",
			}
		}()
	}

	// Phase 3: Test verification
	effectiveTestCmd := strings.TrimSpace(testCommand)
	if effectiveTestCmd == "" {
		effectiveTestCmd = detectTestCommand(worktreeDir)
	}
	if effectiveTestCmd != "" {
		checkWg.Add(1)
		go func() {
			defer checkWg.Done()
			testCtx, testCancel := context.WithTimeout(vCtx, validationTestTimeout)
			defer testCancel()
			testOut, testErr := runWorktreeShellCommand(testCtx, worktreeDir, effectiveTestCmd)
			checkCh <- checkResult{
				passed: testErr == nil,
				output: firstNonEmptyString(testOut, "tests passed"),
				errMsg: func() string {
					if testErr != nil {
						return fmt.Sprintf("tests failed: %s", testErr)
					}
					return ""
				}(),
				check: "test",
			}
		}()
	}

	// Phase 4: Lint verification
	lintCmd := detectLintCommand(worktreeDir)
	if lintCmd != "" {
		checkWg.Add(1)
		go func() {
			defer checkWg.Done()
			lintCtx, lintCancel := context.WithTimeout(vCtx, validationLintTimeout)
			defer lintCancel()
			lintOut, lintErr := runWorktreeShellCommand(lintCtx, worktreeDir, lintCmd)
			checkCh <- checkResult{
				passed: lintErr == nil,
				output: firstNonEmptyString(lintOut, "lint passed"),
				errMsg: func() string {
					if lintErr != nil {
						return fmt.Sprintf("lint failed: %s", lintErr)
					}
					return ""
				}(),
				check: "lint",
			}
		}()
	}

	// Phase 5: Security verification
	securityCmd := detectSecurityCommand(worktreeDir)
	if securityCmd != "" {
		checkWg.Add(1)
		go func() {
			defer checkWg.Done()
			secCtx, secCancel := context.WithTimeout(vCtx, validationSecurityTimeout)
			defer secCancel()
			secOut, secErr := runWorktreeShellCommand(secCtx, worktreeDir, securityCmd)
			checkCh <- checkResult{
				passed: secErr == nil,
				output: firstNonEmptyString(secOut, "security scan passed"),
				errMsg: func() string {
					if secErr != nil {
						return fmt.Sprintf("security scan failed: %s", secErr)
					}
					return ""
				}(),
				check: "security",
			}
		}()
	}

	// Close channel when all checks complete
	go func() {
		checkWg.Wait()
		close(checkCh)
	}()

	// Collect results
	var errors []string
	for cr := range checkCh {
		switch cr.check {
		case "build":
			result.BuildOutput = truncateOutput(cr.output, 2000)
			if !cr.passed {
				result.Passed = false
				errors = append(errors, cr.errMsg)
			}
		case "test":
			result.TestOutput = truncateOutput(cr.output, 2000)
			if !cr.passed {
				result.Passed = false
				errors = append(errors, cr.errMsg)
			}
		case "lint":
			result.LintOutput = truncateOutput(cr.output, 2000)
			if !cr.passed {
				result.Passed = false
				errors = append(errors, cr.errMsg)
			}
		case "security":
			result.SecurityOutput = truncateOutput(cr.output, 2000)
			if !cr.passed {
				result.Passed = false
				errors = append(errors, cr.errMsg)
			}
		}
	}
	if result.Errors == "" && len(errors) > 0 {
		result.Errors = strings.Join(errors, "; ")
	} else if len(errors) > 0 {
		result.Errors += "; " + strings.Join(errors, "; ")
	}

	// Set defaults for checks that had no command
	if result.BuildOutput == "" {
		result.BuildOutput = "no build command detected"
	}
	if result.TestOutput == "" {
		result.TestOutput = "no test command detected"
	}
	if result.LintOutput == "" {
		result.LintOutput = "no lint command detected"
	}
	if result.SecurityOutput == "" {
		result.SecurityOutput = "no security command detected"
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
	if result.LintOutput != "" {
		builder.WriteString(fmt.Sprintf("Lint: %s\n", result.LintOutput))
	}
	if result.SecurityOutput != "" {
		builder.WriteString(fmt.Sprintf("Security: %s\n", result.SecurityOutput))
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

// detectLintCommand inspects the worktree to determine the lint command.
func detectLintCommand(worktreeDir string) string {
	if fileExists(worktreeDir, "Taskfile.yaml") {
		return "task lint"
	}
	if fileExists(worktreeDir, ".golangci.yml") || fileExists(worktreeDir, ".golangci.yaml") {
		return "golangci-lint run"
	}
	if fileExists(worktreeDir, "go.mod") {
		return "golangci-lint run"
	}
	if fileExists(worktreeDir, "package.json") {
		return "npm run lint --if-present"
	}
	if fileExists(worktreeDir, "Cargo.toml") {
		return "cargo clippy"
	}
	return ""
}

// detectSecurityCommand inspects the worktree to determine the security scan command.
func detectSecurityCommand(worktreeDir string) string {
	if fileExists(worktreeDir, "Taskfile.yaml") {
		return "task security"
	}
	if fileExists(worktreeDir, "go.mod") {
		return "gosec ./..."
	}
	if fileExists(worktreeDir, "package.json") {
		return "npm run security --if-present"
	}
	return ""
}

// fileExists checks if a file exists in the given directory using os.Stat
// instead of spawning a subprocess (much faster).
func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// truncateOutput truncates output to maxLen and adds a suffix if truncated.
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "\n... (truncated)"
}

func defaultSubAgentCommitMessage(title, taskKey string) string {
	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		cleanTitle = strings.TrimSpace(taskKey)
	}
	if cleanTitle == "" {
		cleanTitle = "sub-agent changes"
	}
	return fmt.Sprintf("agent: %s", cleanTitle)
}

func autoCommitWorktree(ctx context.Context, worktreeDir, message string) error {
	if strings.TrimSpace(worktreeDir) == "" {
		return fmt.Errorf("worktree dir is required")
	}
	_, err := runWorktreeCommand(ctx, worktreeDir, "git", "add", "-A")
	if err != nil {
		return err
	}
	output, err := runWorktreeCommand(ctx, worktreeDir, "git", "commit", "-m", message)
	if err != nil {
		if strings.Contains(strings.ToLower(output), "nothing to commit") {
			return finalizeSnapshotTip(ctx, worktreeDir, message)
		}
		return err
	}
	return nil
}

func finalizeSnapshotTip(ctx context.Context, worktreeDir, message string) error {
	lastSubject, err := runWorktreeCommand(ctx, worktreeDir, "git", "log", "-1", "--pretty=%s")
	if err != nil {
		return nil
	}
	lastSubject = strings.TrimSpace(lastSubject)
	if !strings.HasPrefix(lastSubject, "chore(snapshot): sapphire auto-snapshot ") && !strings.HasPrefix(lastSubject, "snapshot: ") {
		return nil
	}
	_, err = runWorktreeCommand(ctx, worktreeDir, "git", "commit", "--allow-empty", "-m", message)
	if err != nil {
		return err
	}
	return nil
}

// runValidationGateAsync runs the validation gate asynchronously and logs the result.
// It returns the validation result on the provided channel.
func runValidationGateAsync(ctx context.Context, worktreeDir, baseBranch, testCommand string, resultCh chan<- validationResult) {
	go func() {
		vCtx, vCancel := context.WithTimeout(ctx, validationBuildTimeout+validationTestTimeout+validationLintTimeout+validationSecurityTimeout+validationGateTimeout)
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
