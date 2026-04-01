# Skill Writing Guide

Detailed writing guide for improving the quality of skills created by the harness. Supplementary reference for Phase 4 in `SKILL.md`.

---

## Table of Contents

1. [Description Writing Patterns](#1-description-writing-patterns)
2. [Body Writing Style](#2-body-writing-style)
3. [Output Format Definition Patterns](#3-output-format-definition-patterns)
4. [Example Writing Patterns](#4-example-writing-patterns)
5. [Progressive Disclosure Patterns](#5-progressive-disclosure-patterns)
6. [Criteria for Bundling Scripts](#6-criteria-for-bundling-scripts)
7. [Data Schema Standards](#7-data-schema-standards)
8. [What Not to Include in a Skill](#8-what-not-to-include-in-a-skill)

---

## 1. Description Writing Patterns

The description is the primary trigger mechanism for a skill. Sapphire routes from the local skill inventory using the skill name and description. Extended install is fallback only, not the default discovery path.

### Understand the Trigger Mechanism

Sapphire tends not to call a skill for simple work it believes it can handle with its default tools. A simple request such as "read this PDF" may not trigger even with a perfect description. Complex, multi-step, specialist work is far more likely to trigger the skill.

### Writing Rules

1. State both **what the skill does** and **the specific trigger situations**
2. State the boundary conditions that distinguish similar cases which must not trigger the skill
3. Be slightly aggressive to compensate for Sapphire's conservative trigger behavior

### Good Examples

```yaml
description: "Handles all PDF work, including reading PDF files, extracting text
  and tables, merging, splitting, rotating, watermarking, encryption/decryption,
  and OCR. If the user mentions a .pdf file or requests a PDF output, this skill
  must be used. It is especially useful when the task requires conversion,
  editing, or analysis rather than merely 'reading' a PDF."
```

```yaml
description: "Handles all spreadsheet work for Excel/CSV/TSV files, including
  adding columns, formula calculation, formatting, chart creation, and data
  cleanup. If the user mentions a spreadsheet file, even casually ('the xlsx in
  Downloads'), use this skill."
```

### Bad Examples

- `"A skill for processing data"` — too vague, unclear file type and unclear task type
- `"PDF-related tasks"` — no concrete actions listed, no trigger conditions stated

---

## 2. Body Writing Style

### Why-First Principle

If the LLM understands the reason, it makes better decisions on edge cases. Context is more effective than unsupported forceful rules.

**Bad example:**
```markdown
ALWAYS use pdfplumber for table extraction. NEVER use PyPDF2 for tables.
```

**Good example:**
```markdown
Use pdfplumber for table extraction. PyPDF2 is optimized for text extraction and
does not preserve row/column structure in tables. pdfplumber detects cell
boundaries and returns structured data.
```

### Generalization Principle

When feedback or test results reveal a problem, fix it at the **principle level** instead of making a narrow rule that only matches one example.

**Overfit fix:**
```markdown
If a column named "Q4 Revenue" exists, convert that column to numeric.
```

**Generalized fix:**
```markdown
If a column name contains keywords that imply numeric values, such as "revenue",
"amount", or "quantity", convert that column to numeric. If conversion fails,
preserve the original value.
```

### Imperative Tone

Use direct instruction form, not soft phrasing. A skill is an instruction document.

### Conserve Context

The context window is shared infrastructure. Every sentence must justify its token cost:
- "Does Sapphire already know this?" → delete it
- "Will Sapphire make a mistake without this explanation?" → keep it
- "Will one concrete example be better than a long explanation?" → replace with an example

---

## 3. Output Format Definition Patterns

Use this when output format matters:

```markdown
## Report Structure
Follow this template exactly:

# [Title]
## Summary
## Key Findings
## Recommendations
```

Keep format definitions concise. A concrete example is usually more effective.

---

## 4. Example Writing Patterns

Examples are often more effective than long explanations:

```markdown
## Commit Message Format

**Example 1:**
Input: add user authentication using JWT tokens
Output: feat(auth): implement JWT-based authentication

**Example 2:**
Input: fix the password visibility button that does not work on the login page
Output: fix(login): fix password visibility toggle button behavior
```

---

## 5. Progressive Disclosure Patterns

### Pattern 1: Split by Domain

```
bigquery-skill/
├── SKILL.md (overview + domain selection guide)
└── references/
    ├── finance.md (revenue, billing metrics)
    ├── sales.md (opportunities, pipeline)
    └── product.md (API usage, features)
```

If the user asks about revenue, load only `finance.md`.

### Pattern 2: Conditional Detail

```markdown
# DOCX Processing

## Document Creation
Create new documents with docx-js. → see [DOCX-JS.md](references/docx-js.md).

## Document Editing
For simple edits, modify XML directly.
**If tracked changes are required**: see [REDLINING.md](references/redlining.md)
```

### Pattern 3: Large Reference File Structure

Reference files longer than 300 lines must include a table of contents at the top:

```markdown
# API Reference

## Table of Contents
1. [Authentication](#authentication)
2. [Endpoint List](#endpoint-list)
3. [Error Codes](#error-codes)
4. [Rate Limits](#rate-limits)

---

## Authentication
...
```

---

## 6. Criteria for Bundling Scripts

Observe agent transcripts during test runs. Bundle when these patterns appear:

| Signal | Action |
|------|------|
| the same helper script is generated in all 3 of 3 tests | bundle it into `scripts/` |
| the same `pip install` or `npm install` runs every time | state dependency installation in the skill |
| the same multi-step approach repeats every time | define it as the standard procedure in the skill body |
| the same error appears repeatedly followed by the same workaround | document the known issue and the resolution in the skill |

Every bundled script must be execution-tested.

---

## 7. Data Schema Standards

Use standard schemas to keep data exchange consistent across skills. These schemas may be used for testing and evaluation of harness-generated skills.

### eval_metadata.json

Metadata for each test case:

```json
{
  "eval_id": 0,
  "eval_name": "descriptive-name-here",
  "prompt": "user task prompt",
  "assertions": [
    "output contains X",
    "file is created in Y format"
  ]
}
```

### grading.json

Assertion-based scoring result:

```json
{
  "expectations": [
    {
      "text": "output includes 'Seoul'",
      "passed": true,
      "evidence": "confirmed 'extract Seoul region data' in step 3"
    }
  ],
  "summary": {
    "passed": 2,
    "failed": 1,
    "total": 3,
    "pass_rate": 0.67
  }
}
```

**Field name rule:** use `text`, `passed`, and `evidence` exactly. Do not substitute variants such as `name`, `met`, or `details`.

### timing.json

Execution time and token metrics:

```json
{
  "total_tokens": 84852,
  "duration_ms": 23332,
  "total_duration_seconds": 23.3
}
```

Save available execution metadata as soon as the worker run completes and the result is collected.

---

## 8. What Not to Include in a Skill

- supplementary documents such as `README.md`, `CHANGELOG.md`, and `INSTALLATION_GUIDE.md`
- meta-information about the skill creation process, such as test results and iteration history
- documentation intended for end users (a skill is an instruction document for AI agents)
- general knowledge Sapphire already knows
