Launch a new sub-agent with its own terminal and the same core execution capabilities needed for scoped investigation and implementation. Use the Agent tool when delegation will improve context quality, latency, or independence of verification.

<usage>
- Use this when the task is large, isolated, or noisy enough that keeping it in the main context would reduce quality.
- Good fits: codebase mapping, dependency tracing, isolated research, independent verification, or parallel workstreams with clear file boundaries.
- Do not use this for tiny tasks, vague tasks, or work that shares mutable state with another ongoing task.
</usage>

<usage_notes>
1. If multiple sub-agents are truly independent, launch them in parallel. If they share files or state, keep the work sequential.
2. The sub-agent returns one final report to you. The user does not see that report unless you summarize or apply its outcome.
3. Each invocation is stateless. Write a precise prompt with explicit scope, constraints, deliverables, and success criteria.
4. Sub-agents can read files, search the web, run terminal commands, and modify code. Give them clear file boundaries so parallel work does not conflict.
5. Sub-agents cannot spawn sub-agents. Nesting is not allowed.
6. Verify sub-agent output before relying on it for the final answer.
</usage_notes>
