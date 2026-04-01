package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ContextEvidence struct {
	Structured   []string
	Read         []string
	Verification []string
}

func recordContextEvidence(state *ToolUsageState, toolName string, input map[string]any) {
	if state == nil {
		return
	}
	evidence := ExtractContextEvidence(toolName, input)
	state.MarkStructuredEvidence(evidence.Structured...)
	state.MarkReadEvidence(evidence.Read...)
	state.MarkVerificationEvidence(evidence.Verification...)
}

func ExtractContextEvidence(toolName string, input map[string]any) ContextEvidence {
	canonical := canonicalToolNameForModePolicy(toolName)
	if canonical == "" {
		canonical = normalizeToolName(toolName)
	}
	switch canonical {
	case ToolSearchToolName, RGFilesToolName, RGToolName, GlobToolName, GrepToolName, LSToolName:
		return ContextEvidence{Structured: extractStructuredEvidence(canonical, input)}
	case AgenticViewToolName, ViewToolName, SingleViewToolName:
		readTargets := extractReadEvidence(canonical, input)
		verificationTargets := []string(nil)
		if len(readTargets) > 0 {
			for _, target := range readTargets {
				if looksLikeVerificationTarget(target) {
					verificationTargets = append(verificationTargets, target)
				}
			}
		}
		return ContextEvidence{
			Read:         readTargets,
			Verification: compactContextEvidence(verificationTargets),
		}
	case DiagnosticsToolName:
		return ContextEvidence{Verification: []string{DiagnosticsToolName}}
	case BashToolName:
		command, _ := input["command"].(string)
		command = strings.TrimSpace(command)
		if !looksLikeValidationCommand(command) {
			return ContextEvidence{}
		}
		return ContextEvidence{Verification: []string{normalizeContextEvidence(command)}}
	case "python":
		code, _ := input["code"].(string)
		code = strings.TrimSpace(code)
		if !looksLikeValidationSnippet(code) {
			return ContextEvidence{}
		}
		return ContextEvidence{Verification: []string{"python_validation"}}
	default:
		return ContextEvidence{}
	}
}

func extractStructuredEvidence(toolName string, input map[string]any) []string {
	values := []string{}
	switch toolName {
	case ToolSearchToolName:
		values = append(values, collectLooseEvidenceStrings(input["query"])...)
		values = append(values, collectLooseEvidenceStrings(input["q"])...)
	case RGFilesToolName:
		values = append(values, collectLooseEvidenceStrings(input["query"])...)
		values = append(values, collectLooseEvidenceStrings(input["pattern"])...)
		values = append(values, collectLooseEvidenceStrings(input["glob"])...)
		values = append(values, collectLooseEvidenceStrings(input["path"])...)
	case RGToolName, GrepToolName:
		values = append(values, collectLooseEvidenceStrings(input["query"])...)
		values = append(values, collectLooseEvidenceStrings(input["pattern"])...)
		values = append(values, collectLooseEvidenceStrings(input["file_path"])...)
		values = append(values, collectLooseEvidenceStrings(input["path"])...)
	case GlobToolName:
		values = append(values, collectLooseEvidenceStrings(input["pattern"])...)
		values = append(values, collectLooseEvidenceStrings(input["path"])...)
	case LSToolName:
		values = append(values, collectLooseEvidenceStrings(input["path"])...)
	}
	return compactContextEvidence(values)
}

func extractReadEvidence(toolName string, input map[string]any) []string {
	values := []string{}
	switch toolName {
	case SingleViewToolName:
		values = append(values, collectLooseEvidenceStrings(input["file_path"])...)
		values = append(values, collectLooseEvidenceStrings(input["path"])...)
	case ViewToolName, AgenticViewToolName:
		values = append(values, collectLooseEvidenceStrings(input["file_path"])...)
		values = append(values, collectLooseEvidenceStrings(input["path"])...)
		values = append(values, collectLooseEvidenceStrings(input["files"])...)
		values = append(values, collectLooseEvidenceStrings(input["paths"])...)
	}
	return compactContextEvidence(values)
}

func collectLooseEvidenceStrings(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{normalizeContextEvidence(typed)}
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeContextEvidence(item))
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, collectLooseEvidenceStrings(item)...)
		}
		return out
	case fmt.Stringer:
		return []string{normalizeContextEvidence(typed.String())}
	default:
		return nil
	}
}

func compactContextEvidence(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeContextEvidence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.Join(strings.Fields(value), " ")
	if looksLikePathLikeEvidence(value) {
		value = filepath.ToSlash(filepath.Clean(value))
	}
	if len(value) > 160 {
		value = strings.TrimSpace(value[:159]) + "…"
	}
	return value
}

func looksLikePathLikeEvidence(value string) bool {
	if value == "" {
		return false
	}
	if strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return true
	}
	lower := strings.ToLower(value)
	return strings.HasSuffix(lower, ".go") ||
		strings.HasSuffix(lower, ".ts") ||
		strings.HasSuffix(lower, ".tsx") ||
		strings.HasSuffix(lower, ".js") ||
		strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".json") ||
		strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".yml")
}

func looksLikeVerificationTarget(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "agents.md") ||
		strings.Contains(lower, "agent.md") ||
		strings.Contains(lower, "readme.md") ||
		strings.Contains(lower, "design") ||
		strings.Contains(lower, "spec")
}

func looksLikeValidationCommand(command string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	if command == "" {
		return false
	}
	prefixes := []string{
		"go test", "go build", "go vet", "cargo test", "cargo check",
		"npm test", "npm run test", "npm run build", "pnpm test", "pnpm build",
		"yarn test", "yarn build", "bun test", "bun run test", "pytest",
		"vitest", "jest", "ruff check", "golangci-lint", "make test", "make build",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}

func looksLikeValidationSnippet(code string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return false
	}
	signals := []string{
		"pytest", "unittest", "assert ", "go test", "cargo test", "mypy", "ruff", "compileall", "tsc", "eslint", "vitest",
	}
	for _, signal := range signals {
		if strings.Contains(code, signal) {
			return true
		}
	}
	return false
}
