Searches the web using DuckDuckGo and returns search results.

<usage>
- Provide a search query to find information on the web
- Use `queries` to execute multiple related searches in one parallel call
- Returns a list of search results with titles, URLs, and snippets
- Use this to find relevant web pages, then use web_fetch to get full content
</usage>

<parameters>
- query: The search query string (required)
- queries: Optional list of search queries to run in parallel
- max_results: Maximum number of results to return (default: 10, max: 20)
</parameters>

<tips>
- Use specific, targeted search queries for better results
- Batch related searches into one call instead of issuing many sequential searches
- After getting results, use web_fetch to get the full content of relevant pages
- Combine multiple searches to gather comprehensive information
</tips>
