# Analyze this Codebase/Repository and Create/Update {{.Config.Options.InitializeAs}}

Create or update `{{.Config.Options.InitializeAs}}` so future agents can understand and work in this codebase/repository correctly.

---

## Goal

Document everything an agent needs to work in this codebase: commands, patterns, conventions, architecture, runtime paths, and gotchas. Aim for **completeness over brevity**. Initialization is a deep repository survey task, not a shallow summary task.

---

## AGENTS.md Semantics

### Purpose

- `AGENTS.md` is a repository-scoped instruction file for coding agents working in the repository.

### Scope

- An `AGENTS.md` file applies to the entire directory tree rooted at the folder that contains it.
- If multiple `AGENTS.md` files apply, the more deeply nested file takes precedence within its subtree.
- Direct system, developer, and user instructions override any `AGENTS.md` instruction.

### Rules

- Before modifying any file, check whether an `AGENTS.md` file applies to that file's path.
- For every file you touch, follow all applicable `AGENTS.md` instructions for style, structure, naming, testing, and workflow.
- Instructions in `AGENTS.md` apply only within that file's scope unless the file explicitly says otherwise.

### Discovery

- The root `AGENTS.md` file and any `AGENTS.md` files from the current working directory up to the repo root may already be provided by the harness.
- If working in a subdirectory or outside the current working directory, explicitly check for additional applicable `AGENTS.md` files.

### Conflict Resolution

1. System instructions
2. Developer instructions
3. User instructions
4. Nearest applicable `AGENTS.md`
5. Higher-level `AGENTS.md`

---

## Initialization Discipline

### Non-Negotiables

- Read first. Write second.
- Document only what is explicitly observed.
- Do not invent commands, paths, architecture, conventions, or workflows.
- Context quality is part of the deliverable. Read the main important files for each major domain deeply enough to explain actual behavior, architecture, conventions, and integration points.
- Do not stop after root files, dependency files, and one or two entry points.

### Strict Tool Routing

- Use `tool_search` first when the right file, symbol, subsystem anchor, or product term is still unknown.
- Use `rg_files` when the filename or path shape is already known.
- Use `rg` when the exact text or symbol string is already known.
- Use `wc_l` before long reads when exact line counts matter, and `wc` when file size or density matters.
- Use `ls` only for layout inspection or exact directory verification.
- Use `glob` only when explicit glob expansion is the best fit.
- Use `grep` only when you explicitly need grep behavior or `rg` is insufficient.
- `agentic_view` is the default read tool for initialization, including one-file anchor reads when you want the default path.
- Use `single_view` only for an explicitly local, trivial one-file follow-up.
- `bash` is an absolute fallback for shell-native work only. Do not use `bash` for repo discovery, code search, or file reading when structured tools exist.
- These tools are not interchangeable. Follow this routing exactly:
  unknown location -> `tool_search`
  known path shape -> `rg_files`
  known exact text or symbol -> `rg`
  line counts -> `wc_l`
  size or density -> `wc`
  layout inspection -> `ls`
  file reads -> `agentic_view`
  trivial one-file read -> `single_view`
  shell-native execution only -> `bash`

### Parallelism Protocol

- Initialization is non-trivial by default. Use structured parallel discovery unless the repo is tiny and obviously simple.
- Batch repeated structured discovery into one call first. Prefer `ls.paths`, `rg_files.paths`, `rg.paths`, `grep.paths`, and multi-path `agentic_view` over repeated single-target calls.
- Run independent structured searches and reads in parallel. Do not serialize unrelated discovery work.
- For initialization, use `agentic_view` in aggressive broad sweeps of about 20-30 files when available.
- If the repo has fewer meaningful files, read all of them.
- For large or very large repos, after the initial locator pass, run multiple parallel `agentic_view` sweeps to cover distinct domains or runtime paths. A 3-5 sweep burst is normal for a broad repo; go higher only when repo breadth clearly justifies it.
- Keep sweeping until every major domain has representative coverage across source, config, tests, scripts/build, and rules/docs.
- If the repo is broad and durable indexing is cold or too narrow, ask to run `index_codebase` for orientation. The index is orientation only, not evidence; still read exact files before writing.
- `orchestrate_worktrees` is a batch helper for pre-scoped parallel worktrees.
- Do not use sub-agents as a substitute for the main agent's primary coverage sweep.
- Use sub-agents only when repo complexity justifies them after the main agent has already surveyed the repo broadly.

---

## Pre-Flight Check

Run `ls`.

If the directory is empty or only contains config files, output exactly:

> `Directory appears empty or only contains config. Add source code first, then run
> this command to generate {{.Config.Options.InitializeAs}}.`

Then stop.

---

## Step 1 — Assess Complexity and Coverage Depth

Inspect the repo and classify it into exactly one tier:

| Tier       | Definition                                                                 |
|------------|----------------------------------------------------------------------------|
| **Small**  | Limited surface area, one main domain                                      |
| **Medium** | Multiple meaningful subsystems                                             |
| **Large**  | Broad surface area, many domains, multiple frameworks/languages, or monorepo |

### What to check

- Repo size and layout
- Languages and frameworks
- Build tooling (Makefiles, CI configs, scripts, package managers)
- Major domains present: app, services, infra, tests, docs, config, schemas,
  migrations, SDKs, generated code
- Use `wc_l` when you need exact file length before deciding whether a full-file read is reasonable.

### Minimum coverage expectation

- **Small**: if the repo has 20 or fewer meaningful files, read all of them. Otherwise read at least 20 meaningful files across all major areas.
- **Medium**: read at least 30 meaningful files if available, spanning every major domain.
- **Large**: read at least 40 meaningful files if available, spanning every major domain, then keep reading until additional sweeps stop changing the architectural picture materially.
- **Very large / monorepo cases**: after the first broad survey, continue with targeted parallel sweeps and domain follow-ups until the main runtime paths, build surface, tests, and rules are actually understood.

