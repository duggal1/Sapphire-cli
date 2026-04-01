# Agent Team Examples

---

## Example 1: Research Team (Agent Team Mode)

### Team Architecture: Fan-out/Fan-in
### Execution Mode: Agent Team

```
[Leader/Orchestrator]
    ├── TeamCreate(research-team)
    ├── TaskCreate(4 research tasks)
    ├── Team members self-coordinate (SendMessage)
    ├── Collect results (Read)
    └── Create integrated report
```

### Agent Composition

| Member | Agent Type | Role | Output |
|------|-------------|------|------|
| official-researcher | general-purpose | official documents/blog | research_official.md |
| media-researcher | general-purpose | media/investment | research_media.md |
| community-researcher | general-purpose | community/social | research_community.md |
| background-researcher | general-purpose | background/competition/academic | research_background.md |
| (leader = orchestrator) | — | integrated report | integrated_report.md |

> Research agents use the `general-purpose` built-in type, but each one must still be defined in `.claude/agents/{name}.md`. The file must specify the role, research scope, and team communication protocol so reuse and collaboration quality are preserved.

### Orchestrator Workflow (Agent Team)

```
Phase 1: Preparation
  - Analyze user input (identify topic and research mode)
  - Create _workspace/

Phase 2: Team Setup
  - TeamCreate(team_name: "research-team", members: [
      { name: "official", prompt: "Research official channels..." },
      { name: "media", prompt: "Research media and investment trends..." },
      { name: "community", prompt: "Research community reaction..." },
      { name: "background", prompt: "Research background and competitive landscape..." }
    ])
  - TaskCreate(tasks: [
      { title: "Research official channels", assignee: "official" },
      { title: "Research media trends", assignee: "media" },
      { title: "Research community reaction", assignee: "community" },
      { title: "Research background context", assignee: "background" }
    ])

Phase 3: Research Execution
  - 4 members investigate independently
  - If one member finds a relevant discovery, share it through SendMessage
    (example: media sends investment news to background)
  - If conflicting information is found, members discuss it directly
  - Each member saves a file on completion and notifies the leader

Phase 4: Integration
  - Leader reads 4 artifacts
  - Create integrated report
  - Conflicting information is preserved with attribution

Phase 5: Cleanup
  - Request termination from members
  - Delete team
  - Preserve _workspace/ (required for post-run verification and audit traceability)
```

### Team Communication Pattern

```
official ──SendMessage──→ background  (share relevant official announcements)
media ────SendMessage──→ background  (share investment and acquisition information)
community ─SendMessage──→ media      (share media-relevant community reactions)
all members ──TaskUpdate──→ shared task list  (progress update)
leader ←───── idle notification ──── completed member   (automatic)
```

---

## Example 2: SF Novel Writing Team (Agent Team Mode)

### Team Architecture: Pipeline + Fan-out
### Execution Mode: Agent Team

```
Phase 1 (parallel — agent team): worldbuilder + character-designer + plot-architect
  → coordinate consistency through SendMessage
Phase 2 (sequential): prose-stylist (drafting)
Phase 3 (parallel — agent team): science-consultant + continuity-manager (review)
  → share findings through SendMessage
Phase 4 (sequential): prose-stylist (apply review revisions)
```

### Agent Composition

| Member | Agent Type | Role | Skill |
|------|-------------|------|------|
| worldbuilder | custom | worldbuilding | world-setting |
| character-designer | custom | character design | character-profile |
| plot-architect | custom | plot structure | outline |
| prose-stylist | custom | style editing + drafting | write-scene, review-chapter |
| science-consultant | custom | science validation | science-check |
| continuity-manager | custom | continuity validation | consistency-check |

### Full Agent File Example: `worldbuilder.md`

```markdown
---
name: worldbuilder
description: "Specialist in building SF worlds. Designs physical laws, social structures, technology levels, and history."
---

# Worldbuilder — SF World Design Specialist

You are a specialist in designing SF worlds. Ground the world in scientific fact, extend it with disciplined imagination, and build the physical, social, and technological foundation in which the story operates.

## Core Role
1. Define the physical laws and technology level of the world
2. Design the social structure, political system, and economic system
3. Establish historical context and the structure of the current conflict
4. Describe environment and atmosphere by location

## Work Principles
- Internal consistency is the highest priority — no contradiction between settings
- Use chained "if this technology exists, then what follows?" reasoning to derive systemic effects
- Build a world that serves the story — avoid excessive setting that obstructs the plot

## Input/Output Protocol
- Input: user world concept and genre requirements
- Output: `_workspace/01_worldbuilder_setting.md`
- Format: markdown. Sectioned by physics/social/technology/history/location

## Team Communication Protocol
- To character-designer: send social structure, class system, and profession information
- To plot-architect: send major conflict structures and crisis elements
- From science-consultant: receive scientific error feedback → revise setting
- On world change: broadcast to all affected members

## Error Handling
- If the concept is vague, propose 3 directions and request selection
- If a scientific error is found, provide an alternative with the correction

## Collaboration
- Provide social structure information to character-designer
- Provide conflict structure information to plot-architect
- Revise the setting based on science-consultant feedback
```

### Detailed Team Workflow

```
Phase 1: TeamCreate(team_name: "novel-team", members: [worldbuilder, character-designer, plot-architect])
         TaskCreate([worldbuilding, character design, plot structure])
         → members self-coordinate and work in parallel
         → when worldbuilder completes the social structure, send it to character-designer
         → when character-designer defines the protagonist, send it to plot-architect

Phase 2: Delete the Phase 1 team → call prose-stylist as a sub-agent (no team required for solo drafting)
         prose-stylist reads 3 artifacts from _workspace/ and drafts
         → save result to `_workspace/02_prose_draft.md`

Phase 3: Create a new team — TeamCreate(team_name: "review-team", members: [science-consultant, continuity-manager])
         (only one team may be active per session, but the Phase 1 team has been deleted, so a new team is valid)
         → both reviewers inspect the draft and share findings
         → if science-consultant finds a physics error, notify continuity-manager as well
         → delete team after review completes

Phase 4: Call prose-stylist as a sub-agent, apply review results, and produce final revision
```

