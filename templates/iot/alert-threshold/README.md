# Alert Threshold

Sensor threshold alerting and notification pipeline with multi-channel delivery. Reads sensor data, evaluates against warning/critical thresholds, generates AI-powered alert messages, and delivers notifications via configured channels.

## Usage Example

```yaml
params:
  sensor_id: "temp-sensor-001"
  metric: "temperature"
  warning_threshold: 80
  critical_threshold: 95
  notification_channels: "email,webhook"
  sensor_api: "https://api.sensors.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| sensor_id | string | "" | Target sensor device ID |
| metric | string | temperature | Metric to monitor |
| warning_threshold | number | 80 | Warning level threshold value |
| critical_threshold | number | 95 | Critical level threshold value |
| notification_channels | string | email,webhook | Comma-separated notification channels |
| sensor_api | string | https://api.sensors.local/v1 | Sensor data API endpoint |

## Nodes Used

- **http_request** (read_sensor): Reads current sensor metric value
- **code_interpreter**: Evaluates reading against warning/critical thresholds
- **agent**: AI-generated alert notification content with recommendations
- **http_request** (notify_channels): Sends notifications through configured channels
- **file_write**: Logs alert event to persistent storage

## Category

iot