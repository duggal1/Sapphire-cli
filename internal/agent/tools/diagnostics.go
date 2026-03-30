package tools

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/duggal1/Sapphire-cli/internal/lsp"
)

type DiagnosticsParams struct {
	FilePath string `json:"file_path,omitempty" description:"The path to the file to get diagnostics for (leave empty for project diagnostics)"`
}

type DiagnosticsSummary struct {
	FileErrors       int
	FileWarnings     int
	ProjectErrors    int
	ProjectWarnings  int
	CompilerErrors   int
	CompilerWarnings int
}

const DiagnosticsToolName = "lsp_diagnostics"

//go:embed diagnostics.md
var diagnosticsDescription []byte

// NewDiagnosticsTool creates a tool for retrieving diagnostic messages from active LSP clients.
func NewDiagnosticsTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		DiagnosticsToolName,
		string(diagnosticsDescription),
		func(ctx context.Context, params DiagnosticsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if lspManager != nil && lspManager.Clients().Len() > 0 {
				notifyLSPs(ctx, lspManager, params.FilePath)
			}
			output := getDiagnostics(ctx, params.FilePath, lspManager)
			return fantasy.NewTextResponse(output), nil
		})
}

// openInLSPs ensures LSP servers are running and aware of the file, but does
// not notify changes or wait for fresh diagnostics. Use this for read-only
// operations like view where the file content hasn't changed.
func openInLSPs(
	ctx context.Context,
	manager *lsp.Manager,
	filepath string,
) {
	if filepath == "" || manager == nil {
		return
	}

	manager.Start(ctx, filepath)

	for client := range manager.Clients().Seq() {
		if !client.HandlesFile(filepath) {
			continue
		}
		_ = client.OpenFileOnDemand(ctx, filepath)
	}
}

// waitForLSPDiagnostics waits briefly for diagnostics publication after a file
// has been opened. Intended for read-only situations where viewing up-to-date
// files matters but latency should remain low (i.e. when using the view tool).
func waitForLSPDiagnostics(
	ctx context.Context,
	manager *lsp.Manager,
	filepath string,
	timeout time.Duration,
) {
	if filepath == "" || manager == nil || timeout <= 0 {
		return
	}

	var wg sync.WaitGroup
	for client := range manager.Clients().Seq() {
		if !client.HandlesFile(filepath) {
			continue
		}
		wg.Go(func() {
			client.WaitForDiagnostics(ctx, timeout)
		})
	}
	wg.Wait()
}

// notifyLSPs notifies LSP servers that a file has changed and waits for
// updated diagnostics. Use this after edit/multiedit operations.
func notifyLSPs(
	ctx context.Context,
	manager *lsp.Manager,
	filepath string,
) {
	if filepath == "" || manager == nil {
		return
	}

	manager.Start(ctx, filepath)

	var wg sync.WaitGroup
	for client := range manager.Clients().Seq() {
		if !client.HandlesFile(filepath) {
			continue
		}
		wg.Add(1)
		go func(c *lsp.Client) {
			defer wg.Done()
			_ = c.OpenFileOnDemand(ctx, filepath)
			_ = c.NotifyChange(ctx, filepath)
			c.WaitForDiagnostics(ctx, 2*time.Second) // Reduced from 5s for faster concurrency
		}(client)
	}
	wg.Wait()
}

func getDiagnostics(ctx context.Context, filePath string, manager *lsp.Manager) string {
	output, _ := getDiagnosticsWithSummary(ctx, filePath, manager)
	return output
}

