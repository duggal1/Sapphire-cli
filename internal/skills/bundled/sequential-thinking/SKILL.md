# SKILL: Sequential Thinking — Planning Protocol

This skill defines the planning procedure to apply before executing any complex task. Built-in model reasoning operates during execution. This protocol operates before execution. Its purpose is to structure decomposition, surface dependencies, identify risks, and determine parallel workstreams before any action is taken.

**Activate when:** The task involves 3 or more distinct workstreams, multiple files, deployment steps, architectural decisions, or an unclear execution order.
**Skip when:** The task is a single, unambiguous step with an immediately clear solution.

**Last verified:** March 2026

---

## PROTOCOL — 6 STEPS, EXECUTED IN ORDER

Do not begin execution until all 6 steps are complete.

---

### STEP 1 — UNDERSTAND THE GOAL

Read the full request. Resolve the following before proceeding:

- What is the intended outcome? (Distinguish between what was stated and what was meant.)
- What is explicitly defined versus implied?
- Is there any ambiguity that would produce the wrong outcome if assumed incorrectly?

**If critical ambiguity is present:** Pause. Ask one clarifying question — the most consequential one only. Do not proceed until it is resolved.

**If ambiguity is minor:** State the assumption explicitly in the plan, then continue.

---

### STEP 2 — DECOMPOSE THE TASK

Break the task into discrete, non-overlapping units of work. Each unit must:

- Produce a single, clearly defined output
- Be completable without depending on the in-progress output of another unit
- Be small enough that its result can be verified independently

Document them explicitly:

```
1. [unit name] — [output it produces]
2. [unit name] — [output it produces]
3. [unit name] — [output it produces]
```

---

### STEP 3 — MAP DEPENDENCIES AND SEQUENCE

For each unit, determine:

- Which units must complete before this one can start?
- Which units have no dependencies and can run concurrently?
- What is the critical path — the sequence whose delay delays the entire task?

Label each unit:

- `BLOCKING` — must complete before dependent work begins
- `PARALLEL` — no dependency on other in-progress units; can run concurrently
- `TERMINAL` — final step; depends on all prior work

---

### STEP 4 — IDENTIFY RISKS

Before any file, tool, or terminal is touched:

- What is the most likely point of failure in this plan?
- What is the highest-consequence failure? (data loss, broken deployment, corrupted state)
- Which actions are irreversible?

**Irreversible, high-consequence actions require explicit acknowledgment before execution.** Do not perform destructive operations silently.

---

### STEP 5 — SUB-AGENT ASSIGNMENT

For each `PARALLEL` unit, evaluate:

1. Is the unit large enough that executing it inline would meaningfully grow the main context window?
2. Is the specification clear enough to hand off without ambiguity?
3. Does it operate on different files than the main workstream?

If all three are true → assign to sub-agent with a complete, self-contained spec.
If any one is false → execute in the main agent.

A vague sub-agent spec produces incorrect output. Only delegate when the spec is complete and unambiguous.

---

### STEP 6 — STATE THE PLAN, THEN EXECUTE

Write the plan in this format before taking any action:

```
PLAN
────
Goal: [one sentence describing the successful outcome]

Workstreams:
1. [name] — [output] — [BLOCKING / PARALLEL / TERMINAL]
2. [name] — [output] — [BLOCKING / PARALLEL / TERMINAL]
...

Critical path:    [ordered sequence of BLOCKING workstreams]
Parallel units:   [PARALLEL workstreams and their sub-agent assignment, if applicable]
Risks:            [top 1–2 risks with mitigation approach]
Assumptions:      [any ambiguities resolved by assumption]

Executing.
```

Proceed to execution immediately after. Do not request approval unless a step is irreversible and high-consequence.

---

## REPLAN TRIGGER

If mid-execution the task changes materially — a required file does not exist, a dependency produces unexpected output, an external call fails — stop execution. Rerun Steps 2 through 6 with updated information. State what changed and why the plan is being revised, then continue.

Do not improvise around an unexpected condition without replanning first.

---

## SCOPE

This protocol applies to complex tasks only. It is not a general-purpose requirement for every action. Applying it to simple tasks wastes context and time. The activation criteria in the header define when it is required.

This protocol governs planning structure. It does not replace extended thinking, which governs reasoning during execution. Both operate independently.
