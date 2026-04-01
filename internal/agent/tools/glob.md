Fast file pattern matching tool that finds files by name/pattern, returning paths sorted by modification time (newest first).

<strict_contract>
- Use exactly one `pattern` string per call.
- Use `path` for one root or `paths` for multiple roots.
- Never pass multiple patterns, and never use `glob` for content search.
- If the roots are unknown, discover them first with `tool_search`, `rg_files`, `rg`, or `ls`.
</strict_contract>

<usage>
- Provide glob pattern to match against file paths
- Optional starting directory (defaults to current working directory)
- Use `paths` to search multiple roots in one parallel call
- Results sorted with most recently modified files first
</usage>

<pattern_syntax>
- '\*' matches any sequence of non-separator characters
- '\*\*' matches any sequence including separators
- '?' matches any single non-separator character
- '[...]' matches any character in brackets
- '[!...]' matches any character not in brackets
</pattern_syntax>

<examples>
- '*.js' - JavaScript files in current directory
- '**/*.js' - JavaScript files in any subdirectory
- 'src/**/*.{ts,tsx}' - TypeScript files in src directory
- '*.{html,css,js}' - HTML, CSS, and JS files
</examples>

<limitations>
- Results limited to 100 files (newest first)
- Does not search file contents (use Grep for that)
- Hidden files (starting with '.') skipped
</limitations>

<cross_platform>
- Path separators handled automatically (/ and \ work)
- Uses ripgrep (rg) if available, otherwise Go implementation
- Patterns should use forward slashes (/) for compatibility
</cross_platform>

<tips>
- If you need the same glob across multiple roots, batch them into one `glob` call with `paths`
- Combine with Grep: find files with Glob, search contents with Grep
- Prefer `rg_files` when the filename/path shape is already known, and prefer `tool_search` when the code location is still unknown
- For iterative exploration requiring multiple searches, consider Agent tool
- Check if results truncated and refine pattern if needed
</tips>
