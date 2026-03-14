package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/sapphire/internal/shell"
)

type CompilerDiagnostics struct {
	Output   string
	Errors   int
	Warnings int
}

func getCompilerDiagnostics(ctx context.Context, filePath string) CompilerDiagnostics {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return runGoDiagnostics(ctx, filePath)
	case ".ts", ".tsx", ".js", ".jsx":
		return runTypeScriptDiagnostics(ctx, filePath)
	case ".py":
		return runPythonDiagnostics(ctx, filePath)
	case ".rs":
		return runRustDiagnostics(ctx, filePath)
	default:
		return CompilerDiagnostics{}
	}
}

func runGoDiagnostics(ctx context.Context, filePath string) CompilerDiagnostics {
	pkgDir := filepath.Dir(filePath)
	_, ok := findUpwards(pkgDir, "go.mod")
	if !ok {
		return CompilerDiagnostics{}
	}
	return runCompilerCommand(ctx, pkgDir, "go test -run=^$")
}

func runTypeScriptDiagnostics(ctx context.Context, filePath string) CompilerDiagnostics {
	startDir := filepath.Dir(filePath)
	tsconfigPath, ok := findUpwards(startDir, "tsconfig.json")
	if !ok {
		return CompilerDiagnostics{}
	}
	root := filepath.Dir(tsconfigPath)
	tscPath := filepath.Join(root, "node_modules", ".bin", "tsc")
	command := fmt.Sprintf("%s --noEmit -p %s", tscPath, tsconfigPath)
	if _, err := os.Stat(tscPath); err != nil {
		command = fmt.Sprintf("npx tsc --noEmit -p %s", tsconfigPath)
	}
	return runCompilerCommand(ctx, root, command)
}

func runPythonDiagnostics(ctx context.Context, filePath string) CompilerDiagnostics {
	dir := filepath.Dir(filePath)
	command := fmt.Sprintf("python -m py_compile %s", shellEscape(filePath))
	return runCompilerCommand(ctx, dir, command)
}

func runRustDiagnostics(ctx context.Context, filePath string) CompilerDiagnostics {
	startDir := filepath.Dir(filePath)
	cargoPath, ok := findUpwards(startDir, "Cargo.toml")
	if !ok {
		return CompilerDiagnostics{}
	}
	root := filepath.Dir(cargoPath)
	return runCompilerCommand(ctx, root, "cargo check -q")
}

func runCompilerCommand(ctx context.Context, workdir, command string) CompilerDiagnostics {
	ctx, cancel := withBoundedTimeout(ctx, 12*time.Second)
	defer cancel()

	s := shell.NewShell(&shell.Options{WorkingDir: workdir})
	stdout, stderr, err := s.Exec(ctx, command)
	output := strings.TrimSpace(strings.Join([]string{stdout, stderr}, "\n"))

	errorsCount, warningsCount := countCompilerIssues(output)
	if err != nil && errorsCount == 0 {
		errorsCount = 1
	}

	if output == "" && err == nil {
		return CompilerDiagnostics{}
	}

	return CompilerDiagnostics{
		Output:   output,
		Errors:   errorsCount,
		Warnings: warningsCount,
	}
}

func countCompilerIssues(output string) (int, int) {
	if output == "" {
		return 0, 0
	}
	var errorsCount, warningsCount int
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "warning") {
			warningsCount++
			continue
		}
		if strings.Contains(lower, "error") {
			errorsCount++
		}
	}
	return errorsCount, warningsCount
}

func withBoundedTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) <= timeout {
			return context.WithDeadline(ctx, deadline)
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func findUpwards(start, target string) (string, bool) {
	dir := start
	for {
		candidate := filepath.Join(dir, target)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func shellEscape(path string) string {
	if path == "" {
		return path
	}
	if strings.ContainsAny(path, " \t\n\"'\\") {
		return fmt.Sprintf("%q", path)
	}
	return path
}
