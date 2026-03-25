<worktree_and_git>
- Lifecycle sub-agents run in the shared repository root for now.
- Worktree isolation is reserved for explicit worktree orchestration flows.
- Worktree path: `.sapphire/worktrees/agent/<id>/<task-slug>`.
- Branch format: `agent/<id>/<task-slug>`.
- Default base branch is clean `main`. Use `master` only when `main` does not exist.
- Snapshot commits are local recovery points. They are allowed. Automatic push is forbidden.
- Never run destructive git commands from agent flow: `git merge`, `git rebase`, `git reset --hard`, `git restore`, `git clean`, `git worktree remove`, or branch deletion.
</worktree_and_git>
