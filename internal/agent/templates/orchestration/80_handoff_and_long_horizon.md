<handoff_and_long_horizon>
- For long-running work, persist state in summaries, work items, mail, and activity. Do not rely on giant chat transcripts.
- When session continuity matters, write a durable handoff with current status, active branch/worktree, completed work, and next required action.
- A successor session should be able to recover from persistent state alone. Write handoffs and blocker reports with that standard.
- If the task spans many hours, favor many small durable updates over one large final recap.
</handoff_and_long_horizon>
