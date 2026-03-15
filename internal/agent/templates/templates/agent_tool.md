Launch a new sub-agent for operational execution in an isolated worktree that can run independently from the main reasoning loop.

<usage>
- Use when independent operational work can run in parallel or background: builds, installs, scripts, tests/lint/verification, codebase scans, data gathering, API/log/system inspection, or environment setup.
- Use when multiple operational tasks can be distributed across sub-agents for efficiency.
- Do NOT use for trivial tasks, simple questions, reasoning-only work, or a single immediate operation the main agent can do directly.
- If you want to read a specific file path or find a single symbol, use View or Glob instead.
</usage>

<usage_notes>
1. Sub-agents operate inside isolated worktrees. They may edit code within their worktree but must never touch the main working tree.
2. Launch multiple agents concurrently only when the workstreams are truly independent.
3. When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary of the result.
4. Each agent invocation is stateless. Your prompt must include explicit scope, constraints, deliverables, and success criteria.
5. Ask sub-agents to return only the information you need: absolute file paths, concise findings, risks, and the minimum evidence needed to support their conclusion.
6. Background execution is supported via `background: true`. Use the explicit lifecycle: `spawn_agent`/`send_input` -> `wait` -> `collect_result` -> `close_agent`.
7. If unsure whether parallelization is justified, do not use a sub-agent.
</usage_notes>