func getDiagnosticsWithSummary(ctx context.Context, filePath string, manager *lsp.Manager) (string, DiagnosticsSummary) {
	var fileDiagnostics []string
	var projectDiagnostics []string

	if manager != nil {
		for lspName, client := range manager.Clients().Seq2() {
			for location, diags := range client.GetDiagnostics() {
				path, err := location.Path()
				if err != nil {
					slog.Error("Failed to convert diagnostic location URI to path", "uri", location, "error", err)
					continue
				}
				isCurrentFile := path == filePath
				for _, diag := range diags {
					formattedDiag := formatDiagnostic(path, diag, lspName)
					if isCurrentFile {
						fileDiagnostics = append(fileDiagnostics, formattedDiag)
					} else {
						projectDiagnostics = append(projectDiagnostics, formattedDiag)
					}
				}
			}
		}
	}

	sortDiagnostics(fileDiagnostics)
	sortDiagnostics(projectDiagnostics)

	var output strings.Builder
	writeDiagnostics(&output, "file_diagnostics", fileDiagnostics)
	writeDiagnostics(&output, "project_diagnostics", projectDiagnostics)

	summary := DiagnosticsSummary{}
	compilerDiagnostics := getCompilerDiagnostics(ctx, filePath)
	compilerFileOutput, compilerProjectOutput := splitCompilerOutputByFile(compilerDiagnostics.Output, filePath)
	if compilerFileOutput != "" {
		output.WriteString("\n<compiler_file_diagnostics>\n")
		output.WriteString(compilerFileOutput)
		output.WriteString("\n</compiler_file_diagnostics>\n")
		summary.CompilerErrors, summary.CompilerWarnings = countCompilerIssues(compilerFileOutput)
	}
	if compilerProjectOutput != "" {
		output.WriteString("\n<compiler_project_diagnostics>\n")
		output.WriteString(compilerProjectOutput)
		output.WriteString("\n</compiler_project_diagnostics>\n")
	}

	if len(fileDiagnostics) > 0 || len(projectDiagnostics) > 0 || summary.CompilerErrors > 0 || summary.CompilerWarnings > 0 {
		summary.FileErrors = countSeverity(fileDiagnostics, "Error")
		summary.FileWarnings = countSeverity(fileDiagnostics, "Warn")
		summary.ProjectErrors = countSeverity(projectDiagnostics, "Error")
		summary.ProjectWarnings = countSeverity(projectDiagnostics, "Warn")
		output.WriteString("\n<diagnostic_summary>\n")
		fmt.Fprintf(&output, "Current file: %d errors, %d warnings\n", summary.FileErrors, summary.FileWarnings)
		fmt.Fprintf(&output, "Project: %d errors, %d warnings\n", summary.ProjectErrors, summary.ProjectWarnings)
		if summary.CompilerErrors > 0 || summary.CompilerWarnings > 0 {
			fmt.Fprintf(&output, "Compiler (current file): %d errors, %d warnings\n", summary.CompilerErrors, summary.CompilerWarnings)
		}
		output.WriteString("</diagnostic_summary>\n")
		if summary.FileErrors+summary.FileWarnings+summary.CompilerErrors+summary.CompilerWarnings > 0 {
			output.WriteString("\n<diagnostic_gate>\n")
			output.WriteString("Current file is blocked. Fix all current-file errors and warnings to zero before editing other files or finishing.\n")
			output.WriteString("</diagnostic_gate>\n")
		}
	}

	out := output.String()
	slog.Debug("Diagnostics", "output", out)
	return out, summary
}

func writeDiagnostics(output *strings.Builder, tag string, in []string) {
	if len(in) == 0 {
		return
	}
	output.WriteString("\n<" + tag + ">\n")
	output.WriteString(strings.Join(in, "\n"))
	output.WriteString("\n</" + tag + ">\n")
}

func sortDiagnostics(in []string) []string {
	sort.Slice(in, func(i, j int) bool {
		iIsError := strings.HasPrefix(in[i], "Error")
		jIsError := strings.HasPrefix(in[j], "Error")
		if iIsError != jIsError {
			return iIsError // Errors come first
		}
		return in[i] < in[j] // Then alphabetically
	})
	return in
}

