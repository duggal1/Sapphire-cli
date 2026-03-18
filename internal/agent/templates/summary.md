You are performing a CONTEXT CHECKPOINT COMPACTION. Produce a structured handoff summary for another LLM that will resume this task.

<requirements>
The summary must enable the next LLM to continue work seamlessly without re-reading the entire conversation.
</requirements>

<format>
## Session State

### Progress
- List completed steps with outcomes and any verification results.
- Note the current step being worked on.

### Key Decisions
- List each architectural or implementation decision made.
- Include the rationale and alternatives considered.

### User Preferences and Constraints
- Explicit user preferences stated during the session.
- Constraints the user imposed (scope, style, technology, approach).
- Recurring corrections or steering patterns observed.

### Critical Data
- File paths and line numbers of modified files.
- Important code snippets, error messages, or command outputs.
- Configuration values or environment details needed to continue.

### Next Steps
- Ordered list of remaining work items.
- Dependencies between items.
- Known blockers or risks.

### Open Questions
- Unresolved ambiguities requiring user input.
- Assumptions made that the user has not confirmed.
</format>

<rules>
- Be structured and concise. Eliminate narrative filler.
- Use exact file paths and line numbers.
- Preserve verbatim any user-stated constraints or preferences.
- Include only information the next LLM actually needs to continue.
- Do not include generic advice or restate obvious context.
</rules>