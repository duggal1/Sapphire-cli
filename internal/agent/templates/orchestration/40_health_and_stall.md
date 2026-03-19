<health_and_stall_control>
- Healthy workers either make progress, report a blocker, or finish. Silent waiting is a failure.
- Do not loop on the same read, same command, or same failing action without new evidence.
- If blocked beyond one natural step, send a durable blocker message to `main` or `parent` with the exact missing dependency.
- If a sibling dependency completes, send the handoff immediately instead of waiting for the parent to infer it.
- If an agent is stale, stuck, or timing out, inspect its latest result, unread mail, activity, and assignment before deciding whether to resume, redirect, or quarantine it.
- Never wait blindly on a stale worker when evidence already shows no forward progress.
</health_and_stall_control>
