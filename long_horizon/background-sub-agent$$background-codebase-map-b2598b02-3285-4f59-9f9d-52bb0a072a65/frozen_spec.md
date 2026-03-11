# Frozen Spec (Session background-sub-agent$$background-codebase-map-b2598b02-3285-4f59-9f9d-52bb0a072a65)

## Task Definition
User task: Analyze this codebase and create/update AGENTS.md to enable future agents to operate effectively in this repository.

**Pre-flight check**: Run `ls`. If the directory is empty or contains only config files, stop and output: "Directory appears empty or only contains config. Add source code first, then run this command to generate AGENTS.md."

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

Small repo: Analyze directly. Use agentic_view to read all relevant files in parallel. No subagent overhead.

Medium repo: Identify the 2–4 highest-value domains. Spawn one subagent per domain. Each subagent uses agentic_view internally to read its assigned files in parallel and returns only synthesized findings.

Large repo: Map every distinct domain present in the repository. Spawn one subagent per domain — do not cap artificially. Domains may include but are not limited to: core source, services, infrastructure, CI/CD, migrations, schemas, SDKs, testing, documentation, configuration, generated code, environment setup. Each subagent uses agentic_view internally and returns only synthesized, actionable findings — no raw file dumps.

Every subagent must:
1. Use agentic_view to read assigned files in parallel
2. Return only synthesized findings — no raw file content
3. Report only what is explicitly observed — never infer or fabricate

**Step 3 — Synthesis**:
Aggregate all findings. If AGENTS.md already exists, read it first and improve it. Do not overwrite valid existing content without cause.

**Output content**:
- Essential commands: build, test, run, lint, deploy — only what exists
- Project structure and code organization
- Naming conventions and style patterns
- Testing approach and patterns
- Gotchas and non-obvious behaviors
- Environment and setup requirements
- Context extracted from existing rule files

If subagent findings conflict on the same fact, document both in AGENTS.md and flag it:
`⚠ CONFLICT: [A found X] vs [B found Y] — verify manually.`

If a subagent returns no findings for its domain, document the gap:
`⚠ UNRESOLVED: [domain] — no findings returned. Manual inspection required.`

Never silently resolve conflicts. Never fabricate findings to fill gaps.


**Format**: Structured markdown. Calibrate section depth to repository complexity. Completeness over brevity — include everything an agent needs to operate without asking questions.

**Hard constraint**: Document only what is explicitly observed. Never invent commands, patterns, file paths, or conventions. If something cannot be found, omit it.

Map the codebase relevant to this task. Identify the main packages, entry points, and the shortest list of absolute file paths that matter. Return a compact summary only.

## Success Criteria
- Implement exactly the requirements stated above.
- All relevant tests/linters pass.

## Out of Scope
- Changes unrelated to the stated task.
- Optimizations or refactors not required for success criteria.
