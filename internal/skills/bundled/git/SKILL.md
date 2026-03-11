# SKILL: GitHub & Git — Workflow Conventions

This skill defines how Git and GitHub operations are performed in this project. These are not suggestions. Follow these conventions exactly on every task, every run, every sub-agent. Do not use your own defaults.

**Last verified:** March 2026

---

## COMMIT CONVENTIONS

- Always use imperative present tense: `fix:`, `feat:`, `chore:`, `refactor:`, `docs:`
- Stage all changes with `git add -A` unless a partial commit is explicitly required
- Never amend a commit that has already been pushed
- Pull before pushing on shared branches: `git pull --rebase origin main`

## BRANCH CONVENTIONS

- New branches always off `main`: `git switch -c feature/name`
- Branch names: lowercase, hyphen-separated, prefixed by type: `feature/`, `fix/`, `chore/`
- Never commit directly to `main`

## PULL REQUEST CONVENTIONS

- Always use `gh pr create --fill` unless title/body are explicitly specified
- Always squash on merge: `gh pr merge NUMBER --squash --delete-branch`
- Always delete branch after merge — never leave stale branches
- Set auto-merge when CI is running: `gh pr merge NUMBER --squash --delete-branch --auto`

## CI CONVENTIONS

- On CI failure: always read failed logs first before touching code: `gh run view RUN_ID --log-failed`
- Rerun failed jobs only — never rerun the full workflow unless explicitly told: `gh run rerun RUN_ID --failed`
- Never merge a PR with failing checks — fix CI first

## KNOWN TRAPS — DO NOT HALLUCINATE THESE

- Inline PR review comments require `gh api` — `gh pr comment` does NOT support inline
- `gh run rerun --job` requires `databaseId` — NOT the job number from the browser URL. Get it with: `gh run view RUN_ID --json jobs --jq '.jobs[] | {name, databaseId}'`