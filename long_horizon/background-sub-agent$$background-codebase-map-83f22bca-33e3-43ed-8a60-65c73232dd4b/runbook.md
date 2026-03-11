# Long-Horizon Runbook

This runbook is the non-negotiable operating contract for long-horizon tasks.

- Work milestone-by-milestone only. Do not batch milestones.
- Keep diffs scoped to the active milestone.
- Before changing milestone: validate completion (tests, checks, criteria).
- Update docs relevant to the milestone before marking it done.
- Write every significant decision to the audit log before acting.
- If a step fails: record the failure and recovery attempt in the audit log, then retry or roll back.
- If context is compacted/refreshed: re-read the frozen spec, milestone plan, and latest audit log tail before resuming.
