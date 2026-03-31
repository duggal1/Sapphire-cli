Makes batched edits across one or more files in one operation. Built on Edit tool for efficient multiple find-and-replace operations across a file set.

**Note**: This tool is only available to the main agent. Sub-agents do not have multi-edit capability to prevent token explosion.

<prerequisites>
1. **STRONGLY PREFERRED**: Use `agentic_view` to read the full target files before editing. It is the default repository read path, even for one file when the scope may expand. Use `single_view` only for an explicitly narrow trivial one-file read. If you already know the exact edit and the runtime can proceed, do not repeat the same blocked attempt; read once, then continue.
2. **TRUNCATION CHECK**: If the view output shows `... lines hidden`, you **MUST** expand the view using `offset` and `limit` to read those sections. NEVER edit a truncated file.
3. Verify directory paths are correct.
4. **CRITICAL**: Note exact whitespace, indentation, and formatting from View output.
</prerequisites>

<parameters>
1. file_edits: Array of file edit objects (max 25 files), each containing:
   - file_path: Absolute path to file (required)
   - edits: Array of edit operations (max 25 per file), each containing:
     - old_string: Text to replace (must match exactly including whitespace/indentation)
     - new_string: Replacement text
     - replace_all: Replace all occurrences (optional, defaults to false)
   - This is the preferred canonical shape.
2. Compatible single-file shorthand:
   - `file_path` + `edits`
   - `file_path` + `old_string` + `new_string` (+ optional `replace_all`)
3. Compatible per-file shorthand inside `file_edits`:
   - `file_path` + direct `old_string` + `new_string` (+ optional `replace_all`)
4. Compatible top-level batched shorthand:
   - `edits` may contain file edit objects with `file_path` + edit fields, but prefer `file_edits` to reduce ambiguity
</parameters>

<operation>
- Edits applied sequentially in provided order.
- Each edit operates on result of previous edit.
- PARTIAL SUCCESS: If some edits fail, successful edits are still applied. Failed edits are returned in the response.
- File is modified if at least one edit succeeds.
- Ideal for several changes to different parts of same file.
</operation>

<inherited_rules>
All instructions from the Edit tool documentation apply verbatim to every edit item:
- Critical requirements for exact matching and uniqueness
- Warnings and common failures (tabs vs spaces, blank lines, brace placement, etc.)
- Verification steps before using, recovery steps, best practices, and whitespace checklist
Use the same level of precision as Edit. Multiedit often fails due to formatting mismatches—double-check whitespace for every edit.
</inherited_rules>

<critical_requirements>
1. Apply Edit tool rules to EACH edit (see edit.md).
2. Edits are applied in order; successful edits are kept even if later edits fail.
3. Plan sequence carefully: earlier edits change the file content that later edits must match.
4. Ensure each old_string is unique at its application time (after prior edits).
5. Check the response for failed edits and retry them if needed.
</critical_requirements>

<verification_before_using>
1. View the file and copy exact text (including whitespace) for each target.
2. Check how many instances each old_string has BEFORE the sequence starts.
3. Dry-run mentally: after applying edit #N, will edit #N+1 still match? Adjust old_string/new_string accordingly.
4. Prefer fewer, larger context blocks over many tiny fragments that are easy to misalign.
5. If edits are independent, consider separate multiedit batches per logical region.
</verification_before_using>

<warnings>
- Operation continues even if some edits fail; check response for failed edits.
- Earlier edits can invalidate later matches (added/removed spaces, lines, or reordered text).
- Mixed tabs/spaces, trailing spaces, or missing blank lines commonly cause failures.
- replace_all may affect unintended regions—use carefully or provide more context.
</warnings>

<recovery_steps>
If some edits fail:
1. Check the response metadata for the list of failed edits with their error messages.
2. View the file again to see the current state after successful edits.
3. Adjust the failed edits based on the new file content.
4. Retry the failed edits with corrected old_string values.
5. Consider breaking complex batches into smaller, independent operations.
</recovery_steps>

<best_practices>
- Ensure all edits result in correct, idiomatic code; don't leave code broken.
- Continue repairing edited files until current-file errors and warnings are zero.
- Use absolute file paths (starting with /).
- Use replace_all only when you're certain; otherwise provide unique context.
- Match existing style exactly (spaces, tabs, blank lines).
- Review failed edits in the response and retry with corrections.
- Prefer smaller precise batches over large speculative ones. Use `agentic_edit` whenever batching improves clarity or reliability, even for one file with multiple linked edits.
- **Verify target context**: Ensure the surrounding context exactly matches the file before calling the tool.
- **NEVER GUESS**: If a batch edit fails, re-read the file. Guessing whitespace or context in a multi-edit operation is guaranteed to fail.
</best_practices>

<whitespace_checklist>
For EACH edit, verify:
- [ ] Viewed the file first
- [ ] Counted indentation spaces/tabs
- [ ] Included blank lines if present
- [ ] Matched brace/bracket positioning
- [ ] Included 3–5 lines of surrounding context
- [ ] Verified text appears exactly once (or using replace_all deliberately)
- [ ] Copied text character-for-character, not approximated
</whitespace_checklist>

<examples>
✅ Correct: Sequential edits where the second match accounts for the first change

```
file_edits: [
  {
    file_path: "/absolute/path/to/file.go",
    edits: [
      {
        old_string: "func A() {\n    doOld()\n}",
        new_string: "func A() {\n    doNew()\n}",
      },
      {
        // Uses context that still exists AFTER the first replacement
        old_string: "func B() {\n    callA()\n}",
        new_string: "func B() {\n    callA()\n    logChange()\n}",
      },
    ]
  }
]
```

❌ Incorrect: Second old_string no longer matches due to whitespace change introduced by the first edit

```
file_edits: [
  {
    file_path: "/absolute/path/to/file.go",
    edits: [
      {
        old_string: "func A() {\n    doOld()\n}",
        new_string: "func A() {\n\n    doNew()\n}", // Added extra blank line
      },
      {
        old_string: "func A() {\n    doNew()\n}", // Missing the new blank line, will FAIL
        new_string: "func A() {\n    doNew()\n    logChange()\n}",
      },
    ]
  }
]
```

✅ Correct: Handling partial success

```
// If edit 2 fails, edit 1 is still applied
// Response will indicate:
// - edits_applied: 1
// - edits_failed: [{index: 2, error: "...", edit: {...}}]
// You can then retry edit 2 with corrected context
```
</examples>
