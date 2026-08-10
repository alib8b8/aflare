# Demand Forecast

AI-powered demand forecasting using time series analysis with exponential smoothing decomposition.

## Description

This workflow ingests historical sales data from an API endpoint, applies Holt-Winters style exponential smoothing with trend and seasonality components, generates forward-looking forecasts with confidence bounds, and produces AI-driven replenishment recommendations.

## Usage

```yaml
params:
  product_id: "SKU-12345"
  historical_data_endpoint: "https://api.example.com/sales/history"
  forecast_horizon_days: 30
  seasonality_period: 7
  output_file: "/tmp/demand_forecast.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| product_id | string | yes | - | Product identifier for forecasting |
| historical_data_endpoint | string | yes | - | API endpoint for historical sales data |
| forecast_horizon_days | integer | no | 30 | Number of days to forecast ahead |
| seasonality_period | integer | no | 7 | Seasonality period in days |
| output_file | string | no | /tmp/demand_forecast.json | Path for forecast output file |

## Nodes Used

- **http_request** - Fetches historical sales data from external API
- **code_interpreter** - Runs Python time series forecasting with exponential smoothing
- **agent** - Generates AI-powered demand insights and replenishment recommendations
- **file_write** - Persists forecast results and insights to output file

## Category

supply-chain