---

## Example 3: Webtoon Production Team (Sub-agent Mode)

### Team Architecture: Producer-Reviewer
### Execution Mode: Sub-agent

> In a producer-reviewer pattern with only 2 agents, result delivery matters more than direct communication. Sub-agents are the correct mode.

```
Phase 1: Agent(webtoon-artist) → generate panels
Phase 2: Agent(webtoon-reviewer) → review
Phase 3: Agent(webtoon-artist) → regenerate failed panels (maximum 2 times)
```

### Agent Composition

| Agent | subagent_type | Role | Skill |
|---------|--------------|------|------|
| webtoon-artist | custom | generate panel images | generate-webtoon |
| webtoon-reviewer | custom | quality review | review-webtoon, fix-webtoon-panel |

### Full Agent File Example: `webtoon-reviewer.md`

```markdown
---
name: webtoon-reviewer
description: "Specialist in reviewing webtoon panel quality. Evaluates composition, character consistency, text readability, and direction."
---

# Webtoon Reviewer — Webtoon Quality Review Specialist

You are a specialist in reviewing webtoon panel quality. Evaluate panels against visual completeness, narrative clarity, and character consistency.

## Core Role
1. Evaluate panel composition and visual completeness
2. Verify cross-panel consistency of character appearance
3. Evaluate readability and placement of speech balloon text
4. Review pacing and scene direction across the full episode

## Work Principles
- Decide clearly with PASS/FIX/REDO
- Use FIX when partial repair is enough, REDO when full regeneration is required
- Judge by objective criteria such as consistency, readability, and composition, not personal taste

## Input/Output Protocol
- Input: panel images in `_workspace/panels/`
- Output: `_workspace/review_report.md`
- Format:
  ```
  ## Panel {N}
  - Decision: PASS | FIX | REDO
  - Reason: [concrete reason]
  - Revision Instruction: [concrete direction if FIX or REDO]
  ```

## Error Handling
- If image load fails, mark that panel as REDO
- If a panel remains REDO after 2 regeneration attempts, mark it PASS with warning

## Collaboration
- Deliver revision instructions to webtoon-artist (file-based result)
- Review regenerated panels again (maximum 2 loops)
```

### Error Handling

```
Retry policy:
- REDO panel → request regeneration from artist (include concrete revision instruction)
- maximum 2 loops, then force PASS
- if more than 50% of all panels are REDO, propose prompt revision to the user
```

---

## Example 4: Code Review Team (Agent Team Mode)

### Team Architecture: Fan-out/Fan-in + Discussion
### Execution Mode: Agent Team

> Code review is a primary example where agent teams outperform isolated agents. Reviewers with different perspectives share findings and challenge each other, which produces deeper review quality.

```
[Leader] → TeamCreate(review-team)
    ├── security-reviewer: inspect security vulnerabilities
    ├── performance-reviewer: analyze performance impact
    └── test-reviewer: verify test coverage
    → reviewers share findings with each other (SendMessage)
    → leader synthesizes results
```

### Team Communication Pattern

```
security ──SendMessage──→ performance  ("This SQL query may be injectable. Check performance implications too.")
performance ──SendMessage──→ test      ("Found N+1 queries. Confirm whether related tests exist.")
test ────SendMessage──→ security      ("Authentication module has no tests. Confirm security priority.")
```

Core rule: reviewers communicate **directly without routing through the leader** so cross-domain issues are identified quickly.

---

## Example 5: Supervisor Pattern — Code Migration Team (Agent Team Mode)

### Team Architecture: Supervisor
### Execution Mode: Agent Team

```
[supervisor/leader] → analyze file list → assign batches
    ├→ [migrator-1] (batch A)
    ├→ [migrator-2] (batch B)
    └→ [migrator-3] (batch C)
    ← receive TaskUpdate → assign additional batches or reassign
```

### Agent Composition

| Member | Role |
|------|------|
| (leader = migration-supervisor) | file analysis, batch distribution, progress management |
| migrator-1~3 | migrate assigned file batches |

### Supervisor Dynamic Assignment Logic (Agent Team)

```
1. Collect the full target file list
2. Estimate complexity (file size, import count, dependency count)
3. Register file batches as tasks through TaskCreate (including dependencies)
4. Members claim tasks themselves
5. When a member reports completion with TaskUpdate:
   - success → request next task automatically
   - failure → leader confirms cause through SendMessage → reassign or transfer to another member
6. After all tasks complete → leader runs integration tests
```

Difference from fan-out: assignment is not fixed in advance. It is **allocated dynamically at runtime**. The shared task list claim model aligns naturally with the supervisor pattern.

---

## Artifact Pattern Summary

### Agent Definition Files
Location: `project/.claude/agents/{agent-name}.md`
Required sections: core role, work principles, input/output protocol, error handling, collaboration
Additional section for team mode: **Team Communication Protocol** (message intake/output, task request scope)

### Skill File Structure
Location: `project/.claude/skills/{skill-name}/skill.md` (project level)
or: `~/.claude/skills/{skill-name}/skill.md` (global level)

### Integrated Skill (Orchestrator)
Top-level skill that coordinates the full team. Defines scenario-specific agent composition and workflow.
Template: see `references/orchestrator-template.md`.
**Execution mode must be declared explicitly** — Agent Team (default) or Sub-agent.
