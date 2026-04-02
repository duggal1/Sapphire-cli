---
name: data-pipeline
description: "data pipelineof count, transformation, -based, verification, monitoring inthisbefore teamthis to ·lower  pipeline. 'data pipeline ', 'ETL pipeline ', 'data count automatic-ize', 'data lower pipeline', 'ELT ', ' pipeline', 'tree pipeline', 'Airflow DAG only', 'dbt model ', 'data  verification ' etc. data pipeline · beforein this  for. existing pipelineof  verificationthis monitoringonly necessary inalso supported. , real-time tree (Flink/Spark Streaming) direct execution,  infrastructure provisioning, database administrator(DBA)  this of scope ."
---

# Data Pipeline — data pipeline ·

data pipelineof count→transformation→-based→verification→monitoring inthisbefore teamthis to  in ·.

## execution 

**inthisbefore team** — 5peoplethis SendMessageas direct and  verification.

## inthisbefore setup

| inthisbefore | day | role | type |
|---------|------|------|------|
| etl-architect | `.claude/agents/etl-architect.md` | analysis, schema, pipeline | general-purpose |
| data-quality-manager | `.claude/agents/data-quality-manager.md` | verificationrule, profiling, or moredetection | general-purpose |
| scheduler-engineer | `.claude/agents/scheduler-engineer.md` | DAG, of, retrystrategy | general-purpose |
| monitoring-specialist | `.claude/agents/monitoring-specialist.md` | metric, alert, dashboard, SLA | general-purpose |
| pipeline-reviewer | `.claude/agents/pipeline-reviewer.md` | verification, , operationsalso | general-purpose |

## workflow

### Phase 1:  (this direct count)

1. user from :
    - **data **:  system(DB, API, day, tree)
    - ** system**: data lower, data this, analysis 
    - **business required**:  cycle, SLA, data also
    - ** ** (optional):  vendor, existing stack, budget
    - **existing day** (optional): user provided schema, query, configuration day
2. `_workspace/`  project rootin creation
3.  to `_workspace/00_input.md`in 
4. existing daythis  `_workspace/`in and corresponding Phase cases
5. request scopein  **execution  decision** ( " per " )

### Phase 2: team setup and execution

team setupand  .  between of  and :

|  |  | responsible | of |  |
|------|------|------|------|--------|
| 1 | ETL architecture  | etl-architect |  | `_workspace/01_etl_architecture.md` |
| 2a | data  plan | data-quality-manager |  1 | `_workspace/02_data_quality_plan.md` |
| 2b | scheduling configuration | scheduler-engineer |  1 | `_workspace/03_scheduler_config.md` |
| 3 | monitoring configuration | monitoring-specialist |  1, 2a, 2b | `_workspace/04_monitoring_setup.md` |
| 4 | pipeline review | pipeline-reviewer |  1, 2a, 2b, 3 | `_workspace/05_review_report.md` |

 2a()and 2b(scheduling) **parallel execution**.    1(architecture)inonly ofloweras in startto count .

**team between  :**
- etl-architect completed → quality-managerto schema·business rule before, schedulerto of·resource required before, monitoringto core metric before
- quality-manager completed → schedulerto verification  insert location· cases before, monitoringto  metric·SLA before
- scheduler completed → monitoringto DAG execution metric before
- reviewer all   verification. 🔴 required modification   corresponding inthisbeforeto modification request →  → verification (maximum 2)

### Phase 3: integrated and final 

reviewerof report as final  :

1. `_workspace/` within all day confirmation
2. review reportof 🔴 required modificationthis   confirmation
3. final  userto report:
    - ETL architecture — `01_etl_architecture.md`
    -   plan — `02_data_quality_plan.md`
    - scheduling configuration — `03_scheduler_config.md`
    - monitoring configuration — `04_monitoring_setup.md`
    - review report — `05_review_report.md`
    - pipeline code — `pipeline_code/`

##  per 

user requestof scopein   inthisbefore :

| user request pattern | execution  |  inthisbefore |
|----------------|----------|-------------|
| "data pipeline ", "ETL before " | ** pipeline** | 5people before |
| "data  verification  only" | ** ** | etl-architect + quality-manager + reviewer |
| "Airflow DAG " | ** ** | etl-architect + scheduler + reviewer |
| "pipeline monitoring dashboard setup" | **monitoring ** | monitoring + reviewer |
| "this pipeline review" | **review ** | reviewer  |

**existing day for**: user schema, DAG code etc. existing day providedlower, corresponding day `_workspace/`of -based locationin and corresponding phaseof inthisbefore cases.

## data before as

| strategy |  | foralso |
|------|------|------|
| day  | `_workspace/`  | week   and shared |
| message  | SendMessage | real-time core information before, modification request |
|   | TaskCreate/TaskUpdate | in progress upper tracking, of   |

daypeople : `{}_{inthisbefore}_{}.{extension}`

## error 

| error type | strategy |
|----------|------|
|   information  | key placeholder  templateas in progress, in " information " people |
|  stack people | AWS/GCP/Azure 3 per  , user optional also |
|  information  | (day 1onlycases)·(day 100onlycases)·(day 1cases) 3phase architecture  |
| inthisbefore failure | 1 retry → failure  corresponding  this in progress, review reportin  people |
| reviewfrom 🔴  | corresponding inthisbeforein modification request →  → verification (maximum 2) |

## test 

### normal 
****: "PostgreSQL order data BigQuery data loweras -basedlower dayday  pipeline . Airflow as."
** result**:
- ETL architecture: PostgreSQL CDC/minutes as strategy, Raw→Staging→Curated→Analytics 4this
-  plan: order data   P0/P1/P2 verification rule, revenue aggregation cross-check
- scheduling: Airflow DAG code(extract→stage→curate→analytics→quality_check), retry policy
- monitoring: dayday -based casescount, latencybetween,  count dashboard
- review: beforeitem  

### existing day for 
****: "this dbt model dayin about data  verificationand monitoring " + dbt model day 
** result**:
- existing dbt model `_workspace/`in 
-   + monitoring  : quality-manager + monitoring + reviewer 
- etl-architect, scheduler cases

### error 
****: "data pipeline  ,   items   "
** result**:
- key day-based  pipeline templateas in progress
- per  placeholder included, per 3phase  
- review reportin " information  —  after  necessary" people


## inthisbeforeper extension 

|  | as | -ize upper inthisbefore | role |
|------|------|-----------------|------|
| data-quality-framework | `.claude/skills/data-quality-framework/skill.md` | data-quality-manager |  6, Great Expectations, dbt tests, data  |
| dag-orchestration-patterns | `.claude/skills/dag-orchestration-patterns/skill.md` | scheduler-engineer | Airflow DAG pattern, etc., retry, , of |
