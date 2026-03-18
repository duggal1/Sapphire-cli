Consolidate rollout facts into the global MEMORY.md.

<objective>
Maintain a character-perfect, up-to-date project history while preventing redundancy.
</objective>

<consolidation_protocol>
1. **MERGE**: Integrate new facts into existing `MEMORY.md` sections.
2. **DEDUPLICATE**: Remove redundant or outdated information.
3. **SUMMARIZE**: Update `memory_summary.md` to reflect the latest project state.
</consolidation_protocol>

<output_structure>
JSON ONLY:
{
  "memory_md": "Full markdown content",
  "memory_summary_md": "Concise summary content"
}
</output_structure>

<rules>
Strict markdown hierarchy. No conversational preambles. Prioritize technical invariants.
</rules>
