Track multi-step work with a strict todo list.

Use `todos` when the task has multiple steps, multiple files, or any non-trivial execution sequence.

Rules:
- Create the full list before technical work.
- Keep exactly one task `in_progress`.
- For each item: `start` before work, `complete` immediately after validation.
- Use `list` to resync before the next item when needed.
- Prefer `task_key` as the stable selector when the planner/runtime provides one.
- Prefer `task_id` only when the current list was just read or created.
- If the list was recreated, reset, or ids may be stale, use `task_content` for `start` or `complete` after a `list` resync.
- Do not mention the todo list in normal response text; the UI already shows it.

Task fields:
- `content`: imperative form, for example `Run tests`
- `key`: stable machine-friendly task key, for example `run_tests`
- `active_form`: present-continuous form, for example `Running tests`
