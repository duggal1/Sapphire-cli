---
name: harness
description: "Builds or restructures a Sapphire harness. Use when a task needs a strict multi-agent workflow, reusable project skills, or a stronger orchestration design for non-trivial work."
---

# Harness — Agent Team & Skill Architect

Meta skill for building a Sapphire harness that improves complex-task execution through strict routing, reusable skills, and explicit orchestration.

**Core principles:**
1. Call `run_harness` first on complex work. Treat its execution contract as the routing baseline.
2. Use local skills first. Load bundled or already-installed local skills before implementation.
3. Use extended skills only as fallback. Call `install_skill` only when local search returns no direct fit or only weak generic fits.
4. Implement teams with real Sapphire primitives only: `spawn_agent`, `send_input`, `wait`, `collect_result`, `close_agent`, `agent_mail_send`, `agent_mail_inbox`, `agent_mail_ack`, and `agent_directory`.
5. Use file artifacts and durable coordination, not implicit shared state.

## Workflow

### Phase 1: Domain Analysis
1. Identify the domain, task class, and final deliverable.
2. Inspect the codebase and current runtime constraints.
3. Search existing local skills first with `search_skills`.
4. If local search returns a strong match, load it immediately with `load_skill`.
5. If local search is empty or clearly insufficient, use `install_skill`, then treat the installed result as a local skill and load it.
6. Decide whether the work is simple, isolated, or truly multi-phase.
7. If the work is complex, call `run_harness` and use its contract as the starting point.

### Phase 2: Team Architecture Design

#### 2-1. Select Execution Mode: Agent Team vs Isolated Worker

**Default for complex work: agent team.** In Sapphire, an agent team is implemented as multiple spawned sub-agents coordinating through durable mail and shared file artifacts.

Use an **agent team** when:
- 2 or more specialists must collaborate
- peer review or adversarial checking improves quality
- the task spans multiple domains
- partial results must be integrated

Use an **isolated worker** when:
- 1 worker can complete the job
- peer communication is unnecessary
- the task is a one-shot analysis or implementation pass

**Implementation rule:**
- Agent team: multiple `spawn_agent` calls + `agent_mail_*` + `_workspace/` artifacts + `wait`/`collect_result`
- Isolated worker: `agent` for one-shot execution, or 1 `spawn_agent` if follow-up interaction is required

> For the comparison table and decision logic, see `references/agent-design-patterns.md`.

#### 2-2. Select an Architecture Pattern

1. Decompose the work into specialist areas.
2. Choose the structure that matches the dependency pattern:
   - **Pipeline**: sequential dependent work
   - **Fan-out/Fan-in**: parallel independent work
   - **Expert Pool**: selective invocation by situation
   - **Producer-Reviewer**: generation followed by verification
   - **Supervisor**: dynamic work assignment at runtime
   - **Hierarchical Delegation**: staged delegation with limited depth

#### 2-3. Agent Separation Criteria

Decide across four axes: expertise, parallelism, context load, and reuse.

### Phase 3: Define Agent Work Packets

Sapphire does **not** currently use a custom agent-definition registry such as `.sapphire/agents/{name}.md` as a runtime requirement. Do not invent one as an execution dependency.

Instead, define each spawned worker through a strict work packet expressed in the `spawn_agent.message` plus tool parameters.

Every worker packet must specify:
- selected profile: `coder` or `task`
- exact owned files, domains, or artifact boundaries
- required local skills to load
- output path under `_workspace/`
- blocker protocol
- definition of done

**Profile selection:**
- `coder`: implementation, mutation, or active code change ownership
- `task`: read-heavy analysis, QA, review, validation, or research without repository mutation

**Model selection:** do not hardcode a fixed model in the harness. Set `model` only when the user or runtime policy requires it.

**Team reconfiguration:** if later phases need different specialists, preserve earlier artifacts, `close_agent` the old set, then `spawn_agent` the next set.

**Required rules when including a QA worker:**
- use profile `task` by default
- use `coder` only if QA is explicitly allowed to make fixes
- the core QA task is cross-boundary comparison, not isolated existence checks
- run QA incrementally after each module, not only once at the end
- see `references/qa-agent-guide.md`

### Phase 4: Create Skills

Create each reusable project skill at `<repo-root>/.sapphire/skills/{name}/SKILL.md`.

#### 4-1. Skill Structure

```
skill-name/
├── SKILL.md (required)
│   ├── YAML frontmatter (name, description required)
│   └── Markdown body
└── Bundled Resources (optional)
    ├── scripts/    - executable code for repetitive or deterministic work
    ├── references/ - reference documents loaded conditionally
    └── assets/     - files used in output
```

#### 4-2. Description Writing — Drive Strong Triggering

The description is the trigger boundary for a skill. Sapphire is conservative, so the description must state both the task and the triggering situation clearly.

**Bad example:** `"A skill for processing PDF documents"`
**Good example:** `"Handles PDF extraction, transformation, validation, and output generation. Use when the task mentions a PDF file or requires PDF output."`

State:
- what the skill does
- when it must trigger
- nearby cases that must not trigger it
- whether the workflow is covered by a local skill already and extended install is therefore forbidden

#### 4-3. Body Writing Principles

| Principle | Description |
|------|------|
| **Explain Why** | State the operating reason for important rules. |
| **Keep It Lean** | Keep `SKILL.md` under 500 lines. Move detail into `references/` when needed. |
| **Generalize** | Encode principles, not one-off fixes. |
| **Bundle Repeated Code** | If the same script appears repeatedly in testing, move it into `scripts/`. |
| **Write Imperatively** | Use direct instruction form. |

