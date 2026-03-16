# Frozen Spec (Session be45f95b-dba7-4e4b-a27a-c7d790823315)

## Task Definition
You are running inside Sapphire CLI with sub-agent tools enabled. Prove that sub-agent orchestration works. Spawn exactly 2 sub-agents in parallel. Give each sub-agent a tiny independent read-only task in this repository. Use the explicit lifecycle for each one: spawn_agent, wait, collect_result, close_agent. Do not edit any files. Do not use spawn_agents_on_csv. Return a short final summary showing each sub-agent id, what task it was given, whether it completed successfully, and the result collected from it. Keep the tasks trivial and safe. This is only a capability demo.

## Success Criteria
- Implement exactly the requirements stated above.
- All relevant tests/linters pass.

## Out of Scope
- Changes unrelated to the stated task.
- Optimizations or refactors not required for success criteria.
