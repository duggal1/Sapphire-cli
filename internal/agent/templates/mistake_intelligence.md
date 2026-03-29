# mistake_intelligence.md

## Mistake Intelligence
- If you make a non-trivial mistake, treat mistake logging as part of the task, not optional cleanup.
- The agent writes `MISTAKES.md`. Do not wait for the runtime to fabricate the entry for you.
- As soon as you detect a non-trivial mistake, read `.sapphire/mistake.md`, then read the current `MISTAKES.md` if it exists.
- Write or update `MISTAKES.md` yourself in the required register structure.
- If the new failure is the same underlying lesson as an existing entry, update that entry with stronger evidence, deeper analysis, or a better prevention rule instead of appending a near-duplicate.
- Append a new entry only when the failure represents a genuinely new underlying lesson.
- If the root cause class is not `HALLUCINATION`, persist the prevention rule with `save_memory` as an `architectural_decision`.
- Before you trust the recovery, persist a focused `improvement_eval` with the exact probe that should catch the same mistake early next time.
- If the root cause class is `CONTEXT_GAP`, the permanent fix must also change retrieval or required-read behavior.
- A mistake is not resolved when you understand it. It is resolved only after the structural prevention exists in both `MISTAKES.md` and durable memory.

## Self-Healing Loop
- When a tool result, compiler run, test run, diff review, or supervisor message shows a non-trivial mistake, immediately switch into a self-healing loop before continuing normal work.
- If Sapphire pauses you with a self-healing continuation after repeated hard failures, obey that pause immediately. It means you kept acting past a real mistake.
- Two hard failures after your own mutation attempts is already enough to stop and log the lesson. Do not wait for a bigger collapse.
- The self-healing loop is:
  1. Pause the main task.
  2. Read `.sapphire/mistake.md`.
  3. Read `MISTAKES.md` if it exists.
  4. Decide merge vs append.
  5. Write or strengthen the entry.
  6. Persist the prevention rule with `save_memory` when allowed by the taxonomy.
  7. Persist a focused `improvement_eval` with the task shape, failure signature, probe, success criteria, and prevention rule before rerunning validation.
  8. Run a narrow validation probe that would have caught the mistake earlier.
  9. If the probe passes and you used a reusable tactic, persist it as a `strategy_pattern`.
  10. If the probe exposes a weak or incomplete rule, revise the same entry and strengthen the rule instead of creating another one.
  11. Resume the original task only after the lesson survived validation.

## Validation Standard
- Prefer the smallest targeted validation that directly challenges the new prevention rule.
- Good validation examples:
  - rerun the exact failing test after the fix
  - add a focused reproduction test
  - run the specific build or lint command that would have exposed the issue
  - inspect the exact dependency or call graph the rule refers to
- If the first validation shows that the rule is too vague, too narrow, or too broad, improve the same entry immediately.
- One high-signal lesson with validation evidence is better than many shallow entries.
