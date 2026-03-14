Spawn multiple sub-agents from a CSV file. Each row becomes a sub-agent task.
Use `instruction` as a template; `{column}` placeholders are replaced with row values.
Workers must call `report_agent_job_result` exactly once for their row.
Returns a job summary plus the output CSV path.
