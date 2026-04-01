# Orchestrator Skill Template

The orchestrator is the top-level skill that coordinates the full team. Two templates are provided by execution mode.

---

## Template A: Agent Team Mode (Default)

The agent team is created with `TeamCreate` and coordinated through the shared task list and `SendMessage`.

```markdown
---
name: {domain}-orchestrator
description: "Orchestrator that coordinates the {domain} agent team. {trigger keywords}."
---

# {Domain} Orchestrator

Integrated skill that coordinates the {domain} agent team and produces {final artifact}.

## Execution Mode: Agent Team

## Agent Composition

| Member | Agent Type | Role | Skill | Output |
|------|-------------|------|------|------|
| {teammate-1} | {custom or built-in} | {role} | {skill} | {output-file} |
| {teammate-2} | {custom or built-in} | {role} | {skill} | {output-file} |
| ... | | | | |

## Workflow

### Phase 1: Preparation
1. Analyze user input — {what must be identified}
2. Create `_workspace/` in the working directory
3. Save input data to `_workspace/00_input/`

### Phase 2: Team Setup

1. Create team:
   ```
   TeamCreate(
     team_name: "{domain}-team",
     members: [
       { name: "{teammate-1}", agent_type: "{type}", model: "opus", prompt: "{role description and work instruction}" },
       { name: "{teammate-2}", agent_type: "{type}", model: "opus", prompt: "{role description and work instruction}" },
       ...
     ]
   )
   ```

2. Register tasks:
   ```
   TaskCreate(tasks: [
     { title: "{task1}", description: "{detail}", assignee: "{teammate-1}" },
     { title: "{task2}", description: "{detail}", assignee: "{teammate-2}" },
     { title: "{task3}", description: "{detail}", depends_on: ["{task1}"] },
     ...
   ])
   ```

   > 5 to 6 tasks per member is the correct range. Declare dependency with `depends_on` when required.

### Phase 3: {Primary Work — example: research/generation/analysis}

**Execution mode:** team members self-coordinate

Team members claim work from the shared task list and execute independently.
The leader monitors progress and intervenes only when necessary.

**Communication rules between members:**
- {teammate-1} sends {what information} to {teammate-2} through `SendMessage`
- {teammate-2} saves results to a file on completion and notifies the leader
- If a member needs another member's result, request it through `SendMessage`

**Artifact storage:**

| Member | Output path |
|------|----------|
| {teammate-1} | `_workspace/{phase}_{teammate-1}_{artifact}.md` |
| {teammate-2} | `_workspace/{phase}_{teammate-2}_{artifact}.md` |

**Leader monitoring:**
- Receive automatic notification when a member becomes idle
- If a member is blocked, issue instructions through `SendMessage` or reassign the task
- Check total progress with `TaskGet`

### Phase 4: {Follow-up Work — example: verification/integration}
1. Wait until all member tasks are complete (check status with `TaskGet`)
2. Collect each member artifact with `Read`
3. {integration/verification logic}
4. Create final artifact: `{output-path}/{filename}`

### Phase 5: Cleanup
1. Request termination from team members (`SendMessage`)
2. Delete team (`TeamDelete`)
3. Preserve the `_workspace/` directory (do not delete intermediate artifacts — required for post-run verification and audit traceability)
4. Report the result summary to the user

> **If team reconfiguration is required:** if different phases require different specialist combinations, delete the current team with `TeamDelete`, then create the next phase team with a new `TeamCreate`. Previous artifacts remain in `_workspace/`, so the new team can access them with `Read`.

## Data Flow

```
[Leader] → TeamCreate → [teammate-1] ←SendMessage→ [teammate-2]
                          │                           │
                          ↓                           ↓
                    artifact-1.md              artifact-2.md
                          │                           │
                          └───────── Read ────────────┘
                                     ↓
                              [Leader: Integration]
                                     ↓
                               Final artifact
```

## Error Handling

| Situation | Strategy |
|------|------|
| 1 member fails/stops | leader detects → confirm state with `SendMessage` → restart or create a replacement member |
| Majority of members fail | notify the user and confirm whether to continue |
| Timeout | use partial results collected so far and terminate incomplete members |
| Data conflict between members | preserve both sources with attribution; do not delete either |
| Task status delay | leader checks with `TaskGet`, then updates manually with `TaskUpdate` |

## Test Scenarios

### Normal Flow
1. The user provides {input}
2. Phase 1 derives {analysis result}
3. Phase 2 creates the team ({N} members + {M} tasks)
4. Phase 3 members self-coordinate and perform the work
5. Phase 4 integrates artifacts and creates the final result
6. Phase 5 cleans up the team
7. Expected result: `{output-path}/{filename}` is created

### Error Flow
1. In Phase 3, {teammate-2} stops due to an error
2. The leader receives the idle notification
3. The leader confirms state through `SendMessage` and attempts a restart
4. If restart fails, reassign {teammate-2}'s work to {teammate-1}
5. Continue to Phase 4 with the remaining results
6. State "{teammate-2} scope partially not collected" in the final report
```

