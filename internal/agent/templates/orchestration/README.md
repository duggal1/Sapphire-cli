## Sapphire Orchestration Prompt Stack

This directory holds the modular orchestration prompt layer for Sapphire CLI.

Purpose:
- keep Sapphire's existing sub-agent runtime as the base system
- add stricter orchestration behavior without rewriting the runtime
- align startup, mail, recovery, liveness, and handoff behavior with the strongest Gastown patterns that fit Sapphire's architecture

Rules:
- these files are prompt modules, not standalone runtime logic
- use them to compose orchestrator and sub-agent instructions
- keep them short, operational, and evidence-driven
- do not duplicate the whole stack inside `coder.md.tpl`

Composition:
- main-agent orchestration overlay:
  - `00_shared_principles.md`
  - `10_startup_and_recovery.md`
  - `20_worktree_and_git.md`
  - `30_mail_protocol.md`
  - `40_health_and_stall.md`
  - `50_reporting_and_validation.md`
  - `60_orchestrator_role.md`
  - `80_handoff_and_long_horizon.md`
- sub-agent orchestration protocol:
  - `00_shared_principles.md`
  - `10_startup_and_recovery.md`
  - `20_worktree_and_git.md`
  - `30_mail_protocol.md`
  - `40_health_and_stall.md`
  - `50_reporting_and_validation.md`
  - `70_subagent_role.md`
  - `80_handoff_and_long_horizon.md`
