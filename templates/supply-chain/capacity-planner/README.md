# Capacity Planner

Production capacity planning with constraint analysis and bottleneck identification.

## Description

This workflow analyzes production line capacities (regular and overtime) against demand forecasts across a configurable planning horizon. It identifies capacity gaps, computes overtime requirements, flags bottleneck constraints, and generates AI-powered recommendations for overtime optimization, capital investment, and make-vs-buy analysis.

## Usage

```yaml
params:
  production_lines: '[{"line_id":"L-001","name":"Assembly A","hours_per_week":40,"units_per_hour":25,"products":["P-100","P-200"]}]'
  demand_forecast: '{"products":[{"product_id":"P-100","weekly_demand":500},{"product_id":"P-200","weekly_demand":300}]}'
  planning_horizon_weeks: 12
  overtime_limit_pct: 20
  output_file: "/tmp/capacity_plan.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| production_lines | string | yes | - | JSON array of lines with capacities |
| demand_forecast | string | yes | - | JSON with demand per product per period |
| planning_horizon_weeks | integer | no | 12 | Number of weeks to plan |
| overtime_limit_pct | number | no | 20 | Maximum overtime percentage |
| output_file | string | no | /tmp/capacity_plan.json | Output file |

## Nodes Used

- **code_interpreter** - Analyzes capacity vs demand and builds weekly plan
- **agent** - Generates bottleneck and investment recommendations
- **file_write** - Saves capacity plan to output file

## Category

supply-chain