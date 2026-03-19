# Sub-Agent Orchestration Protocol

This is an operating manual. Follow it exactly. No conversation. No filler.

## Objective
Execute sub-agent tasks with strict isolation, reproducibility, and validation. Never trade safety for speed.

## Worktree Policy (Non-Negotiable)
1. One worktree per task/agent. Never share worktrees between agents.
2. Worktree path format: `.sapphire/worktrees/agent/<id>/<task-slug>`.
3. Branch format: `agent/<id>/<task-slug>`.
4. Always create worktrees from a clean `main` base. If the base is dirty, stop and report.
5. Reuse is allowed only for explicit resume of the same worktree (`--resume` flow).

## Lifecycle Ownership
1. Orchestrator creates the worktree.
2. Orchestrator writes task context into `TASK.md`.
3. Orchestrator launches the sub-agent with explicit scope and constraints.
4. Orchestrator monitors progress and status.
5. Orchestrator runs validation gates after completion.
6. Orchestrator quarantines or cleans up the worktree based on results.

## Task Queue Discipline
Each agent must:
1. Claim exactly one task.
2. Create a dedicated worktree and branch.
3. Execute the task inside that worktree only.
4. Validate, commit, and report.
5. Exit cleanly.
6. Snapshot commits are local safety points only. Never push automatically.

## Validation Gate (Mandatory)
Run in this order:
1. `git diff --stat` against base.
2. Build.
3. Tests.
4. Lint.
5. Security scan.
If any step fails, the worktree is quarantined.

## Failure Handling
1. Failed worktrees with changes are quarantined, not deleted.
2. Zero-change worktrees are deleted immediately.
3. On crash or interruption, never auto-clean the worktree.
4. Resume uses the existing worktree only.

## Prohibitions
1. Never push or merge directly to `main`.
2. Never operate outside the assigned worktree.
3. Never skip validation.
4. Never run destructive git commands (`git push`, `git reset --hard`, `git restore`, `git clean`, `git rebase`, `git worktree remove`).
