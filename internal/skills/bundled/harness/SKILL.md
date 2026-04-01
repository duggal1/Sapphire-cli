---
name: harness
description: "Builds a harness. Meta skill for defining specialist agents and creating the skills they use. Use when (1) the user asks to 'build a harness' or 'set up a harness', (2) the user asks for 'harness design' or 'harness engineering', (3) a harness-based automation system must be built for a new domain or project, or (4) an existing harness must be restructured or extended."
---

# Harness — Agent Team & Skill Architect

Meta skill for building a domain- or project-specific harness, defining each agent role, and creating the skills those agents use.

**Core principles:**
1. Create agent definitions (`.claude/agents/`) and skills (`.claude/skills/`).
2. **Use agent teams as the default execution mode.**

## Workflow

### Phase 1: Domain Analysis
1. Identify the domain or project from the user request
2. Identify the core task types (generation, verification, editing, analysis, and so on)
3. Check existing agents and skills to prevent conflicts and duplication
4. Inspect the project codebase to identify the tech stack, data model, and major modules
5. **Detect user proficiency** — infer the technical level from conversation signals such as terminology and question depth, then adjust later communication tone. Do not use terms such as "assertion" or "JSON schema" without explanation for users with limited coding experience.

### Phase 2: Team Architecture Design

#### 2-1. Select Execution Mode: Agent Team vs Sub-agent

**The default is an agent team.** If two or more agents must collaborate, evaluate the agent team first. Team members self-coordinate through direct communication (`SendMessage`) and a shared task list (`TaskCreate`). Shared discovery, conflict discussion, and gap coverage improve result quality.

Use sub-agents only when there is a single agent or when agent-to-agent communication is unnecessary and only final result delivery is required.

> For the comparison table and decision tree, see the "Execution Modes" section in `references/agent-design-patterns.md`.

#### 2-2. Select an Architecture Pattern

1. Decompose the work into specialist areas
2. Decide the agent team structure (see `references/agent-design-patterns.md` for architecture patterns)
   - **Pipeline**: sequential dependent work
   - **Fan-out/Fan-in**: parallel independent work
   - **Expert Pool**: selective invocation by situation
   - **Producer-Reviewer**: generation followed by quality review
   - **Supervisor**: a central agent manages state and dynamic assignment
   - **Hierarchical Delegation**: a higher-level agent delegates recursively to lower-level agents

#### 2-3. Agent Separation Criteria

Decide across four axes: expertise, parallelism, context load, and reusability. For the detailed criteria table, see the "Agent Separation Criteria" section in `references/agent-design-patterns.md`.

### Phase 3: Create Agent Definitions

**Every agent must be defined as `project/.claude/agents/{name}.md`.** It is forbidden to place the role directly into an Agent tool prompt without an agent definition file. Reasons:
- The definition must exist as a file so it can be reused in later sessions
- The team communication protocol must be explicit to preserve collaboration quality
- The core value of the harness is separation between the agent ("who") and the skill ("how")

Create an agent definition file even when using built-in types (`general-purpose`, `Explore`, `Plan`). Set the built-in type through the Agent tool `subagent_type` parameter. Store the role, principles, and protocol in the agent definition file.

**Model setting:** Every agent uses `model: "opus"`. Every Agent tool call must explicitly include `model: "opus"`. Harness quality depends directly on agent reasoning quality, and `opus` is the required model.

**Team reconfiguration:** Only one agent team can be active per session, but a team may be dissolved and replaced between phases. If a pattern such as a pipeline requires different specialist combinations by phase, save the previous team's outputs to files, shut down that team, and create the next team.

Define each agent in `project/.claude/agents/{name}.md`. Required sections: core role, work principles, input/output protocol, error handling, and collaboration. In agent team mode, add a `## Team Communication Protocol` section that states message sources, message targets, and task request scope.

> For the definition template and complete file examples, see "Agent Definition Structure" in `references/agent-design-patterns.md` and `references/team-examples.md`.

**Required rules when including a QA agent:**
- Use the `general-purpose` type for the QA agent (`Explore` is read-only and cannot run verification scripts)
- The core QA task is not "existence checking" but **"cross-boundary comparison"** — read the API response and the frontend hook together, then compare the shapes
- Do not run QA only once at the end. Run it **incrementally immediately after each module is completed**
- For the detailed guide, see `references/qa-agent-guide.md`

### Phase 4: Create Skills

Create the skill each agent will use at `project/.claude/skills/{name}/skill.md`. For detailed writing guidance, see `references/skill-writing-guide.md`.

#### 4-1. Skill Structure

```
skill-name/
├── skill.md (required)
│   ├── YAML frontmatter (name, description required)
│   └── Markdown body
└── Bundled Resources (optional)
    ├── scripts/    - executable code for repetitive or deterministic work
    ├── references/ - reference documents loaded conditionally
    └── assets/     - files used in output (templates, images, and so on)
```

