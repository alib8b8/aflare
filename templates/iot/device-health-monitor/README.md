# Device Health Monitor

IoT device health and uptime monitoring with heartbeat tracking, SLA compliance reporting, AI-powered diagnostics, and remediation recommendations. Tracks CPU, memory, disk, network, and battery metrics.

## Usage Example

```yaml
params:
  device_ids: "device-001,device-002,device-003"
  heartbeat_timeout: 300
  health_metrics: "cpu,memory,disk,network,battery"
  sla_target: 99.9
  monitor_api: "https://api.monitor.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| device_ids | string | "" | Comma-separated device IDs to monitor |
| heartbeat_timeout | integer | 300 | Heartbeat timeout in seconds |
| health_metrics | string | cpu,memory,disk,network,battery | Health metrics to track |
| sla_target | number | 99.9 | Target SLA percentage for uptime |
| monitor_api | string | https://api.monitor.local/v1 | Device monitoring API endpoint |

## Nodes Used

- **http_request**: Checks heartbeat status of monitored devices
- **code_interpreter**: Computes health scores and SLA compliance
- **agent**: AI-powered device diagnostics and remediation
- **file_write**: Generates and persists health report

## Category

iot