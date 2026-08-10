# Environmental Monitor

Environmental sensor monitoring for temperature, humidity, pressure, and air quality. Computes statistics, assesses comfort levels, detects trends, and provides AI-powered health and safety recommendations.

## Usage Example

```yaml
params:
  sensor_ids: "env-001,env-002"
  metrics: "temperature,humidity,pressure,air_quality"
  aggregation_window: "24h"
  temp_unit: "celsius"
  env_api: "https://api.environment.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| sensor_ids | string | "" | Comma-separated environmental sensor IDs |
| metrics | string | temperature,humidity,pressure,air_quality | Metrics to track |
| aggregation_window | string | 24h | Aggregation time window |
| temp_unit | string | celsius | Temperature unit - celsius or fahrenheit |
| env_api | string | https://api.environment.local/v1 | Environmental monitoring API |

## Nodes Used

- **http_request**: Fetches environmental readings from sensors
- **code_interpreter**: Computes statistics and comfort levels
- **agent**: AI-powered environmental analysis and recommendations
- **file_write**: Saves environmental monitoring report

## Category

iot