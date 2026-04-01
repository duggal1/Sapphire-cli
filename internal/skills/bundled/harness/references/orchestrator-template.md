# Orchestrator Skill Template

The orchestrator is the top-level skill that coordinates the full workflow. Two templates are provided by execution mode.

---

## Template A: Agent Team Mode (Default for Complex Work)

The team is implemented with multiple `spawn_agent` workers plus explicit coordination.

```markdown
---
name: {domain}-orchestrator
description: "Coordinates the {domain} Sapphire workflow. {trigger keywords}."
---

# {Domain} Orchestrator

Integrated skill that coordinates the {domain} workflow and produces {final artifact}.

## Execution Mode: Agent Team

## Agent Composition

| Member | Profile | Role | Skill | Output |
|------|-------------|------|------|------|
| {worker-1} | {coder|task} | {role} | {skill} | {output-file} |
| {worker-2} | {coder|task} | {role} | {skill} | {output-file} |
| ... | | | | |

## Workflow

### Phase 1: Preparation
1. Analyze user input.
2. Create `_workspace/` in the working directory.
3. Save shared inputs under `_workspace/00_input/`.
4. If the task is complex, call `run_harness` and keep the contract aligned with the orchestrator.

### Phase 2: Spawn Workers

1. Spawn each worker with `spawn_agent`.
2. For each worker, specify:
   - `agent`: `coder` or `task`
   - `message`: role, owned scope, exact outputs, required skills, blocker protocol
   - `write_manifest`: owned write scope
   - `definition_of_done`: explicit completion criteria
   - `fork_context`: only if the worker truly needs recent parent context

Example shape:

```text
spawn_agent(
  message: "Role: {role}. Owned scope: {scope}. Load skills: {skill-list}. Write {artifact-path}. Escalate blockers by mail. Do not edit outside scope.",
  agent: "{coder|task}",
  write_manifest: ["{owned-path-1}", "{owned-path-2}"],
  definition_of_done: "{done-criteria}",
  fork_context: true
)
```

3. Record returned agent ids.
4. Use `agent_directory` to inspect route aliases and current topology if needed.

### Phase 3: Team Execution

**Execution mode:** workers operate independently, coordinate through durable mail, and write artifacts to `_workspace/`.

**Communication rules between workers:**
- worker-to-worker coordination uses `agent_mail_send`
- workers read mail with `agent_mail_inbox`
- workers acknowledge handled messages with `agent_mail_ack`
- the leader uses `send_input` only for intervention, correction, or rerouting

**Artifact storage:**

| Member | Output path |
|------|----------|
| {worker-1} | `_workspace/{phase}_{worker-1}_{artifact}.md` |
| {worker-2} | `_workspace/{phase}_{worker-2}_{artifact}.md` |

**Leader monitoring:**
- inspect active workers with `agent_directory`
- use `wait` for terminal status
- if a worker stalls or drifts, use `send_input` or replace it with a new `spawn_agent`

### Phase 4: Integration
1. `wait` for workers to finish.
2. `collect_result` for the finished workers.
3. Read artifacts from `_workspace/`.
4. Run integration or verification logic.
5. Create final artifact: `{output-path}/{filename}`.

### Phase 5: Cleanup
1. `close_agent` each spawned worker.
2. Preserve `_workspace/`.
3. Report the result summary.

> If team reconfiguration is required, preserve artifacts, close the current workers, then spawn the next phase workers.

## Data Flow

```text
[Leader] → spawn_agent → [worker-1] ←agent_mail→ [worker-2]
                         │                        │
                         ↓                        ↓
                  artifact-1.md             artifact-2.md
                         │                        │
                         └──────── Read ──────────┘
                                    ↓
                             [Leader: Integration]
                                    ↓
                              Final artifact
```

## Error Handling

| Situation | Strategy |
|------|------|
| 1 worker fails | retry once by `send_input` or respawn |
| Majority of workers fail | notify the user and decide whether to continue |
| Timeout | use collected partial results when safe |
| Data conflict | preserve both sources with attribution |
| Drift or scope violation | send corrective input or replace the worker |

## Test Scenarios

### Normal Flow
1. The user provides {input}
2. Preparation derives {analysis result}
3. Workers run in parallel and produce artifacts
4. Integration combines the artifacts into the final result
5. Cleanup closes all workers
6. Expected result: `{output-path}/{filename}` is created

### Error Flow
1. {worker-2} fails during execution
2. The leader retries once
3. If retry fails, the leader continues with remaining results where safe
4. The final report states the missing scope explicitly
```

---

## Template B: Isolated Worker Mode

Use this when peer communication is unnecessary.

```markdown
---
name: {domain}-orchestrator
description: "Coordinates isolated {domain} worker execution. {trigger keywords}."
---

# {Domain} Orchestrator

Integrated skill that coordinates isolated worker execution and produces {final artifact}.

## Execution Mode: Isolated Worker

## Agent Composition

| Worker | Primitive | Profile | Role | Skill | Output |
|---------|--------------|------|------|------|------|
| {worker-1} | {agent|spawn_agent} | {coder|task} | {role} | {skill} | {output-file} |
| {worker-2} | {agent|spawn_agent} | {coder|task} | {role} | {skill} | {output-file} |

## Workflow

### Phase 1: Preparation
1. Analyze user input.
2. Create `_workspace/`.
3. Save shared inputs under `_workspace/00_input/`.

### Phase 2: Primary Work

**Execution method:** {parallel | sequential}

If the work is fully one-shot, use `agent`.

If follow-up or explicit lifecycle control is required:
1. `spawn_agent`
2. `wait`
3. `collect_result`
4. `close_agent`

### Phase 3: Integration
1. Read `_workspace/` artifacts.
2. Run integration or verification logic.
3. Create final artifact: `{output-path}/{filename}`.

### Phase 4: Cleanup
1. Preserve `_workspace/`.
2. Report the result summary.

## Data Flow

```text
Input → [worker-1] → artifact-1 ─┐
                                 ├→ [Integration] → Final artifact
Input → [worker-2] → artifact-2 ─┘
```

## Error Handling

| Situation | Strategy |
|------|------|
| 1 worker fails | retry once, then continue without that result if safe |
| Majority fail | notify the user |
| Timeout | use collected partial results where safe |
| Data conflict | preserve both sources with attribution |

## Test Scenarios

### Normal Flow
1. The user provides {input}
2. Workers produce their artifacts
3. Integration creates the final result

### Error Flow
1. {worker-2} fails
2. Retry once
3. Continue with remaining results if safe
4. State the omission in the final report
```

---

## Writing Rules

1. **Declare the execution mode first.**
2. **Use only real Sapphire primitives.**
3. **Specify worker ownership and artifact paths explicitly.**
4. **Do not invent custom agent types or team-management APIs.**
5. **State dependencies between phases.**
6. **Use realistic error handling.**
7. **Test scenarios are mandatory.**

## Reference for Real Orchestrators

Base structure for a fan-out/fan-in orchestrator:
prepare → `spawn_agent` x N → worker mail/artifacts → `wait` + `collect_result` → integration → `close_agent`.

See `references/team-examples.md`.
