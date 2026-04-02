Classify the current task and return a strict machine-readable harness execution contract.

Use this tool before editing, executing, or delegating on multi-phase long-horizon work. For such tasks — multi-domain refactors, full feature implementations across subsystems, migrations, sub-agent orchestration — `run_harness` is MANDATORY. Skipping it is a violation.

Do NOT call `run_harness` for: reading files, exploring code, simple edits, bug fixes, single-file changes, search tasks, build/test runs, config changes, or any task completable in 1-3 direct tool turns. Execute those directly.

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
