Reads and displays file contents with line numbers for examining code, logs, or text data. Supports reading multiple files in parallel.

<usage>
- **CRITICAL ROUTING RULE**: Use `single_view` only when exactly 1 verified repository file is truly sufficient. For any non-trivial task, any 2+ file read, any subsystem read, any architecture trace, any initialization pass, or any broad repo slice / codebase request, use `agentic_view`. Normal non-trivial investigation should start with about 10-20 relevant files. Initialization or broad codebase mapping should use aggressive 20-30 file sweeps and continue until major domains are covered. For a narrow but complex task, read all main relevant files tied to the task before editing. NEVER use repeated `single_view` calls for multi-file reads.
- Provide "file_paths" (array of strings) to read multiple files concurrently
- Provide "file_path" (string) to read a single file (legacy)
- Optional offset: start reading from specific line (1-based index, defaults to 1)
- Optional limit: control lines read (default 2000)
- Optional mode: "slice" (default) or "indentation" (Codex-compatible indentation-aware reading)
- Optional indentation (when mode is "indentation"): Object with `anchor_line`, `max_levels`, `include_siblings`, `include_header`, `max_lines`
- Don't use for directories (use LS tool instead)
- Supports image files (PNG, JPEG, GIF, BMP, SVG, WebP)
</usage>

<features>
- Parallel reading: `agentic_view` can read up to 250 files simultaneously
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
- For code exploration: Grep to find relevant files, then use `agentic_view` to inspect them in parallel
- For “read the repo”, “inspect this subsystem”, initialization, or architecture requests, default to large `agentic_view` batches instead of serial `single_view`
- For initialization or codebase mapping, prefer aggressive 20-30 file batches, then follow with targeted sweeps until the important domains and runtime paths are actually understood
- For large files: read segmented ranges with `offset`/`limit`; issue multiple parallel View calls for different ranges
- View tool automatically detects and renders image files
</tips>
