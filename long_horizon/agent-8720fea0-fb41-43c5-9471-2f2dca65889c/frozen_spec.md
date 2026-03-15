# Frozen Spec (Session agent-8720fea0-fb41-43c5-9471-2f2dca65889c)

## Task Definition
You are a dedicated sub-agent. Execute the assignment below autonomously.

Assignment ID: subagent-1773603192365379000
Parent session: 0e124ca0-4c08-485a-a6c8-c7411640e753
Workdir: /Users/harshitduggal/desktop/sapphire-cli/worktrees/please-audit-the-internalagenttools-directory-i-need-a-report-identifying-every-tool-implemented-there-a-summary-of-their
Branch: subagent/please-audit-the-internalagenttools-directory-i-need-a-report-identifying-every-tool-implemented-there-a-summary-of-their-8720fea0
Domains: infra, docs

Task:
Please audit the 'internal/agent/tools' directory. I need a report identifying every tool implemented there, a summary of their functionality, and a specific check for safety constraints or restricted commands within their Go source files.

Definition of done:
Provide a detailed audit of the tools defined in internal/agent/tools. Identify each tool, its primary function, and any safety constraints implemented in the source code.

Constraints:
- Stay within the assigned domain and task scope.
- Use tools and terminal commands as needed; run commands inside the workdir.
- Write access is restricted: no writes outside the provided manifest.
- Report absolute file paths for any findings or edits.
- If blocked, say so explicitly and state the missing information.

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
