# Skill Testing & Iterative Improvement Guide

Methodology for verifying and iteratively improving the quality of skills created by the harness. Supplementary reference for Phase 6 in `SKILL.md`.

---

## Table of Contents

1. [Test Framework Overview](#1-test-framework-overview)
2. [How to Write Test Prompts](#2-how-to-write-test-prompts)
3. [Execution Test: With-skill vs Baseline](#3-execution-test-with-skill-vs-baseline)
4. [Quantitative Evaluation: Assertion-Based Scoring](#4-quantitative-evaluation-assertion-based-scoring)
5. [Use of Specialist Agents](#5-use-of-specialist-agents)
6. [Iterative Improvement Loop](#6-iterative-improvement-loop)
7. [Description Trigger Verification](#7-description-trigger-verification)
8. [Workspace Structure](#8-workspace-structure)

---

## 1. Test Framework Overview

Skill quality verification uses a combination of **qualitative evaluation** and **quantitative evaluation**.

| Evaluation type | Method | Appropriate skills |
|----------|------|-----------|
| **Qualitative** | user reviews the output directly | writing style, design, creative output, and other subjective quality |
| **Quantitative** | assertion-based automatic scoring | file creation, data extraction, code generation, and other objectively verifiable work |

Core loop: **write → run tests → evaluate → improve → re-test**

---

## 2. How to Write Test Prompts

### Principle

Test prompts must be **concrete, natural sentences that a real user would plausibly enter**. Abstract or artificial prompts have low test value.

### Bad Examples

```
"Process a PDF"
"Extract data"
"Generate a chart"
```

### Good Examples

```
"In the file 'Q4_revenue_final_v2.xlsx' in the Downloads folder, use column C
(revenue) and column D (cost) to add a profit_margin_pct column. Then sort by
profit margin in descending order."
```

```
"Extract the table on page 3 of this PDF and convert it to CSV. The table header
has 2 rows, where the first row is the category and the second row is the real
column name."
```

### Prompt Diversity

- mix **formal / casual** tone
- mix **explicit / implicit** intent (directly naming the file type vs requiring inference from context)
- mix **simple / complex** tasks
- include some abbreviations, typos, and casual wording

### Coverage

Start with 2 to 3 prompts, but cover:
- 1 core use case
- 1 edge case
- (optional) 1 composite task

---

## 3. Execution Test: With-skill vs Baseline

### 3-1. Comparison Execution Structure

For each test prompt, spawn two sub-agents **at the same time**:

**With-skill run:**
```
Prompt: "{test prompt}"
Skill path: {skill path}
Output path: _workspace/iteration-N/eval-{id}/with_skill/outputs/
```

**Baseline run:**
```
Prompt: "{test prompt}"  (same prompt)
Skill: none
Output path: _workspace/iteration-N/eval-{id}/without_skill/outputs/
```

### 3-2. Baseline Selection

| Situation | Baseline |
|------|----------|
| new skill creation | run the same prompt without the skill |
| existing skill improvement | the pre-change skill version (preserve a snapshot) |

### 3-3. Capture Timing Data

When the sub-agent completion notification arrives, save `total_tokens` and `duration_ms` **immediately**. This data is only accessible at notification time and cannot be recovered later.

```json
{
  "total_tokens": 84852,
  "duration_ms": 23332,
  "total_duration_seconds": 23.3
}
```

---

## 4. Quantitative Evaluation: Assertion-Based Scoring

### 4-1. Write Assertions

If the output is objectively verifiable, define assertions for automatic scoring.

**Good assertions:**
- objectively pass/fail
- named clearly enough that the result alone shows what was checked
- verify the core value of the skill

**Bad assertions:**
- always pass regardless of skill presence (example: "output exists")
- require subjective judgment (example: "well written")

### 4-2. Prefer Programmable Verification

If an assertion can be checked by code, implement it as a script. Scripted verification is faster, more reliable, and reusable across iterations.

### 4-3. Watch for Non-discriminating Assertions

If an assertion passes 100% in both configurations, it does not measure the differentiating value of the skill. Remove it or replace it with a more demanding assertion.

### 4-4. Scoring Result Schema

```json
{
  "expectations": [
    {
      "text": "profit margin column is added",
      "passed": true,
      "evidence": "confirmed `profit_margin_pct` in column E"
    },
    {
      "text": "sorted by profit margin descending",
      "passed": false,
      "evidence": "original order remained, no sort applied"
    }
  ],
  "summary": {
    "passed": 1,
    "failed": 1,
    "total": 2,
    "pass_rate": 0.50
  }
}
```

---

## 5. Use of Specialist Agents

Using specialist roles during testing and evaluation improves quality.

### 5-1. Grader

Runs assertion-based scoring and extracts verifiable claims from the output for cross-checking.

**Role:**
- decide pass/fail for each assertion and provide evidence
- extract factual claims from the output and verify them
- provide feedback on evaluation quality itself (example: assertion is too easy or ambiguous)

### 5-2. Comparator

Anonymizes two outputs as A/B and judges quality without knowing which one used the skill.

**Use when:** strict confirmation is needed that the new version is actually better. This is optional for standard iterative improvement.

**Judgment criteria:**
- content: accuracy, completeness
- structure: organization, formatting, usability
- overall score

### 5-3. Analyzer

Analyzes statistical patterns in benchmark data:
- non-discriminating assertions (both configurations pass → no differentiation)
- high-variance evaluations (results vary significantly between runs → unstable)
- time/token tradeoffs (skill improves quality but increases cost)

---

## 6. Iterative Improvement Loop

### 6-1. Collect Feedback

Show the output to the user and collect feedback. Treat empty feedback as "no issue."

### 6-2. Improvement Rules

1. **Generalize feedback** — a narrow fix that only matches one test example is overfitting. Fix at the principle level.
2. **Remove weight that adds no value** — read the transcript. If the skill causes the agent to do unproductive work, delete that part.
3. **Explain why** — even if user feedback is short, understand why it matters and encode that reasoning into the skill.
4. **Bundle repeated work** — if every test run generates the same helper script, include it in `scripts/` in advance.

### 6-3. Iteration Procedure

```
1. Modify the skill
2. Re-run all test cases in a new `iteration-N+1/` directory
3. Present results to the user (compare against the previous iteration)
4. Collect feedback
5. Modify again → repeat
```

**Stop conditions:**
- the user is satisfied
- all feedback is empty (no issues in any output)
- no meaningful improvement remains

### 6-4. Draft → Review Pattern

When modifying a skill, write a draft, then **read it again from a fresh perspective** and improve it. Do not attempt a perfect version in one pass. Use a draft-review cycle.

---

## 7. Description Trigger Verification

### 7-1. Write Trigger Evaluation Queries

Write 20 evaluation queries — 10 `should-trigger` + 10 `should-NOT-trigger`.

**Query quality rules:**
- concrete, natural sentences that a real user would plausibly enter
- include concrete details such as file paths, personal context, column names, or company names
- mix length, tone, and format
- focus on **boundary cases (edge cases)** rather than only obvious answers

**Should-trigger queries (8~10):**
- same intent phrased in different ways (formal/casual)
- cases where the skill or file type is not stated explicitly but is clearly required
- non-mainstream use cases
- cases where another skill competes but this skill must win

**Should-NOT-trigger queries (8~10):**
- **Near-miss is the core requirement** — keywords are similar, but another tool or skill is correct
- clearly unrelated queries such as "write a Fibonacci function" have no test value
- use adjacent domains, ambiguous wording, and overlapping keywords with different context

### 7-2. Check Conflicts with Existing Skills

Confirm that the new skill description does not overlap the trigger scope of existing skills:

1. Collect descriptions from the existing skill list
2. Confirm that `should-trigger` queries for the new skill do not incorrectly trigger existing skills
3. If conflict is found, make the boundary conditions in the description more explicit

### 7-3. Automatic Optimization (Optional Advanced Feature)

If description optimization is required:

1. Split the 20 evaluation queries into Train (60%) / Test (40%)
2. Measure trigger accuracy with the current description
3. Analyze failures and generate an improved description
4. Select the best description on the Test set, not the Train set (prevents overfitting)
5. Repeat up to 5 times

> Run this process with an automation script that uses `claude -p`. Token cost is high. Use it only at the final stage after the skill is already stable.

---

## 8. Workspace Structure

Directory structure for managing test and evaluation results systematically:

```
{skill-name}-workspace/
├── iteration-1/
│   ├── eval-descriptive-name-1/
│   │   ├── eval_metadata.json
│   │   ├── with_skill/
│   │   │   ├── outputs/
│   │   │   ├── timing.json
│   │   │   └── grading.json
│   │   └── without_skill/
│   │       ├── outputs/
│   │       ├── timing.json
│   │       └── grading.json
│   ├── eval-descriptive-name-2/
│   │   └── ...
│   └── benchmark.json
├── iteration-2/
│   └── ...
└── evals/
    └── evals.json
```

**Rules:**
- evaluation directories use **descriptive names**, not numbers (example: `eval-multi-page-table-extraction`)
- preserve each iteration in an independent directory. Do not overwrite previous iterations
- do not delete `_workspace/` — it is required for post-run verification and audit traceability
