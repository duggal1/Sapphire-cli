package tools

import (
	"encoding/json"
	"strings"
)

// AgenticFetchToolName is the name of the agentic fetch tool.
const AgenticFetchToolName = "agentic_fetch"

// WebFetchToolName is the name of the web_fetch tool.
const WebFetchToolName = "web_fetch"

// WebSearchToolName is the name of the web_search tool for sub-agents.
const WebSearchToolName = "web_search"

// LargeContentThreshold is the size threshold for saving content to a file.
const LargeContentThreshold = 50000 // 50KB

// AgenticFetchParams defines the parameters for the agentic fetch tool.
type AgenticFetchParams struct {
	URL    string `json:"url,omitempty" description:"The URL to fetch content from (optional - if not provided, the agent will search the web)"`
	Prompt string `json:"prompt" description:"The prompt describing what information to find or extract"`
}

// AgenticFetchPermissionsParams defines the permission parameters for the agentic fetch tool.
type AgenticFetchPermissionsParams struct {
	URL    string `json:"url,omitempty"`
	Prompt string `json:"prompt"`
}

// WebFetchParams defines the parameters for the web_fetch tool.
type WebFetchParams struct {
	URL string `json:"url" description:"The URL to fetch content from"`
}

// WebSearchParams defines the parameters for the web_search tool.
type WebSearchParams struct {
	Query      string `json:"query" description:"The search query to find information on the web"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of results to return (default: 10, max: 20)"`
}

// GoogleSearchParams defines the parameters for the google_search tool.
// Query is optional to align with Gemini grounding, which can infer queries from the prompt.
type GoogleSearchParams struct {
	Query      string `json:"query,omitempty" description:"The search query to ground (optional; defaults to the current user request)"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of results to return (default: 10, max: 20)"`
}

// FetchParams defines the parameters for the simple fetch tool.
type FetchParams struct {
	URL     string `json:"url" description:"The URL to fetch content from"`
	Format  string `json:"format" description:"The format to return the content in (text, markdown, or html)"`
	Timeout int    `json:"timeout,omitempty" description:"Optional timeout in seconds (max 120)"`
}

// FetchPermissionsParams defines the permission parameters for the simple fetch tool.
type FetchPermissionsParams struct {
	URL     string `json:"url"`
	Format  string `json:"format"`
	Timeout int    `json:"timeout,omitempty"`
}

func (p *AgenticFetchParams) UnmarshalJSON(data []byte) error {
	type rawAgenticFetchParams struct {
		URL    string `json:"url,omitempty"`
		URLs   string `json:"urls,omitempty"`
		Links  string `json:"links,omitempty"`
		Prompt string `json:"prompt,omitempty"`
		Query  string `json:"query,omitempty"`
		Search string `json:"search,omitempty"`
		Q      string `json:"q,omitempty"`
	}

	var raw rawAgenticFetchParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.URL = firstFetchString(raw.URL, raw.URLs, raw.Links)
	p.Prompt = firstFetchString(raw.Prompt, raw.Query, raw.Search, raw.Q)
	return nil
}

func (p *WebSearchParams) UnmarshalJSON(data []byte) error {
	type rawWebSearchParams struct {
		Query      string `json:"query,omitempty"`
		Q          string `json:"q,omitempty"`
		Search     string `json:"search,omitempty"`
		SearchTerm string `json:"search_query,omitempty"`
		Term       string `json:"term,omitempty"`
		MaxResults int    `json:"max_results,omitempty"`
		NumResults int    `json:"num_results,omitempty"`
		Count      int    `json:"count,omitempty"`
		Limit      int    `json:"limit,omitempty"`
		Results    int    `json:"results,omitempty"`
	}

	var raw rawWebSearchParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.Query = firstFetchString(raw.Query, raw.Q, raw.Search, raw.SearchTerm, raw.Term)
	p.MaxResults = firstFetchInt(raw.MaxResults, raw.NumResults, raw.Count, raw.Limit, raw.Results)
	return nil
}

func (p *FetchParams) UnmarshalJSON(data []byte) error {
	type rawFetchParams struct {
		URL          string `json:"url,omitempty"`
		URI          string `json:"uri,omitempty"`
		Link         string `json:"link,omitempty"`
		Href         string `json:"href,omitempty"`
		Address      string `json:"address,omitempty"`
		Format       string `json:"format,omitempty"`
		Output       string `json:"output,omitempty"`
		OutputFormat string `json:"output_format,omitempty"`
		Type         string `json:"type,omitempty"`
		Timeout      int    `json:"timeout,omitempty"`
		TimeoutSecs  int    `json:"timeout_seconds,omitempty"`
		MaxTimeout   int    `json:"max_timeout,omitempty"`
	}

	var raw rawFetchParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.URL = firstFetchString(raw.URL, raw.URI, raw.Link, raw.Href, raw.Address)
	p.Format = firstFetchString(raw.Format, raw.Output, raw.OutputFormat, raw.Type)
	p.Timeout = firstFetchInt(raw.Timeout, raw.TimeoutSecs, raw.MaxTimeout)
	return nil
}

func firstFetchString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstFetchInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
