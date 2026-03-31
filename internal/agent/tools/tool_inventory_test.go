package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/duggal1/Sapphire-cli/internal/lsp"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestDownloadTool(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("downloaded content"))
	}))
	defer srv.Close()

	workingDir := t.TempDir()
	tool := NewDownloadTool(permission.NewPermissionService(workingDir, true, nil), workingDir, srv.Client())
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-download")

	resp := runTool(t, tool, DownloadToolName, DownloadParams{
		URL:      srv.URL,
		FilePath: "artifact.txt",
	}, ctx)

	require.Contains(t, resp.Content, "Successfully downloaded")
	content, err := os.ReadFile(filepath.Join(workingDir, "artifact.txt"))
	require.NoError(t, err)
	require.Equal(t, "downloaded content", string(content))
}

func TestFetchToolHTMLAndMarkdown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html><body><h1>Hello</h1><p>World</p></body></html>")
	}))
	defer srv.Close()

	workingDir := t.TempDir()
	tool := NewFetchTool(permission.NewPermissionService(workingDir, true, nil), workingDir, srv.Client())
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-fetch")

	textResp := runTool(t, tool, FetchToolName, FetchParams{
		URL:    srv.URL,
		Format: "text",
	}, ctx)
	require.Contains(t, textResp.Content, "Hello")
	require.Contains(t, textResp.Content, "World")

	mdResp := runTool(t, tool, FetchToolName, FetchParams{
		URL:    srv.URL,
		Format: "markdown",
	}, ctx)
	require.Contains(t, mdResp.Content, "```")
	require.Contains(t, mdResp.Content, "Hello")
}

func TestWebFetchTool(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html><body><article><h1>Test Page</h1><p>Body</p></article></body></html>")
	}))
	defer srv.Close()

	tool := NewWebFetchTool(t.TempDir(), srv.Client())
	resp := runTool(t, tool, WebFetchToolName, WebFetchParams{URL: srv.URL}, t.Context())
	require.Contains(t, resp.Content, "Fetched content from")
	require.Contains(t, resp.Content, "Test Page")
}

func TestGlobTool(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workingDir, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "main.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "nested", "worker.go"), []byte("package nested"), 0o644))

	tool := NewGlobTool(workingDir)
	resp := runTool(t, tool, GlobToolName, GlobParams{Pattern: "**/*.go"}, t.Context())
	require.Contains(t, resp.Content, "main.go")
	require.Contains(t, resp.Content, "worker.go")

	otherDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(otherDir, "extra.go"), []byte("package other"), 0o644))
	multiResp := runTool(t, tool, GlobToolName, GlobParams{
		Pattern: "**/*.go",
		Paths:   []string{workingDir, otherDir},
	}, t.Context())
	require.Contains(t, multiResp.Content, "Searched 2 roots in parallel")
	require.Contains(t, multiResp.Content, "extra.go")
}

func TestLSTool(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workingDir, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "nested", "file.txt"), []byte("x"), 0o644))

	tool := NewLsTool(permission.NewPermissionService(workingDir, true, nil), workingDir, config.ToolLs{})
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-ls")
	resp := runTool(t, tool, LSToolName, LSParams{Path: workingDir, Depth: 2}, ctx)
	require.Contains(t, resp.Content, filepath.ToSlash(workingDir)+"/")
	require.Contains(t, resp.Content, "nested/")
	require.Contains(t, resp.Content, "file.txt")

	otherDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(otherDir, "another.txt"), []byte("y"), 0o644))
	multiResp := runTool(t, tool, LSToolName, LSParams{
		Paths: []string{workingDir, otherDir},
		Depth: 2,
	}, ctx)
	require.Contains(t, multiResp.Content, "Listed 2 directories in parallel")
	require.Contains(t, multiResp.Content, "another.txt")
}

func TestGrepToolParallelPaths(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	firstDir := filepath.Join(workingDir, "first")
	secondDir := filepath.Join(workingDir, "second")
	require.NoError(t, os.MkdirAll(firstDir, 0o755))
	require.NoError(t, os.MkdirAll(secondDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(firstDir, "main.go"), []byte("package main\nconst target = 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(secondDir, "worker.go"), []byte("package worker\nconst target = 2\n"), 0o644))

	tool := NewGrepTool(workingDir, config.ToolGrep{})
	resp := runTool(t, tool, GrepToolName, GrepParams{
		Pattern: "target",
		Paths:   []string{firstDir, secondDir},
		Include: "*.go",
	}, t.Context())
	require.Contains(t, resp.Content, "Searched 2 roots in parallel")
	require.Contains(t, resp.Content, "main.go")
	require.Contains(t, resp.Content, "worker.go")
}

func TestRGFilesTool(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workingDir, "internal", "worker"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "internal", "worker", "handler.go"), []byte("package worker"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "internal", "worker", "service.go"), []byte("package worker"), 0o644))

	tool := NewRGFilesTool(workingDir)
	resp := runTool(t, tool, RGFilesToolName, RGFilesParams{
		Query: "handler",
		Path:  workingDir,
	}, t.Context())
	require.Contains(t, resp.Content, "handler.go")
}

