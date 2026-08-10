# Wearable Data

Wearable device data aggregation with health metrics analysis (heart rate, steps, sleep, SpO2, stress, calories), wellness scoring, health alerts, and AI-powered personalized wellness recommendations.

## Usage Example

```yaml
params:
  user_ids: "user-001,user-002"
  device_types: "watch,band,ring"
  health_metrics: "heart_rate,steps,calories,sleep,spo2,stress"
  aggregation_period: "daily"
  wearable_api: "https://api.wearable.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| user_ids | string | "" | Comma-separated user IDs with wearable devices |
| device_types | string | watch,band,ring | Wearable device types |
| health_metrics | string | heart_rate,steps,calories,sleep,spo2,stress | Health metrics to track |
| aggregation_period | string | daily | Aggregation period - hourly, daily, or weekly |
| wearable_api | string | https://api.wearable.local/v1 | Wearable device data API |

## Nodes Used

- **http_request**: Fetches health data from wearable devices
- **code_interpreter**: Computes wellness scores and health alerts
- **agent**: AI-powered personalized wellness recommendations
- **file_write**: Saves health and wellness report

## Category

iot