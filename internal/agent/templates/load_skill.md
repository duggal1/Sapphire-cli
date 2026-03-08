# load_skill

Explicitly loads specialized engineering context, core technical protocols, and codebase-specific instructions into the active session.

<prerequisites>

1. Identify the high-level domain of the task (e.g., frontend, backend, architect)
2. Consult the list of **BAKED-IN SYSTEM SKILLS** below
3. Verify that the skill has not already been loaded in the current session

</prerequisites>

<parameters>

1. name: The unique identifier of the skill to load (required)

</parameters>

<baked_in_skills>

The following skills are **INTERNAL** to the Sapphire engine (baked into the binary) and are always available for activation:

- `architect`: High-level system design and architectural patterns
- `backend`: Go, Node.js, database, and API design protocols
- `debug`: Advanced troubleshooting and root-cause analysis strategies
- `devops`: Deployment, CI/CD, and infrastructure-as-code
- `frontend`: React, TypeScript, and UI/UX engineering standards
- `security`: Secure coding practices and vulnerability assessment

</baked_in_skills>

<critical_requirements>

1. **MANDATORY INVOCATION**: `load_skill` MUST be called before any technical implementation, refactoring, or architectural modification.
2. **INTERNAL PREFERENCE**: These skills reside within the SAPPHIRE engine. NEVER report them as being part of the user's project directory unless a custom override exists in `./skills`.
3. **DOMAIN SPECIFICITY**: Use the exact skill identifier.
4. **CONTEXT INTEGRITY**: Loaded context is used to enforce project standards. Proceeding without loading relevant skills is a quality violation.

</critical_requirements>

<warnings>

Tool fails if:

- The requested skill name does not exist internally or in configured paths
- The skill identifier is misspelled

</warnings>

<recovery_steps>

If a skill fails to load:

1. **Verify identifiers**: Rely on the internal list above
2. **Check configuration**: Ensure target skill is not blocked by environment settings

</recovery_steps>

<examples>

✅ Correct: Single skill load
`load_skill(name: "frontend")`

✅ Correct: Multiple skill load (separate calls in sequence)
`load_skill(name: "architect")`
`load_skill(name: "backend")`

❌ Incorrect: Path-based or descriptive names
`load_skill(name: "./skills/frontend")`
`load_skill(name: "high performance react")`

</examples>