---

## Template B: Sub-agent Mode (Lightweight)

Sub-agents are called directly with the `Agent` tool and return results only to the main agent.

```markdown
---
name: {domain}-orchestrator
description: "Orchestrator that coordinates {domain} agents. {trigger keywords}."
---

# {Domain} Orchestrator

Integrated skill that coordinates the {domain} agents and produces {final artifact}.

## Execution Mode: Sub-agent

## Agent Composition

| Agent | subagent_type | Role | Skill | Output |
|---------|--------------|------|------|------|
| {agent-1} | {custom or built-in type} | {role} | {skill} | {output-file} |
| {agent-2} | {custom or built-in type} | {role} | {skill} | {output-file} |
| ... | | | | |

## Workflow

### Phase 1: Preparation
1. Analyze user input — {what must be identified}
2. Create `_workspace/` in the working directory
3. Save input data to `_workspace/00_input/`

### Phase 2: {Primary Work — example: research/generation/analysis}

**Execution method:** {parallel | sequential | conditional}

{If parallel}
Call N Agent tools in the same message:

| Agent | Input | Output | model | run_in_background |
|---------|------|------|-------|-------------------|
| {agent-1} | {input source} | `_workspace/{phase}_{agent}_{artifact}.md` | opus | true |
| {agent-2} | {input source} | `_workspace/{phase}_{agent}_{artifact}.md` | opus | true |

{If sequential}
Pass the previous agent output into the next agent:

1. Run {agent-1} → create `_workspace/01_{artifact}.md`
2. Run {agent-2} (input: output of 01) → create `_workspace/02_{artifact}.md`

### Phase 3: {Follow-up Work — example: verification/integration}
1. Collect Phase 2 artifacts with `Read`
2. {integration/verification logic}
3. Create final artifact: `{output-path}/{filename}`

### Phase 4: Cleanup
1. Preserve the `_workspace/` directory (do not delete intermediate artifacts — required for post-run verification and audit traceability)
2. Report the result summary to the user

## Data Flow

```
Input → [agent-1] → artifact-1 ─┐
                                ├→ [Integration] → Final artifact
Input → [agent-2] → artifact-2 ─┘
```

## Error Handling

| Situation | Strategy |
|------|------|
| 1 agent fails | retry once. If it fails again, continue without that result and state the omission in the report |
| Majority of agents fail | notify the user and confirm whether to continue |
| Timeout | use partial results collected so far |
| Data conflict between agents | preserve both sources with attribution; do not delete either |

## Test Scenarios

### Normal Flow
1. The user provides {input}
2. Phase 1 derives {analysis result}
3. In Phase 2, {N} agents run in parallel and each produces an artifact
4. In Phase 3, artifacts are integrated into the final report
5. Expected result: `{output-path}/{filename}` is created

### Error Flow
1. In Phase 2, {agent-2} fails
2. Retry once, and it fails again
3. Continue to Phase 3 with the remaining results only
4. State "{agent-2} scope data not collected" in the final report
5. Notify the user that the run completed partially
```

---

## Writing Rules

1. **Declare the execution mode first** — state "Agent Team" or "Sub-agent" at the top of the orchestrator
2. **In agent team mode, specify TeamCreate/SendMessage/TaskCreate usage concretely** — team setup, task registration, communication rules
3. **In sub-agent mode, specify every Agent tool parameter** — name, `subagent_type`, prompt, `run_in_background`
4. **Use absolute file paths in meaning** — do not leave path handling ambiguous; `_workspace/` paths must be explicit
5. **State inter-phase dependencies** — identify which phase depends on which output
6. **Use realistic error handling** — do not assume universal success
7. **Test scenarios are mandatory** — at least 1 normal flow and 1 error flow

## Reference for Real Orchestrators

Base structure for a fan-out/fan-in orchestrator:
prepare → `TeamCreate` + `TaskCreate` → N members run in parallel → `Read` + integration → cleanup.
See the research team example in `references/team-examples.md`.
