# Sapphire Supercharge Prompt

## Current Ratings

These ratings are brutally honest and constrained by evidence quality.

| System | Rating (/10) | Basis |
|---|---:|---|
| Codex | 9.0 | Full core engine is inspectable. Strongest verified architecture, orchestration, safety model, transport model, and extensibility stack. |
| Claude Code | 8.0 | Very strong documented product surface, but full engine is not publicly inspectable from the repo at Codex-equivalent depth. |
| Sapphire CLI | 6.3 | Ambitious orchestration ideas, but weaker systems rigor, weaker safety model, and less mature large-codebase control plane. |

## Current Winner For Extremely Large Codebases

For enterprise repositories with 10,000+ files and 10M+ lines of code, the most advanced and most credible agentic CLI right now is `Codex`.

Reason:

- It has the strongest verified control plane.
- It has the strongest verified session/thread/state model.
- It has the strongest verified sandbox and approval architecture.
- It has the best evidence of being built as a platform, not just a single CLI loop.
- It has a first-class app-server boundary, which matters at enterprise scale.

`Claude Code` is clearly top-tier as a product, but I cannot honestly place it above Codex for this question because the full engine is not inspectable from the public repository.

`Sapphire CLI` has strong ideas, especially around subagents and worktrees, but today it is not in the same maturity class.

## Realistic Target Rating If Sapphire Executes This Well

If Sapphire adopts the right architectural changes and executes them well, a realistic target is:

- `8.8/10` after major architectural overhaul and successful hardening

Not `10/10`.

Why not 10:

- Codex-level runtime rigor is expensive and slow to build.
- Claude Code-level product ergonomics plus plugin surface are also expensive.
- Large-codebase agent reliability is dominated by safety, state discipline, and failure recovery, not by prompt cleverness.

If Sapphire implements only half of this, it likely lands around:

- `7.2 to 7.8/10`

## Use This Prompt Verbatim

