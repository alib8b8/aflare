# Warehouse Planner

Warehouse layout and slotting optimization using ABC classification for efficient storage.

## Description

This workflow performs ABC inventory classification based on turnover rates, computes optimal shelf and aisle configurations from warehouse dimensions, and assigns SKUs to zones and slots to maximize picking efficiency. AI generates layout recommendations and efficiency improvement estimates.

## Usage

```yaml
params:
  warehouse_dimensions: '{"length": 100, "width": 50, "height": 12}'
  inventory_data: '[{"sku":"SKU-001","turnover_rate":500,"quantity":200},{"sku":"SKU-002","turnover_rate":50,"quantity":50}]'
  storage_strategy: "abc"
  aisle_width: 3.0
  output_file: "/tmp/warehouse_plan.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| warehouse_dimensions | string | yes | - | JSON with warehouse length, width, height in meters |
| inventory_data | string | yes | - | JSON array of SKU data with dimensions, turnover rate, and quantity |
| storage_strategy | string | no | abc | Storage strategy (abc, dedicated, random, class-based) |
| aisle_width | number | no | 3.0 | Aisle width in meters |
| output_file | string | no | /tmp/warehouse_plan.json | Output file for warehouse layout plan |

## Nodes Used

- **code_interpreter** - ABC classification and slotting computation
- **agent** - Generates layout recommendations and efficiency estimates
- **file_write** - Saves warehouse layout plan to output file

## Category

supply-chain