#### 4-4. Progressive Disclosure

The skill should manage context with three layers:

| Stage | Load timing | Target size |
|------|----------|----------|
| **Metadata** (name + description) | always present in inventory | short |
| **`SKILL.md` body** | after `load_skill` | <500 lines |
| **`references/`** | only when needed | unrestricted |

**Size control rules:**
- If `SKILL.md` approaches 500 lines, move detail to `references/`.
- If a reference file exceeds 300 lines, add a table of contents.
- Split domain-specific variants so only the relevant reference is loaded.

#### 4-5. Skill-Agent Mapping Principle

- 1 worker may use 1 to N skills
- the same skill may serve multiple workers
- a skill defines **how**
- a worker packet defines **who owns what**

> For detailed patterns and examples, see `references/skill-writing-guide.md`.

### Phase 5: Integration and Orchestration

The orchestrator is the top-level skill or execution plan that coordinates the workers and their artifacts.

#### 5-0. Orchestrator Pattern by Mode

**Agent Team Mode (default for complex work):**

```
[Orchestrator]
    ├── spawn_agent x N
    ├── agent_directory for current topology and aliases
    ├── agent_mail_send / agent_mail_inbox / agent_mail_ack for peer coordination
    ├── send_input for leader intervention
    ├── wait + collect_result
    └── close_agent
```

**Isolated Worker Mode:**

```
[Orchestrator]
    ├── agent(...) for one-shot isolated work
    └── or
    ├── spawn_agent
    ├── wait
    ├── collect_result
    └── close_agent
```

#### 5-1. Data Transfer Protocol

State the transfer method explicitly:

| Strategy | Method | Execution mode | Appropriate use |
|------|------|----------|-----------|
| **Mail-based** | `agent_mail_send` / `agent_mail_inbox` / `agent_mail_ack` | agent team | direct coordination, blockers, peer review |
| **Leader-control** | `send_input`, `wait`, `collect_result`, `close_agent`, `agent_directory` | both | orchestration control, monitoring, intervention |
| **File-based** | agreed paths under `_workspace/` | both | artifacts, structured results, audit trace |

**Recommended combination for complex work:** file-based artifacts + mail-based peer coordination + leader control-plane tools.

Rules for file-based transfer:
- create `_workspace/` under the working directory
- use deterministic filenames such as `{phase}_{worker}_{artifact}.{ext}`
- keep intermediate artifacts in `_workspace/`
- write final outputs only to the requested destination

#### 5-2. Error Handling

Define a strict policy:
- retry once
- if the retry fails, continue without that result when possible
- preserve conflicting sources with attribution
- if a worker stalls, either `send_input` a corrective instruction or replace it with a new `spawn_agent`

#### 5-3. Team Size Guidelines

| Work scale | Recommended active workers | Notes |
|----------|------------|------|
| Small | 2 to 3 | keep coordination light |
| Medium | 3 to 5 | enough specialization without drift |
| Large | 5 to 7 | only if integration is explicit and well-scoped |

### Phase 6: Verification and Testing

#### 6-1. Structure Verification

- confirm skills live in the expected location
- validate `SKILL.md` frontmatter (`name`, `description`)
- confirm that every referenced tool name exists in Sapphire
- confirm that no fake runtime primitive remains in the harness

#### 6-2. Execution-Mode-Specific Verification

- Agent team mode: verify `spawn_agent` ownership, mail routes, artifact paths, and `wait`/`collect_result`/`close_agent` flow
- Isolated worker mode: verify `agent` or single-worker `spawn_agent` routing, outputs, and cleanup

#### 6-3. Skill Execution Testing

For each generated skill:
1. write 2 to 3 realistic prompts
2. compare with-skill vs baseline when possible
3. evaluate with qualitative review or explicit assertions
4. generalize fixes before changing the skill
5. bundle repeated deterministic work into `scripts/`

#### 6-4. Trigger Verification

For each skill:
1. write 8 to 10 `should-trigger` prompts
2. write 8 to 10 `should-NOT-trigger` prompts
3. verify no conflict with existing local skills

#### 6-5. Dry-Run Testing

- verify the phase order is executable
- verify every tool mentioned is real
- verify every artifact path is explicit
- verify every fallback path is reachable

#### 6-6. Write Test Scenarios

- add a `## Test Scenarios` section to the orchestrator skill or execution brief
- include at least one normal flow and one error flow

## Output Checklist

Confirm after generation:

- [ ] `run_harness` contract is reflected in the final orchestration design
- [ ] `<repo-root>/.sapphire/skills/` contains the required reusable skills (`SKILL.md` + `references/` where needed)
- [ ] 1 orchestrator skill or equivalent top-level execution brief exists
- [ ] execution mode is declared clearly
- [ ] every spawned worker has explicit profile, scope, artifact path, and definition of done
- [ ] no fake runtime primitive is referenced
- [ ] no conflict with existing skills
- [ ] skill descriptions are specific enough to trigger
- [ ] every `SKILL.md` stays lean or is split into `references/`
- [ ] local skills were searched and loaded before any extended install was considered
- [ ] any extended install was justified by an insufficient local match
- [ ] execution validation completed with 2 to 3 test prompts
- [ ] trigger validation completed

## References

- Harness patterns: `references/agent-design-patterns.md`
- Existing harness examples: `references/team-examples.md`
- Orchestrator template: `references/orchestrator-template.md`
- **Skill writing guide**: `references/skill-writing-guide.md`
- **Skill testing guide**: `references/skill-testing-guide.md`
- **QA agent guide**: `references/qa-agent-guide.md`