---

## Step 2 — Discovery Process

Execute in this exact order:

### 2.1 — Directory Contents

Run `ls`. Understand top-level layout before anything else.

### 2.2 — Existing Rule and Context Files

Look for the following files. **Only read them if they exist.**

```

.cursor/rules/\*.md
.cursorrules
.github/copilot-instructions.md
claude.md
agents.md
CLAUDE.md
AGENTS.md

```

Extract all project-specific context, constraints, and conventions from these files.
They take precedence over inferred patterns.

### 2.3 — Existing `{{.Config.Options.InitializeAs}}`

If the file already exists, **read it early**. Improve it incrementally instead of blindly overwriting it. Keep only verified material; remove or rewrite stale claims when the repo evidence disagrees.

### 2.4 — Project Type and Tooling

Identify project type from:
- Config files (`package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`, etc.)
- Directory structure and naming
- CI configuration files (`.github/workflows/`, `Makefile`, `justfile`, etc.)

### 2.5 — Commands

Find build, test, run, lint, and deploy commands from:
- Config files
- Scripts directories
- Makefiles and justfiles
- CI workflow definitions

Do **not** invent commands. If you cannot confirm a command exists, do not include it.

### 2.6 — Source Code Patterns and Runtime Paths

Use `agentic_view` for aggressive broad mixed sweeps, typically about 20-30 files at a time when available. It can also read one file when you want to stay on the default path. Read source, test, config,
build, and schema files deeply enough to understand:
- Code organization within files
- Naming conventions (variables, functions, files, directories)
- Testing approach and structure
- Any non-obvious or project-specific patterns

The survey must include all of the following when they exist:
- Entry points and bootstraps
- Primary runtime modules/packages/services
- Representative tests
- Build, lint, CI, and script files
- Config, schema, migration, or manifest files
- Existing rules/context files

Do not stop at file names or top-level summaries. Read the main important files for each domain deeply enough to explain actual runtime behavior and integration points.

After the first broad sweep, run targeted follow-up sweeps for any uncovered major domain or critical runtime path. If the repo is large, do those follow-up sweeps in parallel where domains are independent.

---

## Step 3 — Analyze by Complexity Tier

### Small Repo
Analyze directly. No sub-agents.

### Medium Repo
Spawn sub-agents only for major domains where coverage justifies it.

### Large Repo
Spawn one sub-agent per major domain only after the main agent has already completed the first broad `agentic_view` survey.

### Sub-Agent Requirements (if used)

Each sub-agent must:

1. Read at least one anchor file with `agentic_view` by default; use `single_view` only if the anchor read is explicitly one-file and trivial
2. Use `agentic_view` for broad coverage of its assigned domain
3. Return synthesized findings only — no raw file dumps
4. Report only explicitly observed facts

---

## Step 4 — Synthesize and Resolve

- If findings from different sources or sub-agents conflict, do **not** silently
  resolve them.
- Write unresolved issues explicitly using these markers:

```

⚠ CONFLICT: [describe the conflict and both sides]
⚠ UNRESOLVED: [describe what could not be confirmed]

```

---

## Required Output Structure

Write `{{.Config.Options.InitializeAs}}` using this exact section structure.
Adapt depth per section based on what was actually observed. Omit nothing major.
Cite exact file paths throughout the document.

```markdown
# {{.Config.Options.InitializeAs}}

## Repository Overview
<!-- What this repo is, what it does, who it's for -->

## Architecture Summary
<!-- High-level system design, major components and how they relate -->

## Repository Layout
<!-- Annotated directory tree, one line per entry, only major paths -->

## Entry Points and Core Files
<!-- Where execution begins, main files, critical config -->

## Key Code Areas
<!-- Important modules, packages, services — what each does -->

## Build / Run / Test / Lint
<!-- Every confirmed command. Format: what it does + the exact command -->

## Conventions and Patterns
<!-- Naming, file structure, code style, testing patterns, anything consistent -->

## Environment and Setup
<!-- Required env vars, setup steps, external dependencies, toolchain versions -->

## Rules / Memory / Existing Context Files
<!-- Everything extracted from .cursorrules, claude.md, agents.md, etc. -->

## Gotchas and Non-Obvious Behavior
<!-- Things that will bite an agent that aren't obvious from reading the code -->

## Open Questions / Conflicts / Gaps
<!-- ⚠ CONFLICT and ⚠ UNRESOLVED items go here -->
```

---

## Hard Constraints

- Do **not** write output before broad repo coverage is complete.
- Do **not** default to a one-size-fits-all short boilerplate. Document length must scale with observed repo size, file count, and domain count.
- Tiny or genuinely simple repos may land around 20-80 lines if the evidence is truly small.
- Small repos with real code often need roughly 60-150 lines of verified material.
- Anything materially under roughly 120 lines for a medium or large repository is presumptively incomplete.
- Large repos should usually land in several hundred lines of verified material, often about 300-600 lines when the evidence supports it.
- Very large repos may require more than that, but keep the document compressed, evidence-dense, and specific instead of padded.
- If the repo genuinely cannot support that depth, explain the missing evidence instead of padding or inventing content.
- Do **not** leave major sections as one-line placeholders.
- Do **not** use sub-agents unless justified by complexity.
- Do **not** dump raw file content anywhere in the output.
- Do **not** omit any major observed domain.
- Do **not** claim certainty without evidence.
- Do **not** include any command, path, pattern, or convention you did not explicitly
  observe in the repository.

---

## Operating Principle

**observe → reason → act → observe**

Every claim in the output must trace back to something you read. Nothing else.
