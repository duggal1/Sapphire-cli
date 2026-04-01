# Agent Team Design Patterns

## Execution Modes: Agent Team vs Sub-agent

Understand the core difference between the two execution modes and choose the correct mode.

### Agent Team — Default Mode

The team leader builds the team with `TeamCreate`. Team members run as independent Claude Code instances. Team members communicate directly through `SendMessage` and self-coordinate through the shared task list (`TaskCreate`/`TaskUpdate`).

```
[Leader] ←→ [Member A] ←→ [Member B]
  ↕          ↕          ↕
  └──── Shared Task List ────┘
```

**Core tools:**
- `TeamCreate`: create the team and spawn members
- `SendMessage({to: name})`: send a message to a specific member
- `SendMessage({to: "all"})`: broadcast to all members (high cost, rare use only)
- `TaskCreate`/`TaskUpdate`: manage the shared task list

**Characteristics:**
- Members can talk to, challenge, and verify each other directly
- Information exchange happens between members without passing through the leader
- Members self-coordinate through the shared task list and may request work themselves
- Idle members automatically notify the leader
- Plan approval mode can be used to review risky work before execution

**Constraints:**
- Only one team can be **active** per session (however, a team may be dismantled and replaced between phases)
- Nested teams are not allowed (a team member cannot create another team)
- The leader is fixed and cannot be transferred
- Token cost is higher

**Team reconfiguration pattern:**
If different phases require different specialist combinations, save the previous team's outputs to files, dismantle that team, then create the next team. The previous team's artifacts remain in `_workspace/`, so the new team can access them with `Read`.

### Sub-agent — Lightweight Mode

The main agent creates sub-agents with the `Agent` tool. A sub-agent returns results only to the main agent and does not communicate with other sub-agents.

```
[Main] → [Sub A] → Return result
      → [Sub B] → Return result
      → [Sub C] → Return result
```

**Core tools:**
- `Agent(prompt, subagent_type, run_in_background)`: create a sub-agent

**Characteristics:**
- Lightweight and fast
- Results return to main context in summarized form
- Token-efficient

**Constraints:**
- No communication between sub-agents
- The main agent handles all coordination
- No real-time collaboration or challenge loop

### Mode Selection Decision Tree

```
Are there 2 or more agents?
├── Yes → Is agent-to-agent communication required?
│         ├── Yes → Agent Team (default)
│         │         Cross-verification, shared discovery, and real-time feedback improve quality.
│         │
│         └── No → Sub-agents are also possible
│                  Use for producer-reviewer, expert pool, and similar cases where only result delivery is needed.
│
└── No (1 agent) → Sub-agent
                  A team is unnecessary for a single agent.
```

> **Core rule:** Agent team is the default. Before choosing sub-agents, ask one question: "Is communication between members truly unnecessary?"

---

## Agent Team Architecture Types

### 1. Pipeline
Sequential workflow. The output of the previous agent becomes the input of the next agent.

```
[Analysis] → [Design] → [Implementation] → [Verification]
```

**Appropriate when:** each stage depends strongly on the output of the previous stage
**Example:** novel writing — worldbuilding → characters → plot → drafting → editing
**Caution:** a bottleneck delays the entire pipeline. Design each stage to be as independent as possible.
**Team mode fit:** limited, because sequential dependency dominates. Still useful if the pipeline contains parallel segments.

### 2. Fan-out/Fan-in
Parallel processing followed by integration. Independent work runs concurrently.

```
         ┌→ [Specialist A] ─┐
[Router] → ├→ [Specialist B] ─┼→ [Integration]
         └→ [Specialist C] ─┘
```

**Appropriate when:** the same input requires analysis from different perspectives or domains
**Example:** composite research — official/media/community/background research in parallel → integrated report
**Caution:** integration quality determines total quality.
**Team mode fit:** the most natural agent team pattern. **This must be implemented as an agent team.** Members can share findings, challenge each other, and update investigation direction in real time. This materially improves quality over isolated work.

### 3. Expert Pool
Select and invoke the correct specialist based on the situation.

```
[Router] → { Specialist A | Specialist B | Specialist C }
```

**Appropriate when:** different input types require different handling
**Example:** code review — invoke only the security, performance, or architecture reviewer that matches the case
**Caution:** router classification accuracy is critical.
**Team mode fit:** sub-agents are usually better. Only the required specialist is invoked, so a persistent team is unnecessary.

### 4. Producer-Reviewer
A producer agent and a reviewer agent operate as a pair.

```
[Producer] → [Reviewer] → (if needed) → [Producer] rerun
```

**Appropriate when:** output quality must be guaranteed and objective verification criteria exist
**Example:** webtoon production — artist generates → reviewer inspects → failed panels regenerate
**Caution:** set a hard retry limit of 2 to 3 attempts to prevent loops.
**Team mode fit:** agent team is useful. `SendMessage` enables real-time feedback between producer and reviewer.

### 5. Supervisor
A central agent manages work state and dynamically assigns work to lower-level agents.

```
         ┌→ [Worker A]
[Supervisor] ─┼→ [Worker B]    ← Supervisor observes state and assigns dynamically
         └→ [Worker C]
```

**Appropriate when:** workload is variable or work assignment must be decided at runtime
**Example:** large-scale code migration — the supervisor analyzes the file list and assigns batches to workers
**Difference from fan-out:** fan-out uses fixed pre-assignment. Supervisor adjusts dynamically based on progress.
**Caution:** avoid turning the supervisor into a bottleneck. Make delegation units large enough.
**Team mode fit:** the shared task list maps naturally to the supervisor pattern. Register tasks with `TaskCreate`, then let members request them.

