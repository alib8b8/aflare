# Logistics Tracker

Real-time shipment and logistics tracking across multiple carriers with alert generation.

## Description

This workflow fetches real-time tracking data from a multi-carrier API, analyzes shipment statuses to detect delays and exceptions, and generates AI-powered escalation recommendations including root cause analysis, customer communication templates, and alternative fulfillment options.

## Usage

```yaml
params:
  tracking_numbers: '[{"number":"1Z999AA10123456784","carrier":"UPS"},{"number":"9205599999999999999999","carrier":"USPS"}]'
  carrier_api_endpoint: "https://api.shipping.com/v1"
  api_key: "your-api-key"
  alert_threshold_hours: 24
  output_file: "/tmp/shipment_status.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| tracking_numbers | string | yes | - | JSON array of tracking numbers with carrier codes |
| carrier_api_endpoint | string | yes | - | Multi-carrier tracking API endpoint |
| api_key | string | yes | - | API key for carrier tracking service |
| alert_threshold_hours | number | no | 24 | Hours of inactivity before triggering alert |
| output_file | string | no | /tmp/shipment_status.json | Output file |

## Nodes Used

- **http_request** - Fetches real-time tracking from carrier API
- **code_interpreter** - Analyzes statuses, detects delays and exceptions
- **agent** - Generates alerts and escalation recommendations
- **file_write** - Saves tracking analysis to output file

## Category

supply-chain