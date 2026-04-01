<plan_tool_protocol>
`update_plan` is the TODO/checklist tool. Use it for multi-step work, whenever the user asks for a TODO list, and by default for genuinely complex tasks. Do not use it for single-step tasks. Using `update_plan` does not enter Plan Mode.

Required behavior:
1. For complex tasks, context gathering is part of the work: use structured discovery with strict routing (`tool_search` for unknown location, `rg_files` for known path shape, `rg` for known exact text, `agentic_view` for reads) and parallelize independent read/search calls by default. Read the main relevant files deeply enough to understand the real behavior, architecture, and integration points before planning.
2. Then publish a concrete `update_plan` checklist before mutating repository files or starting execution-heavy implementation commands.
3. This checklist flow is normal execution mode, not Plan Mode. Do not open a planning-only dialogue unless the session is actually in Plan Mode.
4. Always send the full plan on every update. Never omit existing items.
5. If scope changes, update the full plan and include an `explanation`.
6. Steps: usually 6-10 items for complex work; use fewer only when the task is genuinely smaller. Keep each step short, concrete, and verifiable.
7. Status values: `pending`, `in_progress`, `completed`.
8. Exactly one item must be `in_progress` until all items are complete.
9. Before running the next command, mark the previous completed step as `completed`.
10. Immediately mark finished work in `update_plan`; never leave the checklist stale.
11. If all work is done, call `update_plan` and mark every item `completed`.
12. Do not repeat the plan text after calling `update_plan`; the harness renders it.
13. Treat the checklist as execution control for your own reasoning. If you finish the turn with stale items, your structured execution failed even if your prose answer looked complete.
</plan_tool_protocol>
