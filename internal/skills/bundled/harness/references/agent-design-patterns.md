# Agent Team Design Patterns

## Execution Modes: Agent Team vs Isolated Worker

Understand the core difference between the two execution modes and choose the correct mode.

### Agent Team — Default Mode for Complex Work

In Sapphire, an agent team is implemented with multiple `spawn_agent` workers plus explicit coordination.

```
[Leader]
  ├── spawn_agent(worker-a)
  ├── spawn_agent(worker-b)
  ├── agent_directory
  ├── agent_mail_send / inbox / ack
  ├── wait / collect_result
  └── close_agent
```

**Core tools:**
- `spawn_agent`: create workers with explicit ownership and definition of done
- `agent_mail_send` / `agent_mail_inbox` / `agent_mail_ack`: durable peer coordination
- `agent_directory`: inspect active agents, route aliases, and work-item relationships
- `send_input`: leader follow-up or correction
- `wait` / `collect_result` / `close_agent`: completion, result retrieval, cleanup

**Characteristics:**
- workers can challenge and verify each other through durable mail
- peer communication does not need to route through the leader
- artifacts remain in files, not only in conversation state
- the leader can re-route work without rebuilding the entire flow

**Constraints:**
- no shared task-list tool exists; use explicit ownership, artifacts, and mail instead
- worktree isolation is not the default path for `spawn_agent`
- coordination quality depends on narrow scopes and clear artifact contracts
- too many active workers increase integration overhead

**Team reconfiguration pattern:**
If different phases need different specialists, preserve `_workspace/` artifacts, `close_agent` the old set, then `spawn_agent` the next set.

### Isolated Worker — Lightweight Mode

Use one worker when peer communication is unnecessary.

```
[Main] → agent(...) → final result
```

or

```
[Main] → spawn_agent → wait → collect_result → close_agent
```

**Core tools:**
- `agent`: one-shot isolated worker execution
- `spawn_agent`: isolated worker with lifecycle control

**Characteristics:**
- fast and simple
- low coordination cost
- best for bounded one-shot tasks

**Constraints:**
- no peer review loop
- no direct worker-to-worker collaboration
- main agent owns all integration

### Mode Selection Decision Tree

```
Are there 2 or more meaningful specialist roles?
├── Yes → Does peer communication improve quality or reduce integration risk?
│         ├── Yes → Agent Team
│         └── No  → Multiple isolated workers are possible
└── No  → Isolated Worker
```

> Core rule: for complex multi-domain work, start with agent team thinking. Downgrade to isolated workers only when peer coordination adds no value.

---

## Agent Team Architecture Types

### 1. Pipeline
Sequential workflow. One worker's output becomes the next worker's input.

```
[Analysis] → [Design] → [Implementation] → [Verification]
```

**Appropriate when:** each stage depends strongly on the previous result  
**Caution:** bottlenecks slow the entire flow  
**Sapphire fit:** use one worker per stage, pass artifacts through `_workspace/`, and use `send_input` or mail only when stage correction is required

### 2. Fan-out/Fan-in
Parallel processing followed by integration.

```
         ┌→ [Specialist A] ─┐
[Router] ├→ [Specialist B] ─┼→ [Integration]
         └→ [Specialist C] ─┘
```

**Appropriate when:** one input needs multiple perspectives or domains  
**Caution:** integration quality determines total quality  
**Sapphire fit:** strongest default team pattern. Use multiple `spawn_agent` calls, durable mail for cross-checking, then `wait` + `collect_result`

### 3. Expert Pool
Invoke only the specialist required by the case.

```
[Router] → { Specialist A | Specialist B | Specialist C }
```

**Appropriate when:** only one domain applies at a time  
**Caution:** routing accuracy is critical  
**Sapphire fit:** isolated worker mode is usually sufficient

### 4. Producer-Reviewer
Generation followed by verification.

```
[Producer] → [Reviewer] → (if needed) → [Producer] rerun
```

**Appropriate when:** objective review materially improves output  
**Caution:** cap reruns to prevent loops  
**Sapphire fit:** strong team pattern. Use mail for concrete review feedback and leader-controlled retries

### 5. Supervisor
A central worker assigns or reassigns work dynamically.

```
         ┌→ [Worker A]
[Supervisor] ├→ [Worker B]
         └→ [Worker C]
```

**Appropriate when:** workload is variable or needs dynamic redistribution  
**Caution:** avoid turning the supervisor into the bottleneck  
**Sapphire fit:** use `_workspace/coordination/` files plus `agent_directory` and mail, not an imaginary shared task API

