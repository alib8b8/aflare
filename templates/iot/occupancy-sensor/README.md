# Occupancy Sensor

Occupancy and space utilization tracking with real-time analytics, utilization scoring, overcrowding detection, and AI-powered workspace optimization. Supports multiple sensor types: PIR, thermal, camera, and desk sensors.

## Usage Example

```yaml
params:
  space_ids: "room-101,room-102,room-103"
  sensor_types: "pir,thermal,desk"
  capacity_map: '{"room-101":50,"room-102":30,"room-103":20}'
  reporting_window: "daily"
  occupancy_api: "https://api.occupancy.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| space_ids | string | "" | Comma-separated space or room IDs |
| sensor_types | string | pir,thermal,camera,desk | Occupancy sensor types |
| capacity_map | string | {} | JSON mapping of space IDs to max capacity |
| reporting_window | string | daily | Reporting window - hourly, daily, or weekly |
| occupancy_api | string | https://api.occupancy.local/v1 | Occupancy sensor API |

## Nodes Used

- **http_request**: Fetches real-time occupancy data from sensors
- **code_interpreter**: Analyzes space utilization and occupancy patterns
- **agent**: AI-powered space utilization optimization
- **file_write**: Saves occupancy and utilization report

## Category

iot