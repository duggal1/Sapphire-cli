Analyze this codebase and create/update **{{.Config.Options.InitializeAs}}** to help future agents work effectively in this repository.

Capabilities:
- Tool discovery: `list_tools` → `search_tools` → `tool_suggest` → `connect_mcp`.
- Structured repo reads: `ls` for paths, `glob` for filename search, `grep` for content search, `single_view` for exactly 1 file, `agentic_view` for 2+ files.
- `bash` is not a repository discovery or file-reading tool. Do not use `bash` for `find`, `ls`, `cat`, `head`, `tail`, `grep`, `rg`, `tree`, or prompt/CSV setup when structured tools exist.
- Never create temporary `.txt` or `.csv` payload files just to call `spawn_agent` or related tools. Pass prompts directly as tool parameters.
- `view_memory` is the long-horizon recovery tool. Use it to recover exact earlier decisions, older tool/result trails, and prior-session context when current context is no longer enough.
- `refresh_memory` forces regeneration of `memory.md` from the live codebase map and session state. Use it only after meaningful new information.
- Explicit sub-agent lifecycle: `spawn_agent` (supports `isolation`, `model`, `reasoning_effort`, `fork_context`, `worktree_path`, `branch`, `write_manifest`, `definition_of_done`) → `resume_agent` → `send_input` → `wait` → `collect_result` → `close_agent`.
- Batch worker helper: `spawn_agents_on_csv`, `report_agent_job_result`. Use only for CSV row execution, not to replace the explicit sub-agent lifecycle.
- Worktree orchestration: `orchestrate_worktrees` is a batch helper for pre-scoped parallel worktrees. Do not use it when the task is about real sub-agent behavior, coordination, handoffs, wait/collect flow, or sub-agent debugging.
- Write isolation: `write_manifest` restricts writes only; reads/commands are unrestricted. Empty list = read-only.
- Execution loop: observe → reason → act (one tool) → wait → observe.

**First**: Check if directory is empty or contains only config files. If so, stop and say "Directory appears empty or only contains config. Add source code first, then run this command to generate {{.Config.Options.InitializeAs}}."

**Goal**: Document what an agent needs to know to work in this codebase - commands, patterns, conventions, gotchas.

**Discovery process**:

1. Check directory contents with `ls`
2. Read exactly 1 file with `single_view`. Read 2 or more files with `agentic_view`. Keep each `agentic_view` batch to 2–30 files. If more than 30 files are needed, split into multiple `agentic_view` calls.
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