### 6. Hierarchical Delegation
Higher-level decomposition with bounded depth.

```
[Lead] → [Domain Lead A] → [Executor A1]
       → [Domain Lead B] → [Executor B1]
```

**Appropriate when:** the problem decomposes naturally into layers  
**Caution:** keep depth low to prevent latency and context loss  
**Sapphire fit:** use at most 2 levels; beyond that, flatten into a simpler team

## Composite Patterns

Composite patterns are common in production use:

| Composite pattern | Composition | Example |
|----------|------|------|
| **Fan-out + Producer-Reviewer** | parallel generation, then review per branch | multilingual content generation |
| **Pipeline + Fan-out** | sequential stages with one or more parallel stages | analysis → parallel implementation → integration |
| **Supervisor + Expert Pool** | dynamic routing to specialists | runtime triage and targeted remediation |

### Execution Mode for Composite Patterns

Use an agent team by default when the composite pattern benefits from peer review, coordination, or dynamic routing.

| Scenario | Recommended mode | Reason |
|---------|----------|------|
| Research + analysis | Agent team | findings improve when workers cross-check each other |
| Design + implementation + verification | Agent team | feedback loops reduce rework |
| Supervisor + workers | Agent team | the leader can reassign based on progress |
| One specialist at a time | Isolated worker | team coordination would add unnecessary overhead |

## Agent Profile Selection

When spawning a worker, set the profile with the `agent` parameter.

### Built-in Profiles

| Profile | Capability shape | Appropriate use |
|------|----------|-----------|
| `coder` | implementation profile with repository mutation tools | coding, refactors, writing outputs, active fixes |
| `task` | read-heavy profile without repository mutation tools | analysis, QA, research, validation, review |

### Custom Types

Sapphire does **not** currently expose a runtime custom `subagent_type` registry. Do not rely on `.sapphire/agents/{name}.md` as an executable primitive.

Encode specialization through:
- the `spawn_agent.message`
- the selected profile (`coder` or `task`)
- explicit owned scope
- local skills loaded inside the worker

### Selection Criteria

| Situation | Recommended choice | Reason |
|------|------|------|
| Code implementation or edits | `coder` | requires mutation tools |
| QA, review, research, validation | `task` | safer read-heavy profile |
| Reusable specialist behavior | profile + strict message + skills | current runtime uses prompt+skill composition, not custom agent registration |

## Agent Work Packet Structure

Use this structure in the `spawn_agent.message` or an optional planning note:

```markdown
# Worker: {name}

## Profile
{coder|task}

## Core Role
- primary responsibility
- decision boundary

## Owned Scope
- files or domains owned
- explicit non-owned areas

## Skills To Load
- exact local skill names

## Output Contract
- output path
- required format

## Blocker Protocol
- when to send mail
- when to ask leader for reroute

## Definition Of Done
- concrete completion criteria
```

## Agent Separation Criteria

| Criterion | Separate | Merge |
|------|------|------|
| Expertise | separate if domains differ materially | merge if the same reasoning handles both |
| Parallelism | separate if work can run independently | merge if the dependency chain is tight |
| Context | separate if one worker would overload | merge if one worker can hold the context cleanly |
| Reuse | separate if the role recurs | merge if it is one-off and narrow |

## Skill vs Agent

| Category | Skill | Worker |
|------|-------------|-----------------|
| Definition | procedural knowledge + bundled references/scripts | owned scope + responsibility + output contract |
| Location | `<repo-root>/.sapphire/skills/` or installed local skill store | `spawn_agent.message` or optional planning note |
| Trigger | user request or `search_skills` / `load_skill` after local-first discovery | explicit spawn by the leader |
| Purpose | "how" | "who owns what" |

A skill is a procedural guide.  
A worker is an execution unit with explicit ownership.

## Skill ↔ Agent Integration Methods

| Method | Implementation | Appropriate use |
|------|------|-----------|
| **Local skill load** | `search_skills` → `load_skill` inside the worker | reusable domain workflow |
| **Inline instructions** | include short procedural rules directly in the worker message | narrow dedicated behavior |
| **Reference load** | read `references/` only when needed | large conditional detail |

Recommended rule: use local skill loads for reusable workflows, inline only for short narrow rules, and reference loading for large conditional detail.

Strict skill rule:
- Search local skills first.
- Load a local skill immediately when it is a strong fit.
- Use `install_skill` only when local search has no direct fit or only weak generic fits.
