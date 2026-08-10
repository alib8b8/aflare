# MQTT Bridge

MQTT protocol bridge and message router with topic mapping, payload transformation, multi-broker support, and AI-powered health monitoring. Subscribes to source broker topics, applies transformations, and publishes to target broker.

## Usage Example

```yaml
params:
  source_broker: "mqtt://broker-source.local:1883"
  target_broker: "mqtt://broker-target.local:1883"
  topic_mappings: '{"sensors/+/temp":"cloud/sensors/temperature","sensors/+/humidity":"cloud/sensors/humidity"}'
  transform_rules: '{"sensors/+/temp":[{"type":"convert","key":"value"},{"type":"add","key":"unit","value":"celsius"}]}'
  qos: 1
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| source_broker | string | mqtt://broker-source.local:1883 | Source MQTT broker URL |
| target_broker | string | mqtt://broker-target.local:1883 | Target MQTT broker URL |
| topic_mappings | string | {} | JSON topic mapping rules (source -> target) |
| transform_rules | string | {} | JSON message transformation rules |
| qos | integer | 1 | MQTT QoS level for publishing |

## Nodes Used

- **http_request** (subscribe_source): Subscribes to source MQTT broker topics
- **code_interpreter**: Transforms and routes messages based on rules
- **http_request** (publish_target): Publishes transformed messages to target broker
- **agent**: AI-powered bridge health monitoring and reporting
- **file_write**: Logs bridge activity to persistent storage

## Category

iot