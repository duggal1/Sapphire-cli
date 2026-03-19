package tools

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/permission"
)

type DownloadParams struct {
	URL      string `json:"url" description:"The URL to download from"`
	FilePath string `json:"file_path" description:"The local file path where the downloaded content should be saved"`
	Timeout  int    `json:"timeout,omitempty" description:"Optional timeout in seconds (max 600)"`
}

func (p *DownloadParams) UnmarshalJSON(data []byte) error {
	type rawDownloadParams struct {
		URL         string `json:"url,omitempty"`
		URI         string `json:"uri,omitempty"`
		Link        string `json:"link,omitempty"`
		Href        string `json:"href,omitempty"`
		Source      string `json:"source,omitempty"`
		FilePath    string `json:"file_path,omitempty"`
		Path        string `json:"path,omitempty"`
		Destination string `json:"destination,omitempty"`
		Dest        string `json:"dest,omitempty"`
		File        string `json:"file,omitempty"`
		Output      string `json:"output,omitempty"`
		Timeout     int    `json:"timeout,omitempty"`
	}

	var raw rawDownloadParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.URL = firstFetchString(raw.URL, raw.URI, raw.Link, raw.Href, raw.Source)
	p.FilePath = firstFetchString(raw.FilePath, raw.Path, raw.Destination, raw.Dest, raw.File, raw.Output)
	p.Timeout = raw.Timeout
	return nil
}

type DownloadPermissionsParams struct {
	URL      string `json:"url"`
	FilePath string `json:"file_path"`
	Timeout  int    `json:"timeout,omitempty"`
}

const DownloadToolName = "download"

//go:embed download.md
var downloadDescription []byte

func NewDownloadTool(permissions permission.Service, workingDir string, client *http.Client) fantasy.AgentTool {
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns = 100
		transport.MaxIdleConnsPerHost = 10
		transport.IdleConnTimeout = 90 * time.Second

		client = &http.Client{
			Timeout:   5 * time.Minute, // Default 5 minute timeout for downloads
			Transport: transport,
		}
	}
	return fantasy.NewParallelAgentTool(
		DownloadToolName,
		string(downloadDescription),
		func(ctx context.Context, params DownloadParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.URL == "" {
				return fantasy.NewTextErrorResponse("URL parameter is required"), nil
			}

			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path parameter is required"), nil
			}

			if !strings.HasPrefix(params.URL, "http://") && !strings.HasPrefix(params.URL, "https://") {
				return fantasy.NewTextErrorResponse("URL must start with http:// or https://"), nil
			}

			filePath := filepathext.SmartJoin(workingDir, params.FilePath)
			relPath, _ := filepath.Rel(workingDir, filePath)
			relPath = filepath.ToSlash(cmp.Or(relPath, filePath))

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for downloading files")
			}

			p, err := permissions.Request(ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        filePath,
					ToolName:    DownloadToolName,
					Action:      "download",
					Description: fmt.Sprintf("Download file from URL: %s to %s", params.URL, filePath),
					Params:      DownloadPermissionsParams(params),
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			// Handle timeout with context
			requestCtx := ctx
			if params.Timeout > 0 {
				maxTimeout := 600 // 10 minutes
				if params.Timeout > maxTimeout {
					params.Timeout = maxTimeout
				}
				var cancel context.CancelFunc
				requestCtx, cancel = context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
				defer cancel()
			}

			req, err := http.NewRequestWithContext(requestCtx, "GET", params.URL, nil)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("User-Agent", "crush/1.0")

			resp, err := client.Do(req)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to download from URL: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Request failed with status code: %d", resp.StatusCode)), nil
			}

			// Create parent directories if they don't exist
			if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to create parent directories: %w", err)
			}

			// Create the output file
			outFile, err := os.Create(filePath)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to create output file: %w", err)
			}
			defer outFile.Close()

			// Copy data without an explicit size limit.
			// The overall download is still constrained by the HTTP client's timeout
			// and any upstream server limits.
			bytesWritten, err := io.Copy(outFile, resp.Body)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
			}

			contentType := resp.Header.Get("Content-Type")
			responseMsg := fmt.Sprintf("Successfully downloaded %d bytes to %s", bytesWritten, relPath)
			if contentType != "" {
				responseMsg += fmt.Sprintf(" (Content-Type: %s)", contentType)
			}

			return fantasy.NewTextResponse(responseMsg), nil
		})
}
