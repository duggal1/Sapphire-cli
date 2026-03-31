Run real ripgrep content search across one or more roots.

Use this tool when:
- you want explicit ripgrep behavior, not the `grep` fallback path
- you need exact high-speed text search across a large repo
- you already know the content pattern and want the matching files and lines

Do not use this tool for:
- filename/path discovery only; use `rg_files`
- broad codebase location from vague intent; use `tool_search`
- broad repository reading; use `agentic_view`

Parameters:
- `pattern`: ripgrep pattern to search for
- `path` / `paths`: one or more roots
- `include`: optional file glob
- `literal_text`: escape regex metacharacters and search exact text
- `case_sensitive`: default false
- `limit`: cap returned matches
