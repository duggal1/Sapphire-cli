Launch a new sub-agent with its own terminal to run independently from the main reasoning loop. By default it runs against the shared repository root.

<usage>
- Use when independent operational work can run in parallel or background: builds, installs, scripts, tests/lint/verification, codebase scans, data gathering, API/log/system inspection, or environment setup.
- Use when multiple operational tasks can be distributed across sub-agents for efficiency.
- Do NOT use for trivial tasks, simple questions, reasoning-only work, or a single immediate operation the main agent can do directly.
</usage>

<usage_notes>
1. Sub-agents default to the shared repository root. Lifecycle sub-agent worktree isolation is disabled for now.
2. Provide a `write_manifest` to restrict writes to owned files. Empty manifest = read-only.
3. Use `orchestrate_worktrees` or explicit resume-worktree flows when you truly need isolated worktree execution.
2. If multiple sub-agents are truly independent, launch them in parallel aggressively. If they share files or state, keep the work sequential.
3. The sub-agent returns one final report to you. The user does not see that report unless you summarize or apply its outcome.
4. Each invocation is stateless. Write a precise prompt with explicit scope, constraints, deliverables, and success criteria.
5. Use the explicit lifecycle: `spawn_agent` -> `send_input` -> `wait` -> `collect_result` -> `close_agent`.
6. Nested sub-agents are allowed only through the explicit lifecycle: `spawn_agent` -> `send_input` -> `wait` -> `collect_result` -> `close_agent`.
7. Verify sub-agent output before relying on it for the final answer.
</usage_notes>
