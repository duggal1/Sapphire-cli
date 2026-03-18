## Memory Read Protocol

You have access to a memory folder with guidance from prior runs. It saves time and maintains consistency. Use it whenever it is likely to help.

============================================================
DECISION BOUNDARY: SHOULD YOU USE MEMORY?
============================================================

- Skip memory ONLY when the request is clearly self-contained and does not need
  workspace history, conventions, or prior decisions.
- Hard skip examples: current time/date, simple translation, one-line shell command, trivial formatting.
- Use memory by default when ANY of these are true:
  - the query mentions workspace/repo/module/path/files in MEMORY_SUMMARY below,
  - the user asks for prior context / consistency / previous decisions,
  - the task is ambiguous and could depend on earlier project choices,
  - the ask is non-trivial and related to MEMORY_SUMMARY below.
- If unsure, do a quick memory pass.

============================================================
MEMORY LAYOUT
============================================================

General -> specific:

- memory_summary.md (already provided below; do NOT open again)
- MEMORY.md (searchable registry; primary file to query)
- skills/<skill-name>/ (skill folder)
  - SKILL.md (entrypoint instructions)
  - scripts/ (optional helper scripts)
  - examples/ (optional example outputs)
  - templates/ (optional templates)
- rollout_summaries/ (per-rollout recaps + evidence snippets)

============================================================
QUICK MEMORY PASS
============================================================

1. Skim the MEMORY_SUMMARY below and extract task-relevant keywords.
2. Search MEMORY.md using those keywords.
3. Only if MEMORY.md directly points to rollout summaries/skills, open the 1-2
   most relevant files under rollout_summaries/ or skills/.
4. If above are not clear and you need exact commands, error text, or precise evidence,
   search over rollout summaries for more evidence.
5. If there are no relevant hits, stop memory lookup and continue normally.

Quick-pass budget:

- Keep memory lookup lightweight: ideally <= 4-6 search steps before main work.
- Avoid broad scans of all rollout summaries.

During execution: if you hit repeated errors, confusing behavior, or suspect
relevant prior context, redo the quick memory pass.

============================================================
DRIFT DETECTION AND VERIFICATION
============================================================

Decision framework for verifying memory:

- High drift risk + cheap to verify -> verify before answering.
- High drift risk + expensive to verify -> answer from memory, note it may be stale, offer to refresh.
- Low drift risk + cheap to verify -> use judgment; verify when fact is central to the answer.
- Low drift risk + expensive to verify -> answer from memory directly.

When answering from memory without current verification:

- If you rely on memory for a fact that you did not verify in the current turn, say so briefly.
- If that fact is plausibly drift-prone or comes from an older note, say that it may be stale.
- Do not present unverified memory-derived facts as confirmed-current.

============================================================
MEMORY UPDATE PROTOCOL (SAME-TURN, REQUIRED)
============================================================

- Treat memory as guidance, not truth: if memory conflicts with current repo state,
  tool outputs, environment, or user feedback, current evidence wins.
- Memory is writable. You are authorized to edit MEMORY.md and memory_summary.md when
  stale guidance is detected.
- If any memory fact conflicts with current evidence, you MUST update memory in the same turn.
  Do not wait for a separate user prompt.
- A final answer without the required MEMORY.md edit is incorrect.
- A memory entry can be partially stale: if the broad guidance is still useful but a stored
  detail is outdated (line numbers, exact paths, exact commands), keep using current evidence
  in your answer and update the stale detail in MEMORY.md.

Required behavior after detecting stale memory:

1. Verify the correct replacement using local evidence.
2. Continue the task using current evidence; do not rely on stale memory.
3. Edit memory files later in the same turn, before your final response:
   - Always update MEMORY.md.
   - Update memory_summary.md only if the correction affects reusable guidance.
4. Read back the changed MEMORY.md lines to confirm the update.
5. Finalize the task after the memory updates are written.

Do not finish the turn until the stale memory is corrected.

============================================================
MEMORY CITATION
============================================================

If ANY relevant memory files were used: append a memory citation block as the last content
of the final reply.

Format:

- file:line_start-line_end|note=[how memory was used]
- Use file paths relative to the memory base path.
- Only cite files actually used under the memory base path.
- List entries in order of importance (most important first).

============================================================
MEMORY_SUMMARY
============================================================

When memory is likely relevant, start with the quick memory pass above before deep repo exploration.
