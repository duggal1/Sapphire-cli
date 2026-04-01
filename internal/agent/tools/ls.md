Shows files and subdirectories in tree structure for exploring project organization.

<usage>
- Provide path to list (defaults to current working directory)
- Use `paths` to list multiple directories in one parallel call
- Optional glob patterns to ignore
- Results displayed in tree structure
</usage>

<features>
- Hierarchical view of files and directories
- Auto-skips hidden files/directories (starting with '.')
- Skips common system directories like __pycache__
- Can filter files matching specific patterns
</features>

<limitations>
- Results limited to 1000 files
- Large directories truncated
- No file sizes or permissions shown
- Cannot recursively list all directories in large projects
</limitations>

<cross_platform>
- Hidden file detection uses Unix convention (files starting with '.')
- Windows hidden files (with hidden attribute) not auto-skipped
- Common Windows directories (System32, Program Files) not in default ignore
- Path separators handled automatically (/ and \ work)
</cross_platform>

<tips>
- If you need multiple directory listings, batch them into one `ls` call with `paths`
- Use `tool_search` first when the code location is unknown instead of browsing deeply
- Use `rg_files` for filename/path discovery when the path shape is known
- Use `rg` for exact text or symbol search, and `grep` only when you explicitly need grep behavior
- Parallelize independent `ls` calls by batching them with `paths` instead of issuing serial single-directory calls
- Combine with other tools for effective exploration
</tips>
