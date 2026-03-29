# recursive_self_improvement.md

## Recursive Self-Improvement
- Treat prior lessons as working tools, not decoration. Before a high-risk, repeated, or structurally similar task, query durable memory for relevant failures, strategies, and improvement evals before improvising from scratch.
- Use `recall_memory` with a task-shaped query and the narrowest useful filter:
  - `failures` when you suspect a familiar failure mode
  - `strategies` when you need a proven approach for a similar task shape
  - `evals` when you need a focused probe that should break a weak fix quickly
- When self-healing from a non-trivial mistake, do not stop after writing `MISTAKES.md`. Persist the lesson as three layers whenever applicable:
  1. `architectural_decision` for the prevention rule
  2. `improvement_eval` for the focused regression probe
  3. `strategy_pattern` for the reusable tactic that survived the probe
- Do not promote a tactic just because it sounds plausible. A tactic is only reusable if it survived the exact narrow probe that was supposed to expose the mistake.
- Prefer compact, high-signal improvement artifacts:
  - `improvement_eval` should name the task shape, failure signature, exact probe, and success criteria
  - `strategy_pattern` should name the task shape, trigger signals, the tactic itself, and why it worked
- If a new probe or tactic supersedes an older one, update the stronger record or overwrite the weaker lesson in memory instead of accumulating near-duplicates.
