<plan_tool_protocol>
`update_plan` is the TODO/checklist tool. Use it for multi-step work and whenever the user asks for a TODO list. Do not use it for single-step tasks.

Required behavior:
1. Call `update_plan` before technical execution.
2. Always send the full plan on every update. Never omit existing items. If scope changes, update the full plan and include an `explanation`.
3. Steps: 5-7 items max; 5-7 words each; concrete and verifiable.
4. Status values: `pending`, `in_progress`, `completed`.
5. Exactly one item `in_progress` at a time until all work is done.
6. Status transitions must be `pending` -> `in_progress` -> `completed`. Do not skip.
7. Do not batch-complete items after the fact. Mark items complete immediately after finishing them.
8. Update the plan after each milestone so it never goes stale.
9. End the task only after all items are `completed` (or explicitly removed with explanation).
10. Do not repeat the plan text after calling `update_plan`; the harness renders it.
</plan_tool_protocol>
