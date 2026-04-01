# QA Agent Design Guide

Guide for including a QA agent in a build harness. Based on bug patterns and root-cause analysis from a real project (SatangSlide), this document provides a verification methodology for systematically catching defects that QA agents often miss.

---

## Table of Contents

1. Defect Patterns QA Agents Miss
2. Integration Coherence Verification
3. QA Agent Design Principles
4. Verification Checklist Template
5. QA Worker Template

---

## 1. Defect Patterns QA Agents Miss

### 1-1. Boundary Mismatch

This is the most frequent defect. Two components are each implemented "correctly," but the contract breaks at the connection point.

| Boundary | Mismatch example | Why QA misses it |
|--------|-----------|-----------|
| API response → frontend hook | API returns `{ projects: [...] }`, hook expects `SlideProject[]` | each side passes isolated validation, but no cross-comparison is performed |
| API response field name → type definition | API uses `thumbnailUrl` (camelCase), type uses `thumbnail_url` (snake_case) | if cast through a TypeScript generic, the compiler does not catch it |
| File path → link href | page exists at `/dashboard/create`, but link points to `/create` | file structure and href are not cross-compared |
| State transition map → actual status update | map defines `generating_template → template_approved`, but code omits the transition | QA confirms the map exists and does not trace every update path |
| API endpoint → frontend hook | API exists, but no matching hook exists and nothing calls it | API list and hook list are not mapped one-to-one |
| Immediate response → async result | API returns `{ status }` immediately, but frontend reads `data.failedIndices` | type is checked without distinguishing sync vs async response shape |

### 1-2. Why Static Code Review Does Not Catch It

- **Limit of TypeScript generics**: `fetchJson<SlideProject[]>()` — even if the runtime response is `{ projects: [...] }`, compilation still passes
- **`npm run build` passing does not mean runtime correctness**: if type casts, `any`, or generics are used, the build can pass while runtime still fails
- **Existence verification vs connection verification**: "Does the API exist?" and "Does the API response match the caller expectation?" are completely different checks

---

## 2. Integration Coherence Verification

These are mandatory **cross-comparison verification** areas for a QA agent.

### 2-1. API Response ↔ Frontend Hook Type Cross-Verification

**Method**: Compare the `NextResponse.json()` payload in each API route against the `fetchJson<T>` type parameter in the corresponding hook.

```
Verification steps:
1. Extract the shape of the object passed to `NextResponse.json()` in the API route
2. Check the `T` type used by `fetchJson<T>` in the corresponding hook
3. Compare the shape and `T`
4. Check wrapping behavior (if the API returns `{ data: [...] }`, confirm the hook unwraps `.data`)
```

**Patterns that require special attention:**
- pagination APIs: `{ items: [], total, page }` vs frontend expecting an array
- mismatch across snake_case DB fields → camelCase API response → frontend type definition
- difference between immediate response (202 Accepted) and final result shape

### 2-2. File Path ↔ Link/Router Path Mapping

**Method**: Extract URL paths from `page` files under `src/app/`, then compare them against every `href`, `router.push()`, and `redirect()` value in the code.

