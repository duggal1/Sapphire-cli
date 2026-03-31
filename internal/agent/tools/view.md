Reads and displays file contents with line numbers for examining code, logs, or text data. Supports reading multiple files in parallel.

<usage>
- **CRITICAL ROUTING RULE**: `agentic_view` is the default repository read tool. It can read one file or many. Use it for any non-trivial task, any subsystem read, any architecture trace, any initialization pass, and any broad repo slice or codebase request. Use `single_view` only for an explicitly user-narrowed or guaranteed-trivial one-file read. Normal general or semi-complex investigation should start with about 12-20 relevant files. Initialization or broad codebase mapping should use aggressive 20-30 file sweeps and continue until major domains are covered. For a narrow but complex task, read all main relevant files tied to the task before editing. NEVER use repeated `single_view` calls for multi-file reads.
- Provide "file_paths" (array of strings) for any batch size, including 1 file
- Provide "file_path" (string) only as legacy single-file shorthand
- Optional offset: start reading from specific line (1-based index, defaults to 1)
- Optional limit: control lines read (default 2000)
- Optional mode: "slice" (default) or "indentation" (Codex-compatible indentation-aware reading)
- Optional indentation (when mode is "indentation"): Object with `anchor_line`, `max_levels`, `include_siblings`, `include_header`, `max_lines`
- Don't use for directories (use LS tool instead)
- Supports image files (PNG, JPEG, GIF, BMP, SVG, WebP)
</usage>

<features>
- Parallel reading: `agentic_view` can read up to 250 files simultaneously
- Recommended working batches: about 12-20 relevant files for normal work and about 20-30 for broad mapping
- Displays contents with line numbers
- Can read from any file position using 1-indexed offset
- Indentation-aware context gathering using two-pointer expansion
- Automatically detects comments and expands tabs to 4 spaces
- Handles large files by limiting lines read
- Auto-truncates very long lines for display
- Suggests similar filenames when file not found
- Renders image files directly in terminal
</features>

<limitations>
- Max file size: 25MB
- Default limit: 2000 lines
- Lines >2000 chars truncated
- Binary files (except images) cannot be displayed
- **TRUNCATION**: If the file is larger than the limit, or if certain sections are hidden (e.g., `... (146 lines hidden)`), you **MUST** use `offset` and `limit` to read those hidden sections before performing any edits. Failing to read the full file is a critical error.
</limitations>

<cross_platform>
- Handles Windows (CRLF) and Unix (LF) line endings
- Works with forward slashes (/) and backslashes (\)
- Auto-detects text encoding for common formats
</cross_platform>

<tips>
- Use `ls` or `tree` to discover file names before reading
- Use with Glob to find files after a directory listing
- For code exploration: use `tool_search` first when the code location is unknown, `rg_files` when the path shape is known, `rg` when the exact text is known, then use `agentic_view` to inspect the winners in parallel
- For “read the repo”, “inspect this subsystem”, initialization, or architecture requests, default to `agentic_view` immediately, even if you are starting from one file
- For initialization or codebase mapping, prefer aggressive 20-30 file batches, then follow with targeted sweeps until the important domains and runtime paths are actually understood
- For large files: read segmented ranges with `offset`/`limit`; issue multiple parallel View calls for different ranges
- View tool automatically detects and renders image files
</tips>
