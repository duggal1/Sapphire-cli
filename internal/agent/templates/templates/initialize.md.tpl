Analyze this codebase and create/update **{{.Config.Options.InitializeAs}}** to help future agents work effectively in this repository.

Capabilities:
- Tool discovery: `list_tools` → `search_tools` → `tool_suggest` → `connect_mcp`.
- Explicit sub-agent lifecycle: `spawn_agent` (supports `model`, `reasoning_effort`, `fork_context`, `worktree_path`, `branch`, `write_manifest`, `definition_of_done`) → `resume_agent` → `send_input` → `wait` → `collect_result` → `close_agent`.
- Batch worker helper: `spawn_agents_on_csv`, `report_agent_job_result`. Use only for CSV row execution, not to replace the explicit sub-agent lifecycle.
- Worktree orchestration: `orchestrate_worktrees` (parallel worktrees, optional test runners, optional integration agent).
- Write isolation: `write_manifest` restricts writes only; reads/commands are unrestricted. Empty list = read-only.
- Execution loop: observe → reason → act (one tool) → wait → observe.

**First**: Check if directory is empty or contains only config files. If so, stop and say "Directory appears empty or only contains config. Add source code first, then run this command to generate {{.Config.Options.InitializeAs}}."

**Goal**: Document what an agent needs to know to work in this codebase - commands, patterns, conventions, gotchas.

**Discovery process**:

1. Check directory contents with `ls`
2. If multiple files are needed, use `agentic_view` and read in parallel, but cap each batch to 10–15 files (default to 10). Only exceed 15 when files are tiny and the batch is still small in total tokens.
3. For very large files, split into line ranges and read multiple ranges in parallel using separate `agentic_view` calls.
4. Look for existing rule files (`.cursor/rules/*.md`, `.cursorrules`, `.github/copilot-instructions.md`, `claude.md`, `agents.md`) - only read if they exist
5. Identify project type from config files and directory structure
6. Find build/test/lint commands from config files, scripts, Makefiles, or CI configs
7. Read representative source files to understand code patterns
8. If {{.Config.Options.InitializeAs}} exists, read and improve it

**Content to include**:

- Essential commands (build, test, run, deploy, etc.) - whatever is relevant for this project
- Code organization and structure
- Naming conventions and style patterns
- Testing approach and patterns
- Important gotchas or non-obvious patterns
- Any project-specific context from existing rule files

**Format**: Clear markdown sections. Use your judgment on structure based on what you find. Aim for completeness over brevity - include everything an agent would need to know.

**Critical**: Only document what you actually observe. Never invent commands, patterns, or conventions. If you can't find something, don't include it.
