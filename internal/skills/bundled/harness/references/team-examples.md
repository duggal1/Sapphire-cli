# Agent Team Examples

---

## Example 1: Research Team (Agent Team Mode)

### Team Architecture: Fan-out/Fan-in
### Execution Mode: Agent Team

```text
[Leader/Orchestrator]
    ├── spawn_agent x 4
    ├── agent_directory
    ├── agent_mail_* for peer coordination
    ├── wait / collect_result
    └── Create integrated report
```

### Agent Composition

| Member | Profile | Role | Output |
|------|-------------|------|------|
| official-researcher | task | official documents/blog | research_official.md |
| media-researcher | task | media/investment | research_media.md |
| community-researcher | task | community/social | research_community.md |
| background-researcher | task | background/competition/academic | research_background.md |
| (leader = orchestrator) | — | integrated report | integrated_report.md |

> Research workers use profile `task`. Specialization is expressed through the `spawn_agent.message` and loaded skills, not a custom agent-definition registry.

### Orchestrator Workflow (Agent Team)

```text
Phase 1: Preparation
  - Analyze user input
  - Create _workspace/

Phase 2: Spawn Workers
  - spawn_agent(official-researcher)
  - spawn_agent(media-researcher)
  - spawn_agent(community-researcher)
  - spawn_agent(background-researcher)
  - each worker owns one artifact path under _workspace/

Phase 3: Research Execution
  - all 4 workers investigate independently
  - if one worker finds relevant evidence for another, send it by agent_mail_send
  - all workers write artifacts to _workspace/

Phase 4: Integration
  - wait
  - collect_result
  - leader reads 4 artifacts
  - create integrated report

Phase 5: Cleanup
  - close_agent for all 4 workers
  - preserve _workspace/
```

### Team Communication Pattern

```text
official ──agent_mail_send──→ background
media ────agent_mail_send──→ background
community ─agent_mail_send──→ media
leader ───agent_directory──→ inspect status and route aliases
```

---

## Example 2: SF Novel Writing Team (Hybrid Team)

### Team Architecture: Pipeline + Fan-out
### Execution Mode: Agent Team + Isolated Worker

```text
Phase 1 (parallel): worldbuilder + character-designer + plot-architect
  → coordinate consistency by mail
Phase 2 (sequential): prose-stylist isolated worker
Phase 3 (parallel): science-consultant + continuity-manager
Phase 4 (sequential): prose-stylist isolated worker applies review
```

### Agent Composition

| Member | Profile | Role | Skill |
|------|-------------|------|------|
| worldbuilder | task | worldbuilding | world-setting |
| character-designer | task | character design | character-profile |
| plot-architect | task | plot structure | outline |
| prose-stylist | coder | drafting and revision | write-scene, review-chapter |
| science-consultant | task | science validation | science-check |
| continuity-manager | task | continuity validation | consistency-check |

### Example Worker Packet: `worldbuilder`

```markdown
# Worker: worldbuilder

## Profile
task

## Core Role
1. Define the physical and technological foundation of the world
2. Define the social and political structure
3. Write a setting artifact that downstream workers can consume

## Owned Scope
- `_workspace/01_worldbuilder_setting.md`
- no drafting or prose revision

## Skills To Load
- world-setting

## Blocker Protocol
- send mail to character-designer when social structure is stable
- send mail to plot-architect when conflict structure is stable

## Definition Of Done
- setting artifact is complete, internally consistent, and usable by downstream workers
```

### Detailed Team Workflow

```text
Phase 1:
  - spawn_agent(worldbuilder)
  - spawn_agent(character-designer)
  - spawn_agent(plot-architect)
  - workers coordinate by agent_mail_send and write artifacts to _workspace/

Phase 2:
  - close Phase 1 workers
  - run prose-stylist as an isolated worker against the Phase 1 artifacts

Phase 3:
  - spawn_agent(science-consultant)
  - spawn_agent(continuity-manager)
  - both review the draft and mail findings when overlap matters

Phase 4:
  - close reviewers
  - run prose-stylist again to apply review results
```

