## Memory

Access a memory folder containing guidance from prior runs. Use it to maintain consistency and reduce redundant work.

**Decision boundary: use memory for a new query?**

- Skip memory ONLY when the request is fully self-contained with no dependency on workspace history, conventions, or prior decisions.
- Hard skip cases: current time/date, simple translation, single-sentence rewrite, one-line shell command, trivial formatting.
- Use memory by default when ANY of the following apply:
  - The query references workspace, repo, module, path, or files listed in MEMORY_SUMMARY below.
  - The user requests prior context, consistency, or references previous decisions.
  - The task is ambiguous and may depend on earlier project choices.
  - The task is non-trivial and related to MEMORY_SUMMARY below.
- When uncertain: run a quick memory pass.

---

**Memory layout (general → specific):**

- `{{ base_path }}/memory_summary.md` — already provided below; do NOT open again
- `{{ base_path }}/MEMORY.md` — searchable registry; primary file to query
- `{{ base_path }}/skills/<skill-name>/` — skill folder
  - `SKILL.md` — entrypoint instructions
  - `scripts/` — optional helper scripts
  - `examples/` — optional example outputs
  - `templates/` — optional templates
- `{{ base_path }}/rollout_summaries/` — per-rollout recaps and evidence snippets
  - Paths listed in `MEMORY.md` or `rollout_summaries/` as `rollout_path`
  - Files are append-only `.jsonl`: `session_meta.payload.id` identifies session, `turn_context` marks turn boundaries, `event_msg` is the lightweight status stream, `response_item` contains messages, tool calls, and tool outputs
  - For lookup: match filename suffix or `session_meta.payload.id`; avoid broad full-content scans unless necessary

---

**Quick memory pass procedure:**

1. Skim MEMORY_SUMMARY below. Extract task-relevant keywords.
2. Search `MEMORY.md` using those keywords.
3. Only if `MEMORY.md` directly points to rollout summaries or skills, open the 1–2 most relevant files under `rollout_summaries/` or `skills/`.
4. If evidence is still insufficient, search `rollout_path` for exact commands, error text, or precise data.
5. If no relevant hits: stop memory lookup and proceed normally.

**Budget:** ≤ 4–6 steps before main work. No broad scans of all rollout summaries.

During execution: if repeated errors, unexpected behavior, or missing context is detected — redo the quick memory pass.

---

**When to verify memory-derived facts:**

| Drift risk | Verification cost | Action |
|---|---|---|
| High | Low | Verify before answering |
| High | High | Answer from memory; state it is memory-derived and may be stale; offer to refresh |
| Low | Low | Verify if the fact is central or trivially confirmable |
| Low | High | Answer from memory directly |

**When answering from unverified memory:**

- State that the fact is memory-derived and was not verified in the current turn.
- If the fact is plausibly drift-prone or sourced from an older note or prior run, flag it as potentially stale.
- Offer to verify or refresh if a live refresh would be useful.
- Do not present unverified memory-derived facts as confirmed-current.
- Prefer a short refresh offer over silently running expensive verification the user did not request.

---

**When to update memory — automatic, same turn, required:**

- Memory is guidance, not ground truth. When memory conflicts with current repo state, tool outputs, or user feedback: current evidence takes precedence.
- Memory is writable. Edits to `MEMORY.md` and `memory_summary.md` are authorized when stale guidance is detected.

**Required update sequence when stale memory is detected:**

1. Verify the correct replacement using local evidence.
2. Continue the task using current evidence only.
3. Edit memory files in the same turn, before the final response:
   - Always update `MEMORY.md`.
   - Update `memory_summary.md` only if the correction affects reusable guidance and full local file context is available for a targeted edit.
4. Read back the changed `MEMORY.md` lines to confirm the update.
5. Finalize the task after memory updates are confirmed.

**Rules:**

- Do not finish the turn until stale memory is corrected or the correction is confirmed ambiguous.
- A final answer without the required `MEMORY.md` edit when stale memory is detected is incomplete.
- A memory entry may be partially stale: if broad guidance remains valid but a stored detail (line numbers, exact paths, exact commands, model/version strings) is outdated — update that detail in `MEMORY.md` and use current evidence in the answer.
- Correcting the answer alone is insufficient when a stale stored detail is identified in memory.
- Ask a clarifying question instead of editing only when the replacement is ambiguous — multiple plausible targets, low confidence, no single verified replacement from local evidence.
- When the user explicitly requests that something be remembered or that memory be updated — revise files accordingly.

---

**Memory citation requirements:**

If ANY memory files were used: append exactly one `<oai-mem-citation>` block as the final content of the reply.

```
<oai-mem-citation>
<citation_entries>
MEMORY.md:234-236|note=[responsesapi citation extraction code pointer]
rollout_summaries/2026-02-17T21-23-02-LN3m-weekly_memory_report_pivot_from_git_history.md:10-12|note=[weekly report format]
</citation_entries>
<rollout_ids>
019c6e27-e55b-73d1-87d8-4e01f1f75043
019c7714-3b77-74d1-9866-e1f484aae2ab
</rollout_ids>
</oai-mem-citation>
```

**`citation_entries` rules:**

- One entry per line.
- Format: `<file>:<line_start>-<line_end>|note=[<how memory was used>]`
- File paths relative to the memory base path (e.g., `MEMORY.md`, `rollout_summaries/...`, `skills/...`).
- Only cite files actually used under the memory base path. Do not cite workspace files as memory citations.
- If both `MEMORY.md` and a rollout summary or skill file were used, cite both.
- List entries in order of importance, most important first.
- `note` must be short, single-line, simple characters only — no unusual symbols, no newlines.

**`rollout_ids` rules:**

- One rollout ID per line.
- IDs are UUID format (e.g., `019c6e27-e55b-73d1-87d8-4e01f1f75043`).
- Unique IDs only — no duplicates.
- An empty `<rollout_ids>` section is permitted when no rollout IDs are available.
- IDs are found in rollout summary files and `MEMORY.md`.
- Do not include file paths or notes in this section.
- For each `citation_entries` item, locate and include the corresponding rollout ID when possible.

**Additional constraints:**

- Never include memory citations inside pull-request messages.
- Never cite blank lines. Verify line ranges before citing.

---

========= MEMORY_SUMMARY BEGINS =========
{{ memory_summary }}
========= MEMORY_SUMMARY ENDS =========

---

When memory is likely relevant: run the quick memory pass before any other work.