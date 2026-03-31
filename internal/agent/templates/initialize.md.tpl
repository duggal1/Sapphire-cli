# Analyze this Codebase/Repository and Create/Update {{.Config.Options.InitializeAs}}

Create or update `{{.Config.Options.InitializeAs}}` so future agents can understand and work in this codebase/repository correctly.

---

## Goal

Document everything an agent needs to work in this codebase — commands, patterns,
conventions, and gotchas. Aim for **completeness over brevity**. Initialization is a
deep repository survey task, not a shallow summary task.

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

## Tool Usage Rules

- Read first. Write second.
- Document only what is explicitly observed.
- Do not invent commands, paths, architecture, conventions, or workflows.
- `agentic_view` is the default read tool for initialization.
- Use `single_view` only when exactly one verified file is sufficient for a local follow-up.
- For initialization, use `agentic_view` in aggressive broad sweeps of about 20-30 files when available.
- If the repo has fewer meaningful files, read all of them.
- Keep sweeping until every major domain has representative coverage across source, config, tests, scripts/build, and rules/docs.
- Context quality is part of the deliverable. Read the main important files for each major domain deeply enough to explain actual behavior, architecture, conventions, and integration points.
- Do not stop after root files, dependency files, and one or two entry points.
- Use `tool_search` first when the right file, symbol, subsystem anchor, or product term is still unknown.
- Use `rg_files` when the filename/path shape is already known, `rg` when the exact text pattern is already known, and `wc_l` / `wc` before large reads when file size matters.
- Use `ls`, `glob`, and `grep` for layout inspection, glob expansion, or fallback discovery when the narrower tool is insufficient.
- Do not use `bash` for repo discovery or file reading when structured tools exist.
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

### 2.3 — Project Type and Tooling

Identify project type from:
- Config files (`package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`, etc.)
- Directory structure and naming
- CI configuration files (`.github/workflows/`, `Makefile`, `justfile`, etc.)

### 2.4 — Commands

Find build, test, run, lint, and deploy commands from:
- Config files
- Scripts directories
- Makefiles and justfiles
- CI workflow definitions

Do **not** invent commands. If you cannot confirm a command exists, do not include it.

### 2.5 — Source Code Patterns

Use `agentic_view` for aggressive broad mixed sweeps, typically about 20-30 files at a time when available. Read source, test, config,
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

After the first broad sweep, run targeted follow-up sweeps for any uncovered major domain or critical runtime path.

### 2.6 — Existing `{{.Config.Options.InitializeAs}}`

If the file already exists, **read it first**. Improve it incrementally instead of
blindly overwriting it.

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

1. Read at least one anchor file with `single_view` only if exactly one anchor file is useful
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
- Do **not** produce a terse summary. Anything materially under roughly 100 lines for a real repository is presumptively incomplete.
- Small repos with real code should still usually produce at least roughly 100-120 lines of verified material.
- Medium and large repos should usually land in several hundred lines of verified material, often about 300-600 lines when the evidence supports it.
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
