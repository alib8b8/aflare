# Cold Chain Monitor

Cold chain temperature monitoring and compliance tracking with IoT sensor data.

## Description

This workflow fetches real-time temperature readings from IoT sensors, detects excursions outside configurable temperature ranges, classifies shipments as compliant/at-risk/non-compliant, and generates an AI-powered compliance report with root cause analysis, product quality impact assessment, and regulatory guidance for FDA and EU GDP.

## Usage

```yaml
params:
  sensor_data_endpoint: "https://api.iot.example.com/coldchain/readings"
  shipment_ids: '["SHIP-001","SHIP-002","SHIP-003"]'
  temperature_range: '{"min": 2.0, "max": 8.0}'
  alert_threshold_minutes: 15
  output_file: "/tmp/cold_chain_monitor.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| sensor_data_endpoint | string | yes | - | API endpoint for IoT temperature sensor data |
| shipment_ids | string | yes | - | JSON array of shipment IDs |
| temperature_range | string | no | {"min":2.0,"max":8.0} | Acceptable temperature range in Celsius |
| alert_threshold_minutes | integer | no | 15 | Minutes of out-of-range before alerting |
| output_file | string | no | /tmp/cold_chain_monitor.json | Output file |

## Nodes Used

- **http_request** - Fetches IoT temperature sensor data
- **code_interpreter** - Analyzes readings and detects excursions
- **agent** - Generates compliance report with regulatory guidance
- **file_write** - Saves monitoring results to output file

## Category

supply-chain