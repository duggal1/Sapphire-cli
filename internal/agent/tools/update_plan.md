# update_plan Tool

Updates the task plan. Provide an optional explanation and a list of plan items with their current status.

## When to Use

Use this tool when:
- Starting a new complex task that requires multiple steps
- The scope of work changes or new information emerges
- You complete a significant milestone
- You need to communicate progress to the user

## When NOT to Use

Do NOT use this tool for:
- Simple, single-step tasks (can be completed in one tool call)
- Trivial updates that don't require tracking
- When already in Plan mode

## Plan Item Guidelines

Each plan item should:
- Be one sentence (max 5-7 words)
- Describe a verifiable, concrete step
- Have a clear completion criterion
- Be logically ordered (dependencies first)

## Status Values

- `pending`: Not started yet
- `in_progress`: Currently working on (at most ONE step can be in_progress)
- `completed`: Fully finished and verified

## Best Practices

1. **Create the plan early**: Call update_plan before starting complex work
2. **Keep it short**: 5-7 steps maximum, each step 5-7 words
3. **Update progress**: Mark steps as in_progress when starting, completed when done
4. **Full list every time**: Always send the complete, current plan. Do not omit existing steps.
5. **One active step**: Only one step should be in_progress at a time
6. **Verify completion**: Mark steps completed only after verification
7. **Don't repeat**: The harness displays the plan - don't repeat it in your response

## Example

```json
{
  "explanation": "Breaking down the authentication feature",
  "plan": [
    {"step": "Design database schema for users", "status": "completed"},
    {"step": "Implement user registration endpoint", "status": "in_progress"},
    {"step": "Add login with JWT tokens", "status": "pending"},
    {"step": "Write integration tests", "status": "pending"},
    {"step": "Update API documentation", "status": "pending"}
  ]
}
```
