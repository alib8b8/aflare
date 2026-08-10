# Telemetry Dashboard

Real-time telemetry dashboard generator for IoT device metrics visualization. Queries telemetry data, aggregates metrics with statistics, detects trends, and generates AI-powered insights.

## Usage Example

```yaml
params:
  device_ids: "sensor-001,sensor-002,sensor-003"
  metrics: "temperature,humidity,pressure,battery"
  time_range: "1h"
  refresh_interval: 30
  telemetry_api: "https://api.telemetry.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| device_ids | string | "" | Comma-separated list of device IDs to monitor |
| metrics | string | temperature,humidity,pressure,battery | Metrics to display |
| time_range | string | 1h | Time range for data aggregation |
| refresh_interval | integer | 30 | Dashboard refresh interval in seconds |
| telemetry_api | string | https://api.telemetry.local/v1 | Telemetry data API endpoint |

## Nodes Used

- **http_request**: Queries telemetry data from API
- **code_interpreter**: Aggregates metrics with statistics and trend detection
- **agent**: AI-powered dashboard insights and recommendations
- **file_write**: Renders and persists dashboard output

## Category

iot