Orchestrate multi-agent worktree execution with strict write manifests.
This is a batch helper built on top of the sub-agent system. It is not the primary tool for demonstrating or debugging the real sub-agent lifecycle.

Required:
- `tasks`: list of worktree task specs, each with `name`, `prompt`, and `write_manifest`.

Optional:
- `test_command`: spawns per-worktree test runner agents (read-only).
- `integration_prompt`: spawns an integration agent to merge branches and validate.
- `integration_branch`: branch name for the integration worktree.

Each task spawns a sub-agent with its own worktree + branch and enforced write manifest.
The tool returns agent ids, submission ids, branches, and worktree paths.
Use `spawn_agent` directly instead when the task requires visible sub-agent spawning, mail handoffs, wait/collect flow, or explicit agent-state debugging.
