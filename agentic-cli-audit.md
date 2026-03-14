****# Agentic CLI Audit

Updated: 2026-03-14

## Summary

This report compares Sapphire CLI, Codex, and Claude Code with a strict rule:
claims are separated into:****

1. Locally inspectable source
2. Official public web documentation
3. Unknown / not publicly inspectable

## Claude Code: Source Availability

### What is true

- The `anthropics/claude-code` GitHub repository is public:
  [github.com/anthropics/claude-code](https://github.com/anthropics/claude-code)
- The public repo is not open source in the same way Codex is.
- The local `claude-code/LICENSE.md` says: "All rights reserved" and use is subject to Anthropic's Commercial Terms of Service.
- Based on both the local repo and the public GitHub file tree, the repo surface is mainly:
  plugins, hooks, slash-command content, examples, scripts, changelog, settings, and GitHub workflow glue.
- I do not see the equivalent of a full core engine source tree like Codex's `codex-rs/core`, `tui`, `exec`, and `app-server`.

### Precise conclusion

It is not accurate to say "Claude Code is not public at all."

It is accurate to say:

- Claude Code has a public repository.
- That public repository is not distributed under a normal open-source software license.
- The public repository does not expose the full CLI engine implementation at the same depth that Codex does.

So the strongest safe wording is:

> Claude Code is publicly distributed, but its public repository does not appear to expose the full production CLI engine as open-source core source code.

## Claude Code: What We Know From Official Public Docs

The following capabilities are documented on Anthropic's official sites, even when the underlying engine source is not publicly inspectable in the repo.

### Core product surface

- Terminal-first coding agent:
  [Claude Code GitHub README](https://github.com/anthropics/claude-code)
- Native IDE integrations for VS Code and Cursor, including selection sharing, diff viewing, diagnostics, multiple sessions, and terminal mode:
  [IDE integrations](https://code.claude.com/docs/en/ide-integrations)
- GitHub Actions support, including `@claude` workflows and programmatic integration:
  [GitHub Actions](https://docs.claude.com/en/docs/claude-code/github-actions)
- A programmatic SDK / headless mode built on the same agent framework:
  [Agent SDK overview](https://docs.claude.com/en/docs/claude-code/sdk/sdk-overview)
  [Headless mode](https://docs.claude.com/en/docs/claude-code/sdk/sdk-headless)

### Agent / orchestration capabilities

- Custom subagents with their own context windows, custom system prompts, specific tool access, and independent permissions:
  [Subagents](https://code.claude.com/docs/en/subagents)
- Slash commands, including built-in, plugin-provided, and MCP-provided commands:
  [Slash commands](https://docs.claude.com/en/docs/claude-code/slash-commands)
- Hooks around tool use and session lifecycle:
  [Hooks reference](https://docs.claude.com/en/docs/claude-code/hooks)
- Persistent project memory via `CLAUDE.md` plus auto memory:
  [Memory](https://code.claude.com/docs/en/memory)

### Tooling / integration capabilities

- MCP connectivity to external tools and data sources:
  [MCP](https://docs.claude.com/en/docs/claude-code/mcp)
- Official docs describe file operations, code execution, web search, and MCP extensibility as part of the SDK feature set:
  [Agent SDK overview](https://docs.claude.com/en/docs/claude-code/sdk/sdk-overview)

### Security / control capabilities

- Permission system with allow / ask / deny rules:
  [Authentication and IAM](https://code.claude.com/docs/en/iam)
- Security model with project-scoped write restrictions:
  [Security](https://code.claude.com/docs/en/security)
- Native sandboxing for the bash tool using OS-level filesystem and network isolation:
  [Sandboxing](https://code.claude.com/docs/en/sandboxing)

### Deployment / runtime surfaces

- Claude Code on the web, with isolated VMs and GitHub-backed cloud sessions:
  [Claude Code on the web](https://docs.claude.com/en/docs/claude-code/claude-code-on-the-web)
- Team and enterprise auth paths, including Claude.ai, Console, Bedrock, Vertex, and Microsoft Foundry:
  [Authentication](https://code.claude.com/docs/en/iam)

## Claude Code: What Is Still Unknown From Public Evidence

The following are not auditable from the public repo at the same level as Codex:

- Internal turn loop implementation
- Internal state model and persistence architecture
- Exact tool router implementation
- Exact sandbox implementation details beyond documented behavior
- Internal approval cache / orchestration logic
- Core session / thread runtime internals
- Core model streaming pipeline

Those may exist in the product, but they are not source-verifiable from the public repository I audited.

## Comparison Update

| System | Public repo status | License posture | Full core engine inspectable from repo? | Evidence quality |
|---|---|---|---|---|
| Codex | Public | Apache-2.0 | Yes | High |
| Sapphire CLI | Local repo available | Repo-local source available for audit | Yes | High |
| Claude Code | Public | All rights reserved / commercial terms | No, not at Codex-equivalent depth | Medium for docs, low for core internals |

## Capability Comparison

### Codex

- Strongest source-verifiable architecture.
- Clear layered separation: core runtime, CLI, TUI, app-server, MCP, plugins, skills, rollout/state.
- Best verifiable safety/control plane.

Primary evidence:
- `codex/codex-rs/core/src/codex.rs`
- `codex/codex-rs/core/src/tools/orchestrator.rs`
- `codex/codex-rs/core/src/thread_manager.rs`
- `codex/codex-rs/app-server/README.md`

### Sapphire CLI

- Strong local source visibility.
- Aggressive subagent and worktree orchestration.
- Weaker safety model than Codex based on inspectable **implementation**.

Primary evidence:
- `internal/agent/agent.go`
- `internal/agent/coordinator.go`
- `internal/agent/subagent_manager.go`
- `internal/agent/worktree_orchestrator.go`
- `internal/permission/permission.go`
- `internal/shell/shell.go`

### Claude Code

- Publicly documented as very capable.
- Public repo proves plugin, hook, slash-command, and marketplace surface.
- Full core runtime is not inspectable from the public repo the way Codex is.

Primary local evidence:
- `claude-code/README.md`
- `claude-code/CHANGELOG.md`
- `claude-code/plugins/README.md`
- `claude-code/plugins/hookify/hooks/pretooluse.py`
- `claude-code/plugins/hookify/core/rule_engine.py`

Primary web evidence:
- [Subagents](https://code.claude.com/docs/en/subagents)
- [Sandboxing](https://code.claude.com/docs/en/sandboxing)
- [Security](https://code.claude.com/docs/en/security)
- [MCP](https://docs.claude.com/en/docs/claude-code/mcp)
- [Memory](https://code.claude.com/docs/en/memory)
- [IDE integrations](https://code.claude.com/docs/en/ide-integrations)
- [GitHub Actions](https://docs.claude.com/en/docs/claude-code/github-actions)
- [Agent SDK overview](https://docs.claude.com/en/docs/claude-code/sdk/sdk-overview)

## Current Verdict

If the standard is "most capable agentic CLI that is verifiable from source," Codex remains the strongest.

If the standard is "most capable product surface documented by the vendor," Claude Code is clearly in the top tier, but I cannot honestly rank its internal architecture above or below Codex from public source alone because the full engine is not exposed in the repo.
