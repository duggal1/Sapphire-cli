Get diagnostics for file and/or project.

<usage>
- Provide file path to get diagnostics for that file
- Leave path empty to get diagnostics for entire project
- Results displayed in structured format with severity levels
</usage>

<features>
- Displays errors, warnings, and hints
- Groups diagnostics by severity
- Provides detailed information about each diagnostic
- Includes exact file, line, and column locations
- Returns the full collected diagnostics without truncating the list
</features>

<limitations>
- Results limited to diagnostics provided by LSP clients
- May not cover all possible code issues
- Does not provide suggestions for fixing issues
- Compiler diagnostics may still include other-file issues separately
</limitations>

<tips>
- Use with other tools for comprehensive code review
- Combine with LSP client for real-time diagnostics
</tips>
