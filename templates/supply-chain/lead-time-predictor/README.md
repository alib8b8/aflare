# Lead Time Predictor

Supplier lead time prediction using historical performance and trend analysis.

## Description

This workflow fetches supplier historical lead time data, applies linear trend analysis with seasonal adjustments, computes confidence intervals at a configurable level, calculates reliability scores, and generates AI-powered sourcing recommendations including safety stock adjustments, dual-sourcing strategies, and order timing calendars.

## Usage

```yaml
params:
  supplier_history_api: "https://api.erp.example.com/supplier/leadtimes"
  supplier_ids: '["SUP-001","SUP-002","SUP-003"]'
  confidence_level: 0.90
  include_factors: true
  output_file: "/tmp/lead_time_prediction.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| supplier_history_api | string | yes | - | API endpoint for historical lead time data |
| supplier_ids | string | yes | - | JSON array of supplier IDs |
| confidence_level | number | no | 0.90 | Confidence level for prediction intervals |
| include_factors | boolean | no | true | Include external factors |
| output_file | string | no | /tmp/lead_time_prediction.json | Output file |

## Nodes Used

- **http_request** - Fetches supplier historical lead time data
- **code_interpreter** - Predicts lead times with trend and confidence intervals
- **agent** - Generates sourcing and safety stock recommendations
- **file_write** - Saves predictions to output file

## Category

supply-chain