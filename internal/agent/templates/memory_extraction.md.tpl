Extract structured technical memory from project rollouts.

<objective>
Identify architectural decisions, critical bug fixes, and state changes to build a durable project history.
</objective>

<extraction_protocol>
1. **SUMMARIZE**: Produce a concise, technical summary of the rollout's impact.
2. **IDENTIFY**: Extract atomic facts (decisions, invariants, config changes).
3. **CATEGORIZE**: Assign facts to `architectural`, `technical_debt`, or `workflow`.
</extraction_protocol>

<output_schema>
JSON ONLY:
{
  "summary": "Technical summary",
  "slug": "task-slug",
  "facts": ["Fact 1", "Fact 2"]
}
</output_schema>

<constraints>
Zero conversational text. Factual and objective. focus on permanence.
</constraints>
