# Energy Monitor

Smart energy consumption monitoring with usage analytics, cost prediction, carbon footprint tracking, and AI-powered optimization recommendations. Supports multiple smart meters and billing periods.

## Usage Example

```yaml
params:
  meter_ids: "meter-001,meter-002"
  billing_period: "monthly"
  cost_per_kwh: 0.12
  carbon_factor: 0.5
  energy_api: "https://api.energy.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| meter_ids | string | "" | Comma-separated smart meter IDs |
| billing_period | string | monthly | Billing period - hourly, daily, weekly, or monthly |
| cost_per_kwh | number | 0.12 | Cost per kWh in local currency |
| carbon_factor | number | 0.5 | Carbon emission factor (kg CO2 per kWh) |
| energy_api | string | https://api.energy.local/v1 | Energy monitoring API endpoint |

## Nodes Used

- **http_request**: Fetches consumption data from smart meters
- **code_interpreter**: Computes energy analytics, cost, and carbon footprint
- **agent**: AI-powered energy optimization recommendations
- **file_write**: Generates and persists energy report

## Category

iot