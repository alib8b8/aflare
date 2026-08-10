# Last Mile Optimizer

Last-mile delivery cost optimization with multi-mode comparison.

## Description

This workflow evaluates six delivery modes (van, bike, cargo-bike, walking, crowdsourced, locker) against zone density, distance, and volume parameters. It computes fixed/variable/parking costs, CO2 emissions, and delivery times for each mode, then generates an AI-powered optimal mode mix recommendation with phased implementation roadmap.

## Usage

```yaml
params:
  delivery_zone: '{"zone_id":"Z-001","density":"urban","avg_distance_km":3.0}'
  order_volume: 200
  available_modes: '["van","bike","cargo-bike","walking","crowdsourced","locker"]'
  cost_constraints: '{"max_cost_per_delivery": 8.00, "max_time_minutes": 60}'
  output_file: "/tmp/last_mile_optimization.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| delivery_zone | string | yes | - | JSON with zone geography, density, average distance |
| order_volume | integer | yes | - | Number of daily deliveries |
| available_modes | string | no | all six | JSON array of delivery modes |
| cost_constraints | string | no | see above | JSON with cost and time constraints |
| output_file | string | no | /tmp/last_mile_optimization.json | Output file |

## Nodes Used

- **code_interpreter** - Computes delivery mode costs and performance
- **agent** - Generates optimal mode mix recommendation
- **file_write** - Saves optimization results to output file

## Category

supply-chain