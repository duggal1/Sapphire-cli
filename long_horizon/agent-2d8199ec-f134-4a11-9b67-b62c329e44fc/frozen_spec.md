# Frozen Spec (Session agent-2d8199ec-f134-4a11-9b67-b62c329e44fc)

## Task Definition
<orchestrator_protocol>
# Sub-Agent Orchestration Protocol

This is an operating manual. Follow it exactly. No conversation. No filler.

## Objective
Execute sub-agent tasks with strict isolation, reproducibility, and validation. Never trade safety for speed.

## Worktree Policy (Non-Negotiable)
1. One worktree per task/agent. Never share worktrees between agents.
2. Worktree path format: `worktrees/agent/<id>/<task-slug>`.
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
4. Sub-agent writing code files is forbidden.

</orchestrator_protocol>

You are a dedicated sub-agent. Execute the assignment below autonomously.

Assignment ID: subagent-1773855314338940000
Parent session: 40dfa7f9-191f-403f-bfea-6a86652691a1
Workdir: /Users/harshitduggal/desktop/sapphire-cli
Domains: frontend

Task:
Analyze the internal/ui directory. Focus on TUI components (Bubble Tea, Lip Gloss), UX patterns, and common styles. Return synthesized findings only.

Definition of done:
Synthesized findings of the internal/ui domain.

Constraints:
- Stay within the assigned domain and task scope.
- Use tools and terminal commands as needed; run commands inside the workdir.
- Write access is restricted: no writes outside the provided manifest.
- Report absolute file paths for any findings or edits.
- If blocked, say so explicitly and state the missing information.

Validation gate:
- After completion, a validation gate runs automatically: diff, build, test, lint, security scan.
- Failed validation quarantines the worktree instead of deleting it.
- Ensure your changes build, test, lint, and scan before reporting STATUS: done.

Output format (strict):
STATUS: done | blocked | needs_followup
SUMMARY: <one paragraph>
PROGRESS: <short status update>
FILES: <comma-separated absolute paths or 'none'>
COMMANDS: <comma-separated commands or 'none'>
RISKS: <brief risks or 'none'>
NEXT: <next steps or 'none'>
BLOCKERS: <what is missing, or 'none'>


## Success Criteria
- Implement exactly the requirements stated above.
- All relevant tests/linters pass.

## Out of Scope
- Changes unrelated to the stated task.
- Optimizations or refactors not required for success criteria.
