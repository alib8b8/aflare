# Cross Docking Planner

Cross-docking schedule optimization for inbound/outbound synchronization.

## Description

This workflow analyzes inbound shipments and outbound orders to match SKU requirements, identifies direct cross-dock vs. storage-needed items, assigns dock doors to shipments with time-slotted schedules, and generates AI-powered optimization recommendations for dock utilization, overflow handling, and labor/equipment planning.

## Usage

```yaml
params:
  inbound_shipments: '[{"shipment_id":"IN-001","arrival_time":"08:00","carrier":"FedEx","items":[{"sku":"SKU-A","quantity":100,"pallets":2}]}]'
  outbound_orders: '[{"order_id":"OUT-001","departure_window":"14:00-16:00","items":[{"sku":"SKU-A","quantity":50,"pallets":1}]}]'
  dock_count: 10
  handling_time_minutes: 30
  output_file: "/tmp/cross_docking_plan.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| inbound_shipments | string | yes | - | JSON array of inbound shipments |
| outbound_orders | string | yes | - | JSON array of outbound orders |
| dock_count | integer | no | 10 | Number of available dock doors |
| handling_time_minutes | integer | no | 30 | Average handling time per pallet |
| output_file | string | no | /tmp/cross_docking_plan.json | Output file |

## Nodes Used

- **code_interpreter** - Analyzes flows and builds dock schedules
- **agent** - Generates optimization recommendations
- **file_write** - Saves cross-docking plan to output file

## Category

supply-chain