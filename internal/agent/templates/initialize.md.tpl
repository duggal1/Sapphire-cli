# Analyze this Codebase/Repository and Create/Update {{.Config.Options.InitializeAs}}

Create or update `{{.Config.Options.InitializeAs}}` so future agents can understand and work in this codebase/repository correctly.

---

## Goal

Document everything an agent needs to work in this codebase — commands, patterns,
conventions, and gotchas. Aim for **completeness over brevity**. Include everything
an agent would need to know. Use judgment on depth based on what you actually find.

---

## Tool Usage Rules

- Read first. Write second.
- Document only what is explicitly observed.
- Do not invent commands, paths, architecture, conventions, or workflows.
- Use `agentic_view` aggressively for broad and multi-file repository reads.
- Use `single_view` for one known file.
- Use `ls`, `glob`, and `grep` for discovery.
- Do not use `bash` for repo discovery or file reading when structured tools exist.
- Do not force sub-agents. Use them only when repo complexity justifies them.

---

## Pre-Flight Check

Run `ls`.

If the directory is empty or only contains config files, output exactly:

> `Directory appears empty or only contains config. Add source code first, then run
> this command to generate {{.Config.Options.InitializeAs}}.`

Then stop.

---

## Step 1 — Assess Complexity

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

Read representative source files to understand:
- Code organization within files
- Naming conventions (variables, functions, files, directories)
- Testing approach and structure
- Any non-obvious or project-specific patterns

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
Spawn one sub-agent per major domain.

### Sub-Agent Requirements (if used)

Each sub-agent must:

1. Read at least one anchor file with `single_view`
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
