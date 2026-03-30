package tools

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"charm.land/fantasy"
)

//go:embed web_search.md
var webSearchToolDescription []byte

// NewWebSearchTool creates a web search tool for sub-agents (no permissions needed).
func NewWebSearchTool(client *http.Client) fantasy.AgentTool {
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns = 100
		transport.MaxIdleConnsPerHost = 10
		transport.IdleConnTimeout = 90 * time.Second

		client = &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		}
	}

	return fantasy.NewParallelAgentTool(
		WebSearchToolName,
		string(webSearchToolDescription),
		func(ctx context.Context, params WebSearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			queries := normalizeBatchTargets(params.Query, params.Queries, "")
			if len(queries) == 0 {
				return NewGuidanceErrorResponse(
					WebSearchToolName,
					"missing_query",
					"Missing search query.",
					"web_search requires query or queries. Do not call it with empty input. Provide one concrete search query string or a non-empty queries array, then retry.",
				), nil
			}

			maxResults := params.MaxResults
			if maxResults <= 0 {
				maxResults = 10
			}
			if maxResults > 20 {
				maxResults = 20
			}

			if len(queries) == 1 {
				maybeDelaySearch()
				results, err := searchDuckDuckGo(ctx, client, queries[0], maxResults)
				slog.Debug("Web search completed", "query", queries[0], "results", len(results), "err", err)
				if err != nil {
					return fantasy.NewTextErrorResponse("Failed to search: " + err.Error()), nil
				}

				return fantasy.NewTextResponse(formatSearchResults(results)), nil
			}

			sections := make([]string, 0, len(queries))
			errors := make([]string, 0)
			for _, result := range ParallelWebSearch(ctx, client, queries, maxResults) {
				if result.Err != nil {
					errors = append(errors, fmt.Sprintf("- %s: %v", result.Query, result.Err))
					continue
				}
				sections = append(sections, fmt.Sprintf("Query: %s\n%s", result.Query, strings.TrimSpace(result.Output)))
			}
			if len(sections) == 0 && len(errors) > 0 {
				return fantasy.NewTextErrorResponse("Failed to search:\n" + strings.Join(errors, "\n")), nil
			}

			output := fmt.Sprintf("Searched %d queries in parallel.\n\n%s", len(sections), strings.Join(sections, "\n\n"))
			if len(errors) > 0 {
				output += "\n\nErrors:\n" + strings.Join(errors, "\n")
			}
			return fantasy.NewTextResponse(output), nil
		})
}
