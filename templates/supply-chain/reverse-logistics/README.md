# Reverse Logistics

Returns and reverse logistics management with intelligent disposition routing.

## Description

This workflow classifies return requests into optimal dispositions (restock, refurbish, recycle, liquidate, return-to-vendor, dispose) using configurable rules based on condition, age, value, and category. It computes processing costs and recovery values, generates an AI-powered routing plan, and submits disposition instructions to the warehouse system.

## Usage

```yaml
params:
  returns_data: '[{"return_id":"RET-001","product_id":"P-100","product_name":"Widget","reason":"defective","condition":"good","item_value":150,"category":"electronics","order_date":"2026-07-01"}]'
  disposition_rules: '{"restock":{"max_days":30,"max_value":500},"refurbish":{"max_days":90,"max_value":2000},"recycle":{"categories":["electronics"]},"liquidate":{"min_value":50}}'
  warehouse_api: "https://api.warehouse.example.com/v1"
  output_file: "/tmp/returns_management.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| returns_data | string | yes | - | JSON array of return requests |
| disposition_rules | string | no | see above | JSON disposition routing rules |
| warehouse_api | string | yes | - | API endpoint for warehouse system |
| output_file | string | no | /tmp/returns_management.json | Output file |

## Nodes Used

- **code_interpreter** - Classifies returns and computes costs/recovery
- **agent** - Generates routing plan and processing recommendations
- **http_request** - Submits dispositions to warehouse system
- **file_write** - Saves returns management plan to output file

## Category

supply-chain