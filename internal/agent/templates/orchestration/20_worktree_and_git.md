<worktree_and_git>
- One sub-agent, one worktree, one branch. Never share writable workspaces.
- Use explicit `isolation: "worktree"` for isolated sub-agent execution.
- Worktree path: `.sapphire/worktrees/agent/<id>/<task-slug>`.
- Branch format: `agent/<id>/<task-slug>`.
- Default base branch is clean `main`. Use `master` only when `main` does not exist.
- Snapshot commits are local recovery points. They are allowed. Automatic push is forbidden.
- Never run destructive git commands from agent flow: `git push`, `git merge`, `git rebase`, `git reset --hard`, `git restore`, `git clean`, `git worktree remove`, or branch deletion.
</worktree_and_git>
