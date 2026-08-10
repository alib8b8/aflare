# Packaging Optimizer

Packaging size and cost optimization with dimensional weight analysis.

## Description

This workflow analyzes product dimensions and weights, auto-generates or uses existing box sizes, matches products to boxes for optimal fill rates, computes dimensional weight chargeable differences, and generates AI-powered recommendations for cost reduction, void fill minimization, box consolidation, and sustainability improvements.

## Usage

```yaml
params:
  product_catalog: '[{"product_id":"P-100","name":"Widget","dimensions_cm":{"length":20,"width":15,"height":10},"weight_kg":1.5,"monthly_volume":500}]'
  available_boxes: '[]'
  dim_weight_factor: 5000
  target_fill_rate: 0.85
  output_file: "/tmp/packaging_optimization.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| product_catalog | string | yes | - | JSON array of products with dimensions |
| available_boxes | string | no | [] | JSON array of available box sizes |
| dim_weight_factor | integer | no | 5000 | Dimensional weight divisor |
| target_fill_rate | number | no | 0.85 | Target box fill rate |
| output_file | string | no | /tmp/packaging_optimization.json | Output file |

## Nodes Used

- **code_interpreter** - Analyzes products and matches to optimal boxes
- **agent** - Generates packaging optimization recommendations
- **file_write** - Saves optimization results to output file

## Category

supply-chain