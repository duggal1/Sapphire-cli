---
name: dag-orchestration-patterns
description: "Airflow DAG  pattern, of , retry strategy, etc. ,  strategy etc. data pipeline orchestration guide. 'Airflow DAG', 'DAG ', 'of', 'retry strategy', 'etc.', '', 'pipeline orchestration', 'Dagster', 'Prefect' etc. pipeline scheduling  this  for. scheduler-engineerof DAG   -ize. , data  rule of monitoring dashboard this of scope ."
---

# DAG Orchestration Patterns — pipeline orchestration pattern guide

Airflow of DAG  patternand operations strategy.

## DAG  pattern

### 1. Extract-Load-Transform (ELT) pattern

```python
with DAG("elt_orders", schedule="0 2 * * *", catchup=False) as dag:
    extract = PythonOperator(task_id="extract", python_callable=extract_orders)
    load_raw = PythonOperator(task_id="load_raw", python_callable=load_to_raw)
    transform = DbtOperator(task_id="transform", select="orders")
    quality = PythonOperator(task_id="quality_check", python_callable=run_checks)
    notify = SlackOperator(task_id="notify", trigger_rule="all_done")

    extract >> load_raw >> transform >> quality >> notify
```

### 2. Fan-out/Fan-in pattern

```python
#   parallel  → integrated transformation
sources = ["mysql", "postgres", "api"]
extract_tasks = [
    PythonOperator(task_id=f"extract_{src}", python_callable=extract, op_args=[src])
    for src in sources
]
merge = PythonOperator(task_id="merge_all", python_callable=merge_sources)
transform = PythonOperator(task_id="transform", python_callable=transform_data)

extract_tasks >> merge >> transform
```

### 3.   event pending

```python
wait_for_data = S3KeySensor(
    task_id="wait_for_file",
    bucket_name="data-lake",
    bucket_key="raw/orders/{{ ds }}/data.parquet",
    timeout=3600,  # 1between pending
    poke_interval=60,
    mode="reschedule"  # pending   
)
```

## etc.  pattern

### partition replacement ( )

```sql
-- date partition before replacement (etc.-based)
DELETE FROM analytics.orders WHERE date_partition = '{{ ds }}';
INSERT INTO analytics.orders
SELECT * FROM staging.orders WHERE date_partition = '{{ ds }}';
```

### MERGE/UPSERT

```sql
MERGE INTO target AS t
USING source AS s ON t.id = s.id
WHEN MATCHED THEN UPDATE SET t.amount = s.amount, t.updated_at = CURRENT_TIMESTAMP
WHEN NOT MATCHED THEN INSERT (id, amount, created_at) VALUES (s.id, s.amount, CURRENT_TIMESTAMP);
```

### etc. list

- [ ] such as DAG 2 executionalso result identical?
- [ ] date parameter(`{{ ds }}`) forto scope limitedlower?
- [ ] INSERT beforein existing data lower?
- [ ] external API in  request ID forlower?

## retry strategy

```python
default_args = {
    "retries": 3,
    "retry_delay": timedelta(minutes=5),
    "retry_exponential_backoff": True,
    "max_retry_delay": timedelta(minutes=60),
    "execution_timeout": timedelta(hours=2),
}
```

### per retry configuration

|  type | retry count | pending between | this |
|----------|-----------|----------|------|
| DB  | 3 | 5minutes count backoff | day-based connection  |
| API  | 5 | 30seconds count backoff | Rate limit, network |
| transformation (SQL) | 1 | immediate | as error retry of |
| day as | 3 | 1minutes | network  |

##  strategy

```python
# safe  configuration
dag = DAG(
    "daily_orders",
    schedule="0 2 * * *",
    start_date=datetime(2024, 1, 1),
    catchup=False,  # automatic  inactive
    max_active_runs=1,  #  execution 
)

# CLIas manual 
# airflow dags backfill daily_orders -s 2024-01-01 -e 2024-01-31
```

###  weekofmatter

| weekof | people |  |
|------|------|------|
|  execution | such as data scope  processing | `max_active_runs=1` |
| API Rate Limit |  and data request |  size limited,  insert |
| resource contention | DB/cluster andlower | parallelalso limited, between variance |
| data  | and data  change | schema -ize processing as |

## of pattern

### Cross-DAG of

```python
# DAG Aof completed pending
wait_for_upstream = ExternalTaskSensor(
    task_id="wait_for_dag_a",
    external_dag_id="dag_a",
    external_task_id="final_task",
    execution_delta=timedelta(hours=0),
    timeout=3600,
    mode="reschedule"
)
```

### Dataset  of (Airflow 2.4+)

```python
# Producer DAG
orders_dataset = Dataset("s3://datalake/orders/")

with DAG("produce_orders", schedule="0 2 * * *") as dag:
    produce = PythonOperator(
        task_id="produce", outlets=[orders_dataset]
    )

# Consumer DAG — automatic tree
with DAG("consume_orders", schedule=[orders_dataset]) as dag:
    consume = PythonOperator(task_id="consume", ...)
```

## alert strategy

```python
def failure_callback(context):
    task = context["task_instance"]
    dag_id = context["dag"].dag_id
    execution_date = context["execution_date"]
    message = f"FAILED: {dag_id}/{task.task_id} at {execution_date}"
    send_slack(message)

default_args = {
    "on_failure_callback": failure_callback,
    "on_retry_callback": retry_callback,
}
```

| event |  | upper |
|--------|------|------|
| P0  failure | Slack + PagerDuty |  engineer |
| SLA  | Slack | data team |
| retry  | Slack (information) | monitoring  |
|  completed | Slack | request |
