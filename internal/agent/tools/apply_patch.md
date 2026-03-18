# Apply Patch Tool

Applies a unified diff patch to a specified file. Use this for precise modifications combining additions and deletions.

## Usage

- Provide "file_path" as the absolute or repository-relative path to the file.
- Provide "unified_diff", which is the exact patch string formatted using Codex's custom structure. It must start with `*** Begin Patch`, indicate files with `*** Add File: `, `*** Update File: ` or `*** Delete File: `, group chunks with `@@` context markers, and end with `*** End Patch`.
- Provide "execution_mode":
  - "direct": (Default) Analyzes the diff hunks and applies them via Go memory manipulation. Fails on inaccurate context lines.
  - "delegate": Writes the diff to a temporary file and invokes the native OS `patch` executable for fuzzier line offset detection.
- Provide "justification" detailing why this patch is necessary to satisfy the audit trail requirements.

## Features

- Unified Diff format support exactly mimicking Codex patch routines.
- Dual-execution modes allowing fallback to delegate patching if direct manipulation errors due to offset deviations.

## Limitations

- Direct mode features comprehensive fuzzy matching for context lines but may fail if the lines dramatically diverge from reality.
- Delegate mode assumes your server environment has `patch` installed (Unix/macOS standard).
