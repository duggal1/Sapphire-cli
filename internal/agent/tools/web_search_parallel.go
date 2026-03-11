package tools

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/sync/errgroup"
)

// ParallelWebSearch executes multiple search queries concurrently.
func ParallelWebSearch(ctx context.Context, client *http.Client, queries []string, maxResults int) ([]string, error) {
	// Bounded fan-out: max 5 parallel searches to avoid rate limits
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(5)

	results := make([]string, len(queries))
	for i, query := range queries {
		i, query := i, query
		g.Go(func() error {
			// maybeDelaySearch is non-blocking and handles staggering
			maybeDelaySearch()
			
			searchRes, err := searchDuckDuckGo(ctx, client, query, maxResults)
			if err != nil {
				return fmt.Errorf("search %q failed: %w", query, err)
			}
			results[i] = formatSearchResults(searchRes)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
