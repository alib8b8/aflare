# Headcount Planner

Workforce headcount planning with budget modeling and multi-scenario analysis.

## Description

Plan workforce headcount with attrition projections, backfill calculations, and three growth scenarios (conservative, moderate, aggressive). Includes department-level allocation, budget utilization tracking, and strategic hiring prioritization.

## Usage Example

```yaml
params:
  departments:
    - name: "Engineering"
      current_headcount: 85
      approved_headcount: 95
      growth_priority: 2
    - name: "Product"
      current_headcount: 22
      approved_headcount: 25
      growth_priority: 1.5
    - name: "Sales"
      current_headcount: 35
      approved_headcount: 40
      growth_priority: 2
    - name: "Marketing"
      current_headcount: 18
      approved_headcount: 20
      growth_priority: 1
  budget_total: 2500000
  planning_period:
    fiscal_year: 2027
    quarters: ["Q1", "Q2", "Q3", "Q4"]
  growth_projections:
    revenue_growth_pct: 25
  attrition_rate: 12
  cost_per_hire: 15000
  scenarios: ["conservative", "moderate", "aggressive"]
  output_file: "headcount_plan_fy2027.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| departments | array | Yes | - | Department data with headcount and growth priorities |
| budget_total | number | Yes | - | Total headcount budget |
| planning_period | object | Yes | - | Planning period with fiscal year and quarters |
| growth_projections | object | No | {} | Revenue and business growth projections |
| attrition_rate | number | No | 12 | Expected annual attrition rate % |
| scenarios | array | No | [conservative, moderate, aggressive] | Growth scenarios |
| cost_per_hire | number | No | 15000 | Average cost per hire |
| output_file | string | No | headcount_plan.json | Output file path |

## Nodes Used

- **code_interpreter** (×2): Calculate baseline metrics with attrition and backfill; model multi-scenario headcount with budget utilization
- **agent**: Generate strategic recommendations and hiring prioritization
- **file_write**: Save headcount plan as JSON

## Category

HR > Workforce Planning