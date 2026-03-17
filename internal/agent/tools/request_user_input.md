# request_user_input Tool

Codex-style structured question tool for Plan Mode.

**Reference:** `codex-rs/core/src/tools/handlers/request_user_input.rs`

---

## When to Use

Use `request_user_input` in **Plan Mode** when:

1. **There is genuine ambiguity** about user intent or preferences
2. **You need to confirm assumptions** before finalizing the plan
3. **There are meaningful tradeoffs** to choose between
4. **The question cannot be answered** through repo exploration

**Do NOT use** for:
- Questions answerable from the repo/system
- Questions that exploration can resolve
- More than 3 questions at once

---

## Requirements (Codex-Strict)

### Question Count
- **Minimum:** 1 question
- **Maximum:** 3 questions

### Options (Multiple-Choice)
- **Required:** Every question MUST have options
- **Minimum:** 2 options per question
- **Recommended:** 2-4 mutually exclusive options
- **Format:** Clear, distinct choices (no filler)

### Response Format
```json
{
  "questions": [
    {
      "question": "What authentication method do you prefer?",
      "options": ["JWT tokens", "OAuth 2.0", "Session-based", "API keys"],
      "is_other": true
    },
    {
      "question": "Which database should we use?",
      "options": ["PostgreSQL", "MySQL", "SQLite"],
      "is_other": false
    }
  ]
}
```

---

## Plan Mode Rules

**Availability:** ONLY available in Plan Mode

In other modes, returns error:
```
"request_user_input is only available in Plan Mode"
```

---

## Examples

### Good Example (Clear Tradeoffs)
```json
{
  "questions": [
    {
      "question": "What authentication method?",
      "options": ["JWT tokens", "OAuth 2.0", "Session-based"]
    }
  ]
}
```

### Bad Example (Answerable from Repo)
```json
{
  "questions": [
    {
      "question": "Where is the user model defined?",
      "options": ["users.go", "models.go", "auth.go"]
    }
  ]
}
```
**Why bad:** This can be answered via `glob` or `grep`, not a preference question.

### Bad Example (Empty Options)
```json
{
  "questions": [
    {
      "question": "What do you think?",
      "options": []
    }
  ]
}
```
**Why bad:** Options are required and must be non-empty.

---

## Response Handling

User responses are captured and returned to the model as:
```json
{
  "answers": ["JWT tokens", "PostgreSQL"]
}
```

Or for free-form "other" responses:
```json
{
  "answers": ["Custom SSO integration"]
}
```

---

## Codex Alignment

This tool implements the exact behavior from:
- `codex-rs/core/src/tools/handlers/request_user_input.rs`
- Validation: 1-3 questions, non-empty options required
- Mode restriction: Plan Mode only
- Response: JSON-serialized answers
