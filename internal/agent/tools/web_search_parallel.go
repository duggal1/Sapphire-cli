package tools

import (
	"context"
	"net/http"

	"golang.org/x/sync/errgroup"
)

type ParallelWebSearchResult struct {
	Query  string
	Output string
	Err    error
}

// ParallelWebSearch executes multiple search queries concurrently.
func ParallelWebSearch(ctx context.Context, client *http.Client, queries []string, maxResults int) []ParallelWebSearchResult {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(5)

	results := make([]ParallelWebSearchResult, len(queries))
	for i, query := range queries {
		i, query := i, query
		g.Go(func() error {
			maybeDelaySearch()
			searchRes, err := searchDuckDuckGo(ctx, client, query, maxResults)
			results[i] = ParallelWebSearchResult{Query: query}
			if err != nil {
				results[i].Err = err
				return nil
			}
			results[i].Output = formatSearchResults(searchRes)
			return nil
		})
	}

	_ = g.Wait()
	for i := range results {
		if results[i].Query == "" && i < len(queries) {
			results[i].Query = queries[i]
		}
		if results[i].Err == nil && results[i].Output == "" && ctx.Err() != nil {
			results[i].Err = ctx.Err()
		}
	}
	return results
}