func TestRankRGFileQueryPrefersImplementationOverGeneratedNoise(t *testing.T) {
	t.Parallel()

	paths := []string{
		"pkg/zapier/service.go",
		"pkg/generated/zapier_service_generated.go",
		"pkg/zapier/service_test.go",
		"pkg/zapier/testdata/service_fixture.json",
	}

	ranked := rankRGFileQuery(paths, "zapier service")

	require.NotEmpty(t, ranked)
	require.Equal(t, "pkg/zapier/service.go", ranked[0])
	require.NotEqual(t, "pkg/generated/zapier_service_generated.go", ranked[0])
}

func TestWCTools(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	path := filepath.Join(workingDir, "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world\nsecond line\n"), 0o644))

	wcResp := runTool(t, NewWCTool(workingDir), WCToolName, WCParams{Path: path}, t.Context())
	require.Contains(t, wcResp.Content, "lines=2")
	require.Contains(t, wcResp.Content, "words=4")

	wcLResp := runTool(t, NewWCLTool(workingDir), WCLToolName, WCParams{Path: path}, t.Context())
	require.Contains(t, wcLResp.Content, "2\t")
}

func TestToolSearchToolUsesIndexedMatchesAndFallbacks(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	targetPath := filepath.Join(workingDir, "internal", "search", "locate_target.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0o755))
	require.NoError(t, os.WriteFile(targetPath, []byte("package search\n\nfunc LocateTarget() {}\n"), 0o644))

	tool := NewToolSearchTool("", workingDir, func(context.Context, string, int) (ToolSearchIndexedResult, error) {
		return ToolSearchIndexedResult{
			Available: true,
			Message:   "Durable codebase graph matches.",
			Matches: []ToolSearchIndexedMatch{
				{
					Kind:      "function",
					Path:      targetPath,
					Name:      "LocateTarget",
					Signature: "func LocateTarget()",
					StartLine: 3,
					EndLine:   3,
					Score:     250,
				},
			},
		}, nil
	})

	resp := runTool(t, tool, ToolSearchToolName, ToolSearchParams{
		Query: "locate",
		Path:  workingDir,
	}, t.Context())
	require.Contains(t, resp.Content, `Tool search results for "locate"`)
	require.Contains(t, resp.Content, "query_variants:")
	require.Contains(t, resp.Content, "Top candidates:")
	require.Contains(t, resp.Content, "LocateTarget")
	require.Contains(t, resp.Content, "sources=indexed:function(LocateTarget), filename")
	require.Contains(t, resp.Content, "skipped text fallback")
}

func TestBuildToolSearchQueryPlanFocusesNaturalLanguageQueries(t *testing.T) {
	t.Parallel()

	plan := buildToolSearchQueryPlan("Please fix the Zapier integration issue in webhook sync flow")

	require.Contains(t, plan.Indexed, "zapier integration")
	require.Contains(t, plan.Indexed, "zapier")
	require.Contains(t, plan.Files, "zapier")
	require.Contains(t, plan.Text, "zapier integration")
	require.NotContains(t, plan.All, "Please fix the Zapier integration issue in webhook sync flow")
}

func TestToolSearchToolFindsNaturalLanguageTargetsWithoutBroadReads(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	zapierPath := filepath.Join(workingDir, "integrations", "zapier", "client.go")
	slackPath := filepath.Join(workingDir, "integrations", "slack", "client.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(zapierPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(slackPath), 0o755))
	require.NoError(t, os.WriteFile(zapierPath, []byte("package zapier\n\nfunc SendZapierWebhook() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(slackPath, []byte("package slack\n\nfunc SendSlackMessage() {}\n"), 0o644))

	tool := NewToolSearchTool("", workingDir, nil)
	resp := runTool(t, tool, ToolSearchToolName, ToolSearchParams{
		Query: "please fix the Zapier integration issue in webhook sync flow",
		Path:  workingDir,
		Limit: 5,
	}, t.Context())

	require.Contains(t, resp.Content, "query_variants: zapier integration | zapier")
	require.Contains(t, resp.Content, filepath.ToSlash(zapierPath))
	require.Contains(t, resp.Content, "Top candidates:")
	require.NotContains(t, resp.Content, "query_variants: please")
}

func TestWebAndGoogleSearchTools(t *testing.T) {
	t.Parallel()

	searchHTML := `
<html><body>
<a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fone">Example One</a>
<td class="result-snippet">First result snippet</td>
<a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Ftwo">Example Two</a>
<td class="result-snippet">Second result snippet</td>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, searchHTML)
	}))
	defer srv.Close()

	client := &http.Client{Transport: rewriteDuckDuckGoTransport(t, srv.URL)}

	webTool := NewWebSearchTool(client)
	webResp := runTool(t, webTool, WebSearchToolName, WebSearchParams{
		Query:      "example query",
		MaxResults: 2,
	}, t.Context())
	require.Contains(t, webResp.Content, "Found 2 search results")
	require.Contains(t, webResp.Content, "https://example.com/one")

	webParallelResp := runTool(t, webTool, WebSearchToolName, WebSearchParams{
		Queries:    []string{"example query", "another query"},
		MaxResults: 2,
	}, t.Context())
	require.Contains(t, webParallelResp.Content, "Searched 2 queries in parallel")
	require.Contains(t, webParallelResp.Content, "Query: example query")

	googleTool := NewGoogleSearchTool(
		nil,
		client,
		"gemini-3-flash",
		func(string) int { return 2 },
		func(string) {},
		func(string) {},
		func(context.Context, string) string { return "default query" },
	)
	googleResp := runTool(t, googleTool, GoogleSearchToolName, GoogleSearchParams{
		MaxResults: 2,
	}, t.Context())
	require.Contains(t, googleResp.Content, "Found 2 search results")
	require.Contains(t, googleResp.Content, "Example One")
}

func TestFormatGoogleSearchResponseIncludesURLContextMetadata(t *testing.T) {
	t.Parallel()

	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "Zapier can be used through an MCP server."},
					},
				},
				GroundingMetadata: &genai.GroundingMetadata{
					WebSearchQueries: []string{"zapier mcp server"},
					GroundingChunks: []*genai.GroundingChunk{
						{
							Web: &genai.GroundingChunkWeb{
								Title: "Zapier MCP",
								URI:   "https://example.com/zapier-mcp",
							},
						},
					},
				},
				URLContextMetadata: &genai.URLContextMetadata{
					URLMetadata: []*genai.URLMetadata{
						{
							RetrievedURL:       "https://zapier.com",
							URLRetrievalStatus: genai.URLRetrievalStatusSuccess,
						},
					},
				},
			},
		},
	}

	out := formatGoogleSearchResponse(resp, 5)
	require.Contains(t, out, "Answer:")
	require.Contains(t, out, "Google search queries:")
	require.Contains(t, out, "Grounded web sources (1):")
	require.Contains(t, out, "URL context retrieval:")
	require.Contains(t, out, "https://zapier.com [URL_RETRIEVAL_STATUS_SUCCESS]")
}

func TestInstallSkillToolReturnsFullMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/search":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{
						"skill_id":      "zapier-automation",
						"folder_name":   "zapier-automation",
						"skill_name":    "zapier-automation",
						"relative_path": "zapier-automation/SKILL.md",
						"markdown_path": "zapier-automation/SKILL.md",
						"size_bytes":    128,
						"is_nested":     false,
						"category":      "automation",
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/zapier-automation":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"skill": map[string]any{
					"skill_id":      "zapier-automation",
					"folder_name":   "zapier-automation",
					"skill_name":    "zapier-automation",
					"relative_path": "zapier-automation/SKILL.md",
					"markdown_path": "zapier-automation/SKILL.md",
					"size_bytes":    128,
					"is_nested":     false,
					"category":      "automation",
				},
				"markdown": "# Zapier Automation\n\nUse Zapier carefully.\n",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("SAPPHIRE_API_KEY", "token")
	t.Setenv("SAPPHIRE_API_BASE_URL", server.URL)

	workingDir, err := os.MkdirTemp("", "install-skill-*")
	require.NoError(t, err)
	defer func() {
		for range 5 {
			if removeErr := os.RemoveAll(workingDir); removeErr == nil || os.IsNotExist(removeErr) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	ctx := context.WithValue(t.Context(), WorkingDirContextKey, workingDir)
	resp := runTool(t, NewInstallSkillTool(), InstallSkillToolName, InstallSkillParams{
		Query: "zapier automation",
	}, ctx)

	require.Contains(t, resp.Content, `Exact local name: "zapier-automation"`)
	require.Contains(t, resp.Content, "<instructions>")
	require.Contains(t, resp.Content, "Use Zapier carefully.")

	data, err := os.ReadFile(filepath.Join(workingDir, ".sapphire", "skills", "zapier-automation", "SKILL.md"))
	require.NoError(t, err)
	require.Contains(t, string(data), "Zapier Automation")
}

func TestSourcegraphFormatting(t *testing.T) {
	t.Parallel()

	result := map[string]any{
		"data": map[string]any{
			"search": map[string]any{
				"results": map[string]any{
					"matchCount":  float64(1),
					"resultCount": float64(1),
					"limitHit":    false,
					"results": []any{
						map[string]any{
							"__typename": "FileMatch",
							"repository": map[string]any{"name": "repo"},
							"file": map[string]any{
								"path":    "main.go",
								"url":     "https://sourcegraph.example/repo/main.go",
								"content": "package main\nfunc main() {}\n",
							},
							"lineMatches": []any{
								map[string]any{
									"lineNumber": float64(2),
									"preview":    "func main() {}",
								},
							},
						},
					},
				},
			},
		},
	}

	out, err := formatSourcegraphResults(result, 1)
	require.NoError(t, err)
	require.Contains(t, out, "Sourcegraph Search Results")
	require.Contains(t, out, "repo/main.go")
	require.Contains(t, out, "func main() {}")
}

func TestMemoryAndLSPAuxiliaryTools(t *testing.T) {
	t.Parallel()

	memTool := NewMemoryQueryTool(fakeMemoryService{})
	memResp := runTool(t, memTool, MemoryQueryToolName, MemoryQueryParams{Query: "auth"}, t.Context())
	require.Contains(t, memResp.Content, "Codebase Knowledge")

	cfg, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(cfg.WorkingDir(), ".sapphire"))
	})
	manager := lsp.NewManager(cfg)

	diagResp := runTool(t, NewDiagnosticsTool(manager), DiagnosticsToolName, DiagnosticsParams{}, t.Context())
	require.False(t, diagResp.IsError)

	refResp := runTool(t, NewReferencesTool(manager), ReferencesToolName, ReferencesParams{Symbol: "main"}, t.Context())
	require.True(t, refResp.IsError)
	require.Contains(t, refResp.Content, "no LSP clients available")

	restartResp := runTool(t, NewLSPRestartTool(manager), LSPRestartToolName, LSPRestartParams{}, t.Context())
	require.True(t, restartResp.IsError)
	require.Contains(t, restartResp.Content, "no LSP clients available")
}

func rewriteDuckDuckGoTransport(t *testing.T, serverURL string) http.RoundTripper {
	t.Helper()

	target := strings.TrimPrefix(serverURL, "http://")
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = "http"
		clone.URL.Host = target
		clone.Host = target
		return http.DefaultTransport.RoundTrip(clone)
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type fakeMemoryService struct{}

func (fakeMemoryService) GetProjectConstitution(context.Context, string) (string, error) {
	return "", nil
}
func (fakeMemoryService) UpsertProjectConstitution(context.Context, string, string) error { return nil }
func (fakeMemoryService) GetStructuredSummary(context.Context, string) (*agentmemory.StructuredSummaryData, error) {
	return nil, nil
}
func (fakeMemoryService) CreateStructuredSummary(context.Context, string, agentmemory.StructuredSummaryData) error {
	return nil
}
func (fakeMemoryService) GetCodebaseKnowledge(context.Context, string) ([]db.CodebaseKnowledge, error) {
	return nil, nil
}
func (fakeMemoryService) UpsertCodebaseKnowledge(context.Context, db.UpsertCodebaseKnowledgeParams) error {
	return nil
}
func (fakeMemoryService) ListStructuredSummaries(context.Context, int) ([]db.StructuredSummary, error) {
	return []db.StructuredSummary{
		{SessionID: "session-1", SummaryData: `{"summary":"fixed auth bug"}`},
	}, nil
}
func (fakeMemoryService) SearchCodebaseKnowledge(context.Context, string, int) ([]db.CodebaseKnowledge, error) {
	return []db.CodebaseKnowledge{
		{
			SymbolName:    "AuthFix",
			SymbolType:    "function",
			FilePath:      "internal/auth.go",
			Documentation: sql.NullString{String: "Fixes auth refresh", Valid: true},
		},
	}, nil
}

func runTool[T any](t *testing.T, tool fantasy.AgentTool, name string, params T, ctx context.Context) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    name + "-1",
		Name:  name,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}
