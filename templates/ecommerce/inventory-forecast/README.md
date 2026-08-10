# Inventory Demand Forecasting

An AI-driven inventory demand forecasting system that analyzes historical sales data, current inventory levels, and seasonal patterns to predict future demand and prevent stockouts.

## Description

This workflow template provides end-to-end inventory forecasting capabilities:
- **Demand Forecasting**: Predicts daily demand for the forecast horizon
- **Stockout Risk Detection**: Identifies products at risk of running out of stock
- **Reorder Recommendations**: Calculates optimal reorder quantities and timing
- **Inventory Health Score**: Overall assessment of inventory position

The workflow combines Python-based time-series forecasting with AI-generated recommendations for actionable insights.

## Usage Example

```yaml
params:
  product_ids: ["prod_111", "prod_222"]
  category_id: "cat_electronics"
  forecast_horizon: 30
  seasonality: "auto"
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| product_ids | array | No | - | Specific products to forecast (empty for all) |
| category_id | string | No | - | Category to forecast |
| forecast_horizon | integer | No | 30 | Number of days to forecast |
| seasonality | string | No | auto | Seasonality mode (auto, additive, multiplicative) |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for recommendations |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch sales history and current inventory levels
- **code_interpreter**: Python-based demand forecasting and stockout risk calculation
- **agent**: AI-generated reorder recommendations and risk mitigation strategies
- **file_write**: Save comprehensive forecast report

## Category

ecommerce