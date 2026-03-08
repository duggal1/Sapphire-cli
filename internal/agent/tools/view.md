Reads and displays file contents with line numbers for examining code, logs, or text data. Supports reading multiple files in parallel.

<usage>
- Provide "file_paths" (array of strings) to read multiple files concurrently
- Provide "file_path" (string) to read a single file (legacy)
- Optional offset: start reading from specific line (0-based, applies to single file only)
- Optional limit: control lines read (default 2000, applies to single file only)
- Don't use for directories (use LS tool instead)
- Supports image files (PNG, JPEG, GIF, BMP, SVG, WebP)
</usage>

<features>
- Parallel reading: Read up to 250 files simultaneously (main agent) or 50 files (sub-agents)
- Displays contents with line numbers
- Can read from any file position using offset
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
- Use with Glob to find files first
- For code exploration: Grep to find relevant files, then View to examine
- For large files: use offset parameter for specific sections
- View tool automatically detects and renders image files
</tips>
