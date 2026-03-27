# load_skill

Loads domain-specific engineering protocols into the active session.

## MANDATORY AUTO-INVOCATION

Invoke `load_skill` BEFORE technical implementation, refactoring, or architectural modification.

## DISCOVERY-FIRST LOADING

Do not guess skill names when the catalog is large.

Preferred sequence:
1. Use `search_skills(query: "...")` for focused routing
2. If the right skill is not local yet, call `install_skill(query: "...")`
3. Use `list_skills()` only when full inventory browsing is actually needed
4. Invoke `load_skill(name: "<exact-name>")`

## EXECUTION SEQUENCE

1. Build a concise domain query from the user task
2. Call `search_skills`
3. If the needed skill is missing, call `install_skill`
4. Choose exact skill names from results
5. Invoke `load_skill(name: "<exact-name>")`
6. Await instructions
7. Proceed with implementation

## EXCEPTIONS

Do NOT load skills for:
- Greetings
- General questions without technical implementation

## RULES

1. Use exact skill identifiers returned by `search_skills` or `list_skills`
2. Load BEFORE implementation
3. Install first when the needed skill is not already local
4. Load multiple skills sequentially for multi-domain tasks
5. Do NOT load for greetings
6. Do NOT hardcode domain-to-skill mappings in your head; discover first

## EXAMPLES

**Correct:**
```
User: "Add a login form"
→ search_skills(query: "frontend ui form auth")
→ install_skill(query: "frontend auth form") if the right skill is missing
→ load_skill(name: "<exact result>")
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
→ install_skill(query: "backend api") if needed
→ install_skill(query: "frontend react ui") if needed
→ load_skill(name: "<first exact result>")
→ load_skill(name: "<second exact result>")
→ Implement
```

**Incorrect:**
- Loading after implementation starts
- Using guessed names without discovery first
- Loading for: "Hello"
