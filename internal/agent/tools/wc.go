package tools

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/fsext"
)

const (
	WCToolName  = "wc"
	WCLToolName = "wc_l"
)

type WCParams struct {
	Path  string   `json:"path,omitempty" description:"The file path to inspect"`
	Paths []string `json:"paths,omitempty" description:"Optional list of file paths to inspect in one parallel call"`
}

type fileCountMetrics struct {
	Lines int
	Words int
	Bytes int
	Runes int
}

//go:embed wc.md
var wcDescription []byte

//go:embed wc_l.md
var wcLinesDescription []byte

func NewWCTool(workingDir string) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		WCToolName,
		string(wcDescription),
		func(ctx context.Context, params WCParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return runWCTool(ctx, workingDir, params, false)
		},
	)
}

func NewWCLTool(workingDir string) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		WCLToolName,
		string(wcLinesDescription),
		func(ctx context.Context, params WCParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return runWCTool(ctx, workingDir, params, true)
		},
	)
}

func runWCTool(ctx context.Context, workingDir string, params WCParams, lineOnly bool) (fantasy.ToolResponse, error) {
	if ctx.Err() != nil {
		return fantasy.ToolResponse{}, ctx.Err()
	}
	targets, err := resolveWCTargets(params, workingDir)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	var output strings.Builder
	for i, target := range targets {
		metrics, err := countFileMetrics(target)
		if err != nil {
			if i > 0 {
				output.WriteString("\n")
			}
			output.WriteString(fmt.Sprintf("%s: error: %v", filepath.ToSlash(target), err))
			continue
		}
		if i > 0 {
			output.WriteString("\n")
		}
		if lineOnly {
			output.WriteString(fmt.Sprintf("%d\t%s", metrics.Lines, filepath.ToSlash(target)))
			continue
		}
		output.WriteString(fmt.Sprintf("%s\n  lines=%d words=%d bytes=%d chars=%d",
			filepath.ToSlash(target),
			metrics.Lines,
			metrics.Words,
			metrics.Bytes,
			metrics.Runes,
		))
	}
	return fantasy.NewTextResponse(strings.TrimSpace(output.String())), nil
}

func resolveWCTargets(params WCParams, workingDir string) ([]string, error) {
	targets := normalizeBatchTargets(params.Path, params.Paths, workingDir)
	if len(targets) == 0 {
		return nil, fmt.Errorf("path is required")
	}
	resolved := make([]string, 0, len(targets))
	for _, target := range targets {
		expanded, err := fsext.Expand(target)
		if err != nil {
			return nil, fmt.Errorf("error expanding path %q: %w", target, err)
		}
		resolved = append(resolved, filepathext.SmartJoin(workingDir, expanded))
	}
	return resolved, nil
}

func countFileMetrics(path string) (fileCountMetrics, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileCountMetrics{}, err
	}
	if info.IsDir() {
		return fileCountMetrics{}, fmt.Errorf("directories are not supported")
	}

	file, err := os.Open(path) //nolint:gosec
	if err != nil {
		return fileCountMetrics{}, err
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 64*1024)
	var metrics fileCountMetrics
	inWord := false
	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			metrics.Bytes += len(chunk)
			metrics.Lines += countNewlines(chunk)
			metrics.Runes += utf8.RuneCount(chunk)
			for len(chunk) > 0 {
				r, size := utf8.DecodeRune(chunk)
				if r == utf8.RuneError && size == 1 {
					size = 1
				}
				if unicode.IsSpace(r) {
					inWord = false
				} else if !inWord {
					metrics.Words++
					inWord = true
				}
				chunk = chunk[size:]
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, os.ErrClosed) {
			return metrics, err
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return metrics, err
		}
		return metrics, err
	}
	return metrics, nil
}

func countNewlines(chunk []byte) int {
	count := 0
	for _, b := range chunk {
		if b == '\n' {
			count++
		}
	}
	return count
}
