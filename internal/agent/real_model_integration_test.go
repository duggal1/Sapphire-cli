//go:build integration

package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	integrationEnvFlag = "SAPPHIRE_INTEGRATION"
	modelEnvFlag       = "SAPPHIRE_MODEL"
)

func TestGeminiToolCallsPhaseI(t *testing.T) {
	if os.Getenv(integrationEnvFlag) != "1" {
		t.Skip("integration test disabled")
	}

	bin := buildSapphireBinary(t)
	model := integrationModel()
	workDir := t.TempDir()

	prompts := make([]string, 0, 8)
	for i := 1; i <= 5; i++ {
		prompts = append(prompts, fmt.Sprintf(
			"Create file tool_test_%02d.txt with text 'line %02d'. Read it. Replace the line with 'updated %02d'.",
			i, i, i,
		))
	}
	prompts = append(prompts,
		"Run `ls -la` and report how many files are in the current directory.",
		"List available MCPs and report the total count only.",
		"Create file tool_test_final.txt with text 'done' and read it back.",
	)

	for i, prompt := range prompts {
		t.Run(fmt.Sprintf("tool-run-%02d", i+1), func(t *testing.T) {
			out := runSapphirePrompt(t, bin, workDir, model, prompt)
			assertNoToolErrors(t, out)
		})
	}
}

func TestGeminiAutonomyPhaseII(t *testing.T) {
	if os.Getenv(integrationEnvFlag) != "1" {
		t.Skip("integration test disabled")
	}

	bin := buildSapphireBinary(t)
	model := integrationModel()
	workDir := t.TempDir()

	prompt := "Create a minimal Go CLI in this repo with main.go and a README.md. The CLI should print 'ok' on run. Keep the code minimal and correct."
	out := runSapphirePrompt(t, bin, workDir, model, prompt)
	assertNoToolErrors(t, out)

	mainPath := filepath.Join(workDir, "main.go")
	readmePath := filepath.Join(workDir, "README.md")
	if _, err := os.Stat(mainPath); err != nil {
		t.Fatalf("expected main.go to be created: %v", err)
	}
	if _, err := os.Stat(readmePath); err != nil {
		t.Fatalf("expected README.md to be created: %v", err)
	}
}

func integrationModel() string {
	if model := strings.TrimSpace(os.Getenv(modelEnvFlag)); model != "" {
		return model
	}
	return "gemini-3.1-flash-lite-preview"
}

func buildSapphireBinary(t *testing.T) string {
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("failed to locate repo root: %v", err)
	}
	prebuilt := filepath.Join(root, "sapphire")
	if info, err := os.Stat(prebuilt); err == nil && info.Mode().IsRegular() {
		return prebuilt
	}
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "sapphire")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true",
		"GOPROXY=https://proxy.golang.org",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("failed to build sapphire: build exceeded 2 minutes")
		}
		t.Fatalf("failed to build sapphire: %v\n%s", err, out)
	}
	return binPath
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func runSapphirePrompt(t *testing.T, binPath, workDir, model, prompt string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "run", "--cwd", workDir, "-m", model, prompt)
	cmd.Env = os.Environ()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("model stall: exceeded 10s timeout\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
		}
		t.Fatalf("run failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	return stdout.String() + "\n" + stderr.String()
}

func assertNoToolErrors(t *testing.T, output string) {
	if strings.Contains(output, "ERROR") || strings.Contains(output, "tool not found") {
		t.Fatalf("tool error detected in output:\n%s", output)
	}
}
