Count file lines only.

Use this tool when:
- you need exact file length before deciding whether to read the whole file
- you want a structured alternative to shell `wc -l`

Do not use this tool for:
- words/bytes/chars; use `wc`
- reading file contents; use `single_view` or `agentic_view`

Parameters:
- `path` / `paths`: one or more files to inspect
