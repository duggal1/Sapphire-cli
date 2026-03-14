Orchestrate multi-agent worktree execution with strict write manifests.

Required:
- `tasks`: list of worktree task specs, each with `name`, `prompt`, and `write_manifest`.

Optional:
- `test_command`: spawns per-worktree test runner agents (read-only).
- `integration_prompt`: spawns an integration agent to merge branches and validate.
- `integration_branch`: branch name for the integration worktree.

Each task spawns a sub-agent with its own worktree + branch and enforced write manifest.
The tool returns agent ids, submission ids, branches, and worktree paths.
