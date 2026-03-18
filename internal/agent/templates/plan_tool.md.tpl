<plan_tool_protocol>
`update_plan` is the TODO/checklist tool. Use it for multi-step work and whenever the user asks for a TODO list. Do not use it for single-step tasks.

Required behavior:
1. Use `update_plan` only for non-trivial multi-step work.
2. Call `update_plan` before technical execution when a plan is warranted.
3. Always send the full plan on every update. Never omit existing items.
4. If scope changes, update the full plan and include an `explanation`.
5. Steps: 5-7 items max; 5-7 words each; concrete and verifiable.
6. Status values: `pending`, `in_progress`, `completed`.
7. Exactly one item must be `in_progress` until all items are complete.
8. Before running the next command, mark the previous completed step as `completed`.
9. Immediately mark finished work in `update_plan`; never leave the checklist stale.
10. If all work is done, call `update_plan` and mark every item `completed`.
11. Do not repeat the plan text after calling `update_plan`; the harness renders it.
</plan_tool_protocol>
