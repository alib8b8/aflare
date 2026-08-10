# Inventory Replenishment

Automated inventory replenishment with reorder point, safety stock, and EOQ calculation.

## Description

This workflow fetches current inventory levels and demand forecasts from APIs, computes safety stock using z-scores for a configurable service level, calculates reorder points and economic order quantities (EOQ), identifies items needing replenishment, and generates AI-powered purchase order recommendations with urgency classification.

## Usage

```yaml
params:
  inventory_api_endpoint: "https://api.example.com/inventory/levels"
  demand_api_endpoint: "https://api.example.com/demand/forecast"
  lead_time_days: 7
  service_level: 0.95
  output_file: "/tmp/replenishment_plan.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| inventory_api_endpoint | string | yes | - | API endpoint for current inventory levels |
| lead_time_days | integer | no | 7 | Supplier lead time in days |
| service_level | number | no | 0.95 | Target service level for safety stock |
| demand_api_endpoint | string | yes | - | API endpoint for demand forecast data |
| output_file | string | no | /tmp/replenishment_plan.json | Output file |

## Nodes Used

- **http_request** - Fetches inventory levels and demand forecasts
- **code_interpreter** - Computes safety stock, reorder points, and EOQ
- **agent** - Generates prioritized PO recommendations
- **file_write** - Saves replenishment plan to output file

## Category

supply-chain