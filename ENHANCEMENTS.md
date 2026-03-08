# Sapphire-CLI Enhancement Summary

## Branding & Terminology Changes

| Action | File Path |
| :--- | :--- |
| Updated application name to **Sapphire** | `internal/config/config.go` |
| Renamed `multiedit` tool to `agentic_edit` | `internal/agent/tools/multiedit.go` |
| Renamed `view` tool to `agentic_view` | `internal/agent/tools/view.go` |
| Updated build output binary to `sapphire` | `Taskfile.yaml` |
| Updated UI logo and rendering text to **Sapphire** | `internal/ui/logo/logo.go` |

## UI/UX Infrastructure & Styling

| Action | File Path |
| :--- | :--- |
| Implemented **Orange-dominant gradient** (Zest → Mustard → Salmon) | `internal/ui/styles/styles.go` |
| Added **Yellow Mode** toggle (Lime → Green gradient) | `internal/ui/styles/styles.go` |
| Darkened CLI background to **near-black** | `internal/ui/styles/styles.go` |
| Added internal padding to **thinking bubbles** | `internal/ui/styles/styles.go` |
| Changed `agentic_edit` (formerly multi-edit) UI color to **Pink** | `internal/ui/styles/styles.go` |
| Implemented **Gradient Shimmer** effect for loading states | `internal/ui/anim/anim.go` |

## Core Agent Capabilities (Concurrency & Limits)

| Action | File Path |
| :--- | :--- |
| Implemented **Parallel File Reading** (Go concurrency) | `internal/agent/tools/view.go` |
| Configured Main Agent parallel limit to **50 files** | `internal/agent/coordinator.go` |
| Configured Sub-Agent parallel limit to **5 files** | `internal/agent/coordinator.go` |
| **Removed** `agentic_edit` capability from Sub-Agents | `internal/config/config.go` |
| Adjusted `agentic_edit` limit to **10 operations** | `internal/agent/tools/multiedit.go` |

## Interaction & Intelligence

| Action | File Path |
| :--- | :--- |
| Implemented **Double-click to copy** for Assistant messages | `internal/ui/chat/assistant.go` |
| Added **Collapsible Thinking Bubble** toggle | `internal/ui/chat/assistant.go` |
| Updated **System Prompts** for tool and parallel awareness | `internal/agent/agent.go` |
| Refactored tool documentation for LLM awareness | `internal/agent/tools/agentic_edit.md`, `view.md`, `edit.md` |
