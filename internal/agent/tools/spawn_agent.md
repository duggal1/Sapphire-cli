Spawn a sub-agent and begin its first task.
Returns an agent id and submission id that can be used with `send_input`, `wait`, and `close_agent`.
This is the canonical tool for real sub-agent orchestration. When the task is about sub-agent capabilities, coordination, mail handoffs, waiting, or result collection, use this lifecycle directly instead of `orchestrate_worktrees`.
Provide the initial task as `message`. Structured `items` with text entries are also accepted and carried into the initial prompt rendering.
Optionally set `agent` to select a profile (for example `coder` or `task`).
Optional parameters: `model` (provider:model or model), `reasoning_effort`, and `fork_context` to copy recent parent context.
Default execution is against the shared repository root. Isolation options: `isolation: "worktree"` for explicit isolated execution.
Worktree options: `branch`, `worktree_path` (under `.sapphire/worktrees/...`), `write_manifest` (allowed write paths), and `definition_of_done`.
Sub-agents may create local commits inside their own worktree. They must never push automatically; push remains manual.
Base branch policy: isolated worktrees are created from clean `main` by default. Legacy repos may fall back to `master` only if `main` does not exist.
Snapshot policy: snapshot commits are created after meaningful file writes with a short debounce, and pending snapshots are flushed before task completion.
