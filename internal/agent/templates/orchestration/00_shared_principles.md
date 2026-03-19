<orchestration_principles>
- Keep Sapphire's native sub-agent lifecycle as the base system: `spawn_agent` -> `resume_agent` -> `send_input` -> `wait` -> `collect_result` -> `close_agent`.
- Treat persistent orchestration state as the source of truth. Chat context is a cache.
- Use sub-agents to parallelize independent work. Do not create overlap-heavy workers just to look busy.
- Coordination, evidence collection, validation, and controlled integration matter more than raw spawn count.
</orchestration_principles>
