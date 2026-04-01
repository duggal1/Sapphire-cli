# Sapphire Singularity: Measured State

## Definition

**Sapphire singularity** in this repo means:

- fixed model weights
- self-improving harness
- learned routing
- learned guardrails
- learned verification requirements
- durable reuse of successful task patterns

It does **not** mean AGI.
It does **not** mean model-weight self-training.

## What Existed Before

At the earlier `v4` checkpoint, Sapphire already had:

- durable route policy storage
- turn audit logs
- auto-generated local skills
- replay of learned policies
- one narrow runtime inefficiency block

What it still lacked in practice:

- deterministic harness-first cold start
- reliable same-turn recovery on broad design/research tasks
- repo-grounding checks for hallucinated symbols
- strong completion gates for evidence and planning
- strong broad implementation classification

## What Was Added After That

Real logic added in code:

- experience compilation: [`experience_compiler.go`](/Users/harshitduggal/workspace/Sapphire-cli/internal/agent/experience_compiler.go)
- repo-grounding verifier: [`repo_grounding_verifier.go`](/Users/harshitduggal/workspace/Sapphire-cli/internal/agent/repo_grounding_verifier.go)
- context evidence tracking: [`context_evidence.go`](/Users/harshitduggal/workspace/Sapphire-cli/internal/agent/tools/context_evidence.go)
- runtime preflight rewrites and guardrails: [`tool_call_preflight.go`](/Users/harshitduggal/workspace/Sapphire-cli/internal/agent/tools/tool_call_preflight.go)
- bounded completion recovery loop: [`agent.go`](/Users/harshitduggal/workspace/Sapphire-cli/internal/agent/agent.go)
- broader cold-start cognitive policy: [`singularity_cognitive.go`](/Users/harshitduggal/workspace/Sapphire-cli/internal/agent/singularity_cognitive.go)
- policy compilation and audit enrichment: [`singularity_learning.go`](/Users/harshitduggal/workspace/Sapphire-cli/internal/agent/singularity_learning.go)

These are harness changes, not prompt cosmetics.

## Hard Evidence

### 1. Recursive learning is real on repeated task families

Gemini broad-init canary progression:

| Turn | State | Confidence | Applied Count | Observed Route |
|---|---:|---:|---:|---|
| 1 | candidate | 58 | 0 | `ls -> ls -> agentic_view -> ls` |
| 2 | candidate | 87 | 1 | learned policy loaded |
| 3 | promoted | 95 | 2 | promoted |
| 4 | promoted | 95 | 3 | `ls -> rg_files -> ls -> agentic_view` |

Source artifacts:

- audit: [/tmp/singularity-testing-v4/data/singularity/turn_audit.jsonl](/tmp/singularity-testing-v4/data/singularity/turn_audit.jsonl)
- policy: [/tmp/singularity-testing-v4/data/singularity/route_policies.json](/tmp/singularity-testing-v4/data/singularity/route_policies.json)

What this proves:

- Sapphire learned from prior turns
- persisted the policy
- promoted the policy
- reused it later
- improved tool routing on the same task family

That is the clearest proof of a narrow recursive learning loop.

### 2. Broad init is materially stronger than before

Earlier strong Gemini init pass:

- status: `completed`
- confidence: `68`
- validation checks: `3`
- route: `run_harness -> tool_search -> rg_files -> agentic_view -> update_plan -> agentic_edit -> update_plan -> single_view -> agentic_view -> update_plan`

Source:

- audit: [/tmp/singularity-testing-v8.4J564H/data-flash-e15/singularity/turn_audit.jsonl](/tmp/singularity-testing-v8.4J564H/data-flash-e15/singularity/turn_audit.jsonl)
- policy: [/tmp/singularity-testing-v8.4J564H/data-flash-e15/singularity/route_policies.json](/tmp/singularity-testing-v8.4J564H/data-flash-e15/singularity/route_policies.json)

Current Qwen init pass:

- status: `completed`
- confidence: `74`
- `context_discipline: strong`
- `planning_discipline: strong`
- `validation_discipline: strong`
- `require_harness: true`
- `require_context_read: true`
- `require_post_write_verification: true`

Source:

- audit: [/tmp/qwen-singularity-canary/data-init-2/singularity/turn_audit.jsonl](/tmp/qwen-singularity-canary/data-init-2/singularity/turn_audit.jsonl)
- policy: [/tmp/qwen-singularity-canary/data-init-2/singularity/route_policies.json](/tmp/qwen-singularity-canary/data-init-2/singularity/route_policies.json)

### 3. Broad implementation no longer collapses on the first wrong move

