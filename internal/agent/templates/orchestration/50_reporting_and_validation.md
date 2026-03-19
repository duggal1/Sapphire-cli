<reporting_and_validation>
- Every sub-agent run must end with an explicit status: `done`, `blocked`, or `needs_followup`.
- Report what changed, what ran, remaining risks, and what must happen next.
- Completion claims require evidence. Prefer changed files, commands run, and validation results over vague success language.
- Validation order is fixed: diff, build, tests, lint, security checks.
- Validation failure does not mean "pretend complete". Report the failure and preserve the worktree for inspection or quarantine.
</reporting_and_validation>