#### 4-2. Description Writing — Drive Aggressive Triggering

The description is the only trigger mechanism for a skill. Claude tends to evaluate triggers conservatively, so write the description **aggressively**.

**Bad example:** `"A skill for processing PDF documents"`
**Good example:** `"Handles all PDF work, including reading PDF files, extracting text and tables, merging, splitting, rotating, watermarking, encryption, and OCR. If the user mentions a .pdf file or requests a PDF output, this skill must be used."`

State both the work the skill performs and the specific trigger situations. Also distinguish nearby cases that must not trigger this skill.

#### 4-3. Body Writing Principles

| Principle | Description |
|------|------|
| **Explain Why** | Do not rely on forceful commands such as "ALWAYS" or "NEVER" without rationale. State why the rule exists. If the LLM understands the reason, it will make correct decisions on edge cases. |
| **Keep It Lean** | The context window is shared infrastructure. Keep the `skill.md` body under 500 lines. Remove low-value content or move it into `references/`. |
| **Generalize** | Do not write narrow rules that fit only one example. Explain principles so the skill handles varied inputs. Do not overfit. |
| **Bundle Repeated Code** | If testing shows that agents repeatedly write the same script, bundle it in `scripts/` in advance. |
| **Write Imperatively** | Use directive language. |

#### 4-4. Progressive Disclosure

The skill manages context with a 3-stage loading system:

