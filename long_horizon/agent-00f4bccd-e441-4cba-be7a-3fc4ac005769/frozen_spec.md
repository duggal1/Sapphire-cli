# Frozen Spec (Session agent-00f4bccd-e441-4cba-be7a-3fc4ac005769)

## Task Definition
You are a dedicated sub-agent. Execute the assignment below autonomously.

Assignment ID: subagent-1773596339631988000
Parent session: 77507d0a-9fa6-4d64-8668-82bacad2c448
Title: Agent Job 73005fd8-157f-499c-a9fe-709ff5fa78cc
Workdir: /Users/harshitduggal/desktop/sapphire-cli
Domains: database

Task:
You are processing one item for a batch agent job.
Job ID: 73005fd8-157f-499c-a9fe-709ff5fa78cc
Item ID: row-1

Task instruction:
Analyze the domain: Core Agent. Analyze internal/agent/ for core logic and coordination. Key files: agent.go, coordinator.go. Use agentic_view to read files.

Input row (JSON):
{
  "domain": "Core Agent",
  "instructions": "Analyze internal/agent/ for core logic and coordination. Key files: agent.go, coordinator.go."
}

Expected result schema (JSON Schema or {}):
{}

You MUST call the `report_agent_job_result` tool exactly once with:
1. `job_id` = "73005fd8-157f-499c-a9fe-709ff5fa78cc"
2. `item_id` = "row-1"
3. `result` = a JSON object that contains your analysis result for this row.

If you need to stop the job early, include `stop` = true in the tool call.

After the tool call succeeds, stop.

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