---

## Example 3: Webtoon Production Team (Producer-Reviewer)

### Team Architecture: Producer-Reviewer
### Execution Mode: Isolated Worker or Small Team

> When only one producer and one reviewer are needed, isolated workers are often sufficient.

```text
Phase 1: spawn_agent(webtoon-artist)
Phase 2: spawn_agent(webtoon-reviewer)
Phase 3: if needed, respawn or re-input artist with revision instructions
```

### Agent Composition

| Worker | Profile | Role | Skill |
|---------|--------------|------|------|
| webtoon-artist | coder | generate panels | generate-webtoon |
| webtoon-reviewer | task | review quality | review-webtoon, fix-webtoon-panel |

### Example Review Worker Packet

```markdown
# Worker: webtoon-reviewer

## Profile
task

## Core Role
1. Evaluate panel composition and visual completeness
2. Verify consistency across panels
3. Report PASS | FIX | REDO with evidence

## Output Contract
- write `_workspace/review_report.md`
- include concrete panel-level revision instructions

## Retry Rule
- allow at most 2 regenerate loops before escalation
```

### Error Handling

```text
Retry policy:
- REDO panel → send concrete revision instructions back to artist
- maximum 2 loops
- if more than 50% of panels fail, escalate prompt-level revision to the user
```

---

## Example 4: Code Review Team (Agent Team Mode)

### Team Architecture: Fan-out/Fan-in + Discussion
### Execution Mode: Agent Team

> Code review is a strong team case because independent reviewers cross-check each other’s findings.

```text
[Leader]
    ├── spawn_agent(security-reviewer)
    ├── spawn_agent(performance-reviewer)
    └── spawn_agent(test-reviewer)
        → reviewers share findings by mail
        → leader synthesizes results
```

### Agent Composition

| Member | Profile | Role |
|------|------|------|
| security-reviewer | task | inspect security risks |
| performance-reviewer | task | inspect performance impact |
| test-reviewer | task | inspect test coverage and test gaps |

### Team Communication Pattern

```text
security ──agent_mail_send──→ performance
performance ──agent_mail_send──→ test
test ────agent_mail_send──→ security
```

Core rule: reviewers communicate directly so cross-domain issues surface early.

---

## Example 5: Supervisor Pattern — Code Migration Team

### Team Architecture: Supervisor
### Execution Mode: Agent Team

```text
[supervisor/leader] → analyze file list → assign batches
    ├→ [migrator-1] (batch A)
    ├→ [migrator-2] (batch B)
    └→ [migrator-3] (batch C)
    ← agent_directory / collect_result / mail-driven reassignment
```

### Agent Composition

| Member | Profile | Role |
|------|------|------|
| (leader = migration-supervisor) | — | batch analysis, reassignment, progress management |
| migrator-1~3 | coder | migrate owned file batches |

### Supervisor Dynamic Assignment Logic

```text
1. Collect the target file list
2. Estimate complexity
3. Spawn 3 migrators with disjoint write_manifest scopes
4. Each migrator writes outputs for its owned batch
5. When a batch finishes:
   - success → assign the next batch through send_input
   - failure → confirm the cause and respawn or reroute
6. After all batches complete → run integration verification
```

Difference from fan-out: assignment is adjusted at runtime instead of being fixed once.

---

## Artifact Pattern Summary

### Worker Packet Notes
Location: optional `_workspace/agents/{worker-name}.md` planning artifact or inline in `spawn_agent.message`  
Required sections: profile, owned scope, skills to load, output contract, blocker protocol, definition of done

### Skill File Structure
Location: `<repo-root>/.sapphire/skills/{skill-name}/SKILL.md` (project level)  
or: installed local skill store after `install_skill`

### Integrated Skill (Orchestrator)
Top-level skill that coordinates the full team.  
Template: see `references/orchestrator-template.md`.  
**Execution mode must be declared explicitly** — Agent Team or Isolated Worker.