```
Verification steps:
1. Extract URL patterns from `page.tsx` file paths under `src/app/`
   - remove `(group)` from the URL
   - treat `[param]` as a dynamic segment
2. Collect every `href=`, `router.push(`, and `redirect(` value in the code
3. Confirm that each link matches a real page path
4. Pay attention to URL prefixes for pages inside route groups (example: under `dashboard/`)
```

### 2-3. State Transition Completeness Tracking

**Method**: Extract every `status:` update in the code and compare it against the state transition map.

```
Verification steps:
1. Extract the allowed transition list from the state transition map (`STATE_TRANSITIONS`)
2. Search every API route for `.update({ status: "..." })`
3. Confirm that each transition is defined in the map
4. Identify transitions defined in the map but never executed in code (dead transitions)
5. In particular, verify that transitions from an intermediate state (example: `generating_template`) to a final state (`template_approved`) are not missing
```

### 2-4. API Endpoint ↔ Frontend Hook One-to-One Mapping

**Method**: List all API routes and frontend hooks, then verify correct pairing.

```
Verification steps:
1. Extract endpoint lists by HTTP method from `route.ts` under `src/app/api/`
2. Extract fetch URLs from `use*.ts` under `src/hooks/`
3. Identify API endpoints that no hook calls → mark as "unused"
4. Decide whether "unused" is intentional (example: admin API) or a defect (missing call path)
```

---

## 3. QA Agent Design Principles

### 3-1. Use `task` by Default, `coder` Only When Fixes Are Authorized

In Sapphire, QA should usually run as a spawned worker with profile `task`.

`task` is the correct default because it supports:
- repository reads and search
- diagnostics and verification commands
- analysis without repository mutation

Use `coder` only when the QA worker is explicitly allowed to patch or rewrite files.

**Required choice**: default to `task`, and state the `verify → report → fix request` protocol in the worker message or reusable QA skill.

### 3-2. Prioritize "Cross-Comparison" Over "Existence Check" in the Checklist

| Weak checklist | Strong checklist |
|---------------|---------------|
| Does the API endpoint exist? | Does the API response shape match the corresponding hook type? |
| Is the state transition map defined? | Do all status update paths in code match the map transitions? |
| Does the page file exist? | Does every link in code point to a real page? |
| Is TypeScript strict mode enabled? | Is type safety bypassed through generic casting? |

### 3-3. Rule: Read Both Sides at the Same Time

To catch boundary bugs, QA must not read only one side. It must read:
- the API route **and** the corresponding hook **together**
- the state transition map **and** the actual update code **together**
- the file structure **and** the link paths **together**

State this rule explicitly in the worker instructions or reusable QA skill.

### 3-4. Run QA Immediately After Each Module, Not Only After the Full Build

If the orchestrator places QA only at "Phase 4: after everything is complete":
- defects accumulate and repair cost increases
- early boundary mismatches propagate into later modules

**Required pattern**: after each backend API is completed, immediately run cross-verification against that API and its corresponding hook (incremental QA).

---

## 4. Verification Checklist Template

Integration coherence checklist for web applications. Include this in the QA worker instructions or QA skill.

```markdown
### Integration Coherence Verification (Web App)

#### API ↔ Frontend Connection
- [ ] every API route response shape matches the generic type in the corresponding hook
- [ ] wrapped responses (`{ items: [...] }`) are unwrapped correctly in the hook
- [ ] snake_case ↔ camelCase conversion is applied consistently
- [ ] immediate responses (202) and final result shapes are distinguished correctly in the frontend
- [ ] every API endpoint has a corresponding frontend hook and is actually called

#### Routing Coherence
- [ ] every `href`/`router.push` value in code matches a real page file path
- [ ] route groups (`(group)`) are accounted for correctly when validating URLs
- [ ] dynamic segments (`[id]`) are filled with correct parameters

#### State Machine Coherence
- [ ] every defined state transition is executed in code (no dead transitions)
- [ ] every `status` update in code is defined in the transition map (no unauthorized transitions)
- [ ] no transition from intermediate state to final state is missing
- [ ] every frontend branch based on state (`if status === "X"`) uses a state value that is actually reachable

#### Data Flow Coherence
- [ ] DB schema field names map consistently to API response field names
- [ ] frontend type definitions use the same field names as the API response
- [ ] null/undefined handling for optional fields is consistent on both sides
```

---

## 5. QA Worker Template

Core sections to include in a QA worker packet or reusable QA skill.

```markdown
# QA Inspector

## Profile
task

## Core Role
Verify implementation quality against the spec and **integration coherence across modules**.

## Verification Priority

1. **Integration coherence** (highest) — boundary mismatches are a primary source of runtime failure
2. **Functional spec compliance** — API/state machine/data model
3. **Design quality** — color/typography/responsiveness
4. **Code quality** — unused code, naming rules

## Verification Method: "Read Both Sides Together"

For boundary verification, always **open both sides of the boundary at the same time** and compare them:

| Verification target | Left side (producer) | Right side (consumer) |
|----------|-------------|---------------|
| API response shape | `NextResponse.json()` in `route.ts` | `fetchJson<T>` in `hooks/` |
| Routing | page file path under `src/app/` | `href`, `router.push` values |
| State transition | `STATE_TRANSITIONS` map | `.update({ status })` code |
| DB → API → UI | table column names | API response fields → type definition |

## Team Communication Protocol

- On discovery, send a concrete fix request immediately through `agent_mail_send` or report to the leader through `collect_result`
- For boundary issues, notify **both** relevant owners when a team is active
- Separate pass/fail/not-verified items in the final report

## Output Contract
- Write the verification report to `_workspace/{phase}_qa_report.md`
- Include concrete file references and evidence

## Escalation Rule
- Stay in `task` unless the leader explicitly respawns or reassigns the QA role as `coder`
```

---

## Real Case: Bugs Found in SatangSlide

Every rule in this guide was extracted from the real bugs below:

| Bug | Boundary | Cause |
|------|--------|------|
| `projects?.filter is not a function` | API→hook | API returned `{projects:[]}`, hook expected array |
| every dashboard link returned 404 | file path→href | `/dashboard/` prefix missing |
| theme image not visible | API→component | `thumbnailUrl` vs `thumbnail_url` |
| theme selection not saved | API→hook | select-theme API existed, hook did not |
| generate page waited forever | state transition→code | `template_approved` transition code missing |
| `data.failedIndices` crash | immediate response→frontend | background result accessed from immediate response |
| slide view after completion returned 404 | file path→href | `/projects/` → `/dashboard/projects/` |
