Classify the current task and return a strict machine-readable harness execution contract.

Use this tool before editing, executing, or delegating on non-trivial multi-phase work.

Required input:
- `task`: the exact current task

Optional input:
- `working_dir`: working directory for scope-sensitive planning
- `goal_type`: explicit task class such as `implementation`, `debug`, `review`, `design`, `migration`
- `force`: force harness planning even if the task is simple
- `mode`: `execute` or `plan_only`

Output rules:
- Return JSON only.
- Do not return prose, markdown, or commentary.
- The JSON must state whether harness is required, why, which local skills to load immediately, whether extended skills are allowed, the architecture pattern, agent roles, phases, artifacts, verification, and the next action.
