Spawn a sub-agent and begin its first task.
Returns an agent id and submission id that can be used with `send_input`, `wait`, and `close_agent`.
This is the canonical tool for real sub-agent orchestration. When the task is about sub-agent capabilities, coordination, mail handoffs, waiting, or result collection, use this lifecycle directly instead of `orchestrate_worktrees`.
Provide the initial task as `message`. Structured `items` with text entries are also accepted and carried into the initial prompt rendering.
Optionally set `agent` to select a profile (for example `coder` or `task`).
Optional parameters: `model` (provider:model or model), `reasoning_effort`, and `fork_context` to copy recent parent context.
Default execution is against the shared repository root.
Lifecycle sub-agent worktree isolation is disabled for now. Use `orchestrate_worktrees` or `ResumeWorktree` for explicit worktree-heavy flows.
Compatibility fields such as `isolation`, `branch`, and `worktree_path` may still be accepted, but normal `spawn_agent` execution stays in the shared repository root.
