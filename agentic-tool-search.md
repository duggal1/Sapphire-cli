# tool_search

## What It Is

`tool_search` is a codebase locator tool for AI agents. Given a query — natural language, a symbol name, or a filename — it returns ranked, line-precise file candidates without reading the repository directly.

It is the search layer between an agent and a large codebase. The agent calls it before opening any file.

---

## Architecture

The tool runs three search stages in a fixed cascade. Each stage can short-circuit the next.

**Stage 1 — Indexed lookup**
Queries the durable local codebase graph. The graph is built by `index_codebase`: files are discovered, hashed, chunked, parsed for Go symbols, embedded with Jina code embeddings, and stored with vector metadata. This stage returns semantic matches with scores, signatures, snippets, and line ranges.

**Stage 2 — Filename search**
Runs ripgrep across file paths matching query terms. Used when indexed results are insufficient.

**Stage 3 — Text search**
Runs ripgrep literal text search across file contents. Used as a final fallback.

Each stage feeds into a shared candidate pool. Candidates accumulate scores across all three sources. If a stage produces high-confidence results, the remaining stages are skipped.

---

## How It Works

**Query planning**
The raw query is decomposed into up to four indexed variants, four filename variants, and three text variants. Stop words are filtered. Terms shorter than three characters are dropped. If the query looks like a path or symbol (contains `/`, `.`, `_`, `-`), it is treated as a precise query and sent directly to the index.

**Candidate scoring**
Every file matched across any stage becomes a candidate. The candidate score is the sum of:

- Indexed score (base score from the graph plus a position bonus)
- Filename score (position-weighted)
- Text score (position-weighted, with a hit-count bonus)
- Cross-source bonus: +25 per additional source beyond the first
- Hit-count bonus: up to +24 for indexed, up to +12 for text
- Path adjustment (see below)

**Path adjustment**
The scoring system applies path-based penalties and bonuses before ranking:

| Path pattern | Adjustment |
|---|---|
| `node_modules/`, `vendor/`, `third_party/` | −72 |
| `dist/`, `build/`, `.next/`, `coverage/`, `target/` | −42 |
| `.pb.go`, `generated/`, `.gen.` | −24 |
| `testdata/`, `fixtures/`, `mocks/` | −16 |
| `internal/` | +6 |
| Depth > 8 directories | −3 per extra level (max −18) |

**Early exit logic**
After each stage, the ranked candidates are checked. The indexed/filename stage stops early if:
- The top candidate scores ≥ 360, or
- Three or more candidates score ≥ 260 (two if limit ≤ 3)

The text stage stops early if:
- Three or more candidates score ≥ 180, or
- The top candidate scores ≥ 300

**Output**
Results are returned as ranked candidates with: file path, line range, total score, source summary (indexed/filename/text), and a snippet or signature up to 180 characters.

---

## Core Capabilities

- Natural language, symbol, and filename queries
- Semantic search against a durable local codebase graph
- Parallel tool execution (via `fantasy.NewParallelAgentTool`)
- Multi-root search: one call can target multiple directory roots
- Query decomposition with stop word filtering
- Three-source candidate fusion with cross-source scoring bonuses
- Confidence-gated early exit at each stage
- Path-aware scoring that deprioritizes generated, vendor, and build artifacts
- Line-precise results: file path plus start/end line numbers
- Snippet and signature extraction from the codebase graph
- Configurable result limit (default 8, max 20)

---

## Built for a 10 Million-Line Codebase

The tool does not scan the repository at query time. The repository is already indexed.

`index_codebase` runs first. It discovers files concurrently, skips binary content and ignored directories, hashes file contents, reuses unchanged files from the previous index, splits large files into chunks, parses Go declarations into symbol-oriented chunks, embeds chunks, and stores vectors and metadata in a local SQLite-backed code index. Incremental re-indexing means only changed files are re-embedded on subsequent runs.

When `tool_search` runs, it queries this pre-built graph. The indexed stage is a vector similarity search — it does not traverse the filesystem. If it returns confident results, the ripgrep stages never run.

For a 10M line repository:
- Query cost is roughly constant regardless of repo size, because it hits the index, not the files
- Ripgrep stages are bounded by a configurable `stageLimit` and short-circuit on confidence
- Generated code, vendor trees, and build artifacts are deprioritized in scoring, so they don't pollute results from large monorepos
- The graph survives across sessions — it is built once, reused across every agent turn

The practical result is that an agent working on a large codebase does not need to read broad swaths of files to find what it needs. It searches first, then reads only the relevant regions using `agentic_view` or `single_view`.