# Memory Status Report

## internal/memory/system.go
- `NewSystem` spins up the persistent memory stack (store + pipeline + Gemini extractor) only when `Config.APIKey` is present so memory remains opt-in; errors in store/extractor construction disable memory but leave the rest of the session running.
- `PushToolResult` feeds every tool invocation into the pipeline as `ExtractionEvent` (session/turn/tool name + combined input/output) so extraction is asynchronous and never blocks the agent.
- Checkpoint helpers (`ShouldRunCheckpoint`, `MarkCheckpointDone`, `ResetCheckpointState`, `RunPreCompactionCheckpoint`) let the agent trigger synchronous extraction + checkpoint writes before compaction cycles, and `BuildContextInjection` serializes all persisted content in the fixed order the prompts expect: constitution, negative constraints, top-K salience-ranked records, latest checkpoint snippet.

## internal/memory/store.go
- Each session gets `dataDir/memory/<projectScopeHash>_<sessionPrefix>.db` (project hash + first 8 session characters) so multiple sessions/projects keep clean isolation, and the schema pairs `memory_records`, a FTS5 index, `project_constitution`, and `compaction_checkpoints`.
- `WriteRecord` deduplicates by `(sessionID, turnIndex, eventType)` via `dedup_hash`, while `QueryRecords` fetches up to `limit*3` rows, applies exponential decay (`salience * exp(-0.05 * hours)`), keeps negative constraints/architectural decisions at full score, and sorts by decayed salience before trimming to the requested window.
- `SearchFTS` backs `recall_memory` queries, `GetNegativeConstraints` always returns the non-decaying constraints injected first, and `GetConstitution`/`UpsertConstitution` keep the project-level guard rails shared between tiered memory and `pmem.BuildContextInjection`.
- `WriteCheckpoint`/`GetLatestCheckpoint` drive the compaction snapshot that gets injected as `<persistent_memory_checkpoint>` when available, and `MarshalRecordsJSON` formats the record list for prompt ingestion.

## internal/memory/pipeline.go
- Pipeline queue parameters: size 256, `batchWindow` 500 ms, `maxBatchSize` 5, `maxRetries` 1, `retryBackoff` 2 s; `Push` drops events (with a log) when the buffer is full, guaranteeing the agent thread never blocks.
- Worker batches raw sources, retries extraction once, and on failure writes raw fallback records tagged `raw_<eventType>` with a salience derived from `salienceForEventType` (errors 0.9, file edits 0.7, etc.).
- Successful extraction converts to structured records via `ResultToRecords`, writes them, and calls `maybeUpdateConstitution` so any new architectural decisions get appended (and capped at ~2048 chars).
- `BuildCheckpointJSON` serializes the most recent 50 decayed records per session for the checkpoint injection block that `System` later writes to disk.
- `ExtractSync` lets the compaction path force single-turn extraction + record writes before checkpointing (used in `RunPreCompactionCheckpoint`).

## internal/memory/extraction.go
- `Extractor` wraps the Gemini GenAI client (`thoughts` locked to low, `temperature` 0.1, `max_output_tokens` 4096) with a fixed system prompt that demands pure JSON matching the `ExtractionResult` schema (architectural decisions, files modified, failures, negative constraints, task progress, codebase discoveries).
- `Extract` strips markdown fences, unmarshals the JSON, validates every path against the workspace (`validateFilePaths`) to prevent hallucinated files, and returns the structured result.
- `ResultToRecords` maps each field to a `MemoryRecord` with deterministic salience (e.g., 0.95 for architecture, 1.0 for negative constraints, importance-based boosts for discoveries) and copies truncated raw context so later reasoning sees provenance.

## internal/memory/tools.go
- `recall_memory` (used when `pmem` is non-nil) defaults to limit 5, caps at 20, accepts `filter` (`all`, `negative_constraints`, `architectural`, `failures`, `progress`), prefers FTS when a query is provided, and falls back to `QueryRecords`; returns `MarshalRecordsJSON` so the agent sees structured entries inside the prompt.
- `save_memory` lets the agent write critical facts synchronously (salience 1.0, turn index 0) for the official event types listed by the pipeline, validating the event type and wrapping non-JSON input inside a JSON object before `WriteRecord`.
- `timeNowUnix` defers to the package-level `timeNow` helper so tests can freeze clocks when verifying the synchronous save path.

## internal/memory/open_modernc.go & internal/memory/open_ncruces.go
- Platform-specific sqlite drivers choose either `modernc.org/sqlite` (WASM-friendly builds) or `ncruces/go-sqlite3` depending on the build tags, but both enforce WAL mode, `NORMAL` synchronous, and a 5 s busy timeout so memory writes stay fast without locks.

## internal/agent/coordinator.go
- `NewCoordinator` creates the agent-level `memory.MemoryService` (db-backed structured summaries, code knowledge, constitution) and also spins up `pmem.NewSystem` when a Gemini API key can be resolved; the resulting `pmem` pointer is nil when memory is disabled so agent startups remain graceful.
- `buildAgent` injects both `MemoryService` (shared structured memory) and `pmem.System` (persistent pipeline) into `SessionAgent`, and `buildTools` unconditionally adds the `memory_query` tool plus (when `pmem` exists) the `recall_memory` + `save_memory` toolset so the model can reach cold memory and the structured store.

## internal/agent/agent.go
- `Summarize` runs both a narrative summary (for UI) and a structured extraction summary; the structured JSON is unmarshaled into `memory.StructuredSummaryData` and persisted via `MemoryService.CreateStructuredSummary`, then `pmem.ResetCheckpointState` clears the compaction marker so the next turn injects fresh memory.
- `injectTieredMemory` layers three tiers before each call: the shared project constitution (Tier 1), the latest structured summary (Tier 2), and `pmem.BuildContextInjection` (Tier 3) which already formats negative constraints, ranked records, and checkpoint text as the prompt expects; pocketed nil checks keep sub-agents or disabled setups safe.

## internal/agent/memory/memory.go
- The `MemoryService` interface unifies project constitution CRUD, structured summaries, and codebase knowledge storage/retrieval; the concrete implementation uses SQLC-generated queries plus raw SQL for listing/searching to back the tiered-memory prompts and the `memory_query` tool.
- `StructuredSummaryData` captures decisions, file changes, failure modes, dependency edges, and todo states so downstream tooling (injector, memory query, dashboards) has a consistent schema.

## internal/agent/indexer.go
- A background ticker (every 5 min) walks the working tree, skips `.git`, `vendor`, `node_modules`, limits to `.go/.ts/.tsx/.js/.py`, extracts symbol names via regex, optionally enriches with LSP hover text, and upserts the findings into `MemoryService.UpsertCodebaseKnowledge` so cold memory keeps a symbolic fingerprint of the codebase independent of session history.

## internal/agent/tools/memory_query.go
- `memory_query` exposes cold memory (past structured summaries + symbol knowledge) as a tool; it aggregates the last 10 structured summaries and any codebase matches (up to 20) and returns Markdown with the summaries/knowledge snippets so an agent can reason across sessions even if `pmem` isn’t enabled.

## internal/agent/templates/coder.md.tpl
- The coder prompt template includes `<persistent_memory>` instructions (lines ~669 and following) that explicitly tell the agent to rely on `recall_memory` for persistent facts, to use `save_memory` for critical updates, to treat the database as memory (not perception), and to always “verify with `recall_memory`” before acting, reinforcing the runtime integration described above.
