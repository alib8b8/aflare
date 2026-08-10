# Supplier Evaluation

Weighted supplier scorecard and evaluation system with multi-dimensional KPI scoring.

## Description

This workflow fetches supplier performance data from an API, computes weighted scores across five KPI categories (quality, delivery, cost, responsiveness, compliance), assigns Gold/Silver/Bronze/At-Risk tier ratings, and generates an AI-powered strategic sourcing report with actionable recommendations.

## Usage

```yaml
params:
  supplier_data_endpoint: "https://api.example.com/suppliers/performance"
  evaluation_period: "last_quarter"
  kpi_weights: '{"quality": 0.30, "delivery": 0.25, "cost": 0.20, "responsiveness": 0.15, "compliance": 0.10}'
  output_file: "/tmp/supplier_evaluation.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| supplier_data_endpoint | string | yes | - | API endpoint for supplier performance data |
| evaluation_period | string | no | last_quarter | Evaluation period |
| kpi_weights | string | no | see above | JSON object with KPI category weights |
| output_file | string | no | /tmp/supplier_evaluation.json | Output file |

## Nodes Used

- **http_request** - Fetches supplier performance data from API
- **code_interpreter** - Computes weighted scorecards and tier assignments
- **agent** - Generates executive evaluation report with sourcing recommendations
- **file_write** - Saves evaluation results to output file

## Category

supply-chain