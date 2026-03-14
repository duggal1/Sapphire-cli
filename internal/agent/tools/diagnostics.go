package tools

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
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
	if compilerDiagnostics.Output != "" {
		output.WriteString("\n<compiler_diagnostics>\n")
		output.WriteString(compilerDiagnostics.Output)
		output.WriteString("\n</compiler_diagnostics>\n")
		summary.CompilerErrors = compilerDiagnostics.Errors
		summary.CompilerWarnings = compilerDiagnostics.Warnings
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
			fmt.Fprintf(&output, "Compiler: %d errors, %d warnings\n", summary.CompilerErrors, summary.CompilerWarnings)
		}
		output.WriteString("</diagnostic_summary>\n")
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
	if len(in) > 10 {
		output.WriteString(strings.Join(in[:10], "\n"))
		fmt.Fprintf(output, "\n... and %d more diagnostics", len(in)-10)
	} else {
		output.WriteString(strings.Join(in, "\n"))
	}
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

func countSeverity(diagnostics []string, severity string) int {
	count := 0
	for _, diag := range diagnostics {
		if strings.HasPrefix(diag, severity) {
			count++
		}
	}
	return count
}
