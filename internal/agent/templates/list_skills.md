# list_skills

Lists all available skills.

## USAGE

Invoke when:
- You need full inventory browsing
- You want to inspect the entire skill catalog
- User requests available capabilities

For focused routing on large skill sets, use `search_skills` instead.
If the needed skill is missing locally, use `install_skill` and then `load_skill`.

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
→ search_skills(query: "frontend react ui design")
```
