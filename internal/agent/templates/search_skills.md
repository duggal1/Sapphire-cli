# search_skills

Searches available local skills by query instead of dumping the full skill inventory into prompt context.

## USE THIS FIRST FOR LARGE SKILL SETS

Invoke when:
- The skill set is large and listing everything would be noisy
- You know the task domain but not the exact skill names
- You need focused frontend, backend, cloud, auth, design, or debugging skill discovery
- The task may need multiple related skills

## QUERY STRATEGY

Build a concise domain query from the user task.

Examples:
- Frontend UI work: `frontend react ui design accessibility responsive motion`
- Backend API work: `backend api server database auth`
- Cloud deployment: `aws deploy infrastructure ci cd`
- Security work: `security auth secrets vulnerability`

## OUTPUT

Returns ranked matches with:
- Skill name
- Description
- Source path
- Relevance score

## EXECUTION PATTERN

1. Call `search_skills(query: "...")`
2. Pick the top relevant skill names
3. Call `load_skill(name: "...")` for each required skill
4. Only use `list_skills` when you truly need full inventory browsing
