Analyze this codebase and create/update {{.Config.Options.InitializeAs}} to enable future agents to operate effectively in this repository.

Capabilities (use precisely):
- Tool discovery: `list_tools` if unsure → `search_tools` → `tool_suggest` → `connect_mcp`.
- Structured repo reads: `ls` for paths, `glob` for filename search, `grep` for content search, `single_view` for exactly 1 file, `agentic_view` for 2+ files. Use `agentic_view` comprehensively for broad repository reads.
- `bash` is not a repository discovery or file-reading tool. Do not use `bash` for `find`, `ls`, `cat`, `head`, `tail`, `grep`, `rg`, `tree`, or prompt/CSV setup when structured tools exist.
- Never create temporary `.txt` or `.csv` payload files just to call `spawn_agent` or related tools. Pass prompts directly as tool parameters.
- `view_memory` is the long-horizon recovery tool. Use it to recover exact earlier decisions, older tool/result trails, and prior-session context when current context is no longer enough.
- `refresh_memory` forces regeneration of `memory.md` from the live codebase map and session state. Use it only after meaningful new information.
- Explicit sub-agent lifecycle: `spawn_agent` (supports `isolation`, `model`, `reasoning_effort`, `fork_context`, `worktree_path`, `branch`, `write_manifest`, `definition_of_done`) → `resume_agent` → `send_input` → `wait` → `collect_result` → `close_agent`.
- Batch worker helper: `spawn_agents_on_csv`, `report_agent_job_result`. Use only for CSV row execution, not to replace the explicit sub-agent lifecycle.
- Worktree orchestration: `orchestrate_worktrees` is a batch helper for pre-scoped parallel worktrees. Do not use it when the task is about real sub-agent behavior, coordination, handoffs, wait/collect flow, or sub-agent debugging.
- Write isolation: `write_manifest` restricts writes only; reads/commands are unrestricted. Empty list = read-only.
- Execution loop: observe → reason → act (one tool) → wait → observe.

**Pre-flight check**: Run `ls`. If the directory is empty or contains only config files, stop and output: "Directory appears empty or only contains config. Add source code first, then run this command to generate {{.Config.Options.InitializeAs}}."

**Step 1 — Complexity estimation**:
Before executing anything, assess repository scale and complexity:
- Count total files and directories
- Identify languages, frameworks, and tooling present
- Detect structural patterns: monorepo, microservices, monolith, library, CLI, etc.
- Identify distinct domains: services, infra, migrations, schemas, SDKs, tests, configs, etc.

Classify the repository into one of three tiers:

- Small: ≤30 files, single language, single domain → execute analysis directly, no subagents
- Medium: 31–200 files, 1–3 languages, 2–5 domains → spawn 2–4 targeted subagents
- Large: 200+ files, multiple languages or frameworks, 5+ domains → spawn 5–12 subagents, one per distinct domain

**Step 2 — Adaptive orchestration**:

Small repo: Analyze directly. Read exactly 1 file with `single_view`. Read 2 or more files with `agentic_view`. Use `agentic_view` comprehensively for broad reads. No subagent overhead.

Medium repo: Identify the 2–4 highest-value domains. Spawn one subagent per domain. Each subagent reads exactly 1 file with `single_view` and reads 2 or more files with `agentic_view`, using `agentic_view` comprehensively for broad domain reads.

Large repo: Map every distinct domain present in the repository. Spawn one subagent per domain. Domains may include but are not limited to: core source, services, infrastructure, CI/CD, migrations, schemas, SDKs, testing, documentation, configuration, generated code, environment setup. Each subagent reads exactly 1 file with `single_view` and reads 2 or more files with `agentic_view`, using `agentic_view` comprehensively for broad domain reads. Return only synthesized, actionable findings — no raw file dumps.

Every subagent must:
1. Read exactly 1 assigned file with `single_view`. Read 2 or more assigned files with `agentic_view`. Use `agentic_view` comprehensively for broad assigned reads.
2. Return only synthesized findings — no raw file content
3. Report only what is explicitly observed — never infer or fabricate

**Step 3 — Synthesis**:
Aggregate all findings. If {{.Config.Options.InitializeAs}} already exists, read it first and improve it. Do not overwrite valid existing content without cause.

**Output content**:
- Essential commands: build, test, run, lint, deploy — only what exists
- Project structure and code organization
- Naming conventions and style patterns
- Testing approach and patterns
- Gotchas and non-obvious behaviors
- Environment and setup requirements
- Context extracted from existing rule files

If subagent findings conflict on the same fact, document both in {{.Config.Options.InitializeAs}} and flag it:
`⚠ CONFLICT: [A found X] vs [B found Y] — verify manually.`

If a subagent returns no findings for its domain, document the gap:
`⚠ UNRESOLVED: [domain] — no findings returned. Manual inspection required.`

Never silently resolve conflicts. Never fabricate findings to fill gaps.


**Format**: Structured markdown. Calibrate section depth to repository complexity. Completeness over brevity — include everything an agent needs to operate without asking questions.

**Hard constraint**: Document only what is explicitly observed. Never invent commands, patterns, file paths, or conventions. If something cannot be found, omit it.
