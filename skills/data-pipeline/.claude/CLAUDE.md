# Data Pipeline Harness

An agent team harness that collaborates to design and implement data pipelines covering ingestion, transformation, loading, quality verification, and monitoring.

## Structure

```
.claude/
├── agents/
│   ├── etl-architect.md        — ETL architecture design (source analysis, schema design, pipeline structure)
│   ├── data-quality-manager.md  — Data quality management (validation rules, profiling, anomaly detection)
│   ├── scheduler-engineer.md    — Scheduling engineer (DAG design, dependencies, retry strategy)
│   ├── monitoring-specialist.md — Monitoring specialist (metrics, alerts, dashboards, SLA)
│   └── pipeline-reviewer.md     — Pipeline reviewer (cross-validation, consistency, operational readiness)
├── skills/
│   ├── data-pipeline/
│   │   └── skill.md              — Orchestrator (team coordination, workflow, error handling)
│   ├── data-quality-framework/
│   │   └── skill.md              — Data quality framework guide
│   └── dag-orchestration-patterns/
│       └── skill.md              — Pipeline orchestration pattern guide
└── CLAUDE.md                    — This file
```

## Usage

Trigger the `/data-pipeline` skill, or make a natural language request such as "design a data pipeline."

## Deliverables

All deliverables are stored in the `_workspace/` directory:
- `00_input.md` — Organized user input
- `01_etl_architecture.md` — ETL architecture design document
- `02_data_quality_plan.md` — Data quality management plan
- `03_scheduler_config.md` — Scheduling configuration and DAG definitions
- `04_monitoring_setup.md` — Monitoring dashboard and alert configuration
- `05_review_report.md` — Review report
- `pipeline_code/` — Pipeline implementation code
