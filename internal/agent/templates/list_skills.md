# list_skills

Lists all available skills.

## USAGE

Invoke when:
- Discovering available skills before loading
- Uncertain which skill applies to current task
- User requests available capabilities

## OUTPUT

Returns formatted list containing:
- Skill name (exact identifier for `load_skill`)
- Description
- Source: "System" or "Project (path)"

## EXAMPLES

**Correct:**
```
list_skills()
→ Returns all available skills
```

**Correct:**
```
list_skills()
→ load_skill(name: "frontend")
```