```md
# Mission

You are redesigning Sapphire CLI to become a top-tier enterprise-grade agentic CLI for extremely large repositories:

- 10,000+ files
- 10M+ lines of code
- monorepos
- multi-language systems
- long-running engineering tasks
- high-stakes refactors
- code review
- incident response
- release engineering
- git-heavy workflows

Your job is not to imitate surface behavior.
Your job is to extract and apply the strongest architectural lessons from:

1. Codex, where the full core architecture is source-verifiable
2. Claude Code, where some product capabilities are publicly documented but not fully source-verifiable

You must be brutally honest, non-speculative, and architecture-first.

## Non-Negotiable Rules

- Do not fabricate hidden internals for Claude Code.
- Only use Claude Code features that are publicly documented.
- Treat Codex as the strongest source-verifiable reference architecture.
- Treat Sapphire's current architecture as promising but insufficient.
- Optimize for correctness, control, resilience, scalability, and operability.
- Do not optimize for demos.
- Do not optimize for marketing language.
- Do not optimize for clever prompts over systems design.

## Ground Truth About The Three Systems

### Codex: source-verifiable strengths we should learn from

Use Codex as the main architectural reference for:

- layered architecture:
  - core runtime
  - CLI
  - TUI
  - exec/headless mode
  - app-server transport layer
- explicit thread/session management
- durable rollout persistence
- state DB backed coordination
- formal tool router and tool registry
- centralized approval orchestration
- sandbox selection and retry strategy
- guardian-style secondary review for risky actions
- MCP as both client and server
- plugin and skills systems as platform features
- separation between runtime logic and UI
- enterprise-grade transport and app-server boundary
- reusable headless execution path
- cross-platform sandbox strategy
- network policy control
- unified exec process management

### Claude Code: publicly documented strengths we should learn from

Only use documented public capabilities such as:

- subagents
- hooks
- slash commands
- MCP integration
- memory model using project guidance files
- IDE integrations
- GitHub Actions / remote workflows
- SDK / headless mode
- sandboxing for command execution
- permission rules
- plugin ecosystem surface

Do not invent internal implementations that are not publicly inspectable.

### Sapphire: current reality we must confront

Current Sapphire strengths:

- strong subagent ambition
- worktree orchestration ideas
- long-horizon task artifacts
- flexible tool set
- MCP and LSP awareness
- Go monolith with relatively fast iteration

Current Sapphire weaknesses:

- too much control logic concentrated in one monolithic agent coordinator layer
- weak separation between core runtime and UI/application shell
- permission model is too soft
- shell safety is command filtering, not real OS sandboxing
- insufficiently strong state and recovery discipline for very large repos
- not enough first-class support for durable orchestration over long-running tasks
- architecture still feels like a powerful local coding assistant, not yet an enterprise-grade execution platform

## Primary Goal

Produce a concrete redesign plan that upgrades Sapphire using the strongest proven ideas from Codex and the strongest publicly documented product capabilities from Claude Code.

The plan must answer:

1. What Sapphire must change structurally
2. What Sapphire must stop doing
3. What must become first-class runtime primitives
4. What must be built for 10M-line-codebase scale
5. What must be built for safe autonomy
6. What can be deferred

## Output Requirements

Your answer must contain these sections exactly:

1. `Executive Verdict`
2. `What Codex Gets Right`
3. `What Claude Code Publicly Demonstrates`
4. `What Sapphire Must Stop Doing`
5. `Target Architecture For Sapphire`
6. `Mandatory Runtime Capabilities`
7. `Large-Codebase Strategy`
8. `Enterprise Safety Model`
9. `Agent Orchestration Redesign`
10. `Tooling And Extensibility Redesign`
11. `UI / Transport / SDK Boundaries`
12. `Implementation Phases`
13. `Biggest Risks`
14. `Realistic Rating Before And After`

## Hard Architectural Directions

### 1. Split Sapphire into real layers

Sapphire must stop centering everything in one coordinator-heavy monolith.

Create distinct layers:

- core runtime
- session/thread state layer
- tool routing / tool execution layer
- approval and policy layer
- sandbox and process-execution layer
- memory / retrieval layer
- skills / plugins / MCP integration layer
- transports:
  - TUI
  - non-interactive CLI
  - app-server / RPC / IDE client

The core rule:

- the TUI must not own the runtime
- the runtime must not depend on the TUI
- headless execution must be first-class

This is a direct lesson from Codex.

### 2. Build a real session-thread-turn state model

Sapphire needs a formal execution model for:

- session
- thread
- turn
- item
- tool call
- approval request
- subagent spawn
- background task
- rollback / replay / resume

This model must be durable.

Requirements:

- crash-safe persistence
- resumability
- replayability
- branch/fork support
- event streaming
- partial-result recovery
- auditability

Large repositories require runtime continuity. Stateless prompt loops are not enough.

### 3. Replace soft permissions with a formal approval engine

Sapphire must stop relying on a permission UX that is effectively too permissive.

Build a first-class approval engine with:

- cacheable approval decisions
- scoped approvals:
  - one-shot
  - per-session
  - per-command-prefix
  - per-path
  - per-network-destination
- explicit deny rules
- managed policy support
- enterprise policy injection
- approval provenance
- approval telemetry

This should resemble Codex's explicit orchestration more than Sapphire's current request broker.

### 4. Introduce real OS sandboxing

This is mandatory.

Command blocking is not enough.

Sapphire must adopt real sandbox execution strategies:

- macOS:
  - Seatbelt or equivalent OS-enforced sandbox
- Linux:
  - bubblewrap / Landlock / seccomp-backed execution path
- Windows:
  - restricted token / job object / appropriate sandbox strategy

The sandbox layer must support:

- read-only mode
- workspace-write mode
- dangerous mode
- per-tool sandbox overrides
- network isolation
- writable roots
- explicit escalation path

Without this, Sapphire cannot credibly claim enterprise-safe autonomy.

### 5. Build a centralized tool orchestrator

Tool execution must be routed through a single runtime control plane that handles:

- approval
- sandbox selection
- execution
- retry rules
- output normalization
- telemetry
- cancellation
- timeout policy
- partial failure semantics

This is one of the clearest Codex advantages.

Sapphire should stop letting orchestration behavior be spread across tool implementations and coordinator logic.

### 6. Formalize subagent orchestration

Sapphire already has the instinct here.
Now it needs discipline.

Subagents must have:

- explicit parent-child relationships
- bounded context
- isolated write scopes
- isolated worktrees when needed
- durable status
- completion signals
- failure states
- resumability
- cancellation
- output contracts

Claude Code publicly demonstrates the product value of subagents.
Codex demonstrates the value of formal thread/agent control.
Sapphire should combine both.

### 7. Add a transport-neutral app-server boundary

Sapphire should not remain only a TUI-first monolith.

Build an app-server / RPC boundary for:

- IDE clients
- web surfaces
- automation
- tests
- programmatic integrations

This is a major Codex advantage.

The protocol must expose:

- thread lifecycle
- turn lifecycle
- item streaming
- approvals
- command execution
- tool state
- subagent state
- config reads/writes
- skills / plugins / MCP inventory

### 8. Treat very large codebases as a first-class operating mode

Sapphire currently behaves too much like a repo-local assistant.

For 10M-line codebases, it needs:

- codebase indexing strategy
- sharded search strategy
- cheap metadata scans before deep reads
- path-level and owner-level narrowing
- structural retrieval before semantic retrieval
- repo partitioning
- worktree isolation
- token-budget discipline
- staged context construction
- background map/review agents

Do not pretend one giant prompt can solve huge codebases.

### 9. Build a better memory architecture

Sapphire's current memory ideas are interesting but not yet enough.

Memory must split into:

- durable project memory
- thread memory
- task memory
- compaction checkpoints
- structured operational memory
- retrieval traces

Memory must also support:

- freshness
- source attribution
- invalidation
- confidence
- size budgeting

### 10. Upgrade extensibility into a platform

Sapphire should combine:

- Codex-style skills/plugins/MCP platform thinking
- Claude Code's publicly documented hooks, slash commands, and ecosystem ergonomics

Sapphire should support:

- skills
- plugins
- hooks
- slash commands
- MCP tools
- MCP resources
- remote and local extension points
- controlled marketplace or curated registries

But do not build this before the runtime is hardened.

## Large-Codebase Strategy

You must design Sapphire around these realities:

- most files are irrelevant
- full-repo reads are pathological
- indexing must be incremental
- orchestration must be decomposition-first
- subagents should map ownership and dependency slices
- write scopes should be narrow
- test execution should be distributed and bounded
- background indexing and background audit agents should exist
- repo-wide tasks should become explicit plans with machine-readable milestones

## Enterprise Safety Model

Sapphire must become trustworthy enough for serious engineering environments.

That requires:

- OS sandboxing
- explicit approvals
- network policy
- path policy
- audit log
- managed config
- policy layering
- enterprise auth and policy injection
- deterministic non-interactive mode
- headless execution with reproducible controls

If you do not solve this, do not claim enterprise readiness.

## Implementation Phases

Phase 1 must include:

- runtime / UI split
- formal tool orchestrator
- formal approval engine
- real sandboxing
- durable state model

Phase 2 must include:

- app-server / protocol
- IDE and SDK boundary
- stronger subagent lifecycle
- better memory and compaction

Phase 3 must include:

- extension platform
- enterprise policy and managed config
- large-repo specialized indexing and retrieval
- remote execution / cloud workflows if justified

## Brutal Honesty Constraint

Your answer must call out:

- where Sapphire is currently weak
- where Codex is plainly ahead
- where Claude Code appears strong only at product-surface level
- what is still unknown because Claude Code internals are not public

Do not flatter Sapphire.
Do not write a marketing roadmap.
Write the architecture plan that a serious systems team should actually follow.
```

## What We Are Looking At If This Works

If Sapphire actually absorbs the best of Codex and the public-product strengths of Claude Code, the result is not "a better prompt-driven CLI."

It becomes:

- a real agent runtime
- a safe execution platform
- a multi-surface engineering system
- a large-codebase decomposition engine
- a controlled autonomy layer for enterprise software work

In practical terms, that would mean:

- credible execution on very large monorepos
- safer autonomous edits and shell execution
- better decomposition of complex engineering work
- durable long-running tasks
- better IDE, CLI, and automation reuse
- lower failure rates on large and messy repos

## Final Brutal Verdict

Right now:

1. Codex is the most advanced and most credible agentic CLI for extremely large codebases.
2. Claude Code is clearly elite as a product surface, but not equally auditable from public source.
3. Sapphire is promising, but not yet in the same maturity class.

If Sapphire follows this prompt with discipline, it can become genuinely competitive.

If it does not adopt:

- real sandboxing
- a real approval engine
- a real session/thread/turn model
- transport separation
- a centralized tool orchestrator

then it will remain an impressive assistant, not the most powerful enterprise-grade agentic CLI.
