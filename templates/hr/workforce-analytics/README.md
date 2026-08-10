# Workforce Analytics Dashboard

Comprehensive HR workforce analytics with KPIs, trends, and predictive insights.

## Description

Generate a multi-section workforce analytics dashboard with headcount, turnover, diversity, and compensation KPIs. Includes trend analysis, comparison against targets, predictive forecasting, and executive-ready insights for data-driven HR decisions.

## Usage Example

```yaml
params:
  workforce_data:
    headcount:
      current: 500
      period_start: 460
      approved: 530
      by_department:
        Engineering: 200
        Product: 50
        Sales: 100
        Marketing: 45
        HR: 25
        Finance: 30
        Operations: 50
      by_location:
        "San Francisco": 250
        "New York": 120
        Remote: 130
      by_type:
        "Full-time": 480
        "Contractor": 20
    turnover:
      total_exits: 35
      voluntary: 28
      involuntary: 7
      regrettable: 12
      first_year_exits: 15
      avg_headcount: 480
      by_department:
        Engineering: 12
        Sales: 10
        Marketing: 5
        Product: 3
        HR: 2
        Finance: 1
        Operations: 2
      by_reason:
        "Compensation": 10
        "Career Growth": 8
        "Management": 5
        "Work-Life Balance": 4
        "Relocation": 3
    compensation:
      average_salary: 115000
      median_salary: 105000
      total_payroll: 57500000
      avg_compa_ratio: 95.5
      promotions: 42
      avg_merit_increase: 4.2
  reporting_period:
    start: "2026-01-01"
    end: "2026-06-30"
  kpi_targets:
    voluntary_turnover_max: 10
    time_to_fill_max: 45
    compa_ratio_target: 100
    diversity_representation_goal: 40
  include_predictions: true
  comparison_period: "previous_year"
  dashboard_sections:
    - headcount
    - turnover
    - compensation
  output_file: "workforce_dashboard_h1_2026.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| workforce_data | object | Yes | - | Comprehensive workforce data |
| reporting_period | object | Yes | - | Reporting period |
| kpi_targets | object | No | {} | Target values for KPIs |
| include_predictions | boolean | No | true | Include predictive analytics |
| dashboard_sections | array | No | [headcount, turnover, diversity, compensation, hiring] | Sections to include |
| comparison_period | string | No | previous_year | Comparison period |
| output_file | string | No | workforce_analytics_dashboard.json | Output file path |

## Nodes Used

- **code_interpreter** (×3): Compute headcount KPIs; compute turnover rates and metrics; compute compensation KPIs
- **agent**: Generate strategic insights, trend analysis, and executive summary
- **file_write**: Save dashboard data as JSON

## Category

HR > Analytics & Reporting