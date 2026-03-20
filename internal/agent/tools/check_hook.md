Check the current hook assignment for the running agent.

Use this to read the durable work item attached to the agent's hook. The hook is the authoritative assignment state across resumes, handoffs, and orchestration restarts.

Returns either:
- structured hook + work item details
- `hook empty`
