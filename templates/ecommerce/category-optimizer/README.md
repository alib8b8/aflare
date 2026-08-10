# Category Performance Optimizer

A product category performance analysis and optimization engine that evaluates category health, benchmarks against market trends, and generates prioritized optimization plans.

## Description

This workflow template provides comprehensive category management capabilities:
- **Multi-Metric Analysis**: Evaluates revenue, margin, conversion, traffic, and returns
- **Market Benchmarking**: Compares category performance against market trends
- **Health Scoring**: Assigns performance scores for each KPI and category
- **Optimization Planning**: Prioritized action plans with ROI projections

The workflow uses dual-stage AI analysis to first diagnose category health and then generate actionable optimization plans.

## Usage Example

```yaml
params:
  category_ids: ["cat_elec", "cat_fashion"]
  metrics: ["revenue", "margin", "conversion", "traffic", "returns"]
  comparison_period: "30d"
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| category_ids | array | No | - | Specific categories to analyze (empty for all) |
| metrics | array | No | [revenue, margin, conversion, traffic, returns] | KPIs to analyze |
| comparison_period | string | No | 30d | Comparison period |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for analysis |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch category performance data and market trends
- **agent**: Dual-stage AI analysis (category health and optimization planning)
- **file_write**: Save comprehensive category optimization report

## Category

ecommerce