# AGENTS.md — Claude Code Architecture & Capabilities

## Repository Overview
This repository contains the source code for **Claude Code**, Anthropic's agentic CLI coding assistant. It provides an interactive agentic loop, tool orchestration, MCP (Model Context Protocol) integration, multi-agent swarms, remote control (bridge), and a comprehensive TUI built with Ink (React for terminals). Runtime: Bun. Language: TypeScript/TSX. Total: ~1978 files across 6 extensions (.ts 67%, .tsx 28%).

## Architecture Summary
- **Core Loop:** `QueryEngine.ts` owns the conversation lifecycle. `submitMessage()` yields SDK messages via async generator. `query()` in `query.ts` drives the agentic loop: API call → stream response → execute tools → repeat until stop. One `QueryEngine` per conversation; each `submitMessage()` starts a new turn.
- **Tools:** 40+ built-in tools defined in `tools/`, built via `buildTool()` factory (`Tool.ts:783-792`). Zod v4 schemas for input validation, Ink React UI for rendering (`renderToolUseMessage`, `renderToolResultMessage`), permission checks via `checkPermissions()`. Tool type has 67 fields including progress tracking, search/read collapsing, auto-classifier input, strict mode, and backfill observables.
- **State:** `AppStateStore.ts` manages ~100 reactive fields wrapped in `DeepImmutable<>`. Key fields: settings, MCP clients/tools/commands/resources, tasks, todos per agent, team context (teammates with tmux session/pane IDs), speculation state (pre-execution prediction), bridge state (10+ fields for always-on bridge), file history snapshots, commit attribution, notifications queue, elicitation queue, thinking config, prompt suggestions, skill improvement suggestions, tungsten (tmux) panel state, computer use MCP state, REPL VM context, inbox messages, worker sandbox permissions.
- **Swarms & Delegation:** AgentTool spawns sub-agents in-process (via `createSubagentContext`) or via tmux panes (separate processes with isolated worktrees). TeamCreateTool/TeamDeleteTool manage multi-agent teams. SendMessageTool routes inter-agent messages by name (via `agentNameRegistry` Map). Per-agent color assignment for UI differentiation. `setAppStateForTasks` always reaches root store for session-scoped infrastructure.
- **MCP:** Full Model Context Protocol client supporting 7 transport types: stdio, SSE, HTTP (Streamable), WebSocket, claude.ai proxy, SDK in-process, IDE (SSE/WS). Auth: OAuth 2.0 with ClaudeAuthProvider, token refresh, step-up detection, 15-min auth cache. Features: tool discovery, resource subscription, elicitation handling (-32042 URL elicitations), prompt support. Memoized connections with auto-reconnection on close/error. In-process servers: Chrome MCP and Computer Use MCP (chicago) run in-process.
- **Extensibility:** Skills (slash commands from `/skills/` dirs, bundled, plugin-provided, MCP-fetched), plugins (versioned marketplace with cache directories), hooks (SessionStart, Setup, PreToolUse, PostToolUse, FileChanged, PreCompact, PostCompact), custom agents (`--agents` JSON or `.claude/agents/`), workflows (bundled scripts).

