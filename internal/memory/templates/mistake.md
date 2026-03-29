# mistake.md

## Purpose
This file is the local mistake-logging protocol for this repository.
Use it when a non-trivial mistake happens and you need to update `MISTAKES.md`.

## Non-Trivial Threshold
Log the mistake if it did any of the following:
- Broke the build.
- Caused a regression.
- Forced backtracking by more than one step.
- Reflected a wrong architectural decision.
- Reflected a wrong assumption about the codebase, environment, or workflow.

Ignore trivial mistakes such as typos, syntax slips, or one-step corrections.

## Root Cause Taxonomy
- `HALLUCINATION`
- `CONTEXT_GAP`
- `COMPLEXITY_OVERLOAD`
- `WRONG_ASSUMPTION`
- `ORCHESTRATION_FAILURE`
- `TOOL_MISUSE`

## Required Logging Protocol
1. Open `MISTAKES.md` at the repository root.
2. Append a new `MISTAKE-XXX` section.
3. If a supervisor message gave you a fingerprint, include it exactly as:
   `<!-- mistake_fingerprint: activity:<id> -->`
4. Describe what happened, classify the root cause, and write a permanent prevention rule.
5. If the class is not `HALLUCINATION`, call `save_memory` with `event_type=architectural_decision` and persist the prevention rule.
6. Mark the entry `RESOLVED` only after the prevention rule is persisted structurally.

## Prevention Rule Standard
- The rule must be imperative.
- The rule must be specific enough to prevent recurrence.
- The rule must describe the structural check or behavior change, not just the lesson.

## Supervisor Contract
If the supervisor blocks execution for a missing mistake log:
- Stop normal work.
- Log the mistake first.
- Persist the prevention rule if the class is not `HALLUCINATION`.
- Only then continue the original task.
