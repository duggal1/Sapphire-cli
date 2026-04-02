---
name: data-quality-framework
description: "data  (accuracy, completeness, timeliness, consistency etc.)per verification rule and Great Expectations, dbt tests etc.of also for guide. 'data ', 'verification rule', 'Great Expectations', 'dbt test', 'data profiling', 'or more detection', 'data ' etc. data    this  for. data-quality-managerof  verification  -ize. , pipeline schedulingthis before architecture  this of scope ."
---

# Data Quality Framework — data  framework guide

data  systematicas of, measurement, monitoringlower framework.

## data  6

|  | of | measurement  | threshold example |
|------|------|----------|-----------|
| **accuracy** (Accuracy) |    |  , business rule verification | also > 99.9% |
| **completeness** (Completeness) | required data  | NULL ratio, required  satisfied | NULL < 1% |
| **timeliness** (Timeliness) |  between within also | latencybetween, data also | latency < 30minutes |
| **consistency** (Consistency) | system between day |  verification,  integrity | day = 0 |
| **day** (Uniqueness) |   |  /key ratio |  = 0% |
| **valid** (Validity) | /scope compliant | , scope  | efficiency > 99% |

## verification rule  pattern

### P0 (required — failure  pipeline )

```yaml
rules:
  - name: pk_uniqueness
    type: uniqueness
    column: order_id
    threshold: 0  #  0cases

  - name: not_null_critical
    type: completeness
    columns: [order_id, customer_id, total_amount]
    max_null_rate: 0

  - name: row_count_sanity
    type: volume
    min_rows: 1000  # dayday minimum order count
    max_deviation: 0.5  # beforeday  50% or more   warning

  - name: referential_integrity
    type: consistency
    source: orders.customer_id
    reference: customers.id
    match_rate: 1.0
```

### P1 (important — warning after in progress)

```yaml
rules:
  - name: email_format
    type: validity
    column: email
    pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
    threshold: 0.99

  - name: amount_range
    type: accuracy
    column: total_amount
    min: 0
    max: 100000000  # 1 exceeding order of

  - name: freshness
    type: timeliness
    column: created_at
    max_age_hours: 24
```

## Great Expectations 

```python
import great_expectations as gx

#  of
suite = context.add_expectation_suite("orders_quality")

# completeness
suite.add_expectation(
    gx.expectations.ExpectColumnValuesToNotBeNull(column="order_id")
)

# day
suite.add_expectation(
    gx.expectations.ExpectColumnValuesToBeUnique(column="order_id")
)

# valid
suite.add_expectation(
    gx.expectations.ExpectColumnValuesToBeBetween(
        column="total_amount", min_value=0, max_value=100000000
    )
)

# 
suite.add_expectation(
    gx.expectations.ExpectTableRowCountToBeBetween(
        min_value=1000, max_value=1000000
    )
)
```

## dbt Tests 

```yaml
# schema.yml
models:
  - name: orders
    columns:
      - name: order_id
        tests:
          - unique
          - not_null
      - name: customer_id
        tests:
          - not_null
          - relationships:
              to: ref('customers')
              field: id
      - name: total_amount
        tests:
          - not_null
          - dbt_utils.accepted_range:
              min_value: 0
              max_value: 100000000
    tests:
      - dbt_utils.recency:
          datepart: hour
          field: created_at
          interval: 24
```

## data profiling list

```
columnper profile:
├── type: actual type vs  type
├── count(Cardinality): value count
├── NULL ratio:  pattern
├── distribution: the, also, also
├── or more: IQR  this
├── pattern: date, thisday, before-ize etc.  day
└── dependency: function-based  

tableper profile:
├──  count:  scope vs actual
├── : before   
├──  integrity: FK violated casescount
└── between distribution: record creation between pattern
```

## or more detection 

|  | -basedfor | / |
|------|------|----------|
| Z-Score | distribution data | \|x - μ\| / σ > 3 |
| IQR | distribution | x < Q1-1.5*IQR or x > Q3+1.5*IQR |
| thisaverage |   | 7day thisaverage  2σ this |
| beforeday  | dayday -based | \|today - yesterday\| / yesterday > 0.5 |

## data  (Data Contract)

```yaml
# data-contract.yml
name: orders
version: "2.0.0"
owner: order-team
description: "order data "

schema:
  - name: order_id
    type: string
    required: true
    unique: true
  - name: total_amount
    type: decimal(10,2)
    required: true
    min: 0

sla:
  freshness: 1h
  availability: 99.9%

quality:
  completeness: 99.9%
  accuracy: 99.99%
```
