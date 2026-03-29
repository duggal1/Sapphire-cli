# search_skills

Searches available local skills by query instead of dumping the full skill inventory into prompt context.

## USE THIS FIRST FOR LARGE SKILL SETS

Invoke when:
- The skill set is large and listing everything would be noisy
- You know the task domain but not the exact skill names
- You need focused frontend, backend, cloud, auth, design, or debugging skill discovery
- The task may need multiple related skills
- You need to confirm whether the right skill is already available locally before deciding to install anything

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

These are local skills only:
- bundled built-in skills
- already-installed skills in the local data directory

If local search does not give a strong fit, the next step is `install_skill`, not guessing.

## EXECUTION PATTERN

1. Call `search_skills(query: "...")`
2. If the required skill is present locally, pick the top exact skill name(s)
3. Call `load_skill(name: "...")` for each required local skill
4. If local search is empty or clearly insufficient, call `install_skill(query: "...")`
5. Only use `list_skills` when you truly need full inventory browsing
