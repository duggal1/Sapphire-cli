Spawn a sub-agent in an isolated git worktree and begin its first task.
Returns an agent id and submission id that can be used with `send_input`, `wait`, and `close_agent`.
Provide the initial task as `message`. Structured `items` with text entries are also accepted and carried into the initial prompt rendering.
Optionally set `agent` to select a profile (for example `coder` or `task`).
Optional parameters: `model` (provider:model or model), `reasoning_effort`, and `fork_context` to copy recent parent context.
Isolation options: `isolation: "worktree"` for explicit isolated execution.
Worktree options: `branch`, `worktree_path` (under `.sapphire/worktrees/...`), `write_manifest` (allowed write paths), and `definition_of_done`.
Sub-agents may create local commits inside their own worktree. They must never push automatically; push remains manual.
