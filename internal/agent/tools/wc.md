Count file lines, words, bytes, and characters.

Use this tool when:
- you need to gauge file size before reading or editing
- you need density/size signals for one or more files
- you need a structured alternative to shell `wc`
- you want to decide between a full read, segmented read, or skipping a very long file

Do not use this tool for:
- line counts only; use `wc_l`
- reading file contents; use `single_view` or `agentic_view`

Parameters:
- `path` / `paths`: one or more files to inspect
