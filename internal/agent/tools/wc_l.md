Count file lines only.

Use this tool when:
- you need exact file length before deciding whether to read the whole file
- you want a structured alternative to shell `wc -l`
- you want to size an `agentic_view` read, or a segmented `single_view` read in the rare explicit trivial one-file case, before opening a long file

Do not use this tool for:
- words/bytes/chars; use `wc`
- reading file contents; use `agentic_view` by default, or `single_view` only for an explicitly narrow trivial one-file read

Parameters:
- `path` / `paths`: one or more files to inspect
