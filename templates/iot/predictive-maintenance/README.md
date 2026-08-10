# Predictive Maintenance

Predictive maintenance for industrial equipment using vibration analysis, temperature trends, current monitoring, pressure, and RPM data. Computes risk scores, predicts failures, estimates time-to-critical, and generates AI-powered maintenance plans with work order creation.

## Usage Example

```yaml
params:
  equipment_ids: "motor-001,motor-002,pump-001"
  equipment_type: "motor"
  monitoring_metrics: "vibration,temperature,current,pressure,rpm,runtime"
  failure_thresholds: '{"vibration":{"warning":7.0,"critical":10.0},"temperature":{"warning":75,"critical":90}}'
  maintenance_api: "https://api.maintenance.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| equipment_ids | string | "" | Comma-separated equipment IDs to monitor |
| equipment_type | string | motor | Equipment type - motor, pump, compressor, conveyor, turbine, generator |
| monitoring_metrics | string | vibration,temperature,current,pressure,rpm,runtime | Predictive maintenance metrics |
| failure_thresholds | string | {} | JSON failure threshold configuration per metric |
| maintenance_api | string | https://api.maintenance.local/v1 | Predictive maintenance API |

## Nodes Used

- **http_request**: Fetches equipment sensor readings
- **code_interpreter**: Predicts failures using sensor data and trend analysis
- **agent**: AI-powered maintenance planning and scheduling
- **http_request** (create_work_orders): Creates maintenance work orders
- **file_write**: Saves predictive maintenance report

## Category

iot