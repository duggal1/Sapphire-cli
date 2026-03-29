# mistake.md

## Role
This file is the repository-local protocol for autonomous mistake logging.
Its only job is to make the agent learn from non-trivial mistakes structurally.
The agent owns `MISTAKES.md`. The runtime does not own the write.

## Non-Trivial Threshold
Log the mistake if it did any of the following:
- Broke the build.
- Caused a regression.
- Forced backtracking by more than one step.
- Made a wrong architectural decision.
- Made a wrong assumption about the codebase, environment, state, or workflow.

Ignore trivial mistakes such as typos, syntax slips, or one-step corrections.

## Canonical Register Rule
`MISTAKES.md` must use the canonical failure register structure.
Do not write freeform notes. Do not invent alternate headings.
If `MISTAKES.md` is missing, create it in canonical form.
If it exists but is malformed, rewrite the new entry in canonical form anyway.
If the current failure belongs to an existing underlying lesson, update that entry instead of appending a near-duplicate.

## Autonomous Self-Healing Loop
When you detect a non-trivial mistake:
1. Pause the main task immediately.
2. Read this file fully.
3. Read `MISTAKES.md` if it exists.
4. Decide whether the failure is a new lesson or a stronger instance of an existing lesson.
5. Write or strengthen the entry.
6. Persist the prevention rule with `save_memory` if the class is not `HALLUCINATION`.
7. Run a narrow validation probe that would have caught the mistake.
8. If the validation shows the rule is weak, revise the same entry and strengthen the rule.
9. Only then resume the original task.

If Sapphire interrupts you after repeated hard failures:
- Treat that interruption as mandatory self-healing mode.
- Do not continue implementation first.
- Log or strengthen the lesson before any more normal work.
- Two hard failures after your own mutation attempts is already enough to require logging.

## Required Section Order
Every new entry must use this exact order:
1. `## MISTAKE-XXX`
2. Optional fingerprint comment:
   `<!-- mistake_fingerprint: activity:<id> -->`
3. `**Date:**`
4. `**Task:**`
5. `**Agent:** ... | **Model:** ...`
6. `**Worktree:**`
7. `### Task Domain`
8. `### What Happened`
9. `### Root Cause Class`
10. `### Root Cause — Deep Analysis`
11. `### Why This Class, Not Another`
12. `### Severity`
13. `### Is It Ignorable?`
14. `### Solution — Permanent Fix`
15. `### Prevention Rule`
16. `### Validation Loop`
17. `### Status`

The file must also keep:
- `## INDEX`
- `## APPENDIX: ROOT CAUSE TAXONOMY`
- `## APPENDIX: RESOLUTION PROTOCOL`

## Canonical Skeleton
Use this exact top-level shape for `MISTAKES.md`:

```md
# MISTAKES.md — Failure Intelligence Register
# Scope: <repo-name> | Reset: per repository | Version: auto-incremented

---

## INDEX

| # | Date | Task Domain | Root Cause Class | Severity | Resolved |
|---|------|-------------|-----------------|----------|----------|

---

## MISTAKE-XXX
<!-- mistake_fingerprint: activity:<id> -->
**Date:** <ISO-8601 UTC>
**Task:** <task>
**Agent:** <agent> | **Model:** <model>
**Worktree:** <worktree>

### Task Domain
<domain>

### What Happened
<facts>

### Root Cause Class
`<class>`

### Root Cause — Deep Analysis
<analysis>

### Why This Class, Not Another
<disambiguation>

### Severity
`<severity>`

### Is It Ignorable?
<YES or NO with reason>

### Solution — Permanent Fix
1. <fix>

### Prevention Rule
> RULE-XXX: <imperative structural prevention rule>

### Validation Loop
1. <targeted probe>

### Status
`RESOLVED` | <what was persisted>

---

## APPENDIX: ROOT CAUSE TAXONOMY
...

## APPENDIX: RESOLUTION PROTOCOL
...
```

## Root Cause Taxonomy
- `HALLUCINATION`
- `CONTEXT_GAP`
- `COMPLEXITY_OVERLOAD`
- `WRONG_ASSUMPTION`
- `ORCHESTRATION_FAILURE`
- `TOOL_MISUSE`

## Required Logging Protocol
1. Open `MISTAKES.md` at the repository root.
2. Decide whether this failure is a new underlying lesson or more evidence for an existing lesson.
3. If it is the same lesson, update the existing entry with stronger evidence and a better prevention rule.
4. If it is a new lesson, append the next canonical `MISTAKE-XXX` entry.
5. If a supervisor message gave you a fingerprint, include it exactly as:
   `<!-- mistake_fingerprint: activity:<id> -->`
6. Classify the mistake using the taxonomy above.
7. Write a permanent prevention rule in imperative form.
8. If the class is not `HALLUCINATION`, call `save_memory` with `event_type=architectural_decision` and persist the prevention rule.
9. If the class is `CONTEXT_GAP`, the permanent fix must change retrieval, RequiredReads, or boot-packet scope.
10. Run a narrow validation probe that would have caught the mistake earlier.
11. Record that probe in `### Validation Loop`.
12. Mark the entry `RESOLVED` only after the prevention rule is persisted structurally and the validation loop is complete.

## Prevention Rule Standard
- The rule must be imperative.
- The rule must be specific enough to prevent recurrence.
- The rule must describe the structural check or behavior change, not just the lesson.
- Prefer one high-signal lesson per root cause cluster over many near-duplicate entries.

## Validation Loop Standard
- Use the narrowest meaningful validation first.
- Prefer one to three targeted probes over broad noisy reruns.
- Good probes include the exact failed test, a focused regression test, the specific build command, or a targeted reference/read check.
- If validation fails, strengthen the same entry. Do not append a duplicate.

## Supervisor Contract
If the supervisor blocks execution for a missing mistake log:
- Stop normal work.
- Log the mistake first.
- Persist the prevention rule if the class is not `HALLUCINATION`.
- Only then continue the original task.
