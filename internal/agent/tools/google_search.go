package tools

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"time"

	"charm.land/fantasy"
	"google.golang.org/genai"
)

//go:embed google_search.md
var googleSearchToolDescription []byte

// GoogleSearchToolName is the name of the google_search tool.
const GoogleSearchToolName = "google_search"

// NewGoogleSearchTool creates a new google_search tool that uses Google Grounding.
// It supports fallback to DuckDuckGo after a certain number of failures.
func NewGoogleSearchTool(
	client *genai.Client,
	httpClient *http.Client,
	model string,
	getFailures func(string) int,
	incFailures func(string),
	resetFailures func(string),
	getDefaultQuery func(context.Context, string) string,
) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		GoogleSearchToolName,
		string(googleSearchToolDescription),
		func(ctx context.Context, params GoogleSearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				sessionID = "default"
			}

			if httpClient == nil {
				httpClient = http.DefaultClient
			}

			query := strings.TrimSpace(params.Query)
			if query == "" && getDefaultQuery != nil {
				query = strings.TrimSpace(getDefaultQuery(ctx, sessionID))
			}
			if query == "" {
				return fantasy.NewTextResponse("No query provided for Google Grounding."), nil
			}

			maxResults := params.MaxResults
			if maxResults <= 0 {
				maxResults = 10
			}

			// Fallback logic: after 2 failures, use DuckDuckGo
			failures := getFailures(sessionID)
			if failures >= 2 {
				maybeDelaySearch()
				results, err := searchDuckDuckGo(ctx, httpClient, query, maxResults)
				if err != nil {
					return fantasy.NewTextErrorResponse("DuckDuckGo fallback failed: " + err.Error()), nil
				}
				return fantasy.NewTextResponse(formatSearchResults(results)), nil
			}

			// Execute Google Grounding search via a separate Gemini request
			content := []*genai.Content{
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: query},
					},
				},
			}
			config := &genai.GenerateContentConfig{
				Tools: []*genai.Tool{
					{GoogleSearch: &genai.GoogleSearch{}},
				},
			}

			searchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			resp, err := client.Models.GenerateContent(searchCtx, model, content, config)
			if err != nil {
				incFailures(sessionID)
				// Retry with DDG immediately on failure
				maybeDelaySearch()
				results, err2 := searchDuckDuckGo(ctx, httpClient, query, maxResults)
				if err2 == nil {
					return fantasy.NewTextResponse(formatSearchResults(results)), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Google Search failed (%v) and DuckDuckGo fallback also failed", err)), nil
			}

			resetFailures(sessionID)

			// Extract Grounding Metadata and format it
			var sb strings.Builder
			hasGrounding := false
			if len(resp.Candidates) > 0 {
				candidate := resp.Candidates[0]
				if candidate.GroundingMetadata != nil && len(candidate.GroundingMetadata.GroundingChunks) > 0 {
					hasGrounding = true

					urlChunks := 0
					for _, chunk := range candidate.GroundingMetadata.GroundingChunks {
						if chunk.Web != nil {
							urlChunks++
						}
					}

					sb.WriteString(fmt.Sprintf("Found %d search results from Google Grounding:\n\n", urlChunks))
					count := 1
					for _, chunk := range candidate.GroundingMetadata.GroundingChunks {
						if chunk.Web != nil {
							title := chunk.Web.Title
							if title == "" {
								title = "Google Search Result"
							}
							fmt.Fprintf(&sb, "%d. %s\n", count, title)
							fmt.Fprintf(&sb, "   URL: %s\n\n", chunk.Web.URI)
							count++
						}
					}
				}
			}

			if !hasGrounding {
				return fantasy.NewTextResponse("No verified search results found from Google Grounding."), nil
			}

			return fantasy.NewTextResponse(sb.String()), nil
		})
}
