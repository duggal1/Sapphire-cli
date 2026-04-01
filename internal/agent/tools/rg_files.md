Run `rg --files` as a real structured tool, then rank matching file paths.

Use this tool when:
- you need filename or path discovery only
- the repo is large and browsing with `ls` would be noisy
- you know part of the filename, extension, directory, or a glob shape
- `tool_search` is unnecessary because the path shape is already clear
- you want one structured parallel filename/path lookup across multiple roots

Do not use this tool for:
- content search inside files; use `rg` or `grep`
- vague codebase intent; use `tool_search`

Parameters:
- `query`: filename/path substring query or glob pattern
- `path` / `paths`: one or more roots
- `limit`: cap returned paths