Current live Qwen implementation canary:

- family: `implementation/broad/backend+docs+frontend+infra+security`
- status: `completed`
- confidence: `56`
- `context_discipline: strong`
- `planning_discipline: strong`
- `validation_discipline: strong`
- route recovered from a wrong first move into:
  - harness
  - structured discovery
  - edit
  - verification

Observed route:

`agentic_view -> update_plan -> run_harness -> update_plan -> tool_search -> agentic_view -> single_edit -> bash -> bash -> job_output -> update_plan`

Source:

- audit: [/tmp/qwen-singularity-canary/data-impl-4/singularity/turn_audit.jsonl](/tmp/qwen-singularity-canary/data-impl-4/singularity/turn_audit.jsonl)
- policy: [/tmp/qwen-singularity-canary/data-impl-4/singularity/route_policies.json](/tmp/qwen-singularity-canary/data-impl-4/singularity/route_policies.json)

### 4. Broad design and research now self-recover instead of just failing closed

Qwen broad design:

- status: `completed`
- family: `design/broad/backend+security`
- confidence: `42`
- `context_discipline: strong`
- `planning_discipline: strong`
- `validation_discipline: weak`

Routes:

- run 1: `run_harness -> tool_search -> rg_files -> agentic_view -> update_plan -> update_plan`
- run 2: `run_harness -> wc -> agentic_view -> tool_search -> update_plan -> grep -> agentic_view -> update_plan`

Sources:

- [/tmp/qwen-singularity-canary/data-design-6/singularity/turn_audit.jsonl](/tmp/qwen-singularity-canary/data-design-6/singularity/turn_audit.jsonl)
- [/tmp/qwen-singularity-canary/data-design-7/singularity/turn_audit.jsonl](/tmp/qwen-singularity-canary/data-design-7/singularity/turn_audit.jsonl)

Qwen broad research:

- status: `completed`
- family: `research/broad/backend+infra`
- confidence: `42`
- correctly stayed grounded on the nonexistent `platform.NewRuntimeConfig` claim

Source:

- audit: [/tmp/qwen-singularity-canary/data-research-1/singularity/turn_audit.jsonl](/tmp/qwen-singularity-canary/data-research-1/singularity/turn_audit.jsonl)
- policy: [/tmp/qwen-singularity-canary/data-research-1/singularity/route_policies.json](/tmp/qwen-singularity-canary/data-research-1/singularity/route_policies.json)

This matters because earlier shallow design/research turns could pass with weak grounding.
Now they either recover or are rejected.

## Measured Startup Latency

Sapphire-side headless cold start to first outbound model request:

- `RunNonInteractive` start: `21:24:12.693829`
- session created: `21:24:12.696318`
- first outbound model request: `21:24:12.924784`
- Sapphire-side startup cost: about `231ms`

Source:

- log: [/tmp/singularity-latency-v1.nvN4bJ/data/logs/sapphire.log](/tmp/singularity-latency-v1.nvN4bJ/data/logs/sapphire.log)

This does **not** include provider latency.
It only proves Sapphire itself is no longer the main cold-start bottleneck.

## What The Benchmarks Actually Prove

They prove:

- recursive harness learning is real in a narrow form
- policy confidence can rise across repeated runs
- learned policies can be promoted and reused
- generated skills can be created and loaded
- broad tasks can now be fail-closed or auto-recovered instead of silently passing
- repo-grounding checks can block hallucinated symbol claims
- completion now depends more on evidence, planning, and verification than before

They do **not** prove:

- general autonomous architecture mastery
- general wrong-objective correction
- general wrong-tradeoff correction
- model-weight learning
- reliable self-improvement on every task family

## Current State, Painfully Stated

What is real now:

- **Harness singularity:** real, narrow, measurable
- **Recursive self-learning:** real for routing, guardrails, evidence requirements, and reuse
- **Runtime recovery:** real
- **Repo-grounding:** real

What is still weak:

- design/research `validation_discipline` is still often `weak`
- policy confidence outside broad init is still modest: `42` to `56`
- Qwen free latency is still variable
- higher-order architecture judgment is still mostly the base model, not the harness

## Bottom Line

From the earlier `v4` state to now, Sapphire moved from:

- memory + policy storage + narrow routing hints

to:

- evidence-aware policy learning
- runtime route correction
- bounded same-turn self-recovery
- repo-grounding verification
- completion gates based on context and planning

That is **real progress**.
It is **not cosmetic**.
It is also **not full singularity**.

The honest description is:

**Sapphire now has a real, bounded recursive harness-improvement loop.**
