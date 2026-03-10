# load_skill

Loads domain-specific engineering protocols into the active session.

## MANDATORY AUTO-INVOCATION

Invoke `load_skill` BEFORE technical implementation, refactoring, or architectural modification.

## DOMAIN-TRIGGERED LOADING

| Domain | Skill | Trigger |
|--------|-------|---------|
| Frontend/UI | `frontend` | React, TypeScript, components, styling, UI/UX |
| Backend/API | `backend` | Server, database, API, business logic |
| Debugging | `debug` | Error investigation, bug fix, failure analysis |
| Architecture | `architect` | System design, structural change, patterns |
| DevOps | `devops` | Deployment, CI/CD, infrastructure, containers |
| Security | `security` | Auth, vulnerabilities, secure coding |

## EXECUTION SEQUENCE

1. Recognize task domain
2. Invoke `load_skill(name: "<domain>")`
3. Await instructions
4. Proceed with implementation

## EXCEPTIONS

Do NOT load skills for:
- Greetings
- General questions without technical implementation

## AVAILABLE SKILLS

- `architect` — System design, architectural patterns
- `backend` — Go, Node.js, databases, APIs, layered architecture
- `debug` — Error investigation, root-cause analysis
- `devops` — Deployment, CI/CD, infrastructure
- `frontend` — React, TypeScript, UI/UX, design systems
- `security` — Auth, vulnerabilities, secure coding

## RULES

1. Use exact skill identifiers
2. Load BEFORE implementation
3. Load multiple skills sequentially for multi-domain tasks
4. Do NOT load for greetings

## EXAMPLES

**Correct:**
```
User: "Add a login form"
→ load_skill(name: "frontend")
→ Implement
```

**Correct:**
```
User: "Why is the API failing?"
→ load_skill(name: "debug")
→ Investigate
```

**Correct:**
```
User: "Build API endpoint with React frontend"
→ load_skill(name: "backend")
→ load_skill(name: "frontend")
→ Implement
```

**Incorrect:**
- Loading after implementation starts
- Using paths: `./skills/frontend`
- Loading for: "Hello"