func formatDiagnostic(pth string, diagnostic protocol.Diagnostic, source string) string {
	severity := "Info"
	switch diagnostic.Severity {
	case protocol.SeverityError:
		severity = "Error"
	case protocol.SeverityWarning:
		severity = "Warn"
	case protocol.SeverityHint:
		severity = "Hint"
	}

	location := fmt.Sprintf("%s:%d:%d", pth, diagnostic.Range.Start.Line+1, diagnostic.Range.Start.Character+1)

	sourceInfo := source
	if diagnostic.Source != "" {
		sourceInfo += " " + diagnostic.Source
	}

	codeInfo := ""
	if diagnostic.Code != nil {
		codeInfo = fmt.Sprintf("[%v]", diagnostic.Code)
	}

	tagsInfo := ""
	if len(diagnostic.Tags) > 0 {
		var tags []string
		for _, tag := range diagnostic.Tags {
			switch tag {
			case protocol.Unnecessary:
				tags = append(tags, "unnecessary")
			case protocol.Deprecated:
				tags = append(tags, "deprecated")
			}
		}
		if len(tags) > 0 {
			tagsInfo = fmt.Sprintf(" (%s)", strings.Join(tags, ", "))
		}
	}

	return fmt.Sprintf("%s: %s [%s]%s%s %s",
		severity,
		location,
		sourceInfo,
		codeInfo,
		tagsInfo,
		diagnostic.Message)
}

func splitCompilerOutputByFile(output, filePath string) (string, string) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", ""
	}
	if strings.TrimSpace(filePath) == "" {
		return "", output
	}

	blocks := splitCompilerBlocks(output)
	fileBlocks := make([]string, 0, len(blocks))
	projectBlocks := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if compilerBlockMentionsFile(block, filePath) {
			fileBlocks = append(fileBlocks, block)
			continue
		}
		projectBlocks = append(projectBlocks, block)
	}
	return strings.Join(fileBlocks, "\n"), strings.Join(projectBlocks, "\n")
}

func splitCompilerBlocks(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	blocks := make([]string, 0, len(lines))
	current := make([]string, 0, 8)
	flush := func() {
		if len(current) == 0 {
			return
		}
		blocks = append(blocks, strings.Join(current, "\n"))
		current = current[:0]
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if len(current) > 0 && startsNewCompilerBlock(line) {
			flush()
		}
		current = append(current, line)
	}
	flush()
	return blocks
}

func startsNewCompilerBlock(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	if strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "^") || strings.HasPrefix(trimmed, "-->") {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "help:") || strings.HasPrefix(lower, "note:") {
		return false
	}
	return true
}

func compilerBlockMentionsFile(block, filePath string) bool {
	for _, line := range strings.Split(block, "\n") {
		if compilerLineMentionsFile(line, filePath) {
			return true
		}
	}
	return false
}

func compilerLineMentionsFile(line, filePath string) bool {
	if strings.TrimSpace(line) == "" || strings.TrimSpace(filePath) == "" {
		return false
	}

	normalizedLine := filepath.ToSlash(line)
	for _, token := range compilerPathTokens(filePath) {
		if token == "" {
			continue
		}
		if strings.Contains(normalizedLine, token+"(") ||
			strings.Contains(normalizedLine, token+":") ||
			strings.Contains(normalizedLine, `"`+token+`"`) ||
			strings.Contains(normalizedLine, token+", line ") ||
			normalizedLine == token {
			return true
		}
	}
	return false
}

func compilerPathTokens(filePath string) []string {
	clean := filepath.ToSlash(filepath.Clean(filePath))
	parts := strings.Split(clean, "/")
	seen := make(map[string]struct{}, len(parts))
	tokens := make([]string, 0, len(parts))
	add := func(token string) {
		token = strings.TrimSpace(token)
		if token == "" {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}

	add(clean)
	for i := range parts {
		add(strings.Join(parts[i:], "/"))
	}
	return tokens
}

func countSeverity(diagnostics []string, severity string) int {
	count := 0
	for _, diag := range diagnostics {
		if strings.HasPrefix(diag, severity) {
			count++
		}
	}
	return count
}
