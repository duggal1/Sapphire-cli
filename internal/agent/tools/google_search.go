package tools

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"charm.land/fantasy"
	"google.golang.org/genai"
)

//go:embed google_search.md
var googleSearchToolDescription []byte

// GoogleSearchToolName is the name of the google_search tool.
const GoogleSearchToolName = "google_search"

var promptURLPattern = regexp.MustCompile(`https?://[^\s<>()]+`)

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

			maxResults := params.MaxResults
			if maxResults <= 0 {
				maxResults = 10
			}
			if maxResults > 20 {
				maxResults = 20
			}

			urls := normalizeGoogleSearchURLs([]string{params.URL}, params.URLs, extractPromptURLs(query))
			if query == "" && len(urls) > 0 {
				query = "Analyze the provided URLs and use grounded web search only if it improves the answer."
			}
			if query == "" {
				return fantasy.NewTextResponse("No query provided for Google Grounding."), nil
			}

			// Fallback logic: after 2 failures, use DuckDuckGo
			failures := getFailures(sessionID)
			if failures >= 2 || client == nil {
				maybeDelaySearch()
				results, err := searchDuckDuckGo(ctx, httpClient, query, maxResults)
				if err != nil {
					if len(urls) > 0 {
						return fantasy.NewTextErrorResponse("Google grounding is unavailable and URL context requires Gemini search support"), nil
					}
					return fantasy.NewTextErrorResponse("DuckDuckGo fallback failed: " + err.Error()), nil
				}
				return fantasy.NewTextResponse(formatSearchResults(results)), nil
			}

			// Execute Google Grounding search via a separate Gemini request
			content := []*genai.Content{genai.NewContentFromText(buildGroundedSearchPrompt(query, urls), genai.RoleUser)}
			genaiTools := []*genai.Tool{
				{GoogleSearch: &genai.GoogleSearch{}},
			}
			if len(urls) > 0 {
				genaiTools = append(genaiTools, &genai.Tool{URLContext: &genai.URLContext{}})
			}
			config := &genai.GenerateContentConfig{
				Tools: genaiTools,
			}

			searchCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
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

			formatted := formatGoogleSearchResponse(resp, maxResults)
			if strings.TrimSpace(formatted) == "" {
				return fantasy.NewTextResponse("No grounded search or URL-context results were returned by Gemini."), nil
			}
			return fantasy.NewTextResponse(formatted), nil
		})
}

func buildGroundedSearchPrompt(query string, urls []string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		query = "Answer the request using URL context and grounded search when helpful."
	}
	if len(urls) == 0 {
		return query
	}

	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString("\n\nUse URL context for these URLs if relevant:\n")
	for _, target := range urls {
		sb.WriteString("- ")
		sb.WriteString(target)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func normalizeGoogleSearchURLs(groups ...[]string) []string {
	seen := make(map[string]struct{})
	urls := make([]string, 0)
	for _, group := range groups {
		for _, raw := range group {
			value := strings.TrimSpace(strings.TrimRight(raw, ".,);]}>"))
			if value == "" {
				continue
			}
			parsed, err := url.Parse(value)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				continue
			}
			normalized := parsed.String()
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			urls = append(urls, normalized)
		}
	}
	slices.Sort(urls)
	return urls
}

func extractPromptURLs(text string) []string {
	matches := promptURLPattern.FindAllString(text, -1)
	return normalizeGoogleSearchURLs(matches)
}

func formatGoogleSearchResponse(resp *genai.GenerateContentResponse, maxResults int) string {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0] == nil {
		return ""
	}

	candidate := resp.Candidates[0]
	var sb strings.Builder

	if answer := strings.TrimSpace(resp.Text()); answer != "" {
		sb.WriteString("Answer:\n")
		sb.WriteString(answer)
		sb.WriteString("\n")
	}

	if candidate.GroundingMetadata != nil {
		if queries := candidate.GroundingMetadata.WebSearchQueries; len(queries) > 0 {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("Google search queries:\n")
			for _, query := range queries {
				query = strings.TrimSpace(query)
				if query == "" {
					continue
				}
				sb.WriteString("- ")
				sb.WriteString(query)
				sb.WriteString("\n")
			}
		}

		webChunks := make([]*genai.GroundingChunkWeb, 0)
		for _, chunk := range candidate.GroundingMetadata.GroundingChunks {
			if chunk != nil && chunk.Web != nil {
				webChunks = append(webChunks, chunk.Web)
			}
		}
		if len(webChunks) > 0 {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			limit := maxResults
			if limit <= 0 || limit > len(webChunks) {
				limit = len(webChunks)
			}
			sb.WriteString(fmt.Sprintf("Grounded web sources (%d):\n", limit))
			for i := 0; i < limit; i++ {
				title := strings.TrimSpace(webChunks[i].Title)
				if title == "" {
					title = "Google Search Result"
				}
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, title))
				sb.WriteString("   URL: ")
				sb.WriteString(strings.TrimSpace(webChunks[i].URI))
				sb.WriteString("\n")
			}
		}
	}

	if candidate.URLContextMetadata != nil && len(candidate.URLContextMetadata.URLMetadata) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("URL context retrieval:\n")
		for _, metadata := range candidate.URLContextMetadata.URLMetadata {
			if metadata == nil {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(metadata.RetrievedURL))
			sb.WriteString(" [")
			sb.WriteString(string(metadata.URLRetrievalStatus))
			sb.WriteString("]\n")
		}
	}

	return strings.TrimSpace(sb.String())
}
