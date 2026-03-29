# google_search

Use Gemini grounding with Google Search to find real-time information, news, and technical documentation.
This tool can combine Google Search with Gemini URL Context when you provide one or more URLs.
It returns grounded sources, Google search queries, and URL retrieval metadata when available.

## Parameters

- `query` (string, optional): The search query to execute. If omitted, the tool uses the current user request.
- `url` (string, optional): One URL to analyze with Gemini URL context.
- `urls` (array, optional): Multiple URLs to analyze with Gemini URL context.
- `max_results` (integer, optional): Maximum number of grounded sources to return (default: 10).