### 6. Hierarchical Delegation
A higher-level agent recursively delegates to lower-level agents. A complex problem is decomposed step by step.

```
[Lead] → [Team Lead A] → [Executor A1]
                  → [Executor A2]
       → [Team Lead B] → [Executor B1]
```

**Appropriate when:** the problem decomposes naturally into a hierarchy
**Example:** full-stack app development — overall lead → frontend lead → (UI/logic/test) + backend lead → (API/DB/test)
**Caution:** beyond 3 levels, latency and context loss increase significantly. Keep depth to 2 levels or less.
**Team mode fit:** nested teams are not allowed in agent team mode. Implement level 1 as a team and level 2 as sub-agents, or flatten into a single team.

## Composite Patterns

In production use, composite patterns are more common than single patterns:

| Composite pattern | Composition | Example |
|----------|------|------|
| **Fan-out + Producer-Reviewer** | parallel generation, then separate review for each branch | multilingual translation — translate 4 languages in parallel → each reviewed by a native reviewer |
| **Pipeline + Fan-out** | sequential stages with one or more parallel segments | analysis (sequential) → implementation (parallel) → integration test (sequential) |
| **Supervisor + Expert Pool** | supervisor invokes specialists dynamically | customer inquiry handling — supervisor classifies the inquiry and assigns the correct specialist |

### Execution Mode for Composite Patterns

**Use an agent team for all composite patterns by default.** Active communication between members is a primary driver of result quality.

| Scenario | Recommended mode | Reason |
|---------|----------|------|
| **Research + Analysis** | Agent team | members share findings and debate conflicting information in real time |
| **Design + Implementation + Verification** | Agent team | feedback loop between designer, implementer, and verifier |
| **Supervisor + Workers** | Agent team | dynamic assignment through the shared task list and shared progress visibility |
| **Producer + Reviewer** | Agent team | real-time feedback minimizes rework |

> Mix in sub-agents only when a single agent performs a fully isolated one-shot task.

## Agent Type Selection

When invoking an agent, set the type with the Agent tool `subagent_type` parameter. Team members in an agent team may also use custom agent definitions.

### Built-in Types

| Type | Tool access | Appropriate use |
|------|----------|-----------|
| `general-purpose` | full access (including WebSearch and WebFetch) | web research, general tasks |
| `Explore` | read-only (no Edit/Write) | codebase exploration, analysis |
| `Plan` | read-only (no Edit/Write) | architecture design, planning |

### Custom Types

If an agent is defined in `.claude/agents/{name}.md`, invoke it with `subagent_type: "{name}"`. Custom agents have access to the full toolset.

### Selection Criteria

| Situation | Recommended choice | Reason |
|------|------|------|
| Role is complex and reused across sessions | **Custom type** (`.claude/agents/`) | manage persona and work principles in a file |
| Task is simple research or collection and a prompt is enough | **`general-purpose`** + detailed prompt | no agent file logic required beyond prompt instructions |
| Only code reading is required (analysis/review) | **`Explore`** | prevents accidental file modification |
| Only design or planning is required | **`Plan`** | focuses on analysis and prevents code changes |
| Implementation requires file edits | **Custom type** | full tool access plus specialist instructions |

**Rule:** Every agent must be defined in `.claude/agents/{name}.md`. Even when using a built-in type, create the agent definition file and state the role, principles, and protocol. The file is required for reuse in later sessions, and the team communication protocol must be explicit to preserve collaboration quality.

**Model:** Every agent uses `model: "opus"`. Every Agent tool call must explicitly include `model: "opus"`.

## Agent Definition Structure

```markdown
---
name: agent-name
description: "1-2 sentence role description. Include trigger keywords."
---

# Agent Name — one-line role summary

You are a specialist for [role] in [domain].

## Core Role
1. Role 1
2. Role 2

## Work Principles
- Principle 1
- Principle 2

## Input/Output Protocol
- Input: [what is received, and from where]
- Output: [what is written, and where]
- Format: [file format, structure]

## Team Communication Protocol (agent team mode)
- Message intake: [who sends what]
- Message output: [who receives what]
- Task requests: [what type of work is requested through the shared task list]

## Error Handling
- [behavior on failure]
- [behavior on timeout]

## Collaboration
- relationship with other agents
```

## Agent Separation Criteria

| Criterion | Separate | Merge |
|------|------|------|
| Expertise | separate if domains differ | merge if domains overlap |
| Parallelism | separate if work can run independently | consider merge if work is sequentially dependent |
| Context | separate if context load is high | merge if it stays light and fast |
| Reusability | separate if it will be used by other teams | consider merge if it is only for this team |

## Skill vs Agent

| Category | Skill | Agent |
|------|-------------|-----------------|
| Definition | procedural knowledge + tool bundle | specialist persona + behavior principles |
| Location | `.claude/skills/` | `.claude/agents/` |
| Trigger | user request keyword matching | explicit invocation through the Agent tool |
| Size | small to large (workflow) | small (role definition) |
| Purpose | "how" | "who" |

A skill is a **procedural guide** referenced while the agent performs work.
An agent is a **specialist role definition** that uses skills.

## Skill ↔ Agent Integration Methods

Three ways an agent can use a skill:

| Method | Implementation | Appropriate use |
|------|------|-----------|
| **Skill tool call** | specify `call /skill-name with the Skill tool` in the agent prompt | when the skill is an independent workflow and may also be user-invoked |
| **Inline in prompt** | include the skill content directly in the agent definition | when the skill is short (50 lines or less) and dedicated to one agent |
| **Reference load** | load the skill `references/` files with `Read` only when needed | when the skill content is large and only conditionally required |

Recommended rule: use the Skill tool for reusable skills, inline for dedicated skills, and reference loading for large content.
