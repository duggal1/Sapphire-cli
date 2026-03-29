# load_skill

Loads domain-specific engineering protocols into the active session.

## MANDATORY AUTO-INVOCATION

Invoke `load_skill` BEFORE technical implementation, refactoring, or architectural modification.

## DISCOVERY-FIRST LOADING

Do not guess skill names when the catalog is large.

Preferred sequence:
1. Use `search_skills(query: "...")` for focused routing
2. If local search returns a strong fit, call `load_skill(name: "<exact-name>")` immediately
3. If the right skill is not local yet, call `install_skill(query: "...")`
4. Read the full `SKILL.md` returned by `install_skill`
5. Use `list_skills()` only when full inventory browsing is actually needed
6. Invoke `load_skill(name: "<exact-name>")`

Local-first rule:
- Bundled skills and already-installed local skills are the default path.
- Do not jump to `install_skill` until `search_skills` has been tried for the current task query.
- If `search_skills` returns nothing useful, extended install is the fallback, not the first move.

## EXECUTION SEQUENCE

1. Build a concise domain query from the user task
2. Call `search_skills`
3. If local results are sufficient, choose the exact local skill name and call `load_skill`
4. If local results are missing or insufficient, call `install_skill`
5. Read the full installed `SKILL.md` returned by `install_skill`
6. Choose exact skill names from results when needed
7. Invoke `load_skill(name: "<exact-name>")`
8. Await instructions
9. Proceed with implementation

## EXCEPTIONS

Do NOT load skills for:
- Greetings
- General questions without technical implementation

## RULES

1. Use exact skill identifiers returned by `search_skills` or `list_skills`
2. Load BEFORE implementation
3. Search local skills first for the current task
4. Install only when the needed skill is not already local or local results are clearly insufficient
5. Load multiple skills sequentially for multi-domain tasks
6. Do NOT load for greetings
7. Do NOT hardcode domain-to-skill mappings in your head; discover first

## EXAMPLES

**Correct:**
```
User: "Add a login form"
→ search_skills(query: "frontend ui form auth")
→ load_skill(name: "<exact local result>") if local search already fits
→ install_skill(query: "frontend auth form") only if local search is missing or weak
→ Implement
```

**Correct:**
```
User: "Why is the API failing?"
→ search_skills(query: "debug api failure error")
→ load_skill(name: "<exact result>")
→ Investigate
```

**Correct:**
```
User: "Build API endpoint with React frontend"
→ search_skills(query: "backend api frontend react")
→ load_skill(name: "<exact local backend result>") if available
→ load_skill(name: "<exact local frontend result>") if available
→ install_skill(query: "backend api") only if backend local search is insufficient
→ install_skill(query: "frontend react ui") only if frontend local search is insufficient
→ load_skill(name: "<first exact result>")
→ load_skill(name: "<second exact result>")
→ Implement
```

**Incorrect:**
- Loading after implementation starts
- Using guessed names without discovery first
- Loading for: "Hello"
