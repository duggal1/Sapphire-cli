## Sapphire Orchestrator Prompt

This file is the human entrypoint for Sapphire's orchestration prompt stack.

The actual orchestration protocol is modular and lives under:
- `internal/agent/templates/orchestration/`

Intent:
- keep Sapphire's native sub-agent runtime as the base
- add stricter orchestration, reporting, handoff, and stuck-agent behavior
- align with the strongest Gastown patterns that fit Sapphire's architecture

Primary modules:
- shared principles
- startup and recovery
- worktree and git discipline
- mail protocol
- health and stall control
- reporting and validation
- orchestrator role
- handoff and long-horizon rules
