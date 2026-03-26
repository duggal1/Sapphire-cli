# request_user_input Tool

Codex-style structured question tool for explicit collaboration modes.

**Reference:** `codex-rs/core/src/tools/handlers/request_user_input.rs`

---

## When to Use

Use `request_user_input` in structured collaboration modes such as **Plan**, **Architect**, **Debug**, **Security**, **Review**, and **Orchestrator** when:

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

---

## Collaboration Mode Rules

**Availability:** Only available in explicit collaboration modes that allow structured questions.

In other modes, returns error:
```
"request_user_input is only available in planning/review collaboration modes"
```

---

## Examples

### Good Example
```json
{
  "questions": [
    {
      "question": "What authentication method do you prefer?",
      "options": ["JWT tokens", "OAuth 2.0", "Session-based", "API keys"]
    }
  ]
}
```

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

## Codex Alignment

This tool implements the exact behavior from:
- `codex-rs/core/src/tools/handlers/request_user_input.rs`
- Validation: 1-3 questions, non-empty options required
- Mode restriction: explicit collaboration modes only