## Entry Points & Core Files
| File | Lines | Purpose |
|------|-------|---------|
| `main.tsx` | 2000+ | CLI entry point. Commander.js parsing with 60+ options. Early prefetches: MDM raw read, keychain prefetch, profile checkpoints. Feature-gated subcommands: DIRECT_CONNECT (cc:// URLs), KAIROS (assistant mode), SSH_REMOTE (ssh host), LODESTONE (deep link URIs). Migrations (11 versions). Trust dialog gating. Parallel setup()/getCommands()/agent loading. Deferred prefetches after first render. |
| `QueryEngine.ts` | 1295 | Conversation lifecycle class. `submitMessage()` async generator yields SDK messages. Handles transcript recording (fire-and-forget in bare mode, awaited otherwise), budget enforcement (USD, turns, structured output retries), snip compaction (HISTORY_SNIP feature), file history snapshots, commit attribution updates, orphaned permission handling. `ask()` convenience wrapper for one-shot usage. |
| `Tool.ts` | 792 | Tool type definition with 67 fields. `buildTool()` factory with 7 defaults (isEnabled→true, isConcurrencySafe→false, isReadOnly→false, isDestructive→false, checkPermissions→allow, toAutoClassifierInput→'', userFacingName→name). ToolPermissionContext with mode, alwaysAllow/Deny/Ask rules, additional working directories. ToolUseContext with 40+ fields including nested memory triggers, dynamic skill dir triggers, discovered skill names, content replacement state, rendered system prompt cache. |
| `tools.ts` | 386 | Tool assembly pipeline. `getAllBaseTools()` returns exhaustive list. `getTools()` applies SIMPLE mode filtering, REPL-only hiding, deny rule filtering, isEnabled() filtering. `assembleToolPool()` merges built-in + MCP tools with deduplication (built-in wins), sorted by name for prompt-cache stability. `getMergedTools()` for complete tool listing. Feature-gated tools: REPLTool, SleepTool, cronTools, RemoteTriggerTool, MonitorTool, WebBrowserTool, WorkflowTool, SnipTool, VerifyPlanExecutionTool, OverflowTestTool, CtxInspectTool, TerminalCaptureTool, WebBrowserTool. |
| `setup.ts` | — | Environment initialization: worktree creation, tmux sessions, UDS messaging socket binding, hooks snapshot capture, terminal backup restoration, plugin prefetch, git status/log/branch spawning. Parallel with commands/agents loading for startup optimization. |
| `commands.ts` | 754 | 80+ slash commands. `getCommands()` loads builtin + skills + plugins + workflows with memoization by cwd. Dynamic skills discovered during file operations. `REMOTE_SAFE_COMMANDS` (13 commands for --remote mode). `BRIDGE_SAFE_COMMANDS` (6 commands for mobile bridge). `isBridgeSafeCommand()` predicate for bridge inbound filtering. `getSkillToolCommands()` and `getSlashCommandToolSkills()` for model-invocable commands. |
| `state/AppStateStore.ts` | 569 | Deep-immutable state store with ~100 fields. SpeculationState for pre-execution prediction (active/idle with mutable refs for messages/writtenPaths/context). Bridge state: 10+ fields (enabled, explicit, outboundOnly, connected, sessionActive, reconnecting, connectUrl, sessionUrl, environmentId, sessionId, error, initialName). Team context with teammate registry (name, agentType, color, tmuxSessionName, tmuxPaneId, cwd, worktreePath). Computer use MCP state (allowedApps, grantFlags, lastScreenshotDims, hiddenDuringTurn, selectedDisplayId, displayPinnedByModel). |
| `services/api/claude.ts` | — | API client: streaming/non-streaming queries, prompt caching (1h TTL with global cache scope), beta headers, tool search, advisor tool, effort params, retry logic with categorization, automatic fallback to alternate models, non-streaming fallback recovery. |
| `services/mcp/client.ts` | — | MCP client: `connectToServer()` with 7 transport types, auth caching, tool/resource/command fetching, connection lifecycle management, error recovery, session expiry detection, elicitation handling. |

## Tool Inventory (40+ Built-in Tools)
| Tool | Summary |
|------|---------|
| **AgentTool** | Spawn sub-agents with custom prompts, models, permission modes. In-process or tmux-based. Supports fork subagents for cache sharing. |
| **BashTool** | Execute shell commands with permission checks, sandbox support, timeout, output truncation. Auto-allow in sandboxed environments. |
| **FileEditTool** | Apply SEARCH/REPLACE diffs to files. Validates patterns, tracks changes, supports multi-edits, custom rejection UI. |
| **FileReadTool** | Read file contents with LRU caching, offset/length limits, binary detection. maxResultSizeChars=Infinity (never persisted). |
| **FileWriteTool** | Write/overwrite file contents. Creates directories, validates paths. |
| **GlobTool** | File pattern matching via fast glob. Returns matching file paths. Collapsed in UI as search operation. |
| **GrepTool** | Content search via ripgrep. Pattern matching across files with context lines. Collapsed in UI as search operation. |
| **WebFetchTool** | Fetch URL content. HTML-to-text conversion, size limits, pre-approved domains. |
| **WebSearchTool** | Web search integration. Returns search results with snippets. Progress UI during search. |
| **TodoWriteTool** | Manage todo lists per agent. Create, update, complete tasks. Updates todo panel (no transcript result). |
| **TaskCreateTool** | Create long-running tasks with descriptions and status tracking (TodoV2 feature). |
| **TaskGetTool** | Retrieve task details and current status. |
| **TaskUpdateTool** | Update task progress, status, and metadata. |
| **TaskListTool** | List all tasks with filtering by status. |
| **TaskStopTool** | Stop/cancel running tasks. |
| **TaskOutputTool** | Retrieve task output and progress. Progress UI with TaskOutputProgress type. |
| **SkillTool** | Invoke skills (specialized slash commands). Dynamic discovery from /skills/ dirs. Progress tracking with SkillToolProgress. |
| **ToolSearchTool** | Deferred tool discovery. Model searches for tools by keyword when tool pool is large. searchHint per tool for matching. |
| **EnterPlanModeTool** | Switch to plan mode for structured planning before execution. Saves pre-plan mode for restoration. |
| **ExitPlanModeTool** | Exit plan mode with verified plan, trigger background verification. AllowedPrompts for session-scoped rules. |
| **AskUserQuestionTool** | Interactive user prompts with multiple choice options. |
| **BriefTool** | Send messages to user in assistant mode (Kairos). Entitled users only. |
| **SleepTool** | Proactive mode: agent sleeps until triggered by events. isEnabled() gates on isProactiveActive(). |
| **MCPTool** | Generic MCP tool wrapper. Calls tools from any connected MCP server. MCPProgress tracking. |
| **ListMcpResourcesTool** | List available resources from MCP servers. |
| **ReadMcpResourceTool** | Read specific resources from MCP servers. |
| **NotebookEditTool** | Edit Jupyter notebooks. Cell-level operations, JSON structure manipulation. |
| **LSPTool** | Language Server Protocol integration. Diagnostics, completions, hover. Enabled via ENABLE_LSP_TOOL env. |
| **ConfigTool** | Read/modify Claude Code settings (ant-only). |
| **SendMessageTool** | Send messages between agents in a team/swarm. Routes via agentNameRegistry. |
| **TeamCreateTool** | Create multi-agent teams with tmux-based spawning. Lazy-loaded to break circular deps. |
| **TeamDeleteTool** | Remove agents from teams. Lazy-loaded. |
| **EnterWorktreeTool** | Create and enter git worktrees for isolated work. Enabled when worktree mode active. |
| **ExitWorktreeTool** | Exit worktree and return to main branch. |
| **REPLTool** | VM-based tool execution environment (ant-only). Wraps primitive tools. REPL VM context persists in AppState. |
| **PowerShellTool** | Windows PowerShell execution (platform-specific). Enabled via isPowerShellToolEnabled(). |
| **SyntheticOutputTool** | Structured JSON output validation against user-provided schema. Added after getTools() filtering. |
| **CronCreate/Delete/ListTool** | Schedule recurring agent tasks (AGENT_TRIGGERS feature). |
| **RemoteTriggerTool** | Remote-triggered agent activation (AGENT_TRIGGERS_REMOTE feature). |
| **WorkflowTool** | Execute bundled workflow scripts. Initializes bundled workflows on load. |
| **VerifyPlanExecutionTool** | Background plan verification (CLAUDE_CODE_VERIFY_PLAN env). |
| **WebBrowserTool** | Browser automation (WEB_BROWSER_TOOL feature). Pill in footer with URL display. |
| **TerminalCaptureTool** | Terminal panel capture (TERMINAL_PANEL feature). |
| **CtxInspectTool** | Context inspection (CONTEXT_COLLAPSE feature). |
| **SnipTool** | History snip compaction (HISTORY_SNIP feature). |
| **ListPeersTool** | UDS inbox peer listing (UDS_INBOX feature). |
| **OverflowTestTool** | Testing tool (OVERFLOW_TEST_TOOL feature). |

## Slash Commands (80+)
**Session Management:** `/clear`, `/compact`, `/resume`, `/rename`, `/exit`, `/fork`
**Configuration:** `/config`, `/model`, `/effort`, `/fast`, `/plan`, `/theme`, `/color`, `/vim`, `/keybindings`, `/statusline`, `/output-style`, `/permissions`, `/privacy-settings`, `/sandbox`
**MCP:** `/mcp` (add/remove/list/auth servers), supports XAA IDP login
**Plugins:** `/plugin` (install/list/enable/disable), `/reload-plugins`
**Skills:** `/skills` (list/search), `/init` (initialize project)
**Git/PR:** `/review`, `/ultrareview`, `/pr_comments`, `/commit`, `/diff`, `/branch`, `/autofix-pr`, `/subscribe-pr`, `/security-review`
**Remote/Bridge:** `/remote-control`, `/bridge`, `/teleport`, `/mobile`, `/desktop`, `/session`, `/share`, `/remote-env`
**Diagnostics:** `/doctor`, `/cost`, `/usage`, `/stats`, `/status`, `/files`, `/context`, `/ctx_viz`, `/heapdump`, `/ant-trace`, `/perf-issue`, `/debug-tool-call`
**Memory:** `/memory` (view/edit session memory), `/rewind` (undo changes), `/thinkback`, `/thinkback-play`
**Misc:** `/help`, `/btw` (quick note), `/feedback`, `/copy`, `/insights`, `/upgrade`, `/release-notes`, `/stickers`, `/hooks`, `/env`, `/rate-limit-options`, `/passes`, `/summary`, `/tag`, `/export`, `/add-dir`
**Internal (ant-only):** `/backfill-sessions`, `/break-cache`, `/bughunter`, `/good-claude`, `/issue`, `/init-verifiers`, `/mock-limits`, `/bridge-kick`, `/version`, `/reset-limits`, `/onboarding`, `/oauth-refresh`, `/agents-platform`
**Feature-gated:** `/proactive` (PROACTIVE/KAIROS), `/brief` (KAIROS_BRIEF), `/assistant` (KAIROS), `/voice` (VOICE_MODE), `/workflows` (WORKFLOW_SCRIPTS), `/torch` (TORCH), `/peers` (UDS_INBOX), `/buddy` (BUDDY), `/ultraplan` (ULTRAPLAN), `/force-snip` (HISTORY_SNIP), `/web` (CCR_REMOTE_SETUP)

## Key Subsystems

### Permission System (`utils/permissions/`)
- **Modes:** default, plan, auto, bypassPermissions. CLI parsing via `initialPermissionModeFromCLI()`.
- **Rules:** alwaysAllow, alwaysDeny, alwaysAsk — per-tool with pattern matching (e.g., `Bash(git *)`). Sources tracked per rule.
- **Auto Mode:** Transcript classifier (TRANSCRIPT_CLASSIFIER feature) analyzes tool calls for safety via `toAutoClassifierInput()` on each tool. Falls back to prompting when denial threshold exceeded. `DenialTrackingState` for local tracking in async subagents.
- **Sandbox:** Docker/container detection via `SandboxManager`, unsandboxed command allowlists, bubblewrap support. `areUnsandboxedCommandsAllowed()`, `isAutoAllowBashIfSandboxedEnabled()`.
- **Dangerous permission stripping:** Bash(*), PowerShell(*) stripped for ant users and auto mode via `stripDangerousPermissionsForAutoMode()`.
- **Permission context:** `ToolPermissionContext` with mode, rules by source, additional working directories, plan mode override, avoid prompts flag, await automated checks flag.
- **Validation:** `validateInput()` on tools before permission checks. `checkPermissions()` for tool-specific logic. General permission system in `permissions.ts`.

### MCP Integration (`services/mcp/`)
- **Transports:** stdio, SSE, HTTP (Streamable), WebSocket, claude.ai proxy, SDK in-process, IDE (SSE/WS). Connection via `connectToServer()`.
- **Auth:** OAuth 2.0 with ClaudeAuthProvider, token refresh, step-up detection, 15-min auth cache. `McpAuthError`, `McpSessionExpiredError` custom errors.
- **Features:** Tool discovery, resource subscription, elicitation handling (URL elicitations via -32042 errors), prompt support. `excludeCommandsByServer()`, `excludeResourcesByServer()` for filtering.
- **Connection management:** Memoized connections, auto-reconnection on close/error, session expiry detection. `clearServerCache()` for invalidation.
- **In-process servers:** Chrome MCP and Computer Use MCP (chicago) run in-process via SDK transport to avoid subprocess overhead.
- **Config parsing:** `parseMcpConfig()`, `parseMcpConfigFromFilePath()` with validation, variable expansion, scope tracking (user/project/local/dynamic). Enterprise MCP config enforcement.
- **claude.ai proxy:** `fetchClaudeAIMcpConfigsIfEligible()` for proxy servers (datadog, Gmail, Slack, BigQuery, PubMed). Policy filtering applied.
- **XAA IDP:** Enterprise identity provider integration via `registerMcpXaaIdpCommand()`. `isXaaEnabled()` gate.

### Agent Swarms (`tools/AgentTool/`, `utils/swarm/`)
- **In-process subagents:** Spawned within same process via `createSubagentContext`. Shares tool permission context, cloned file state cache, content replacement state. `setAppState` is no-op for async agents; `setAppStateForTasks` reaches root store.
- **Tmux-based teammates:** Separate processes in tmux panes with isolated worktrees. `TEAMMATE_SYSTEM_PROMPT_ADDENDUM` appended for identity. Dynamic team context via `setDynamicTeamContext()`.
- **Team management:** TeamCreateTool/TeamDeleteTool for dynamic team composition. `teamContext` in AppState with teammates map (name, agentType, color, tmuxSessionName, tmuxPaneId, cwd, worktreePath, spawnedAt).
- **Message routing:** SendMessageTool with name-based agent registry (`agentNameRegistry: Map<string, AgentId>`). Latest-wins on collision.
- **Color management:** Per-agent color assignment via `agentColorManager`. `standaloneAgentContext` for non-swarm sessions with custom name/color.
- **Plan mode required:** Teammates spawned with `plan_mode_required` start in plan mode. `isPlanModeRequired()` check in teammate utils.
- **Fork subagents:** Fork subagents (FORK_SUBAGENT feature) share parent's prompt cache via `renderedSystemPrompt` to avoid cache busting from GrowthBook cold→warm transitions.
- **Tool result preservation:** `preserveToolUseResults` for in-process teammates whose transcripts are viewable by user.

### Remote Control / Bridge (`bridge/`, `commands/bridge/`)
- **Bidirectional bridge:** WebSocket connection to claude.ai for remote session control. `/remote-control` command activates.
- **Outbound-only mode:** `replBridgeOutboundOnly` — forward events without accepting inbound prompts/control.
- **Command filtering:** `BRIDGE_SAFE_COMMANDS` (6 local commands: compact, clear, cost, summary, releaseNotes, files) and `REMOTE_SAFE_COMMANDS` (13 TUI-only commands: session, exit, clear, help, theme, color, vim, cost, usage, copy, btw, feedback, plan, keybindings, statusline, stickers, mobile).
- **Permission callbacks:** `BridgePermissionCallbacks` for remote permission prompts. Stored in AppState.
- **Channel support:** `ChannelPermissionCallbacks` — permission prompts over Telegram, iMessage, etc. via `--channels` flag. Plugin-kind entries hit marketplace verification + GrowthBook allowlist; server-kind always fails unless dev flag set.
- **Always-on bridge state:** 10+ fields in AppState: enabled, explicit, outboundOnly, connected, sessionActive, reconnecting, connectUrl, sessionUrl, environmentId, sessionId, error, initialName, showRemoteCallout.
- **Deep link URIs:** LODESTONE feature handles `cc://` and `cc+unix://` URLs via `--handle-uri` flag and macOS URL scheme launch.
- **Direct Connect:** DIRECT_CONNECT feature for `claude open <url>` with auth token parsing.

### Skills System (`skills/`, `commands/skills/`)
- **Bundled skills:** Shipped with Claude Code, registered synchronously at startup via `initBundledSkills()`.
- **Directory skills:** Loaded from `.claude/skills/` and project `/skills/` dirs via `getSkillDirCommands()`.
- **Plugin skills:** Provided by installed plugins via `getPluginSkills()`.
- **MCP skills:** Fetched from MCP servers (MCP_SKILLS feature) via `getMcpSkillCommands()`.
- **Dynamic discovery:** Skills discovered during file operations via `getDynamicSkills()`. Deduped against base commands.
- **Skill improvement:** AI-generated suggestions for skill updates stored in `AppState.skillImprovement`.
- **Skill change detection:** `skillChangeDetector` initialized after first render (deferred from setup).
- **Skill telemetry:** `logSkillsLoaded()` per session with context window size.
- **Skill search:** Local search index with `clearSkillIndexCache()`. EXPERIMENTAL_SKILL_SEARCH feature.
- **SkillTool commands:** `getSkillToolCommands()` returns all prompt-based commands the model can invoke (skills + plugin/MCP commands with descriptions).

### Plugin System (`utils/plugins/`, `commands/plugin/`)
- **Versioned plugins:** Installed to versioned cache directories. `initializeVersionedPlugins()` at startup.
- **Marketplace:** Plugin installation from registry via `/plugin install`. Installation status tracked in AppState (pending/installing/installed/failed).
- **Plugin hooks:** Pre/post tool use, session start, file changed. Hot-reload on settings changes via `pluginReconnectKey` increment.
- **Plugin commands:** Loaded via `getPluginCommands()`. Commands and skills from plugins.
- **Managed plugins:** Enterprise-managed plugin deployments via `getManagedPluginNames()`.
- **Plugin directories:** Seed dirs via `getPluginSeedDirs()`. Inline plugins via `--plugin-dir` flag.
- **Plugin cache:** `cleanupOrphanedPluginVersionsInBackground()` for housekeeping. `clearPluginCache()` for invalidation.
- **Plugin telemetry:** `logPluginLoadErrors()`, `logPluginsEnabledForSession()` per session.
- **Bundled plugins:** `initBuiltinPlugins()` at startup. Built-in plugin skill commands via `getBuiltinPluginSkillCommands()`.

### Hooks System (`utils/hooks/`, `hooks/`)
- **SessionStart:** Run at session initialization. `processSessionStartHooks()`.
- **Setup:** Run with init/maintenance triggers. `processSetupHooks()`.
- **PreToolUse/PostToolUse:** Intercept tool execution. `if` conditions with permission-rule pattern matching via `preparePermissionMatcher()`.
- **FileChanged:** Watch for file system changes.
- **PreCompact/PostCompact:** Run before/after context compaction. Progress events tracked via `onCompactProgress`.
- **Hook config snapshot:** Captured at setup to prevent hidden modifications. `setAllHookEventsEnabled()` for SDK mode.
- **Hook events:** SessionStart, Setup, PreToolUse, PostToolUse, FileChanged, PreCompact, PostCompact, PreSampling, PostSampling.
- **Structured output enforcement:** `registerStructuredOutputEnforcement()` via hooks for JSON schema validation.

### Context Management
- **Prompt caching:** Ephemeral cache with 1h TTL option (GrowthBook-gated). Global cache scope for system prompt. Cache breakpoints after last built-in tool.
- **Micro-compact:** Cached micro-compaction with cache editing (CACHED_MICROCOMPACT feature).
- **Context collapse:** HISTORY_SNIP feature with snip boundaries and projection. `snipCompactIfNeeded()` with force option. `SnipTool` for manual snipping.
- **Memory files:** `.claude/CLAUDE.md` and nested memory attachments. `loadMemoryPrompt()` for memory mechanics. `loadedNestedMemoryPaths` dedup set to prevent re-injection.
- **Team memory sync:** Watcher for team memory synchronization.
- **Memory directory override:** `CLAUDE_COWORK_MEMORY_PATH_OVERRIDE` env for custom memory paths.

### TUI / Rendering (`ink/`)
- **Ink framework:** React-based terminal UI. `launchRepl()` renders root component.
- **Layout engine:** Yoga-based flexbox layout for responsive terminal UI.
- **Components:** Box, Text, Button, ScrollBox, Link, Spacer, RawAnsi, Ansi. Spinner with tips.
- **Terminal handling:** Keypress parsing, ANSI escape sequences, bidi text support, hyperlinks. `SHOW_CURSOR` DEC control.
- **Optimization:** Node caching, frame rendering, reconciler. `isResultTruncated()` gates click-to-expand.
- **Events:** Input, click, focus, keyboard, terminal events. Early input seeding via `seedEarlyInput()`.
- **Search highlighting:** Transcript search with highlight overlay. `extractSearchText()` per tool for indexing fidelity.
- **Transcript search:** `transcriptSearch.ts` with render fidelity testing. Count ≡ highlight enforcement.
- **Message selection:** `MessageSelector` for filtering selectable messages. `selectableUserMessagesFilter` excludes synthetic caveats and task notifications.
- **Expanded views:** Tasks, teammates expandable panels. Footer item navigation (tasks, tmux, bagel, teams, bridge, companion).

### Vim Mode (`vim/`)
- **Motions:** h, j, k, l, w, b, e, 0, $, etc.
- **Operators:** d, c, y, >, <, etc.
- **Text objects:** w, p, s, t, quotes, brackets
- **Transitions:** State machine for mode transitions

### Speculation (Pre-execution Prediction)
- **Active speculation:** `SpeculationState` with mutable refs for messages, written paths, context. Pre-executes tool calls to predict outcomes.
- **Pipelined suggestions:** `pipelinedSuggestion` with text, promptId (user_intent/stated_intent), generationRequestId.
- **Time tracking:** `speculationSessionTimeSavedMs` cumulative across session.
- **Boundary tracking:** CompletionBoundary types (complete, bash, edit, denied_tool).

### Telemetry & Analytics (`services/analytics/`, `utils/telemetry/`)
- **GrowthBook:** Feature flag system with cached values. `getFeatureValue_CACHED_MAY_BE_STALE` for synchronous reads. `initializeGrowthBook()` for async refresh.
- **Event logging:** Strict metadata typing via `AnalyticsMetadata_I_VERIFIED_THIS_IS_NOT_CODE_OR_FILEPATHS`. PII-safe.
- **Session tracing:** LLM request spans with context tracking. `profileCheckpoint()` / `profileReport()` for startup profiling.
- **Startup profiling:** Checkpoint-based profiling at key points: main_tsx_entry, main_tsx_imports_loaded, main_client_type_determined, run_function_start, run_commander_initialized, preAction_start, preAction_after_mdm, preAction_after_init, preAction_after_sinks, preAction_after_migrations, preAction_after_remote_settings, preAction_after_settings_sync, action_handler_start, action_after_input_prompt, action_tools_loaded, action_before_setup, action_after_setup, action_commands_loaded.
- **Headless profiling:** Separate `headlessProfilerCheckpoint` for SDK/print mode. Checkpoints: before_getSystemPrompt, after_getSystemPrompt, before_skills_plugins, after_skills_plugins, system_message_yielded.
- **Plugin telemetry:** `logPluginLoadErrors()`, `logPluginsEnabledForSession()` per session.
- **Skill telemetry:** `logSkillsLoaded()` per session with context window size.
- **Startup telemetry:** `tengu_startup_telemetry` event with git status, worktree count, GitHub auth status, sandbox enabled, unsandboxed commands allowed, auto updater disabled, prefers reduced motion, cert env vars.
- **Managed settings logging:** `tengu_managed_settings_loaded` with key count and names.

### Session Management
- **Transcript recording:** JSONL format with UUID tracking. `recordTranscript()` with fire-and-forget in bare mode. `flushSessionStorage()` for eager flush.
- **Session persistence:** Save/resume across process restarts. `loadTranscriptFromFile()`, `processResumedConversation()`.
- **File history:** Snapshots of file state at each user message. `FileHistoryState` with snapshots, tracked files, snapshot sequence. `fileHistoryMakeSnapshot()` per message.
- **Commit attribution:** Track which agent made which changes. `AttributionState` in AppState. `createEmptyAttributionState()`.
- **Session recovery:** Load interrupted conversations via `loadConversationForResume()`.
- **Concurrent sessions:** Track and name multiple sessions. `countConcurrentSessions()`, `registerSession()`, `updateSessionName()`.
- **Session titles:** `cacheSessionTitle()` for --name arg. `searchSessionsByCustomTitle()` for resume picker.
- **Session ID:** Custom UUIDs via `--session-id`. Validation via `validateUuid()`. `asSessionId()` type branding.

### Model Support
- **Providers:** Anthropic API, AWS Bedrock (`CLAUDE_CODE_USE_BEDROCK`), GCP Vertex (`CLAUDE_CODE_USE_VERTEX`), Foundry.
- **Models:** Sonnet, Opus, Haiku with alias resolution. `parseUserSpecifiedModel()`, `normalizeModelStringForAPI()`. Ant model aliases via `tengu_ant_model_override` GrowthBook flag.
- **Features:** Adaptive thinking, extended thinking, fast mode, effort levels (low/medium/high/max). `shouldEnableThinkingByDefault()`.
- **Beta headers:** Tool search, advisor, context management, cache editing, structured outputs, task budgets, AFK mode. `getSdkBetas()`, `filterAllowedSdkBetas()`.
- **Fallback:** Automatic fallback to alternate models on failure. `--fallback-model` CLI option.
- **Non-streaming fallback:** Recovery from streaming failures.
- **Model deprecation:** `getModelDeprecationWarning()` for outdated model alerts.
- **Context windows:** `getContextWindowForModel()` with SDK betas consideration.
- **Advisor tool:** Server-side advisor model for query optimization. `canUserConfigureAdvisor()`, `isValidAdvisorModel()`, `modelSupportsAdvisor()`.
- **Effort params:** `parseEffortValue()`, `getInitialEffortSetting()`.

## CLI Options (Key)
`--print/-p`: Print response and exit (SDK mode). Trust dialog skipped.
`--continue/-c`: Continue most recent conversation in current directory.
`--resume [uuid]`: Resume by session ID or interactive picker with search term.
`--model <model>`: Set model (alias or full name, e.g., 'sonnet', 'claude-sonnet-4-6').
`--permission-mode <mode>`: default/plan/auto/bypassPermissions.
`--dangerously-skip-permissions`: Skip all permission prompts (sandbox-only). Blocked for root/sudo outside sandbox.
`--worktree [name]`: Create isolated git worktree. PR references supported (#N or GitHub URL).
`--tmux`: Create tmux session with worktree. Requires tmux installed.
`--remote [description]`: Remote session mode.
`--remote-control/--rc`: Enable bridge for remote control from claude.ai.
`--chrome`: Enable Claude in Chrome integration. Subscriber check enforced.
`--mcp-config <files>`: Load MCP servers from JSON files or strings.
`--strict-mcp-config`: Only use --mcp-config servers, ignoring all other MCP configurations.
`--settings <file-or-json>`: Load settings from file path or JSON string. Content-hash-based temp paths for cache stability.
`--agents <json>`: Define custom agents as JSON object.
`--add-dir <dirs>`: Additional directories for CLAUDE.md access.
`--output-format`: text/json/stream-json (only with --print).
`--input-format`: text/stream-json (stream-json requires output-format=stream-json).
`--max-budget-usd`: USD cost limit. Positive number > 0.
`--json-schema <schema>`: Structured output validation with JSON Schema.
`--betas <betas>`: API beta headers for API key users.
`--plugin-dir <path>`: Load plugins from directory (repeatable).
`--file <specs>`: Download file resources at startup. Format: file_id:relative_path.
`--session-id <uuid>`: Custom session ID (valid UUID). Can be used with --continue/--resume + --fork-session.
`--name <name>`: Set display name for session (shown in /resume and terminal title).
`--effort <level>`: low/medium/high/max.
`--bare`: Minimal mode. Sets CLAUDE_CODE_SIMPLE=1. Strictly ANTHROPIC_API_KEY auth. Skips hooks, LSP, plugins, CLAUDE.md, auto-discovered MCP.
`--fork`: Fork current session (FORK_SUBAGENT feature).
`--teleport [value]`: Teleport to remote session.
`--from-pr [value]`: Resume session linked to PR.
`--no-session-persistence`: Disable session persistence (print mode only).
`--replay-user-messages`: Re-emit user messages from stdin back on stdout (stream-json only).
`--include-partial-messages`: Include partial messages in output stream (print + stream-json only).
`--allowedTools/--disallowedTools`: Comma or space-separated tool name lists.
`--tools <tools>`: Specify available tools from built-in set. "" to disable all, "default" for all.
`--fallback-model <model>`: Enable automatic fallback when default model is overloaded.
`--system-prompt/--append-system-prompt`: Override or append to system prompt.
`--permission-prompt-tool <tool>`: MCP tool for permission prompts (print mode only).
`--task-budget <tokens>`: API-side task budget in tokens (hidden).
`--workload <tag>`: Workload tag for billing-header attribution (hidden, print only).
`--ide`: Automatically connect to IDE on startup if exactly one valid IDE available.
`--disable-slash-commands`: Disable all skills.
`--debug [filter]`: Enable debug mode with optional category filtering.
`--verbose`: Override verbose mode from config.
`--init/--init-only/--maintenance`: Run hooks with specific triggers.

## Configuration Files
- **Global:** `~/.claude.json` — user-level settings, migration version, last release notes, cached GrowthBook features, tungsten panel visibility, verbose flag, auto-updater disabled, theme, prefers reduced motion, remote control at startup.
- **Project:** `.claude/settings.json` — project-specific settings, models, permission defaults, defaultMode, assistant mode.
- **Local:** `.claude/settings.local.json` — machine-specific overrides.
- **Instructions:** `CLAUDE.md`, `CLAUDE.local.md` — project-specific instructions loaded as system prompt context.
- **Agents:** `.claude/agents/` — custom agent definitions with agentType, description, prompt, initialPrompt, model, permissionMode.
- **Skills:** `.claude/skills/`, `/skills/` — skill definitions loaded as slash commands.
- **Memory:** `.sapphire-memory/` — agent memory and rollout summaries.
- **Hooks:** `.claude/hooks/` — pre/post tool use scripts.
- **MCP:** `.mcp.json`, `.claude/mcp.json` — MCP server configurations with scopes (user/project/local).
- **Settings sources:** user, project, local, policySettings, flagSettings, managedSettings, remoteManagedSettings.

## Build / Run / Test
- **Runtime:** Bun (primary), Node.js 18+ (compatible). `isRunningWithBun()` check.
- **Run:** `bun run ./main.tsx` or compiled binary `claude`.
- **Entry:** `main.tsx` → `run()` → Commander program → `launchRepl()` or `runHeadless()`.
- **Feature flags:** `feature('FLAG')` from `bun:bundle` for dead code elimination. Checked at module load time for conditional imports.
- **Conditional imports:** `feature('X') ? require(...) : null` pattern for tree-shaking. Bundler strips unused code in external builds.
- **Build modes:** `isInBundledMode()` for compiled binary detection. External build eliminates internal-only commands and tools.
- **Bundled mode:** Skills, plugins, workflows bundled into binary. `isInBundledMode()` gate.

## Key Patterns & Conventions
- **Dead code elimination:** `feature('FLAG') ? require(...) : null` — bundler strips unused code. eslint-disable `custom-rules/no-process-env-top-level` for feature-gated requires.
- **Lazy requires:** `require()` inside functions to break circular dependencies. Pattern: `const getX = () => require('./x.js').X`. Used for teammate modules, TeamCreateTool, TeamDeleteTool, SendMessageTool, PowerShellTool.
- **Feature gating:** GrowthBook flags (`getFeatureValue_CACHED_MAY_BE_STALE`) + env vars. Dual gating: feature flag for code elimination, GrowthBook for runtime control.
- **Memoization:** lodash-es/memoize for expensive operations. `getCommands()`, `getTools()`, `COMMANDS()`, `builtInCommandNames()`, `getSkillToolCommands()`, `getSlashCommandToolSkills()`, `loadAllCommands()`. Cache invalidation via `clearCommandsCache()`, `clearPluginCommandCache()`, `clearSkillCaches()`.
- **Immutability:** `DeepImmutable<>` type wrapper on AppState. Functional state updates: `setAppState(prev => ({ ...prev, field: newValue }))`.
- **Async generators:** SDK messages yielded via `async*` generators. `submitMessage()` yields SDKMessage variants.
- **Cleanup registry:** `registerCleanup()` for graceful shutdown. `gracefulShutdown()`, `gracefulShutdownSync()`.
- **Error classes:** Custom errors: `McpAuthError`, `McpSessionExpiredError`, `TeleportOperationError`, `ShellError`, `AbortError`, `DirectConnectError`.
- **Path expansion:** `expandPath()` for `~` and normalization. `safeResolvePath()` with fs implementation abstraction.
- **JSONL storage:** History, transcripts, and session files use JSONL format. `recordTranscript()` writes message-by-message.
- **Prompt cache stability:** Tool descriptions sorted by name. Built-in tools as contiguous prefix before MCP tools. Content-hash-based temp file paths (settings) to avoid cache busting.
- **Profile checkpoints:** `profileCheckpoint()` marks entry points. `profileReport()` generates startup profile. Headless profiler separate for SDK mode.
- **Early input seeding:** `seedEarlyInput()` captures user typing during startup. `stopCapturingEarlyInput()` for non-interactive modes.

## Security Considerations
- **Trust dialog:** Required before executing code in interactive mode. `checkHasTrustDialogAccepted()`. Skipped in -p mode (documented as "only use in directories you trust").
- **Sandbox detection:** Docker, bubblewrap, `IS_SANDBOX` env var. `SandboxManager.isSandboxingEnabled()`, `SandboxManager.areUnsandboxedCommandsAllowed()`.
- **Root protection:** --dangerously-skip-permissions blocked for root/sudo outside sandbox.
- **Path validation:** Tools validate and sanitize file paths. `safeResolvePath()` prevents path traversal.
- **Permission rules:** Fine-grained tool-level permissions with pattern matching. Sources tracked (user, project, local, policy).
- **Enterprise policies:** MDM settings (`startMdmRawRead()`), remote managed settings, policy limits (`loadPolicyLimits()`, `isPolicyAllowed()`). MCP server allowlists/denylists.
- **MCP auth:** OAuth with token refresh, step-up detection, auth cache. `ensureKeychainPrefetchCompleted()` for startup optimization.
- **System context prefetch gating:** Git commands (which can execute arbitrary code via hooks) only run after trust established or in non-interactive mode. `prefetchSystemContextIfSafe()`.
- **Settings from untrusted sources:** `.claude/settings.json` is attacker-controllable in untrusted clones. Assistant mode refuses to activate until directory explicitly trusted.
- **Windows security:** `NoDefaultCurrentDirectoryInExePath=1` prevents PATH hijacking.
- **Debug mode exit:** External builds exit immediately if debugger detected (`isBeingDebugged()`).

## Environment Variables (Key)
**Authentication:**
`ANTHROPIC_API_KEY`: Direct API authentication
`ANTHROPIC_BASE_URL`: Custom API endpoint
`CLAUDE_CODE_SESSION_ACCESS_TOKEN`: Session ingress token for file downloads

**Providers:**
`CLAUDE_CODE_USE_BEDROCK`: Enable AWS Bedrock provider
`CLAUDE_CODE_USE_VERTEX`: Enable GCP Vertex provider
`CLAUDE_CODE_SKIP_BEDROCK_AUTH`: Skip Bedrock auth prefetch
`CLAUDE_CODE_SKIP_VERTEX_AUTH`: Skip Vertex auth prefetch

**Entrypoint & Mode:**
`CLAUDE_CODE_ENTRYPOINT`: cli/sdk-cli/sdk-ts/sdk-py/remote/local-agent/mcp/claude-code-github-action/claude-vscode/claude-desktop
`CLAUDE_CODE_SIMPLE`: Enable bare/simple mode
`CLAUDE_CODE_REMOTE`: Enable remote mode
`CLAUDE_CODE_ENVIRONMENT_KIND`: 'bridge' for remote-control sessions
`CLAUDE_CODE_TASK_LIST_ID`: Task list identifier (ant-only)
`CLAUDE_CODE_AGENT`: Agent name from BG_SESSIONS feature

**Feature Flags:**
`CLAUDE_CODE_COORDINATOR_MODE`: Enable coordinator mode
`CLAUDE_CODE_VERIFY_PLAN`: Enable plan verification
`USER_TYPE`: 'ant' for internal builds (enables REPLTool, ConfigTool, internal commands)
`IS_DEMO`: Demo mode (hides internal commands)

**Behavior:**
`CLAUDE_CODE_EXTRA_BODY`: Additional API request parameters
`CLAUDE_CODE_EXTRA_METADATA`: Additional API metadata
`CLAUDE_CODE_EAGER_FLUSH`: Force immediate transcript flush
`CLAUDE_CODE_DISABLE_THINKING`: Disable model thinking
`CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING`: Force budget thinking
`CLAUDE_CODE_SHELL_PREFIX`: Override shell command prefix
`CLAUDE_CODE_TASK_TIMEOUT`: Task timeout override
`CLAUDE_CODE_SKIP_PROMPT_HISTORY`: Skip command history
`CLAUDE_CODE_INCLUDE_PARTIAL_MESSAGES`: Include partial messages in SDK output
`CLAUDE_CODE_EXIT_AFTER_FIRST_RENDER`: Exit after first render (benchmarking)
`CLAUDE_CODE_IS_COWORK`: Coworker mode (triggers eager flush)
`CLAUDE_CODE_SIMPLE`: Bare mode alias
`CLAUDE_CODE_DISABLE_TERMINAL_TITLE`: Disable process.title setting

**MCP:**
`MCP_TOOL_TIMEOUT`: MCP tool call timeout
`MCP_TIMEOUT`: MCP connection timeout
`CLAUDE_CODE_SYNC_PLUGIN_INSTALL`: Sync plugin installation mode

**API:**
`API_TIMEOUT_MS`: API request timeout
`MAX_STRUCTURED_OUTPUT_RETRIES`: Max retries for structured output (default: 5)

**Memory:**
`CLAUDE_COWORK_MEMORY_PATH_OVERRIDE`: Custom memory directory path

**LSP:**
`ENABLE_LSP_TOOL`: Enable LSP tool integration

## Migrations
11 migration versions covering: auto-updates to settings, bypass-permissions to settings, enable-all-project-MCP-servers to settings, pro-to-opus default, sonnet-1m to sonnet-45, legacy-opus to current, sonnet-45 to sonnet-46, opus to opus-1m, repl-bridge to remote-control, auto-mode opt-in reset, fennec-to-opus (ant-only). Async changelog migration runs fire-and-forget.
