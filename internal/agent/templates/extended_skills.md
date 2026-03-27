<extended_skills_policy>
- Built-in local skills are the first stop. If a built-in skill already covers the task, use it immediately and do not install anything.
- Extended skills are for complex or specialized work that benefits from extra procedural depth, narrower domain coverage, or broader ecosystem coverage than the built-in set.
- Use extended skills for meaningful implementation, debugging, architecture, auth, backend, frontend, infra, cloud, security, deployment, data, or integration work when the built-in set is incomplete for the task at hand.
- Skip extended-skill discovery for trivial chat, tiny one-step tasks, or cases already fully covered by a built-in skill.
- Be autonomous: do not wait for permission to search, install, and load relevant extended skills when they materially improve task quality.
- Use exact tool calls in this order when needed:
  1. `search_skills(query: "...")` to inspect the local skill store
  2. If the needed skill is missing, `install_skill(query: "...")`
  3. After install, call `search_skills(query: "...")` again if you need the exact identifier
  4. `load_skill(name: "<exact-name>")`
- For multi-domain tasks, load multiple skills sequentially. Example pattern:
  `search_skills` -> `install_skill` if needed -> `load_skill` -> `load_skill`
- Installed extended skills become local skills under `<data-dir>/skills` and should be treated as directly available local skills after installation.
- Treat the installed `SKILL.md` as the source of truth once loaded. Follow it exactly.
- Prefer concise search queries that reflect the real task, for example `frontend auth form`, `supabase auth row level security`, `aws terraform deploy`, or `backend api observability`.
- Do not install blindly. Install only when the task is non-trivial and the extended skill is likely to improve execution quality.
</extended_skills_policy>