| Stage | Load timing | Target size |
|------|----------|----------|
| **Metadata** (name + description) | always present in context | ~100 words |
| **skill.md body** | when the skill is triggered | <500 lines |
| **references/** | only when needed | unlimited (scripts can run without loading) |

**Size control rules:**
- If `skill.md` approaches 500 lines, move details into `references/` and leave a pointer in the body that states when to read that file
- If a reference file exceeds 300 lines, include a **table of contents (ToC)** at the top
- If domain- or framework-specific variants exist, split them under `references/` by domain so only relevant files are loaded

```
cloud-deploy/
├── skill.md (workflow + selection guide)
└── references/
    ├── aws.md    ← load only when AWS is selected
    ├── gcp.md
    └── azure.md
```

#### 4-5. Skill-Agent Mapping Principle

- 1 agent ↔ 1~N skills (one-to-one or one-to-many)
- A skill may also be shared by multiple agents
- A skill defines "how"; an agent defines "who"

> For detailed writing patterns, examples, and data schema standards, see `references/skill-writing-guide.md`.

### Phase 5: Integration and Orchestration

The orchestrator is a specialized skill that binds individual agents and skills into a single workflow and coordinates the full team. If the individual skills created in Phase 4 define "what each agent does and how," the orchestrator defines "who collaborates, when, and in what order." For the concrete template, see `references/orchestrator-template.md`.

The orchestrator pattern changes by execution mode:

#### 5-0. Orchestrator Pattern by Mode

**Agent Team Mode (default):**
The orchestrator builds a team with `TeamCreate` and assigns work with `TaskCreate`. Team members communicate directly through `SendMessage` and self-coordinate. The leader monitors progress and synthesizes results.

```
[Orchestrator/Leader]
    ├── TeamCreate(team_name, members)
    ├── TaskCreate(tasks with dependencies)
    ├── Team members self-coordinate (SendMessage)
    ├── Collect and synthesize results
    └── Tear down team
```

**Sub-agent Mode:**
The orchestrator directly invokes sub-agents with the `Agent` tool. Each sub-agent returns results only to the main agent.

```
[Orchestrator]
    ├── Agent(agent-1, run_in_background=true)
    ├── Agent(agent-2, run_in_background=true)
    ├── Wait for and collect results
    └── Produce integrated output
```

#### 5-1. Data Transfer Protocol

State the data transfer method between agents inside the orchestrator:

| Strategy | Method | Execution mode | Appropriate use |
|------|------|----------|-----------|
| **Message-based** | direct agent-to-agent communication through `SendMessage` | agent team | real-time coordination, feedback exchange, lightweight status transfer |
| **Task-based** | shared task state through `TaskCreate`/`TaskUpdate` | agent team | progress tracking, dependency management, task requests |
| **File-based** | write and read files in agreed paths | both | large data, structured artifacts, audit trace requirements |

**Recommended combination for agent team mode:** task-based (coordination) + file-based (artifacts) + message-based (real-time communication)

Rules for file-based transfer:
- Create a `_workspace/` folder under the working directory and store intermediate artifacts there
- Use the filename convention `{phase}_{agent}_{artifact}.{ext}` (example: `01_analyst_requirements.md`)
- Write only final artifacts to the user-specified path. Preserve intermediate files in `_workspace/` for post-run validation and audit traceability

#### 5-2. Error Handling

Include the error handling policy inside the orchestrator. Core rule: retry once, and if the retry also fails, continue without that result and state the omission in the report. If data conflicts, do not delete either source; preserve both with attribution.

> For the error-type strategy table and implementation details, see the "Error Handling" section in `references/orchestrator-template.md`.

#### 5-3. Team-Mode-Only: Team Size Guidelines

| Work scale | Recommended team size | Tasks per member |
|----------|------------|--------------|
| Small (5~10 tasks) | 2~3 members | 3~5 tasks |
| Medium (10~20 tasks) | 3~5 members | 4~6 tasks |
| Large (20+ tasks) | 5~7 members | 4~5 tasks |

> Larger teams increase coordination overhead. Three focused members are better than five unfocused members.

### Phase 6: Verification and Testing

Verify the generated harness. For the detailed testing methodology, see `references/skill-testing-guide.md`.

#### 6-1. Structure Verification

- Confirm that all agent files are in the correct location
- Validate skill frontmatter (`name`, `description`)
- Confirm consistency of inter-agent references
- Confirm that no commands were created

#### 6-2. Execution-Mode-Specific Verification

- Agent team mode: verify inter-member communication paths, task dependencies, and team size appropriateness
- Sub-agent mode: verify each agent input/output linkage and `run_in_background` settings

#### 6-3. Skill Execution Testing

Run live execution tests for each generated skill:

1. **Write test prompts** — For each skill, write 2 to 3 realistic test prompts. Use concrete, natural sentences that a real user would plausibly submit.

2. **Run with-skill vs without-skill comparisons** — When possible, run the skill-enabled execution and the baseline execution in parallel to verify the marginal value of the skill. Spawn two sub-agents:
   - **With-skill**: read the skill and perform the task
   - **Without-skill (baseline)**: perform the same prompt without the skill

3. **Evaluate results** — Evaluate output quality with both qualitative review (user review) and quantitative review (assertion-based). If the output is objectively verifiable, such as file creation or data extraction, define assertions. If the output is subjective, such as writing style or design, rely on user feedback.

4. **Iterative improvement loop** — If testing finds issues:
   - **Generalize** the feedback before modifying the skill. Do not make narrow fixes that only match one example
   - Re-test after each change
   - Repeat until the user is satisfied or no meaningful improvement remains

5. **Bundle repeated patterns** — If testing shows that agents repeatedly write the same code, such as the same helper script across tests, bundle that code into `scripts/` in advance.

#### 6-4. Trigger Verification

Verify that each skill description triggers correctly:

1. **Should-trigger queries** (8~10) — varied phrasings that should trigger the skill (formal/casual, explicit/implicit)
2. **Should-NOT-trigger queries** (8~10) — near-miss queries where keywords are similar but another tool or skill is appropriate

**Near-miss rule:** A clearly unrelated query such as "write a Fibonacci function" has no test value. A useful test case is a boundary query such as "extract the charts in this Excel file as PNG" (xlsx skill vs image conversion), where the boundary is ambiguous.

Also verify trigger conflicts with existing skills in this phase.

#### 6-5. Dry-Run Testing

- Review whether the Phase sequence in the orchestrator skill is logically correct
- Confirm that no dead link exists in the data transfer path
- Confirm that every agent input matches the previous Phase output
- Confirm that fallback paths are executable for each error scenario

#### 6-6. Write Test Scenarios

- Add a `## Test Scenarios` section to the orchestrator skill
- Describe at least one normal flow and at least one error flow

## Output Checklist

Confirm after generation:

- [ ] `project/.claude/agents/` — **agent definition files created as required** (required even for built-in types)
- [ ] `project/.claude/skills/` — skill files (`skill.md` + `references/`)
- [ ] 1 orchestrator skill (includes data flow, error handling, and test scenarios)
- [ ] execution mode declared (agent team or sub-agent)
- [ ] every Agent call explicitly includes `model: "opus"`
- [ ] `.claude/commands/` — nothing created
- [ ] no conflict with existing agents or skills
- [ ] skill descriptions written aggressively enough to trigger
- [ ] `skill.md` body under 500 lines, or split into `references/` if longer
- [ ] execution validation completed with 2 to 3 test prompts
- [ ] trigger validation completed (`should-trigger` + `should-NOT-trigger`)

## References

- Harness patterns: `references/agent-design-patterns.md`
- Existing harness examples (includes complete real files): `references/team-examples.md`
- Orchestrator template: `references/orchestrator-template.md`
- **Skill writing guide**: `references/skill-writing-guide.md` — writing patterns, examples, data schema standards
- **Skill testing guide**: `references/skill-testing-guide.md` — testing, evaluation, and iterative improvement methodology
- **QA agent guide**: `references/qa-agent-guide.md` — use when including a QA agent in a build harness. Includes integration coherence verification methodology, boundary bug patterns, and a QA agent definition template. Based on 7 real bugs found in a production project.
