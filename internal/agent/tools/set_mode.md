# set_mode Tool

Switch between collaboration modes (Codex-inspired).

## Modes

Based on Codex CLI v0.88.0+ collaboration modes:

### Plan Mode (`plan`)
- **Purpose**: Design-focused mode for thinking and planning
- **Forbidden**: File editing, shell commands, background execution
- **Allowed**: Planning (update_plan), research (view, glob, grep), analysis
- **Use when**: Starting complex tasks, requirements unclear, need to surface dependencies

### Pair Programming Mode (`pair_programming`)
- **Purpose**: Default collaborative mode
- **All tools available**
- **Behavior**: Works in small steps, shares reasoning, collaborates with user
- **Use when**: Implementing features, fixing bugs, iterative development

### Execute Mode (`execute`)
- **Purpose**: Autonomous execution mode
- **All tools available**
- **Behavior**: Makes assumptions, executes end-to-end, minimal questions
- **Use when**: Clear tasks, user wants fast execution, routine operations

## When to Use

Use `set_mode` when:
- Starting a new phase of work that requires different interaction style
- The user explicitly requests a mode change
- You need to switch from planning to execution
- You want to enter plan mode to think before acting

## When NOT to Use

Do NOT use `set_mode` for:
- Every single tool call (mode changes are expensive)
- Trivial mode switches without reason
- When already in the correct mode

## Parameters

- `mode` (required): The mode to switch to
  - `"plan"` - Enter plan mode (thinking/planning only)
  - `"pair_programming"` - Enter pair programming mode (default)
  - `"execute"` - Enter execute mode (autonomous)
- `reason` (optional): Brief explanation for the mode switch

## Example

```json
{
  "mode": "plan",
  "reason": "Need to design the architecture before implementing"
}
```

```json
{
  "mode": "pair_programming",
  "reason": "Plan approved, ready to start implementation"
}
```

## Mode Switching Flow

1. **Plan → Pair Programming**: After plan is created and approved
2. **Any → Plan**: When need to think, design, or clarify requirements
3. **Pair Programming → Execute**: When user wants faster, autonomous work

## Important Notes

- Mode is persisted per session
- Sub-agents inherit parent session mode
- Mode affects tool availability (plan mode restricts editing/execution tools)
- Users can also switch modes via `/plan` command or Shift+Tab
