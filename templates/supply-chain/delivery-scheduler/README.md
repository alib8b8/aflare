# Delivery Scheduler

Last-mile delivery scheduling with time window constraints and vehicle assignment.

## Description

This workflow parses delivery orders with time windows and priorities, assigns them to vehicles using a round-robin algorithm with priority weighting, builds time-slotted schedules with arrival and departure times, and generates AI-powered optimization recommendations for utilization improvement and bottleneck resolution.

## Usage

```yaml
params:
  orders_data: '[{"order_id":"ORD-001","address":"123 Main St","priority":1,"time_window":"09:00-12:00","service_minutes":15}]'
  fleet_capacity: 5
  shift_start: "08:00"
  shift_end: "18:00"
  output_file: "/tmp/delivery_schedule.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| orders_data | string | yes | - | JSON array of orders with addresses and time windows |
| fleet_capacity | integer | no | 5 | Number of available delivery vehicles |
| shift_start | string | no | 08:00 | Delivery shift start time |
| shift_end | string | no | 18:00 | Delivery shift end time |
| output_file | string | no | /tmp/delivery_schedule.json | Output file |

## Nodes Used

- **code_interpreter** - Parses orders and assigns vehicles
- **agent** - Generates schedule optimization recommendations
- **file_write** - Saves delivery schedule to output file

## Category

supply-chain