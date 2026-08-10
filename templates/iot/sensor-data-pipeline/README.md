# Sensor Data Pipeline

End-to-end IoT sensor data ingestion and processing pipeline. Subscribes to MQTT topics, validates incoming data, detects statistical anomalies, enriches with AI-powered analysis, and persists results to storage.

## Usage Example

```yaml
params:
  mqtt_topic: "sensors/+/data"
  batch_size: 100
  anomaly_threshold: 3.0
  output_format: "json"
  storage_endpoint: "https://api.example.com"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| mqtt_topic | string | sensors/+/data | MQTT topic to subscribe for sensor data |
| batch_size | integer | 100 | Number of records per batch before processing |
| anomaly_threshold | number | 3.0 | Standard deviation multiplier for anomaly detection |
| output_format | string | json | Output format for processed data |
| storage_endpoint | string | "" | Endpoint for data storage service |

## Nodes Used

- **http_request**: Ingests raw sensor data from MQTT topic
- **code_interpreter** (validate): Validates data completeness and format
- **code_interpreter** (detect_anomalies): Statistical anomaly detection using z-score
- **agent**: AI-powered enrichment and root cause analysis
- **file_write**: Persists processed data to storage

## Category

iot